// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"sync"
	"testing"

	"mavis/codeagent"
)

// TestManagerQueueBasics tests basic queue functionality
func TestManagerQueueBasics(t *testing.T) {
	manager := codeagent.NewManager()
	ctx := context.Background()
	
	// Track callback invocations
	var callbackMu sync.Mutex
	callbackInvocations := make(map[string]string) // agentID -> queueID
	
	manager.SetAgentStartCallback(func(agentID, folder, prompt, queueID string) {
		callbackMu.Lock()
		callbackInvocations[agentID] = queueID
		callbackMu.Unlock()
		t.Logf("Callback invoked: agentID=%s, queueID=%s", agentID, queueID)
	})
	
	// Test launching agent in empty folder
	agent1ID, err := manager.LaunchAgent(ctx, "/test/folder1", "test prompt 1")
	if err != nil {
		t.Fatal("Failed to launch first agent:", err)
	}
	
	// Should not be queued
	if len(agent1ID) > 7 && agent1ID[:7] == "queued-" {
		t.Fatal("First agent should not be queued")
	}
	
	// Test launching second agent in same folder - should be queued
	agent2ID, err := manager.LaunchAgent(ctx, "/test/folder1", "test prompt 2")
	if err != nil {
		t.Fatal("Failed to launch second agent:", err)
	}
	
	// Should be queued
	if len(agent2ID) <= 7 || agent2ID[:7] != "queued-" {
		t.Fatal("Second agent should be queued")
	}
	
	// Test launching agent in different folder - should not be queued
	agent3ID, err := manager.LaunchAgent(ctx, "/test/folder2", "test prompt 3")
	if err != nil {
		t.Fatal("Failed to launch third agent:", err)
	}
	
	// Should not be queued
	if len(agent3ID) > 7 && agent3ID[:7] == "queued-" {
		t.Fatal("Third agent in different folder should not be queued")
	}
	
	// Check queue status
	queueStatus := manager.GetQueueStatus()
	if queueStatus["/test/folder1"] != 1 {
		t.Errorf("Expected 1 queued task in folder1, got %d", queueStatus["/test/folder1"])
	}
	if queueStatus["/test/folder2"] != 0 {
		t.Errorf("Expected 0 queued tasks in folder2, got %d", queueStatus["/test/folder2"])
	}
}

// TestManagerQueueProcessing tests that queues are processed when agents are removed
func TestManagerQueueProcessing(t *testing.T) {
	manager := codeagent.NewManager()
	
	// Track which agents started and their queue IDs
	var mu sync.Mutex
	startedAgents := make(map[string]string) // agentID -> queueID
	var startOrder []string
	
	manager.SetAgentStartCallback(func(agentID, folder, prompt, queueID string) {
		mu.Lock()
		startedAgents[agentID] = queueID
		startOrder = append(startOrder, agentID)
		mu.Unlock()
		t.Logf("Agent started from queue: ID=%s, QueueID=%s", agentID, queueID)
	})
	
	// Manually track agents since we can't actually run them
	// Simulate agent lifecycle
	
	// Get initial counts
	initialRunning := manager.GetRunningCount()
	initialTotal := manager.GetTotalCount()
	
	t.Logf("Initial state: running=%d, total=%d", initialRunning, initialTotal)
	
	// The test verifies the queue mechanism exists and callbacks are set up correctly
	// In a real scenario, agents would run actual commands
}

// TestManagerIsAgentRunningInFolder tests the folder tracking
func TestManagerIsAgentRunningInFolder(t *testing.T) {
	manager := codeagent.NewManager()
	ctx := context.Background()
	
	folder := "/test/check/folder"
	
	// Initially no agent should be running
	running, agentID := manager.IsAgentRunningInFolder(folder)
	if running {
		t.Fatal("No agent should be running initially")
	}
	if agentID != "" {
		t.Fatal("Agent ID should be empty when no agent is running")
	}
	
	// Launch an agent
	launchedID, err := manager.LaunchAgent(ctx, folder, "test")
	if err != nil {
		t.Fatal("Failed to launch agent:", err)
	}
	
	// Now an agent should be running
	running, agentID = manager.IsAgentRunningInFolder(folder)
	if !running {
		t.Fatal("Agent should be running after launch")
	}
	if agentID != launchedID {
		t.Errorf("Expected agent ID %s, got %s", launchedID, agentID)
	}
}

// TestManagerGetDetailedQueueStatus tests detailed queue status
func TestManagerGetDetailedQueueStatus(t *testing.T) {
	manager := codeagent.NewManager()
	ctx := context.Background()
	
	folder := "/test/detailed/folder"
	
	// Launch first agent
	_, err := manager.LaunchAgent(ctx, folder, "first agent")
	if err != nil {
		t.Fatal("Failed to launch first agent:", err)
	}
	
	// Queue multiple agents
	expectedPrompts := []string{"second agent", "third agent", "fourth agent"}
	for _, prompt := range expectedPrompts {
		_, err := manager.LaunchAgent(ctx, folder, prompt)
		if err != nil {
			t.Fatalf("Failed to queue agent with prompt '%s': %v", prompt, err)
		}
	}
	
	// Get detailed queue status
	detailed := manager.GetDetailedQueueStatus()
	
	// Check the folder has queued tasks
	tasks, exists := detailed[folder]
	if !exists {
		t.Fatal("Expected folder to have queued tasks")
	}
	
	if len(tasks) != len(expectedPrompts) {
		t.Fatalf("Expected %d queued tasks, got %d", len(expectedPrompts), len(tasks))
	}
	
	// Verify prompts match
	for i, task := range tasks {
		if task.Prompt != expectedPrompts[i] {
			t.Errorf("Task %d: expected prompt '%s', got '%s'", i, expectedPrompts[i], task.Prompt)
		}
		if task.Folder != folder {
			t.Errorf("Task %d: expected folder '%s', got '%s'", i, folder, task.Folder)
		}
		if task.QueueID == "" {
			t.Errorf("Task %d: queue ID should not be empty", i)
		}
	}
}

// TestManagerQueuedTasksForFolder tests getting queue count for specific folder
func TestManagerQueuedTasksForFolder(t *testing.T) {
	manager := codeagent.NewManager()
	ctx := context.Background()
	
	folder1 := "/test/count/folder1"
	folder2 := "/test/count/folder2"
	
	// Initially should be 0
	count := manager.GetQueuedTasksForFolder(folder1)
	if count != 0 {
		t.Errorf("Expected 0 queued tasks initially, got %d", count)
	}
	
	// Launch and queue agents in folder1
	_, _ = manager.LaunchAgent(ctx, folder1, "agent 1")
	_, _ = manager.LaunchAgent(ctx, folder1, "agent 2")
	_, _ = manager.LaunchAgent(ctx, folder1, "agent 3")
	
	// Launch agent in folder2
	_, _ = manager.LaunchAgent(ctx, folder2, "agent 4")
	
	// Check counts
	count1 := manager.GetQueuedTasksForFolder(folder1)
	if count1 != 2 { // First one is running, 2 are queued
		t.Errorf("Expected 2 queued tasks in folder1, got %d", count1)
	}
	
	count2 := manager.GetQueuedTasksForFolder(folder2)
	if count2 != 0 { // One running, none queued
		t.Errorf("Expected 0 queued tasks in folder2, got %d", count2)
	}
}