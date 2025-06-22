// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"os"
	"testing"
	"time"

	"mavis/codeagent"
)

// TestAgentCompletionAndRemoval tests that agents are properly removed after completion
func TestAgentCompletionAndRemoval(t *testing.T) {
	// Create a test folder
	testFolder := "./test_agent_completion"
	os.MkdirAll(testFolder, 0755)
	defer os.RemoveAll(testFolder)

	// Initialize global agent manager
	agentManager = codeagent.NewManager()

	// Create a mock claude executable in /tmp
	mockClaudePath := "/tmp/claude"
	mockClaudeContent := `#!/bin/bash
echo "Mock claude completed"
exit 0`
	os.WriteFile(mockClaudePath, []byte(mockClaudeContent), 0755)
	defer os.Remove(mockClaudePath)

	// Add /tmp to PATH so our mock claude is found
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", "/tmp:"+originalPath)
	defer os.Setenv("PATH", originalPath)

	// Launch an agent with a simple command
	ctx := context.Background()
	agentID, err := agentManager.LaunchAgent(ctx, testFolder, "echo 'Test completed'")
	if err != nil {
		t.Fatalf("Failed to launch agent: %v", err)
	}

	// Wait for agent to complete
	ctx2, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	agentInfo, err := agentManager.WaitForAgent(ctx2, agentID)
	if err != nil {
		// Check if agent was already removed
		if _, err2 := agentManager.GetAgentInfo(agentID); err2 != nil {
			t.Logf("Agent %s was already removed (expected behavior)", agentID)
			return
		}
		t.Fatalf("Failed to wait for agent: %v", err)
	}

	// Verify agent completed
	if agentInfo.Status == codeagent.StatusRunning {
		t.Error("Agent should not still be running after completion")
	}

	// Verify agent can be removed
	err = agentManager.RemoveAgent(agentID)
	if err != nil && err.Error() != "agent "+agentID+" not found" {
		t.Errorf("Failed to remove agent: %v", err)
	}

	// Verify agent is gone
	_, err = agentManager.GetAgentInfo(agentID)
	if err == nil {
		t.Error("Agent should have been removed")
	}
}

// TestQueueProcessingAfterCompletion tests that queued agents start after the first completes
func TestQueueProcessingAfterCompletion(t *testing.T) {
	// Create a test folder
	testFolder := "./test_queue_completion"
	os.MkdirAll(testFolder, 0755)
	defer os.RemoveAll(testFolder)

	// Initialize global agent manager
	agentManager = codeagent.NewManager()

	// Set up callback to track when queued agents start
	agentStarted := make(chan string, 1)
	agentManager.SetAgentStartCallback(func(agentID, folder, prompt, queueID string) {
		agentStarted <- agentID
	})

	// Create a test script
	scriptPath := testFolder + "/quick_test.sh"
	scriptContent := `#!/bin/bash
echo "Quick test"
sleep 0.5
exit 0`
	os.WriteFile(scriptPath, []byte(scriptContent), 0755)

	// Launch first agent
	ctx := context.Background()
	agent1ID, err := agentManager.LaunchAgent(ctx, testFolder, "First task: bash quick_test.sh")
	if err != nil {
		t.Fatalf("Failed to launch first agent: %v", err)
	}

	// Queue second agent
	agent2ID, err := agentManager.LaunchAgent(ctx, testFolder, "Second task: echo 'queued task'")
	if err != nil {
		t.Fatalf("Failed to queue second agent: %v", err)
	}

	// Second agent should be queued
	if agent2ID[:6] != "queued" {
		t.Errorf("Expected second agent to be queued, got ID: %s", agent2ID)
	}

	// Wait for first agent to complete
	time.Sleep(1 * time.Second)

	// Check if first agent completed
	agent1Info, _ := agentManager.GetAgentInfo(agent1ID)
	if agent1Info.Status == codeagent.StatusRunning {
		// Force mark as completed for test
		if agent1Obj, err := agentManager.GetAgent(agent1ID); err == nil {
			agent1Obj.MarkAsFailed("Test timeout")
		}
	}

	// Remove first agent to trigger queue processing
	agentManager.RemoveAgent(agent1ID)

	// Wait for queued agent to start
	select {
	case newAgentID := <-agentStarted:
		t.Logf("Queued agent started with ID: %s", newAgentID)
	case <-time.After(2 * time.Second):
		t.Error("Queued agent did not start within timeout")
	}

	// Verify queue is now empty
	queueStatus := agentManager.GetQueueStatus()
	if queueStatus[testFolder] != 0 {
		t.Errorf("Expected empty queue, but found %d items", queueStatus[testFolder])
	}
}

// TestAgentZombieProcessDetection tests the zombie process detection logic
func TestAgentZombieProcessDetection(t *testing.T) {
	// Create a test folder
	testFolder := "./test_zombie_detection"
	os.MkdirAll(testFolder, 0755)
	defer os.RemoveAll(testFolder)

	// Initialize global agent manager
	agentManager = codeagent.NewManager()

	// Create a hanging script that we'll kill
	scriptPath := testFolder + "/hanging_script.sh"
	scriptContent := `#!/bin/bash
echo "Starting long process"
sleep 300  # Sleep for 5 minutes
echo "Should not reach here"`
	os.WriteFile(scriptPath, []byte(scriptContent), 0755)

	// Launch agent
	ctx := context.Background()
	agentID, err := agentManager.LaunchAgent(ctx, testFolder, "Run hanging script: bash hanging_script.sh")
	if err != nil {
		t.Fatalf("Failed to launch agent: %v", err)
	}

	// Wait for agent to start
	time.Sleep(1 * time.Second)

	// Get the agent and verify it's running
	agent, err := agentManager.GetAgent(agentID)
	if err != nil {
		t.Fatalf("Failed to get agent: %v", err)
	}

	if agent.GetStatus() != codeagent.StatusRunning {
		t.Fatalf("Expected agent to be running, got: %s", agent.GetStatus())
	}

	// Kill the agent process
	err = agent.Kill()
	if err != nil {
		t.Fatalf("Failed to kill agent: %v", err)
	}

	// Verify status changed to killed
	if agent.GetStatus() != codeagent.StatusKilled {
		t.Errorf("Expected agent status to be killed, got: %s", agent.GetStatus())
	}

	// Verify process is not alive
	if agent.IsProcessAlive() {
		t.Error("Process should not be alive after killing")
	}

	// Clean up
	agentManager.RemoveAgent(agentID)
}