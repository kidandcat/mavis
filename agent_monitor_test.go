package main

import (
	"mavis/codeagent"
	"testing"
	"time"
)

func TestFormatAgentCompletionNotification(t *testing.T) {
	tests := []struct {
		name     string
		agent    codeagent.AgentInfo
		contains []string
	}{
		{
			name: "successful agent",
			agent: codeagent.AgentInfo{
				ID:        "test-123",
				Status:    codeagent.StatusFinished,
				Prompt:    "Fix the bug in main.go",
				Folder:    "/home/user/project",
				StartTime: time.Now().Add(-5 * time.Minute),
				EndTime:   time.Now(),
				Duration:  5 * time.Minute,
				Output:    "Successfully fixed the bug",
			},
			contains: []string{
				"✅ *Code Agent Completed*",
				"Successfully completed",
				"test-123",
				"Fix the bug in main.go",
				"/home/user/project",
				"5m0s",
				"Successfully fixed the bug",
			},
		},
		{
			name: "failed agent",
			agent: codeagent.AgentInfo{
				ID:        "test-456",
				Status:    codeagent.StatusFailed,
				Prompt:    "Deploy to production",
				Folder:    "/home/user/app",
				StartTime: time.Now().Add(-2 * time.Minute),
				EndTime:   time.Now(),
				Duration:  2 * time.Minute,
				Error:     "Permission denied",
			},
			contains: []string{
				"❌ *Code Agent Completed*",
				"Failed",
				"test-456",
				"Deploy to production",
				"Permission denied",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			notification := formatAgentCompletionNotification(tt.agent)

			for _, expected := range tt.contains {
				if !contains(notification, expected) {
					t.Errorf("Expected notification to contain '%s', but it didn't.\nNotification:\n%s", expected, notification)
				}
			}
		})
	}
}

func contains(str, substr string) bool {
	return len(substr) > 0 && len(str) >= len(substr) && (str == substr || len(str) > len(substr) && (str[:len(substr)] == substr || contains(str[1:], substr)))
}

func TestRegisterAgentForUser(t *testing.T) {
	// Clear the map
	agentUserMu.Lock()
	agentUserMap = make(map[string]int64)
	agentUserMu.Unlock()

	// Test registration
	RegisterAgentForUser("agent-123", 12345)

	agentUserMu.RLock()
	userID, exists := agentUserMap["agent-123"]
	agentUserMu.RUnlock()

	if !exists || userID != 12345 {
		t.Errorf("Expected agent-123 to be registered for user 12345, got %v, %v", userID, exists)
	}

	// Test overwrite
	RegisterAgentForUser("agent-123", 67890)

	agentUserMu.RLock()
	userID, exists = agentUserMap["agent-123"]
	agentUserMu.RUnlock()

	if !exists || userID != 67890 {
		t.Errorf("Expected agent-123 to be registered for user 67890, got %v, %v", userID, exists)
	}
}

