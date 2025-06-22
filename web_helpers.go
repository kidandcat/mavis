// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// GetAllAgentsStatus returns status of all agents
func GetAllAgentsStatus() []map[string]interface{} {
	agents := agentManager.ListAgents()
	result := make([]map[string]interface{}, 0, len(agents))
	
	for _, agent := range agents {
		agentInfo := map[string]interface{}{
			"id":        agent.ID,
			"directory": agent.Folder,
			"task":      agent.Prompt,
			"status":    string(agent.Status),
		}
		
		// Add timing info
		if !agent.StartTime.IsZero() {
			agentInfo["started_at"] = agent.StartTime
			agentInfo["runtime"] = time.Since(agent.StartTime).Round(time.Second).String()
		}
		
		result = append(result, agentInfo)
	}
	
	// Add queued tasks
	queueStatus := agentManager.GetDetailedQueueStatus()
	for folder, tasks := range queueStatus {
		for i, task := range tasks {
			result = append(result, map[string]interface{}{
				"id":             task.QueueID,
				"directory":      folder,
				"task":           task.Prompt,
				"status":         "queued",
				"queue_position": i + 1,
			})
		}
	}
	
	return result
}

// GetAgentDetailedStatus returns detailed status of a specific agent
func GetAgentDetailedStatus(agentID string) map[string]interface{} {
	agent, err := agentManager.GetAgent(agentID)
	if err != nil || agent == nil {
		return nil
	}
	
	status := map[string]interface{}{
		"id":        agent.ID,
		"directory": agent.Folder,
		"task":      agent.Prompt,
		"status":    string(agent.Status),
	}
	
	// Add current plan if available
	if agent.Status == "running" {
		planPath := filepath.Join(agent.Folder, "CURRENT_PLAN.md")
		if content, err := os.ReadFile(planPath); err == nil {
			status["current_plan"] = string(content)
		}
	}
	
	// Add output if available
	if agent.Output != "" {
		status["output"] = agent.Output
	}
	
	return status
}

// StopAgent stops a running agent
func StopAgent(agentID string) error {
	agent, err := agentManager.GetAgent(agentID)
	if err != nil || agent == nil {
		return fmt.Errorf("agent not found")
	}
	
	if agent.Status != "running" {
		return fmt.Errorf("agent is not running")
	}
	
	return agentManager.KillAgent(agentID)
}

// CreateCodeAgent creates a new code agent
func CreateCodeAgent(ctx context.Context, workDir, task string, images []string, userID int64) (string, error) {
	// Validate git repo
	if !isGitRepo(workDir) {
		return "", fmt.Errorf("not a git repository: %s", workDir)
	}
	
	// Check usage limits
	if err := checkUsageLimits(); err != nil {
		return "", err
	}
	
	// Launch agent
	agentID, err := agentManager.LaunchAgent(ctx, workDir, task)
	if err != nil {
		return "", err
	}
	
	// TODO: Handle images if needed
	
	// Register agent for user
	RegisterAgentForUser(agentID, userID)
	
	// Broadcast SSE event
	BroadcastSSEEvent("agent_started", map[string]interface{}{
		"agent_id": agentID,
		"directory": workDir,
		"task": task,
	})
	
	return agentID, nil
}

// CreateNewBranchAgent creates a new branch and launches agent
func CreateNewBranchAgent(ctx context.Context, workDir, task string, images []string, userID int64) (string, error) {
	// Similar to handleNewBranchCommand
	if !isGitRepo(workDir) {
		return "", fmt.Errorf("not a git repository")
	}
	
	// Create unique branch name
	timestamp := time.Now().Format("20060102-150405")
	branchName := fmt.Sprintf("mavis-%s", timestamp)
	
	// Create and checkout new branch
	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to create branch: %s", output)
	}
	
	// Push branch to remote
	cmd = exec.Command("git", "push", "-u", "origin", branchName)
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		// Try to checkout main branch again
		exec.Command("git", "checkout", "main").Run()
		return "", fmt.Errorf("failed to push branch: %s", output)
	}
	
	// Launch agent
	agentID, err := CreateCodeAgent(ctx, workDir, task, images, userID)
	if err != nil {
		return "", err
	}
	
	return agentID, nil
}

// CreateEditBranchAgent checks out existing branch and launches agent
func CreateEditBranchAgent(ctx context.Context, workDir, branch, task string, images []string, userID int64) (string, error) {
	if !isGitRepo(workDir) {
		return "", fmt.Errorf("not a git repository")
	}
	
	// Fetch latest
	cmd := exec.Command("git", "fetch")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to fetch: %v", err)
	}
	
	// Checkout branch
	cmd = exec.Command("git", "checkout", branch)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to checkout branch: %s", output)
	}
	
	// Pull latest changes
	cmd = exec.Command("git", "pull")
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to pull: %s", output)
	}
	
	// Launch agent
	return CreateCodeAgent(ctx, workDir, task, images, userID)
}

// getGitDiff returns git diff for a path
func getGitDiff(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	
	var cmd *exec.Cmd
	if info.IsDir() {
		// Check if it's a git repo
		if !isGitRepo(path) {
			return "", fmt.Errorf("not a git repository")
		}
		cmd = exec.Command("git", "diff", "HEAD")
		cmd.Dir = path
	} else {
		// Single file
		dir := filepath.Dir(path)
		if !isGitRepo(dir) {
			return "", fmt.Errorf("not in a git repository")
		}
		relPath, _ := filepath.Rel(dir, path)
		cmd = exec.Command("git", "diff", "HEAD", "--", relPath)
		cmd.Dir = dir
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %s", output)
	}
	
	if len(output) == 0 {
		return "No changes detected", nil
	}
	
	return string(output), nil
}

// commitAndPush commits and pushes changes
func commitAndPush(workDir, message string) (string, error) {
	if !isGitRepo(workDir) {
		return "", fmt.Errorf("not a git repository")
	}
	
	// Check for changes
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git status failed: %s", output)
	}
	
	if len(strings.TrimSpace(string(output))) == 0 {
		return "No changes to commit", nil
	}
	
	// Add all changes
	cmd = exec.Command("git", "add", "-A")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git add failed: %v", err)
	}
	
	// Commit
	if message == "" {
		message = "Update from Mavis web interface"
	}
	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git commit failed: %s", output)
	}
	
	result := string(output)
	
	// Push
	cmd = exec.Command("git", "push")
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git push failed: %s", output)
	}
	
	result += "\n" + string(output)
	return result, nil
}

// runCommand runs a command in a directory
func runCommand(workDir, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = workDir
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %v", err)
	}
	
	return string(output), nil
}

// checkUsageLimits checks if we've hit API usage limits
func checkUsageLimits() error {
	// This is a placeholder - implement actual usage limit checking
	// based on your requirements
	return nil
}

// isGitRepo checks if a directory is a git repository
func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	err := cmd.Run()
	return err == nil
}

// resolvePath resolves a path (alias for ResolvePath)
func resolvePath(path string) string {
	resolved, err := ResolvePath(path)
	if err != nil {
		return path
	}
	return resolved
}