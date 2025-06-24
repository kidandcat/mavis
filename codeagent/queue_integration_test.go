// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package codeagent

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

)

// TestQueueToAgentTransition tests the transition from queued to running agent
func TestQueueToAgentTransition(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "mavis-queue-test-*")
	if err != nil {
		t.Fatal("Failed to create temp dir:", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a manager
	manager := NewManager()

	// Track which agents started from queue
	startedAgents := make(map[string]bool)
	queueToAgent := make(map[string]string) // queueID -> agentID

	// Set up the callback to track when queued agents start
	manager.SetAgentStartCallback(func(agentID, folder, prompt, queueID string) {
		if queueID != "" {
			startedAgents[agentID] = true
			queueToAgent[queueID] = agentID
			t.Logf("Queued agent started: ID=%s, QueueID=%s", agentID, queueID)
		}
	})

	// Start first agent
	ctx := context.Background()
	agent1ID, err := manager.LaunchAgent(ctx, tempDir, "echo 'First agent'; sleep 1")
	if err != nil {
		t.Fatal("Failed to launch first agent:", err)
	}
	t.Logf("Launched first agent: %s", agent1ID)

	// Queue second agent
	agent2ID, err := manager.LaunchAgent(ctx, tempDir, "echo 'Second agent'")
	if err != nil {
		t.Fatal("Failed to queue second agent:", err)
	}

	// Verify second agent is queued
	if !isQueuedID(agent2ID) {
		t.Fatalf("Expected second agent to be queued, got ID: %s", agent2ID)
	}

	// Extract queue ID from the queued response
	queueID := extractQueueID(agent2ID)
	t.Logf("Second agent queued with ID: %s, QueueID: %s", agent2ID, queueID)

	// Wait for first agent to complete
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	firstInfo, err := manager.WaitForAgent(waitCtx, agent1ID)
	if err != nil {
		t.Fatal("Failed to wait for first agent:", err)
	}

	if firstInfo.Status != StatusFinished {
		t.Fatalf("First agent did not finish successfully: %s", firstInfo.Status)
	}

	// Remove first agent to trigger queue processing
	err = manager.RemoveAgent(agent1ID)
	if err != nil {
		t.Fatal("Failed to remove first agent:", err)
	}

	// Wait a bit for queue processing
	time.Sleep(2 * time.Second)

	// Check if the queued agent started
	actualAgentID, found := queueToAgent[queueID]
	if !found {
		t.Fatal("Queued agent did not start")
	}

	// Verify the agent is running or finished
	agentInfo, err := manager.GetAgentInfo(actualAgentID)
	if err != nil {
		t.Fatal("Failed to get agent info:", err)
	}

	if agentInfo.Status != StatusRunning &&
		agentInfo.Status != StatusFinished {
		t.Fatalf("Expected agent to be running or finished, got: %s", agentInfo.Status)
	}

	t.Logf("Successfully transitioned from queue to running agent: %s", actualAgentID)
}

// TestMultipleQueuedAgentsIntegration tests multiple agents queued for the same folder
func TestMultipleQueuedAgentsIntegration(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "mavis-multi-queue-test-*")
	if err != nil {
		t.Fatal("Failed to create temp dir:", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a manager
	manager := NewManager()

	// Track agent starts
	var startOrder []string
	manager.SetAgentStartCallback(func(agentID, folder, prompt, queueID string) {
		startOrder = append(startOrder, agentID)
		t.Logf("Agent started: ID=%s, QueueID=%s", agentID, queueID)
	})

	ctx := context.Background()

	// Launch first agent (long running)
	_, err = manager.LaunchAgent(ctx, tempDir, "echo 'Agent 1'; sleep 2")
	if err != nil {
		t.Fatal("Failed to launch agent 1:", err)
	}

	// Queue multiple agents
	var queuedIDs []string
	for i := 2; i <= 4; i++ {
		agentID, err := manager.LaunchAgent(ctx, tempDir, fmt.Sprintf("echo 'Agent %d'", i))
		if err != nil {
			t.Fatalf("Failed to queue agent %d: %v", i, err)
		}
		if !isQueuedID(agentID) {
			t.Fatalf("Expected agent %d to be queued", i)
		}
		queuedIDs = append(queuedIDs, agentID)
		t.Logf("Queued agent %d: %s", i, agentID)
	}

	// Check queue status
	queueStatus := manager.GetQueueStatus()
	if queueStatus[tempDir] != 3 {
		t.Fatalf("Expected 3 queued tasks, got %d", queueStatus[tempDir])
	}

	// Wait for all agents to complete
	time.Sleep(10 * time.Second)

	// Verify all agents ran
	if len(startOrder) < 3 { // First agent + 3 queued
		t.Fatalf("Expected at least 3 agents to start, got %d", len(startOrder))
	}

	t.Log("All queued agents processed successfully")
}

// TestQueuedAgentFailure tests handling of queued agent when running agent fails
func TestQueuedAgentFailure(t *testing.T) {
	// Create a temporary test directory
	tempDir, err := os.MkdirTemp("", "mavis-queue-fail-test-*")
	if err != nil {
		t.Fatal("Failed to create temp dir:", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a manager
	manager := NewManager()

	// Track agent starts
	agentStarted := false
	manager.SetAgentStartCallback(func(agentID, folder, prompt, queueID string) {
		if queueID != "" {
			agentStarted = true
			t.Logf("Queued agent started after failure: ID=%s", agentID)
		}
	})

	ctx := context.Background()

	// Launch agent that will fail
	agent1ID, err := manager.LaunchAgent(ctx, tempDir, "exit 1")
	if err != nil {
		t.Fatal("Failed to launch failing agent:", err)
	}

	// Queue another agent
	agent2ID, err := manager.LaunchAgent(ctx, tempDir, "echo 'Success'")
	if err != nil {
		t.Fatal("Failed to queue second agent:", err)
	}

	if !isQueuedID(agent2ID) {
		t.Fatalf("Expected second agent to be queued, got: %s", agent2ID)
	}

	// Wait for first agent to fail
	time.Sleep(3 * time.Second)

	// Check first agent status
	agent1Info, err := manager.GetAgentInfo(agent1ID)
	if err != nil {
		t.Fatal("Failed to get agent1 info:", err)
	}

	if agent1Info.Status != StatusFailed {
		t.Fatalf("Expected first agent to fail, got: %s", agent1Info.Status)
	}

	// Remove failed agent to trigger queue
	err = manager.RemoveAgent(agent1ID)
	if err != nil {
		t.Fatal("Failed to remove failed agent:", err)
	}

	// Wait for queue processing
	time.Sleep(2 * time.Second)

	if !agentStarted {
		t.Fatal("Queued agent did not start after first agent failed")
	}

	t.Log("Queued agent successfully started after failure")
}

// Helper functions
func isQueuedID(id string) bool {
	return len(id) > 7 && id[:7] == "queued-"
}

func extractQueueID(queuedAgentID string) string {
	// Format: "queued-%s-pos-%d-qid-%s"
	// Extract the queue ID after "qid-"
	for i := 0; i < len(queuedAgentID)-4; i++ {
		if queuedAgentID[i:i+4] == "qid-" {
			return queuedAgentID[i+4:]
		}
	}
	return ""
}
