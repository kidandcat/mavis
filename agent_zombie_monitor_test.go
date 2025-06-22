// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"mavis/codeagent"

	"github.com/go-telegram/bot"
)

// TestMonitorZombieDetection tests that the monitor detects and cleans up zombie processes
func TestMonitorZombieDetection(t *testing.T) {
	// Initialize global agent manager
	agentManager = codeagent.NewManager()

	// Create a mock claude that hangs
	mockClaudePath := "/tmp/claude_zombie_test"
	mockClaudeContent := `#!/bin/bash
# Simulate a hanging process
sleep 300`
	os.WriteFile(mockClaudePath, []byte(mockClaudeContent), 0755)
	defer os.Remove(mockClaudePath)

	// Add to PATH
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", "/tmp:"+originalPath)
	defer os.Setenv("PATH", originalPath)

	// Rename claude to our test version
	os.Rename(mockClaudePath, "/tmp/claude")
	defer os.Remove("/tmp/claude")

	// Create test folder
	testFolder := "./test_zombie_monitor"
	os.MkdirAll(testFolder, 0755)
	defer os.RemoveAll(testFolder)

	// Register a test user
	testUserID := int64(99999)

	// Launch agent
	ctx := context.Background()
	agentID, err := agentManager.LaunchAgent(ctx, testFolder, "test zombie detection")
	if err != nil {
		t.Fatalf("Failed to launch agent: %v", err)
	}

	// Register agent for user
	RegisterAgentForUser(agentID, testUserID)

	// Wait for agent to start
	time.Sleep(500 * time.Millisecond)

	// Get agent and kill its process to simulate zombie
	agent, err := agentManager.GetAgent(agentID)
	if err != nil {
		t.Fatalf("Failed to get agent: %v", err)
	}

	// Kill the process
	agent.Kill()

	// Start monitor in a goroutine
	monitorCtx, cancelMonitor := context.WithCancel(context.Background())
	defer cancelMonitor()

	// Create a mock bot
	var mockBot *bot.Bot = nil // We'll use nil bot which SendMessage handles

	// We'll just verify the agent gets detected and removed
	// since message sending is hard to mock in this test setup

	// Start monitor
	go MonitorAgentsProcess(monitorCtx, mockBot)

	// Wait for monitor to detect the zombie and mark it as failed
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("Zombie process not detected within timeout")
		case <-ticker.C:
			// Check if agent was marked as failed and removed
			_, err := agentManager.GetAgentInfo(agentID)
			if err != nil {
				// Agent was removed, this is what we expect for zombie detection
				t.Log("Zombie process detected and agent removed successfully")
				return
			}

			// Or check if it was marked as failed but not yet removed
			if info, err := agentManager.GetAgentInfo(agentID); err == nil {
				if info.Status == codeagent.StatusFailed && strings.Contains(info.Error, "Process terminated unexpectedly") {
					t.Logf("Zombie process detected and marked as failed with detailed error: %s", info.Error)

					// Verify the error contains detailed information
					if strings.Contains(info.Error, "Working Directory:") &&
						strings.Contains(info.Error, "Failure Time:") {
						t.Log("SUCCESS: Detailed error information captured")
						return
					} else {
						t.Errorf("Error message lacks expected detailed information: %s", info.Error)
						return
					}
				}
			}
		}
	}
}
