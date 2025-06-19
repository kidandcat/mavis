// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package codeagent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// AgentStatus represents the current state of an agent
type AgentStatus string

const (
	StatusPending  AgentStatus = "pending"
	StatusRunning  AgentStatus = "running"
	StatusFinished AgentStatus = "finished"
	StatusFailed   AgentStatus = "failed"
	StatusKilled   AgentStatus = "killed"
)

// Agent represents a code agent instance
type Agent struct {
	ID           string
	Folder       string
	Prompt       string
	Status       AgentStatus
	Output       string
	Error        string
	StartTime    time.Time
	EndTime      time.Time
	cmd          *exec.Cmd
	mu           sync.RWMutex
	PlanFilename string // Custom plan filename (defaults to CURRENT_PLAN.md)
}

// NewAgent creates a new agent instance
func NewAgent(id, folder, prompt string) *Agent {
	return &Agent{
		ID:           id,
		Folder:       folder,
		Prompt:       prompt,
		Status:       StatusPending,
		PlanFilename: "CURRENT_PLAN.md", // Default plan filename
	}
}

// NewAgentWithPlanFile creates a new agent instance with a custom plan filename
func NewAgentWithPlanFile(id, folder, prompt, planFilename string) *Agent {
	return &Agent{
		ID:           id,
		Folder:       folder,
		Prompt:       prompt,
		Status:       StatusPending,
		PlanFilename: planFilename,
	}
}

// Start launches the agent
func (a *Agent) Start(ctx context.Context) error {
	// Create plan file
	planFile := fmt.Sprintf("%s/%s", a.Folder, a.PlanFilename)
	planContent := `# Current Task Plan

## Task
` + a.Prompt + `

## Plan
(The AI will write its plan here)

## Progress
(The AI will update progress here as it works)
`
	if err := os.WriteFile(planFile, []byte(planContent), 0644); err != nil {
		return fmt.Errorf("failed to create %s: %v", a.PlanFilename, err)
	}

	// Defer cleanup of the plan file
	defer func() {
		os.Remove(planFile)
	}()

	// Modified prompt to instruct the AI to use the plan file
	enhancedPrompt := fmt.Sprintf(`IMPORTANT: Before starting any work, you MUST:
1. Read the file %s in the current directory
2. Write your detailed plan for completing the task in the "## Plan" section
3. As you work, update the "## Progress" section with what you've completed
4. Keep the plan updated as you discover new requirements or change approach

`, a.PlanFilename) + a.Prompt

	// Use a shell to properly execute the claude script
	// Escape single quotes in the prompt to prevent shell injection
	escapedPrompt := strings.ReplaceAll(enhancedPrompt, "'", "'\"'\"'")
	cmdString := fmt.Sprintf("cd '%s' && claude --dangerously-skip-permissions -p '%s'", a.Folder, escapedPrompt)
	a.cmd = exec.CommandContext(ctx, "/bin/sh", "-c", cmdString)

	// Capture output
	output, err := a.cmd.CombinedOutput()

	a.mu.Lock()
	a.Output = string(output)
	a.EndTime = time.Now()

	if err != nil {
		a.Status = StatusFailed
		a.Error = fmt.Sprintf("Command failed: %v\nOutput: %s\nDirectory: %s", err, string(output), a.Folder)
	} else {
		a.Status = StatusFinished
	}
	a.mu.Unlock()

	return nil
}

// StartAsync launches the agent asynchronously
func (a *Agent) StartAsync(ctx context.Context) {
	go func() {
		a.mu.Lock()
		a.Status = StatusRunning
		a.StartTime = time.Now()
		a.mu.Unlock()

		_ = a.Start(ctx)
	}()
}

// Kill terminates the agent if it's running
func (a *Agent) Kill() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Status != StatusRunning {
		return fmt.Errorf("agent %s is not running", a.ID)
	}

	if a.cmd != nil && a.cmd.Process != nil {
		if err := a.cmd.Process.Kill(); err != nil {
			return err
		}
		a.Status = StatusKilled
		a.EndTime = time.Now()
	}

	return nil
}

// GetStatus returns the current status of the agent
func (a *Agent) GetStatus() AgentStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Status
}

// GetOutput returns the output of the agent
func (a *Agent) GetOutput() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Output
}

// GetError returns any error from the agent execution
func (a *Agent) GetError() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Error
}

// GetDuration returns how long the agent has been running or ran
func (a *Agent) GetDuration() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.StartTime.IsZero() {
		return 0
	}

	if a.EndTime.IsZero() {
		return time.Since(a.StartTime)
	}

	return a.EndTime.Sub(a.StartTime)
}

// ToInfo returns a snapshot of the agent's current state
func (a *Agent) ToInfo() AgentInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return AgentInfo{
		ID:        a.ID,
		Folder:    a.Folder,
		Prompt:    a.Prompt,
		Status:    a.Status,
		Output:    a.Output,
		Error:     a.Error,
		StartTime: a.StartTime,
		EndTime:   a.EndTime,
		Duration:  a.GetDuration(),
	}
}

// AgentInfo is a snapshot of an agent's state
type AgentInfo struct {
	ID        string
	Folder    string
	Prompt    string
	Status    AgentStatus
	Output    string
	Error     string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
}

