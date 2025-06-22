package codeagent

import (
	"context"
	"testing"
	"time"
)

func TestNewAgent(t *testing.T) {
	agent := NewAgent("test-1", "/tmp", "test prompt")

	if agent.ID != "test-1" {
		t.Errorf("Expected ID to be test-1, got %s", agent.ID)
	}

	if agent.Folder != "/tmp" {
		t.Errorf("Expected Folder to be /tmp, got %s", agent.Folder)
	}

	if agent.Prompt != "test prompt" {
		t.Errorf("Expected Prompt to be 'test prompt', got %s", agent.Prompt)
	}

	if agent.Status != StatusPending {
		t.Errorf("Expected Status to be pending, got %s", agent.Status)
	}
}

func TestAgentStatus(t *testing.T) {
	agent := NewAgent("test-2", "/tmp", "test")

	// Test initial status
	if status := agent.GetStatus(); status != StatusPending {
		t.Errorf("Expected initial status to be pending, got %s", status)
	}

	// Test status change
	agent.mu.Lock()
	agent.Status = StatusRunning
	agent.mu.Unlock()

	if status := agent.GetStatus(); status != StatusRunning {
		t.Errorf("Expected status to be running, got %s", status)
	}
}

func TestAgentInfo(t *testing.T) {
	agent := NewAgent("test-3", "/tmp", "test prompt")
	agent.mu.Lock()
	agent.Status = StatusFinished
	agent.Output = "test output"
	agent.StartTime = time.Now().Add(-5 * time.Second)
	agent.EndTime = time.Now()
	agent.mu.Unlock()

	info := agent.ToInfo()

	if info.ID != "test-3" {
		t.Errorf("Expected ID to be test-3, got %s", info.ID)
	}

	if info.Status != StatusFinished {
		t.Errorf("Expected Status to be finished, got %s", info.Status)
	}

	if info.Output != "test output" {
		t.Errorf("Expected Output to be 'test output', got %s", info.Output)
	}

	if info.Duration < 4*time.Second || info.Duration > 6*time.Second {
		t.Errorf("Expected Duration to be around 5 seconds, got %s", info.Duration)
	}
}

func TestManager(t *testing.T) {
	manager := NewManager()

	// Test initial state
	if count := manager.GetTotalCount(); count != 0 {
		t.Errorf("Expected initial count to be 0, got %d", count)
	}

	// Test adding agent with custom ID
	err := manager.LaunchAgentWithID(context.Background(), "custom-id", "/tmp", "test")
	if err != nil {
		t.Errorf("Failed to launch agent with custom ID: %v", err)
	}

	// Test duplicate ID
	err = manager.LaunchAgentWithID(context.Background(), "custom-id", "/tmp", "test")
	if err == nil {
		t.Error("Expected error when adding duplicate ID")
	}

	// Test getting agent
	agent, err := manager.GetAgent("custom-id")
	if err != nil {
		t.Errorf("Failed to get agent: %v", err)
	}

	if agent.ID != "custom-id" {
		t.Errorf("Expected agent ID to be custom-id, got %s", agent.ID)
	}

	// Test listing agents
	agents := manager.ListAgents()
	if len(agents) != 1 {
		t.Errorf("Expected 1 agent, got %d", len(agents))
	}

	// Test removing agent
	err = manager.RemoveAgent("custom-id")
	if err != nil {
		t.Errorf("Failed to remove agent: %v", err)
	}

	if count := manager.GetTotalCount(); count != 0 {
		t.Errorf("Expected count to be 0 after removal, got %d", count)
	}
}

func TestManagerAutoID(t *testing.T) {
	manager := NewManager()
	ctx := context.Background()

	// Launch multiple agents
	id1, err := manager.LaunchAgent(ctx, "/tmp", "test1")
	if err != nil {
		t.Fatalf("Failed to launch agent 1: %v", err)
	}

	id2, err := manager.LaunchAgent(ctx, "/tmp2", "test2")
	if err != nil {
		t.Fatalf("Failed to launch agent 2: %v", err)
	}

	// IDs should be different
	if id1 == id2 {
		t.Error("Expected different IDs for different agents")
	}

	// Both should exist
	if _, err := manager.GetAgent(id1); err != nil {
		t.Errorf("Failed to get agent 1: %v", err)
	}

	if _, err := manager.GetAgent(id2); err != nil {
		t.Errorf("Failed to get agent 2: %v", err)
	}
}
