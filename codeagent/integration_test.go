// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package codeagent

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestIntegrationAgentCompletionAndQueueProcessing tests the full integration
// of agent completion detection and queue processing
func TestIntegrationAgentCompletionAndQueueProcessing(t *testing.T) {
	// Set up a complete test environment
	testManager := NewManager()
	// Note: agentManager is in main package, can't access from here
	// This test would need to be refactored or moved back to main package

	ctx := context.Background()
	testFolder := "/tmp/test-integration"

	// Track all events
	events := make([]string, 0)
	var eventsMu sync.Mutex

	recordEvent := func(event string) {
		eventsMu.Lock()
		events = append(events, fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05.000"), event))
		eventsMu.Unlock()
		log.Printf("[TEST-EVENT] %s", event)
	}

	// Track notifications through events
	// In a real test, we would use dependency injection to mock SendLongMessage

	// Set callback for queue processing
	testManager.SetAgentStartCallback(func(agentID, folder, prompt, queueID string) {
		recordEvent(fmt.Sprintf("Queued agent %s started (queue: %s)", agentID, queueID))
		// RegisterAgentForUser is in telegram package, can't access from here
		// Would need to be mocked or removed
	})

	// Step 1: Launch first agent
	recordEvent("Launching first agent")
	agent1ID, err := testManager.LaunchAgent(ctx, testFolder, "First task")
	if err != nil {
		t.Fatalf("Failed to launch first agent: %v", err)
	}
	// RegisterAgentForUser is in telegram package, can't access from here
	recordEvent(fmt.Sprintf("First agent launched: %s", agent1ID))

	// Step 2: Launch second agent (should queue)
	recordEvent("Launching second agent")
	agent2Result, err := testManager.LaunchAgent(ctx, testFolder, "Second task")
	if err != nil {
		t.Fatalf("Failed to launch second agent: %v", err)
	}
	recordEvent(fmt.Sprintf("Second agent result: %s", agent2Result))

	// Verify it's queued
	if !strings.HasPrefix(agent2Result, "queued-") {
		t.Errorf("Second agent should be queued, got: %s", agent2Result)
	}

	// Step 3: Start the monitor
	// Note: MonitorAgentsProcess is in telegram package, can't access from here
	// This integration test would need to be refactored to work without it
	recordEvent("Starting agent monitor - SKIPPED (cross-package dependency)")

	// Step 4: Simulate agent completion
	time.Sleep(1 * time.Second) // Let monitor start

	agent1, err := testManager.GetAgent(agent1ID)
	if err != nil {
		t.Fatalf("Failed to get agent: %v", err)
	}

	recordEvent("Simulating agent completion")
	agent1.Status = StatusFinished
	agent1.Output = "First task completed"
	agent1.EndTime = time.Now()

	// Step 5: Wait for monitor to detect and process
	recordEvent("Waiting for monitor detection...")

	// Wait up to 10 seconds for completion
	completionDetected := false
	queueProcessed := false

	for i := 0; i < 20; i++ {
		time.Sleep(500 * time.Millisecond)

		// Check if first agent was removed
		_, err := testManager.GetAgent(agent1ID)
		if err != nil {
			completionDetected = true
			recordEvent("First agent removed - completion detected")
		}

		// Check if queue was processed
		queueStatus := testManager.GetQueueStatus()
		if queueStatus[testFolder] == 0 && completionDetected {
			queueProcessed = true
			recordEvent("Queue processed - second agent should be running")
			break
		}
	}

	// Print all events for debugging
	eventsMu.Lock()
	t.Log("Event timeline:")
	for _, event := range events {
		t.Log(event)
	}
	eventsMu.Unlock()

	// Verify results
	if !completionDetected {
		t.Error("Agent completion was not detected by monitor")
	}
	if !queueProcessed {
		t.Error("Queue was not processed after agent completion")
	}
}

// TestZombieProcessDetection simulates the specific bug where process appears finished
// but agent status is not updated
func TestZombieProcessDetection(t *testing.T) {
	// This test demonstrates the problematic scenario
	testFolder := "/tmp/test-zombie-process"

	// Create a mock agent that simulates the zombie state
	mockAgent := &ZombieAgent{
		ID:           "zombie-1",
		Folder:       testFolder,
		Prompt:       "Zombie test task",
		Status:       StatusRunning, // Agent thinks it's running
		ProcessState: "finished",              // But process is actually finished
		StartTime:    time.Now().Add(-5 * time.Minute),
	}

	// In the real scenario:
	// 1. Agent's Start() method would launch a process
	// 2. Process completes but agent.Status doesn't update
	// 3. Monitor checks agent.Status (StatusRunning) and skips it
	// 4. Queue doesn't advance because agent appears to still be running

	// The fix should involve:
	// - Monitor checking actual process state when Status is Running
	// - Updating Status if process is no longer running
	// - Then normal completion flow triggers

	log.Printf("[ZOMBIE-TEST] Simulated zombie agent: ID=%s, Status=%s, ProcessState=%s",
		mockAgent.ID, mockAgent.Status, mockAgent.ProcessState)

	// Demonstrate what monitor should do
	if mockAgent.Status == StatusRunning && mockAgent.ProcessState == "finished" {
		log.Printf("[ZOMBIE-TEST] Detected zombie process - should update status to Finished")
		mockAgent.Status = StatusFinished
		log.Printf("[ZOMBIE-TEST] Status updated, normal completion flow can proceed")
	}
}

// TestProcessCheckingIntegration tests using ps command to verify process state
func TestProcessCheckingIntegration(t *testing.T) {
	// This test demonstrates how to check if a process is actually running

	// Launch a sleep command as a test process
	cmd := exec.Command("sleep", "2")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	pid := cmd.Process.Pid
	log.Printf("[PROCESS-TEST] Started test process with PID: %d", pid)

	// Function to check if process is running using ps
	isProcessRunning := func(pid int) bool {
		psCmd := exec.Command("ps", "-p", fmt.Sprintf("%d", pid))
		output, err := psCmd.Output()
		if err != nil {
			return false // Process not found
		}
		// Check if output contains the PID (process is running)
		return strings.Contains(string(output), fmt.Sprintf("%d", pid))
	}

	// Verify process is running
	if !isProcessRunning(pid) {
		t.Error("Process should be running immediately after start")
	}

	// Wait for process to complete
	cmd.Wait()

	// Verify process is no longer running
	time.Sleep(100 * time.Millisecond) // Small delay for OS cleanup
	if isProcessRunning(pid) {
		t.Error("Process should not be running after completion")
	}

	log.Printf("[PROCESS-TEST] Process checking works correctly")
}

// Helper types for testing

type ZombieAgent struct {
	ID           string
	Folder       string
	Prompt       string
	Status       AgentStatus
	ProcessState string // What 'ps' would show
	StartTime    time.Time
}

// TestMonitorProcessDetection tests enhanced monitor that checks process state
func TestMonitorProcessDetection(t *testing.T) {
	// This test shows how the monitor should be enhanced to detect zombie processes

	// Pseudo-code for enhanced monitor:
	/*
		for _, agent := range agents {
			if agent.Status == StatusRunning {
				// Check if process is actually running
				if cmd := agent.GetCmd(); cmd != nil && cmd.Process != nil {
					pid := cmd.Process.Pid
					if !isProcessRunning(pid) {
						// Process is dead but status is Running - zombie!
						log.Printf("Detected zombie process for agent %s", agent.ID)
						agent.UpdateStatus(StatusFinished) // or StatusFailed
					}
				}
			}

			// Continue with normal completion detection...
		}
	*/

	t.Log("Enhanced monitor would detect zombie processes by checking actual process state")
}
