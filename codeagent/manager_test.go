package codeagent

import (
	"context"
	"testing"
	"time"
)

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

	id2, err := manager.LaunchAgent(ctx, "/tmp", "test task 2")
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

