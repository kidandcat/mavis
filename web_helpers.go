// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"syscall"
)

// AgentStatusInfo represents agent information for web display
type AgentStatusInfo struct {
	ID           string
	Task         string
	Status       string
	StartTime    time.Time
	LastActive   time.Time
	MessagesSent int
	QueueStatus  string
	IsStale      bool
}

// GetAllAgentsStatusJSON returns status of all active agents for web interface
func GetAllAgentsStatusJSON() []AgentStatusInfo {
	result := []AgentStatusInfo{}

	// Get active agents from manager
	agents := agentManager.ListAgents()
	for _, agent := range agents {
		status := "active"
		if agent.Status != "active" {
			status = string(agent.Status)
		}

		result = append(result, AgentStatusInfo{
			ID:           agent.ID,
			Task:         agent.Prompt,
			Status:       status,
			StartTime:    agent.StartTime,
			LastActive:   agent.StartTime, // Using StartTime as LastActive for now
			MessagesSent: 0,               // Not tracked in current implementation
			QueueStatus:  "running",
			IsStale:      false,
		})
	}

	return result
}

// GetAllAgentsStatus returns status of all agents
// Helper functions for new web interface
func listFilesNew(dir string) ([]FileInfo, error) {
	if dir == "" {
		dir = "."
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var result []FileInfo
	for _, file := range files {
		info, _ := file.Info()
		result = append(result, FileInfo{
			Name:  file.Name(),
			IsDir: file.IsDir(),
			Size:  info.Size(),
			Mode:  info.Mode().String(),
		})
	}
	return result, nil
}

type FileInfo struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
	Mode  string `json:"mode"`
}


func getAgentByID(agentID string) *AgentStatusInfo {
	agents := GetAllAgentsStatusJSON()
	for _, agent := range agents {
		if agent.ID == agentID {
			return &agent
		}
	}
	return nil
}

func getAgentStatus(agentID string) string {
	agent := getAgentByID(agentID)
	if agent == nil {
		return ""
	}

	var status strings.Builder
	status.WriteString(fmt.Sprintf("Agent ID: %s\n", agent.ID))
	status.WriteString(fmt.Sprintf("Task: %s\n", agent.Task))
	status.WriteString(fmt.Sprintf("Status: %s\n", agent.Status))
	status.WriteString(fmt.Sprintf("Started: %s\n", agent.StartTime.Format("15:04:05")))
	status.WriteString(fmt.Sprintf("Messages Sent: %d\n", agent.MessagesSent))

	if agent.IsStale {
		status.WriteString("\n⚠️  WARNING: This agent appears to be stale\n")
	}

	return status.String()
}

func stopAgent(agentID string) error {
	return StopAgent(agentID)
}

func createCodeAgent(task, workDir string) (string, error) {
	if workDir == "" {
		workDir = "."
	}

	// Use the agent manager to create the agent
	agentID, err := agentManager.LaunchAgent(context.Background(), workDir, task)
	if err != nil {
		return "", err
	}

	return agentID, nil
}

func gitCommit(message string) (string, error) {
	cmd := exec.Command("git", "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	return string(output), err
}



// getDiskUsage returns disk usage information for the current directory
func getDiskUsage() (uint64, error) {
	var stat syscall.Statfs_t
	wd, err := os.Getwd()
	if err != nil {
		return 0, err
	}

	err = syscall.Statfs(wd, &stat)
	if err != nil {
		return 0, err
	}

	// Calculate used space
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	return used, nil
}


// serveStatic serves static files
func serveStatic(w http.ResponseWriter, r *http.Request) {
	http.StripPrefix("/static/", http.FileServer(http.Dir("data/web/static"))).ServeHTTP(w, r)
}

// serveUploads serves uploaded files
func serveUploads(w http.ResponseWriter, r *http.Request) {
	http.StripPrefix("/uploads/", http.FileServer(http.Dir("data/uploads"))).ServeHTTP(w, r)
}

// handleWebDownload handles file downloads
func handleWebDownload(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path required", http.StatusBadRequest)
		return
	}

	// Security check - ensure path doesn't escape
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	http.ServeFile(w, r, cleanPath)
}

// handleWebAgents returns JSON list of agents
func handleWebAgents(w http.ResponseWriter, r *http.Request) {
	agents := GetAllAgentsStatusJSON()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

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
func CreateCodeAgent(ctx context.Context, workDir, task string, images []string) (string, error) {
	// Validate git repo
	if !isGitRepo(workDir) {
		return "", fmt.Errorf("not a git repository: %s", workDir)
	}

	// Check usage limits
	if err := checkUsageLimits(); err != nil {
		return "", err
	}

	// Append image paths to task if provided
	if len(images) > 0 {
		imageList := "\n\nImages to analyze:\n"
		for _, img := range images {
			imageList += fmt.Sprintf("- %s\n", img)
		}
		task += imageList + "\nPlease examine these images and incorporate them into your analysis."
	}

	// Launch agent
	agentID, err := agentManager.LaunchAgent(ctx, workDir, task)
	if err != nil {
		return "", err
	}

	// Check if agent was queued
	if strings.HasPrefix(agentID, "queued-") {
		// Parse queue information from the response
		parts := strings.Split(agentID, "-")
		var queuePos, queueID string
		for i := 0; i < len(parts); i++ {
			if parts[i] == "pos" && i+1 < len(parts) {
				queuePos = parts[i+1]
			} else if parts[i] == "qid" && i+1 < len(parts) {
				// The queue ID includes everything after "qid-"
				queueIDParts := []string{}
				for j := i + 1; j < len(parts); j++ {
					queueIDParts = append(queueIDParts, parts[j])
				}
				queueID = strings.Join(queueIDParts, "-")
				break
			}
		}

		// Register the queued agent for tracking
		if queueID != "" {
			queueTracker.RegisterQueuedAgent(queueID, AdminUserID, workDir, task)
		}

		queuedTasks := agentManager.GetQueuedTasksForFolder(workDir)

		// Broadcast SSE event for queue update
		BroadcastSSEEvent("queue_update", map[string]interface{}{
			"directory":      workDir,
			"queue_position": queuePos,
			"total_queued":   queuedTasks,
		})

		return agentID, nil
	}

	// Register the agent for user (only for non-queued agents)
	RegisterAgentForUser(agentID, AdminUserID)

	// Broadcast SSE event
	BroadcastSSEEvent("agent_started", map[string]interface{}{
		"agent_id":  agentID,
		"directory": workDir,
		"task":      task,
	})

	return agentID, nil
}

// CreateNewBranchAgent creates a new branch and launches agent
func CreateNewBranchAgent(ctx context.Context, workDir, task string, images []string) (string, error) {
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
	agentID, err := CreateCodeAgent(ctx, workDir, task, images)
	if err != nil {
		return "", err
	}

	return agentID, nil
}

// CreateEditBranchAgent checks out existing branch and launches agent
func CreateEditBranchAgent(ctx context.Context, workDir, branch, task string, images []string) (string, error) {
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
	return CreateCodeAgent(ctx, workDir, task, images)
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
