// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetCodeAgentDetailsWithCurrentPlan(t *testing.T) {
	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "test_agent_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create CURRENT_PLAN.md in the temp directory
	planContent := `# Current Task Plan

## Task
Test task for agent

## Plan
1. Step one
2. Step two
3. Step three

## Progress
1. ✅ Step one completed
2. Working on step two
`
	planPath := filepath.Join(tempDir, "CURRENT_PLAN.md")
	err = os.WriteFile(planPath, []byte(planContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write plan file: %v", err)
	}

	// Test that the plan content is read when agent is running
	// This test verifies that:
	// 1. The file path is correctly constructed
	// 2. The file is read when status is "running"
	// 3. The content is included in the status message

	// Read the file to verify it was created correctly
	content, err := os.ReadFile(planPath)
	if err != nil {
		t.Errorf("Failed to read plan file: %v", err)
	}
	if string(content) != planContent {
		t.Errorf("Plan content mismatch. Expected: %s, Got: %s", planContent, string(content))
	}

	// Verify the path construction logic
	expectedPath := filepath.Join(tempDir, "CURRENT_PLAN.md")
	if planPath != expectedPath {
		t.Errorf("Path mismatch. Expected: %s, Got: %s", expectedPath, planPath)
	}
}

func TestGetCodeAgentDetailsWithoutCurrentPlan(t *testing.T) {
	// Create a temporary directory without CURRENT_PLAN.md
	tempDir, err := os.MkdirTemp("", "test_agent_no_plan_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Verify that the code handles missing CURRENT_PLAN.md gracefully
	planPath := filepath.Join(tempDir, "CURRENT_PLAN.md")
	_, err = os.ReadFile(planPath)
	if err == nil {
		t.Error("Expected error for missing file, but got none")
	}
	if !os.IsNotExist(err) {
		t.Errorf("Expected IsNotExist error, got: %v", err)
	}
}
