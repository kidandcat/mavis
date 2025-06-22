// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package codeagent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestQueueProcessingAfterAgentRemoval verifies that removing an agent triggers queue processing
func TestQueueProcessingAfterAgentRemoval(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()
	testFolder := "/tmp/test-queue-processing"

	// Track when queued agents start
	startedAgents := make(chan string, 2)
	manager.SetAgentStartCallback(func(agentID, folder, prompt, queueID string) {
		log.Printf("[TEST] Agent %s started from queue %s", agentID, queueID)
		startedAgents <- agentID
	})

	// Launch first agent
	agent1ID, err := manager.LaunchAgent(ctx, testFolder, "First task")
	if err != nil {
		t.Fatalf("Failed to launch first agent: %v", err)
	}
	log.Printf("[TEST] Launched first agent: %s", agent1ID)

	// Launch second agent (should be queued)
	agent2Result, err := manager.LaunchAgent(ctx, testFolder, "Second task")
	if err != nil {
		t.Fatalf("Failed to launch second agent: %v", err)
	}

	// Verify second is queued
	if !strings.HasPrefix(agent2Result, "queued-") {
		t.Errorf("Second agent should be queued, got: %s", agent2Result)
	}

	// Verify queue status
	queueStatus := manager.GetQueueStatus()
	if queueStatus[testFolder] != 1 {
		t.Errorf("Expected 1 queued task, got %d", queueStatus[testFolder])
	}

	// Remove first agent - this should trigger queue processing
	log.Printf("[TEST] Removing agent %s to trigger queue processing", agent1ID)
	err = manager.RemoveAgent(agent1ID)
	if err != nil {
		t.Fatalf("Failed to remove agent: %v", err)
	}

	// Wait for queued agent to start
	select {
	case startedID := <-startedAgents:
		log.Printf("[TEST] SUCCESS: Queued agent %s started after first was removed", startedID)
	case <-time.After(2 * time.Second):
		t.Fatal("Queued agent did not start after first agent was removed")
	}

	// Verify queue is now empty
	newQueueStatus := manager.GetQueueStatus()
	if newQueueStatus[testFolder] != 0 {
		t.Errorf("Queue should be empty, got %d tasks", newQueueStatus[testFolder])
	}
}

// TestMultipleQueuedAgents tests handling of multiple queued agents
func TestMultipleQueuedAgents(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()
	testFolder := "/tmp/test-multiple-queue"

	// Track order of agent starts
	startOrder := make([]string, 0)
	var orderMu sync.Mutex

	manager.SetAgentStartCallback(func(agentID, folder, prompt, queueID string) {
		orderMu.Lock()
		startOrder = append(startOrder, prompt)
		orderMu.Unlock()
		log.Printf("[TEST] Started: %s", prompt)
	})

	// Launch first agent
	agent1ID, _ := manager.LaunchAgent(ctx, testFolder, "Task 1")

	// Queue multiple agents
	expectedOrder := []string{"Task 2", "Task 3", "Task 4"}
	for i, task := range expectedOrder {
		result, err := manager.LaunchAgent(ctx, testFolder, task)
		if err != nil {
			t.Fatalf("Failed to launch %s: %v", task, err)
		}
		if !strings.HasPrefix(result, "queued-") {
			t.Errorf("%s should be queued", task)
		}

		// Verify queue size
		queueSize := manager.GetQueuedTasksForFolder(testFolder)
		if queueSize != i+1 {
			t.Errorf("Expected queue size %d, got %d", i+1, queueSize)
		}
	}

	// Process agents in order
	currentAgentID := agent1ID
	for i := 0; i < len(expectedOrder); i++ {
		// Remove current agent to trigger next
		err := manager.RemoveAgent(currentAgentID)
		if err != nil {
			t.Logf("Warning: Failed to remove agent %s: %v", currentAgentID, err)
		}

		// Wait a bit for queue processing
		time.Sleep(100 * time.Millisecond)

		// Get the newly started agent
		agents := manager.ListAgentsByStatus(StatusRunning)
		if len(agents) > 0 {
			currentAgentID = agents[0].ID
		}
	}

	// Verify order
	orderMu.Lock()
	defer orderMu.Unlock()

	if len(startOrder) != len(expectedOrder) {
		t.Errorf("Expected %d agents to start, got %d", len(expectedOrder), len(startOrder))
	}

	for i, expected := range expectedOrder {
		if i < len(startOrder) && startOrder[i] != expected {
			t.Errorf("Expected %s at position %d, got %s", expected, i, startOrder[i])
		}
	}
}

// TestAgentCompletionRaceCondition tests for race conditions in completion detection
func TestAgentCompletionRaceCondition(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// Run multiple iterations to catch race conditions
	for iteration := 0; iteration < 5; iteration++ {
		testFolder := fmt.Sprintf("/tmp/test-race-%d", iteration)

		// Launch and immediately try to check status
		agentID, err := manager.LaunchAgent(ctx, testFolder, "Race test")
		if err != nil {
			t.Fatalf("Iteration %d: Failed to launch: %v", iteration, err)
		}

		// Concurrent operations that might race
		var wg sync.WaitGroup
		errors := make(chan error, 3)

		// Goroutine 1: Get agent info
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := manager.GetAgentInfo(agentID)
			if err != nil {
				errors <- fmt.Errorf("GetAgentInfo: %v", err)
			}
		}()

		// Goroutine 2: List agents
		wg.Add(1)
		go func() {
			defer wg.Done()
			agents := manager.ListAgents()
			if len(agents) == 0 {
				errors <- fmt.Errorf("ListAgents returned empty")
			}
		}()

		// Goroutine 3: Check queue status
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = manager.GetQueueStatus()
		}()

		wg.Wait()
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Iteration %d: %v", iteration, err)
		}

		// Clean up
		_ = manager.RemoveAgent(agentID)
	}
}

// TestProcessQueueForFolderDirectly tests the queue processing mechanism
func TestProcessQueueForFolderDirectly(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()
	testFolder := "/tmp/test-direct-queue"

	// Set up tracking
	processedTasks := make([]string, 0)
	var mu sync.Mutex

	manager.SetAgentStartCallback(func(agentID, folder, prompt, queueID string) {
		mu.Lock()
		processedTasks = append(processedTasks, queueID)
		mu.Unlock()
	})

	// Manually add to queue (simulating the internal state)
	manager.queueMu.Lock()
	manager.folderQueues[testFolder] = []QueuedTask{
		{Folder: testFolder, Prompt: "Queued 1", Ctx: ctx, QueueID: "q1"},
		{Folder: testFolder, Prompt: "Queued 2", Ctx: ctx, QueueID: "q2"},
	}
	manager.queueMu.Unlock()

	// Process the queue
	manager.processQueueForFolder(testFolder)

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify first task was processed
	mu.Lock()
	if len(processedTasks) != 1 || processedTasks[0] != "q1" {
		t.Errorf("Expected first queued task to be processed, got %v", processedTasks)
	}
	mu.Unlock()

	// Verify one task remains in queue
	remaining := manager.GetQueuedTasksForFolder(testFolder)
	if remaining != 1 {
		t.Errorf("Expected 1 task remaining in queue, got %d", remaining)
	}
}

// TestAgentRemovalWithNoQueue tests removing an agent when no queue exists
func TestAgentRemovalWithNoQueue(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()
	testFolder := "/tmp/test-no-queue"

	// Launch single agent
	agentID, err := manager.LaunchAgent(ctx, testFolder, "Only task")
	if err != nil {
		t.Fatalf("Failed to launch agent: %v", err)
	}

	// Verify no queue
	queueSize := manager.GetQueuedTasksForFolder(testFolder)
	if queueSize != 0 {
		t.Errorf("Expected no queue, got %d tasks", queueSize)
	}

	// Remove agent - should not panic or error
	err = manager.RemoveAgent(agentID)
	if err != nil {
		t.Errorf("Failed to remove agent: %v", err)
	}

	// Verify folder is cleaned up
	running, _ := manager.IsAgentRunningInFolder(testFolder)
	if running {
		t.Error("No agent should be running in folder after removal")
	}
}

// TestConcurrentQueueOperations tests thread safety of queue operations
func TestConcurrentQueueOperations(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()
	numFolders := 10
	numAgentsPerFolder := 5

	var wg sync.WaitGroup

	// Launch agents concurrently in different folders
	for i := 0; i < numFolders; i++ {
		folder := fmt.Sprintf("/tmp/concurrent-%d", i)

		for j := 0; j < numAgentsPerFolder; j++ {
			wg.Add(1)
			go func(f string, task int) {
				defer wg.Done()

				_, err := manager.LaunchAgent(ctx, f, fmt.Sprintf("Task %d", task))
				if err != nil {
					t.Logf("Error launching agent: %v", err)
				}
			}(folder, j)
		}
	}

	// Concurrently check queue status
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				_ = manager.GetQueueStatus()
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	// Verify state is consistent
	totalAgents := manager.GetTotalCount()
	runningAgents := manager.GetRunningCount()

	if runningAgents > numFolders {
		t.Errorf("Too many running agents: %d (max should be %d)", runningAgents, numFolders)
	}

	log.Printf("[TEST] Total agents: %d, Running: %d", totalAgents, runningAgents)
}

// Original tests preserved

func TestSimpleIDGeneration(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// Test sequential ID generation
	id1, err := manager.LaunchAgent(ctx, "/tmp", "test task 1")
	if err != nil {
		t.Fatalf("Failed to launch agent 1: %v", err)
	}
	if id1 != "1" {
		t.Errorf("Expected first agent ID to be '1', got '%s'", id1)
	}

	id2, err := manager.LaunchAgent(ctx, "/tmp2", "test task 2")
	if err != nil {
		t.Fatalf("Failed to launch agent 2: %v", err)
	}
	if id2 != "2" {
		t.Errorf("Expected second agent ID to be '2', got '%s'", id2)
	}

	// Wait a bit for agents to start
	time.Sleep(100 * time.Millisecond)

	// Kill the agents to clean up
	manager.KillAgent(id1)
	manager.KillAgent(id2)
}

func TestIDReuse(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// Launch and remove an agent
	id1, _ := manager.LaunchAgent(ctx, "/tmp", "test task 1")
	time.Sleep(100 * time.Millisecond)
	manager.KillAgent(id1)
	manager.RemoveAgent(id1)

	// Launch another agent - should reuse ID 1
	id2, _ := manager.LaunchAgent(ctx, "/tmp", "test task 2")
	if id2 != "1" {
		t.Errorf("Expected reused agent ID to be '1', got '%s'", id2)
	}

	// Clean up
	manager.KillAgent(id2)
}
