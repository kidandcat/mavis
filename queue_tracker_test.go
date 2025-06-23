// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"testing"
)

func TestQueueTracker_RegisterAndGet(t *testing.T) {
	// Create a new queue tracker
	qt := &QueueTracker{
		queuedAgents: make(map[string]QueuedAgentInfo),
	}

	// Test data
	queueID := "queue-123"
	userID := int64(12345)
	folder := "/test/folder"
	prompt := "test prompt"

	// Register a queued agent
	qt.RegisterQueuedAgent(queueID, userID, folder, prompt)

	// Get the queued agent info
	info, exists := qt.GetQueuedAgentInfo(queueID)
	if !exists {
		t.Fatal("Expected queued agent to exist")
	}

	// Verify the info
	if info.QueueID != queueID {
		t.Errorf("Expected QueueID %s, got %s", queueID, info.QueueID)
	}
	if info.UserID != userID {
		t.Errorf("Expected UserID %d, got %d", userID, info.UserID)
	}
	if info.Folder != folder {
		t.Errorf("Expected Folder %s, got %s", folder, info.Folder)
	}
	if info.Prompt != prompt {
		t.Errorf("Expected Prompt %s, got %s", prompt, info.Prompt)
	}
}

func TestQueueTracker_RemoveQueuedAgent(t *testing.T) {
	// Create a new queue tracker
	qt := &QueueTracker{
		queuedAgents: make(map[string]QueuedAgentInfo),
	}

	// Register a queued agent
	queueID := "queue-456"
	qt.RegisterQueuedAgent(queueID, 67890, "/test", "test")

	// Verify it exists
	_, exists := qt.GetQueuedAgentInfo(queueID)
	if !exists {
		t.Fatal("Expected queued agent to exist after registration")
	}

	// Remove the queued agent
	qt.RemoveQueuedAgent(queueID)

	// Verify it no longer exists
	_, exists = qt.GetQueuedAgentInfo(queueID)
	if exists {
		t.Fatal("Expected queued agent to not exist after removal")
	}
}

func TestQueueTracker_GetQueuedAgentByFolder(t *testing.T) {
	// Create a new queue tracker
	qt := &QueueTracker{
		queuedAgents: make(map[string]QueuedAgentInfo),
	}

	// Register multiple agents in different folders
	folder1 := "/test/folder1"
	folder2 := "/test/folder2"

	qt.RegisterQueuedAgent("queue-1", 111, folder1, "prompt 1")
	qt.RegisterQueuedAgent("queue-2", 222, folder1, "prompt 2")
	qt.RegisterQueuedAgent("queue-3", 333, folder2, "prompt 3")

	// Get agents for folder1
	agents1 := qt.GetQueuedAgentByFolder(folder1)
	if len(agents1) != 2 {
		t.Errorf("Expected 2 agents in folder1, got %d", len(agents1))
	}

	// Get agents for folder2
	agents2 := qt.GetQueuedAgentByFolder(folder2)
	if len(agents2) != 1 {
		t.Errorf("Expected 1 agent in folder2, got %d", len(agents2))
	}

	// Get agents for non-existent folder
	agents3 := qt.GetQueuedAgentByFolder("/nonexistent")
	if len(agents3) != 0 {
		t.Errorf("Expected 0 agents in nonexistent folder, got %d", len(agents3))
	}
}

func TestQueueTracker_ConcurrentAccess(t *testing.T) {
	// Create a new queue tracker
	qt := &QueueTracker{
		queuedAgents: make(map[string]QueuedAgentInfo),
	}

	// Test concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			queueID := fmt.Sprintf("queue-%d", id)
			qt.RegisterQueuedAgent(queueID, int64(id), "/test", "test")
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all agents were registered
	for i := 0; i < 10; i++ {
		queueID := fmt.Sprintf("queue-%d", i)
		_, exists := qt.GetQueuedAgentInfo(queueID)
		if !exists {
			t.Errorf("Expected queue-%d to exist", i)
		}
	}

	// Test concurrent reads
	for i := 0; i < 10; i++ {
		go func() {
			agents := qt.GetQueuedAgentByFolder("/test")
			if len(agents) != 10 {
				t.Errorf("Expected 10 agents, got %d", len(agents))
			}
			done <- true
		}()
	}

	// Wait for all read goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestQueueTracker_GetNonExistentAgent(t *testing.T) {
	// Create a new queue tracker
	qt := &QueueTracker{
		queuedAgents: make(map[string]QueuedAgentInfo),
	}

	// Try to get a non-existent agent
	_, exists := qt.GetQueuedAgentInfo("nonexistent")
	if exists {
		t.Fatal("Expected non-existent agent to not exist")
	}
}
