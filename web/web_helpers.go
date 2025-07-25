// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mavis/codeagent"
	"mavis/core"
)

var (
	// queueTracker is a reference to the global queue tracker from core package
	queueTracker = core.GetQueueTracker()
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
	Output       string
	Duration     time.Duration
	Error        string
	PlanContent  string
	Command      string
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

		// Get the full agent to access the command string
		var command string
		if fullAgent, err := agentManager.GetAgent(agent.ID); err == nil && fullAgent != nil {
			command = fullAgent.GetCommandString()
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
			Output:       agent.Output,
			Duration:     agent.Duration,
			Error:        agent.Error,
			PlanContent:  agent.PlanContent,
			Command:      command,
		})
	}

	// Add queued tasks
	queueStatus := agentManager.GetDetailedQueueStatus()
	for folder, tasks := range queueStatus {
		for i, task := range tasks {
			result = append(result, AgentStatusInfo{
				ID:           task.QueueID,
				Task:         task.Prompt,
				Status:       "queued",
				StartTime:    time.Now(), // Use current time as placeholder
				LastActive:   time.Now(),
				MessagesSent: 0,
				QueueStatus:  fmt.Sprintf("Position %d in %s", i+1, folder),
				IsStale:      false,
			})
		}
	}

	// Note: Preparing agents are temporary and not tracked in agentManager
	// They will be replaced by actual agents once preparation is complete

	return result
}

// GetAllAgentsStatus returns status of all agents
// Helper functions for new web interface
func listFilesNew(dir string) ([]FileInfo, error) {
	if dir == "" {
		dir = "."
	}

	// Use ResolvePath to ensure paths are resolved from home directory
	resolvedDir, err := ResolvePath(dir)
	if err != nil {
		return nil, err
	}

	files, err := os.ReadDir(resolvedDir)
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

// FileInfo is defined in files.go

func getAgentByID(agentID string) *AgentStatusInfo {
	agents := GetAllAgentsStatusJSON()
	for _, agent := range agents {
		if agent.ID == agentID {
			return &agent
		}
	}
	return nil
}

func getAgentProgress(agentID string) string {
	agent := getAgentByID(agentID)
	if agent == nil {
		return ""
	}

	// Check if this is a queued agent
	if agent.Status == "queued" {
		return fmt.Sprintf("Queued: %s", agent.QueueStatus)
	}

	// Get agent details from agent manager
	agentDetails, err := agentManager.GetAgent(agentID)
	if err != nil || agentDetails == nil {
		return ""
	}

	// Only look for progress in CURRENT_PLAN.md for running agents
	if agentDetails.Status == "running" {
		planPath := filepath.Join(agentDetails.Folder, "CURRENT_PLAN.md")
		if content, err := os.ReadFile(planPath); err == nil {
			// Extract only the progress section
			lines := strings.Split(string(content), "\n")
			inProgress := false
			var progressLines []string

			for _, line := range lines {
				if strings.HasPrefix(line, "## Progress") {
					inProgress = true
					continue
				} else if inProgress && strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "## Progress") {
					// End of progress section - only break on actual section headers (## followed by space)
					// not on subsections like ### or ####
					break
				}

				if inProgress && strings.TrimSpace(line) != "" {
					progressLines = append(progressLines, line)
				}
			}

			if len(progressLines) > 0 {
				return strings.Join(progressLines, "\n")
			}
		}
	}

	return ""
}

func getAgentPlan(agentID string) string {
	agent := getAgentByID(agentID)
	if agent == nil {
		return ""
	}

	// Get agent details from agent manager
	agentDetails, err := agentManager.GetAgent(agentID)
	if err != nil || agentDetails == nil {
		return ""
	}

	// Only look for plan in CURRENT_PLAN.md for running agents
	if agentDetails.Status == "running" {
		planPath := filepath.Join(agentDetails.Folder, "CURRENT_PLAN.md")
		if content, err := os.ReadFile(planPath); err == nil {
			// Extract only the plan section
			lines := strings.Split(string(content), "\n")
			inPlan := false
			var planLines []string

			for _, line := range lines {
				if strings.HasPrefix(line, "## Plan") {
					inPlan = true
					continue
				} else if inPlan && strings.HasPrefix(line, "## ") && !strings.HasPrefix(line, "## Plan") {
					// End of plan section - only break on actual section headers (## followed by space)
					// not on subsections like ### or ####
					break
				}

				if inPlan && strings.TrimSpace(line) != "" {
					planLines = append(planLines, line)
				}
			}

			if len(planLines) > 0 {
				return strings.Join(planLines, "\n")
			}
		}
	}

	return ""
}

func getAgentStatus(agentID string) string {
	agent := getAgentByID(agentID)
	if agent == nil {
		return ""
	}

	// Check if this is a queued agent
	if agent.Status == "queued" {
		var status strings.Builder
		status.WriteString(fmt.Sprintf("Agent ID: %s\n", agent.ID))
		status.WriteString(fmt.Sprintf("Task: %s\n", agent.Task))
		status.WriteString("Status: QUEUED\n")
		status.WriteString(fmt.Sprintf("Queue Status: %s\n", agent.QueueStatus))
		status.WriteString("\nThis agent is waiting for another agent to complete in the same directory.")
		return status.String()
	}

	// Get more detailed status from agent manager
	agentDetails, err := agentManager.GetAgent(agentID)
	if err != nil || agentDetails == nil {
		// Fall back to basic status
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

	var status strings.Builder
	status.WriteString(fmt.Sprintf("Agent ID: %s\n", agentDetails.ID))
	status.WriteString(fmt.Sprintf("Task: %s\n", agentDetails.Prompt))
	status.WriteString(fmt.Sprintf("Status: %s\n", agentDetails.Status))
	status.WriteString(fmt.Sprintf("Directory: %s\n", agentDetails.Folder))

	if !agentDetails.StartTime.IsZero() {
		status.WriteString(fmt.Sprintf("Started: %s\n", agentDetails.StartTime.Format("15:04:05")))
		status.WriteString(fmt.Sprintf("Runtime: %s\n", time.Since(agentDetails.StartTime).Round(time.Second).String()))
	}

	// Check for current plan
	if agentDetails.Status == "running" {
		planPath := filepath.Join(agentDetails.Folder, "CURRENT_PLAN.md")
		if content, err := os.ReadFile(planPath); err == nil {
			status.WriteString("\n--- Current Plan ---\n")
			status.WriteString(string(content))
		}
	}

	// Add output if available
	if agentDetails.Output != "" {
		status.WriteString("\n--- Output ---\n")
		status.WriteString(agentDetails.Output)
	}

	if agent.IsStale {
		status.WriteString("\n⚠️  WARNING: This agent appears to be stale\n")
	}

	return status.String()
}

func stopAgent(agentID string) error {
	return StopAgent(agentID)
}

func createCodeAgent(task, workDir string, selectedMCPs []string) (string, error) {
	if workDir == "" {
		workDir = "."
	}

	// Use ResolvePath to ensure paths are resolved from home directory
	absPath, err := ResolvePath(workDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	workDir = absPath

	// Create MCP config file if MCPs are selected
	var backupFile string
	if len(selectedMCPs) > 0 {
		// First verify MCP servers are available
		if err := VerifyMCPServers(selectedMCPs, mcpStore, workDir); err != nil {
			return "", fmt.Errorf("MCP server verification failed: %w", err)
		}

		backupFile, err = CreateMCPConfigFile(workDir, selectedMCPs, mcpStore)
		if err != nil {
			return "", fmt.Errorf("failed to create MCP config: %w", err)
		}
	}

	// Use the agent manager to create the agent
	agentID, err := agentManager.LaunchAgent(context.Background(), workDir, task)
	if err != nil {
		// Restore MCP config if agent launch failed
		if backupFile != "" {
			RestoreMCPConfigFile(workDir, backupFile)
		}
		return "", err
	}

	// Set up cleanup callback for when agent finishes
	// Always set callback if MCP config was created, even if no backup exists
	if len(selectedMCPs) > 0 {
		if agent, err := agentManager.GetAgent(agentID); err == nil && agent != nil {
			agent.SetCompletionCallback(func(a *codeagent.Agent) {
				// Always clean up MCP config, whether backup exists or not
				RestoreMCPConfigFile(workDir, backupFile)
			})
		}
	}

	// Send Telegram notification about the agent launch
	if b != nil && AdminUserID != 0 {
		message := fmt.Sprintf("🌐 Code agent launched from Web UI!\n🆔 ID: `%s`\n📝 Task: %s\n📁 Directory: %s\n\nUse `/status %s` to check status.",
			agentID, task, workDir, agentID)
		core.SendMessage(context.Background(), b, AdminUserID, message)
	}

	return agentID, nil
}

// serveStatic serves static files
func serveStatic(w http.ResponseWriter, r *http.Request) {
	// Get the file path
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	fullPath := filepath.Join(ProjectDir, "web/static", path)

	// Set proper content type based on file extension
	ext := filepath.Ext(path)
	switch ext {
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".html":
		w.Header().Set("Content-Type", "text/html")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".ico":
		w.Header().Set("Content-Type", "image/x-icon")
	}

	// Serve the file
	http.ServeFile(w, r, fullPath)
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
	// Use ResolvePath to ensure paths are resolved from home directory
	absPath, err := ResolvePath(workDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	workDir = absPath

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
		var queueID string
		for i := 0; i < len(parts); i++ {
			if parts[i] == "qid" && i+1 < len(parts) {
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

		// SSE removed - using meta refresh instead
		// BroadcastSSEEvent("queue_update", map[string]interface{}{
		// 	"directory":      workDir,
		// 	"queue_position": queuePos,
		// 	"total_queued":   queuedTasks,
		// })

		return agentID, nil
	}

	// Register the agent for user (only for non-queued agents)
	RegisterAgentForUser(agentID, AdminUserID)

	// SSE removed - using meta refresh instead
	// BroadcastSSEEvent("agent_started", map[string]interface{}{
	// 	"agent_id":  agentID,
	// 	"directory": workDir,
	// 	"task":      task,
	// })

	// Send Telegram notification about the agent launch
	if b != nil && AdminUserID != 0 {
		message := fmt.Sprintf("🌐 Code agent launched from Web UI!\n🆔 ID: `%s`\n📝 Task: %s\n📁 Directory: %s\n\nUse `/status %s` to check status.",
			agentID, task, workDir, agentID)
		core.SendMessage(ctx, b, AdminUserID, message)
	}

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
	// Use ResolvePath to ensure paths are resolved from home directory
	resolvedPath, err := ResolvePath(path)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return "", err
	}

	var cmd *exec.Cmd
	if info.IsDir() {
		// Check if it's a git repo
		if !isGitRepo(resolvedPath) {
			return "", fmt.Errorf("not a git repository")
		}
		cmd = exec.Command("git", "diff", "HEAD")
		cmd.Dir = resolvedPath
	} else {
		// Single file
		dir := filepath.Dir(resolvedPath)
		if !isGitRepo(dir) {
			return "", fmt.Errorf("not in a git repository")
		}
		relPath, _ := filepath.Rel(dir, resolvedPath)
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

// ResolvePath is a wrapper around core.ResolvePath for the web package
func ResolvePath(path string) (string, error) {
	return core.ResolvePath(path)
}

// launchCommitAgent creates an agent to handle git commits
func launchCommitAgent(ctx context.Context, folder string) {
	// Create the task for the commit agent
	task := "Please analyze the git diff and create an appropriate commit with a descriptive message. Use 'git add' to stage any unstaged changes, then create the commit."

	// Launch the agent using the agent manager
	agentID, err := agentManager.LaunchAgent(ctx, folder, task)
	if err != nil {
		// Log the error but don't block the response
		fmt.Printf("Error launching commit agent: %v\n", err)
		return
	}

	fmt.Printf("Launched commit agent with ID: %s for folder: %s\n", agentID, folder)
}

func launchPRCreateAgent(ctx context.Context, folder, branch, title, body, base string) {
	// Create the task for the PR creation agent
	task := fmt.Sprintf(`Please create a pull request with the following details:
- Branch: %s
- Base Branch: %s
- Title: %s
- Description: %s

First, ensure you're on the correct branch and push it to the remote if needed. Then use 'gh pr create' to create the pull request.`, branch, base, title, body)

	// Launch the agent using the agent manager
	agentID, err := agentManager.LaunchAgent(ctx, folder, task)
	if err != nil {
		// Log the error but don't block the response
		fmt.Printf("Error launching PR create agent: %v\n", err)
		return
	}

	fmt.Printf("Launched PR create agent with ID: %s for folder: %s\n", agentID, folder)
}

func launchPRReviewAgent(ctx context.Context, folder, prURL, action string) {
	// Create the task for the PR review agent based on action
	var task string
	switch action {
	case "review":
		task = fmt.Sprintf(`IMPORTANT PR REVIEW INSTRUCTIONS:
You are tasked with reviewing a pull request. Follow these steps carefully:

1. First, use the gh CLI to get information about the PR: %s
   - Get PR details: gh pr view %s
   - Get PR diff: gh pr diff %s
   - Get PR checks status: gh pr checks %s

2. Analyze the code changes:
   - Look for potential bugs, security issues, or performance problems
   - Check if the implementation aligns with the requirements
   - Verify that the code follows project conventions and best practices
   - Check for missing tests or documentation

4. Write a concise PR review:
   - DO NOT use gh pr comment or post anything to the PR
   - Structure your review as follows:
     * List any bugs or issues found (if any)
     * Code improvement suggestions (following the project conventions and best practices) (if any)
     * Final verdict: Approve, Request Changes, or Needs Discussion

Remember:
- Be concise
- Point out specific line numbers or files when mentioning issues
- If everything looks good, don't say anything
- Send your review message directly to the output

PR URL: %s`, prURL, prURL, prURL, prURL, prURL)
	case "approve":
		task = fmt.Sprintf(`IMPORTANT PR REVIEW AND APPROVAL INSTRUCTIONS:
You must review this pull request and then approve it using the gh CLI.

1. Get PR information:
   - gh pr view %s
   - gh pr diff %s
   - gh pr checks %s

2. Analyze the code for issues:
   - Bugs, security vulnerabilities, logic errors
   - Breaking changes or regressions
   - Missing critical tests for new functionality
   - Verify that the code follows project conventions and best practices
   - Check for missing tests
   
   DO NOT comment on:
   - Code style preferences
   - Minor improvements that work correctly
   - Things that are already good

3. Post your review using EXACTLY ONE command:
   
   If NO ISSUES found:
   gh pr review %s --approve --body "LGTM ✅"
   
   If ISSUES found (but still approve):
   gh pr review %s --approve --body "LGTM ✅

However, I found the following issues that should be addressed in a follow-up:
- Issue 1: [specific problem at file:line]
- Issue 2: [specific problem at file:line]"

CRITICAL RULES:
- You MUST always use the --approve flag regardless of issues found
- ONLY list actual problems that need fixing
- Execute ONLY ONE gh pr review command
- DO NOT run the command twice
- Always approve even if there are issues

PR URL: %s`, prURL, prURL, prURL, prURL, prURL, prURL)
	case "request-changes":
		task = fmt.Sprintf(`IMPORTANT PR REVIEW AND COMMENT INSTRUCTIONS:
You must review this pull request and post your review using the gh CLI.

1. Get PR information:
   - gh pr view %s
   - gh pr diff %s
   - gh pr checks %s

2. Analyze the code for issues ONLY:
   - Bugs, security vulnerabilities, logic errors
   - Breaking changes or regressions
   - Missing critical tests for new functionality
   - Verify that the code follows project conventions and best practices
   - Check for missing tests
   
   DO NOT comment on:
   - Code style preferences
   - Minor improvements that work correctly
   - Things that are already good

3. Post your review using EXACTLY ONE command:
   
   If NO ISSUES found:
   gh pr review %s --approve --body "LGTM"
   
   If ISSUES found:
   gh pr review %s --request-changes --body "- Issue 1: [specific problem at file:line]
- Issue 2: [specific problem at file:line]"

CRITICAL RULES:
- ONLY list actual problems that need fixing
- NO summaries, NO strengths, NO general observations
- If code works correctly, just approve with "LGTM"
- Execute ONLY ONE gh pr review command
- DO NOT run the command twice

PR URL: %s`, prURL, prURL, prURL, prURL, prURL, prURL)
	default:
		task = fmt.Sprintf("Please review the pull request at %s and provide feedback.", prURL)
	}

	// Launch the agent using the agent manager
	agentID, err := agentManager.LaunchAgent(ctx, folder, task)
	if err != nil {
		// Log the error but don't block the response
		fmt.Printf("Error launching PR review agent: %v\n", err)
		return
	}

	fmt.Printf("Launched PR review agent with ID: %s for folder: %s\n", agentID, folder)
}

// RegisterAgentForUser registers an agent for a user (no-op in single-user mode)
func RegisterAgentForUser(agentID string, userID int64) {
	// No-op in single-user mode - just log for debugging
	fmt.Printf("Agent %s started (single-user mode)\n", agentID)
}

// checkBranchExists checks if a branch exists locally or remotely
func checkBranchExists(workDir, branch string) (bool, error) {
	// Check if the branch exists locally
	cmd := exec.Command("git", "branch", "--list", branch)
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to check local branches: %v", err)
	}

	// Check if branch exists locally
	if len(strings.TrimSpace(string(output))) > 0 {
		return true, nil
	}

	// Check remote branches
	cmd = exec.Command("git", "branch", "-r", "--list", fmt.Sprintf("origin/%s", branch))
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to check remote branches: %v", err)
	}

	// Return true if branch exists remotely
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// createAgentWithBranch creates an agent with appropriate git behavior based on branch parameter
func createAgentWithBranch(task, workDir, branch string, selectedMCPs []string) (string, error) {
	if workDir == "" {
		workDir = "."
	}

	// Use ResolvePath to ensure paths are resolved from home directory
	absPath, err := ResolvePath(workDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	workDir = absPath

	// If no branch specified, use default code behavior
	if branch == "" {
		return createCodeAgent(task, workDir, selectedMCPs)
	}

	// Check if it's a git repository
	if !isGitRepo(workDir) {
		return "", fmt.Errorf("branch specified but directory is not a git repository: %s", workDir)
	}

	// Generate a temporary agent ID for tracking
	tempAgentID := fmt.Sprintf("preparing-%s-%d", strings.ReplaceAll(branch, "/", "-"), time.Now().Unix())

	// Launch background goroutine to handle git operations
	go func(selectedMCPs []string) {
		// Create a temp directory for git operations
		tempDir, err := os.MkdirTemp("", "git-agent-*")
		if err != nil {
			log.Printf("Failed to create temp directory: %v", err)
			// Send error notification if possible
			if b != nil && AdminUserID != 0 {
				message := fmt.Sprintf("❌ Failed to prepare git agent:\n%v", err)
				core.SendMessage(context.Background(), b, AdminUserID, message)
			}
			return
		}

		// Copy the repository to temp directory
		cmd := exec.Command("rsync", "-av", "--exclude=node_modules", "--exclude=.DS_Store", workDir+"/", tempDir+"/")
		output, err := cmd.CombinedOutput()
		if err != nil {
			os.RemoveAll(tempDir)
			log.Printf("Failed to copy repository: %v\nOutput: %s", err, string(output))
			// Send error notification if possible
			if b != nil && AdminUserID != 0 {
				message := fmt.Sprintf("❌ Failed to prepare git agent:\nFailed to copy repository: %v", err)
				core.SendMessage(context.Background(), b, AdminUserID, message)
			}
			return
		}

		// Check if branch exists
		branchExists, err := checkBranchExists(tempDir, branch)
		if err != nil {
			os.RemoveAll(tempDir)
			log.Printf("Failed to check branch: %v", err)
			// Send error notification if possible
			if b != nil && AdminUserID != 0 {
				message := fmt.Sprintf("❌ Failed to prepare git agent:\nFailed to check branch: %v", err)
				core.SendMessage(context.Background(), b, AdminUserID, message)
			}
			return
		}

		var gitPrompt string
		if branchExists {
			// Use edit_branch behavior
			gitPrompt = fmt.Sprintf(`IMPORTANT GIT WORKFLOW INSTRUCTIONS:
You are working on a git repository with an existing branch. You MUST follow these steps:

1. First, fetch the latest changes: git fetch origin
2. Checkout the existing branch '%s' using: git checkout %s
3. If the branch exists only on remote, use: git checkout -b %s origin/%s
4. Pull the latest changes from the remote branch: git pull origin %s
5. Make all the necessary changes to complete the task: %s
6. Stage and commit your changes with a descriptive commit message
   IMPORTANT: When staging files, NEVER include *_PLAN_*.md files. Use commands like:
   - git add . && git reset *_PLAN_*.md  (to add all except plan files)
   - Or stage files individually, explicitly excluding *_PLAN_*.md files
7. Try to push the changes to the remote repository using: git push origin %s
8. If the push fails due to authentication or permissions, that's okay - just report the status

Remember:
- You are working on the existing branch '%s'
- Make atomic, well-described commits
- Include a clear commit message explaining what was changed and why
- Ensure you're up to date with the remote branch before making changes
- NEVER commit *_PLAN_*.md files - they're for your planning only

Task: %s`, branch, branch, branch, branch, branch, task, branch, branch, task)
		} else {
			// Use new_branch behavior - create feature branch
			branchName := branch
			if !strings.HasPrefix(branch, "feature/") {
				branchName = fmt.Sprintf("feature/%s", branch)
			}

			gitPrompt = fmt.Sprintf(`IMPORTANT GIT WORKFLOW INSTRUCTIONS:
You are working on a git repository. You MUST follow these steps:

1. First, create a new branch for your changes using: git checkout -b %s
2. Make all the necessary changes to complete the task: %s
3. Stage and commit your changes with a descriptive commit message
   IMPORTANT: When staging files, NEVER include *_PLAN_*.md files. Use commands like:
   - git add . && git reset *_PLAN_*.md  (to add all except plan files)
   - Or stage files individually, explicitly excluding *_PLAN_*.md files
4. Try to push the branch to the remote repository using: git push -u origin %s
5. If the push fails due to authentication or permissions, that's okay - just report the status

Remember:
- Always work on the new branch '%s', never directly on main/master
- Make atomic, well-described commits
- Include a clear commit message explaining what was changed and why
- NEVER commit *_PLAN_*.md files - they're for your planning only

Task: %s`, branchName, task, branchName, branchName, task)
		}

		// Create MCP config file if MCPs are selected
		var backupFile string
		if len(selectedMCPs) > 0 {
			// First verify MCP servers are available
			if err := VerifyMCPServers(selectedMCPs, mcpStore, tempDir); err != nil {
				os.RemoveAll(tempDir)
				log.Printf("MCP server verification failed: %v", err)
				// Send error notification if possible
				if b != nil && AdminUserID != 0 {
					message := fmt.Sprintf("❌ MCP server verification failed:\n%v", err)
					core.SendMessage(context.Background(), b, AdminUserID, message)
				}
				return
			}

			backupFile, err = CreateMCPConfigFile(tempDir, selectedMCPs, mcpStore)
			if err != nil {
				os.RemoveAll(tempDir)
				log.Printf("Failed to create MCP config: %v", err)
				// Send error notification if possible
				if b != nil && AdminUserID != 0 {
					message := fmt.Sprintf("❌ Failed to create MCP config:\n%v", err)
					core.SendMessage(context.Background(), b, AdminUserID, message)
				}
				return
			}
		}

		// Launch the agent with the git-specific prompt
		agentID, err := agentManager.LaunchAgent(context.Background(), tempDir, gitPrompt)
		if err != nil {
			// Restore MCP config if needed
			if backupFile != "" {
				RestoreMCPConfigFile(tempDir, backupFile)
			}
			os.RemoveAll(tempDir)
			log.Printf("Failed to launch agent: %v", err)
			// Send error notification if possible
			if b != nil && AdminUserID != 0 {
				message := fmt.Sprintf("❌ Failed to launch git agent:\n%v", err)
				core.SendMessage(context.Background(), b, AdminUserID, message)
			}
			return
		}

		// Set up cleanup callback for when agent finishes
		if agent, err := agentManager.GetAgent(agentID); err == nil && agent != nil {
			agent.SetCompletionCallback(func(a *codeagent.Agent) {
				// Always clean up MCP config, whether backup exists or not
				if len(selectedMCPs) > 0 {
					RestoreMCPConfigFile(tempDir, backupFile)
				}
				// Note: tempDir cleanup is handled elsewhere
			})
		}

		// Send success notification
		if b != nil && AdminUserID != 0 {
			behaviorType := "edit_branch"
			if !branchExists {
				behaviorType = "new_branch"
			}
			message := fmt.Sprintf("✅ Git-aware code agent successfully launched!\n🆔 ID: `%s`\n📝 Task: %s\n🌿 Branch: %s (%s)\n📁 Original: %s\n📁 Workspace: %s\n\nUse `/status %s` to check status.",
				agentID, task, branch, behaviorType, workDir, tempDir, agentID)
			core.SendMessage(context.Background(), b, AdminUserID, message)
		}
	}(selectedMCPs)

	// Return immediately with a status indicating the agent is being prepared
	return tempAgentID, nil
}

// listGitBranches returns a list of all branches (local and remote) in a git repository
func listGitBranches(workDir string) ([]string, error) {
	branches := []string{}

	// Get local branches
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list local branches: %v", err)
	}

	// Parse local branches
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		branch := strings.TrimSpace(line)
		if branch != "" {
			branches = append(branches, branch)
		}
	}

	// Get remote branches
	cmd = exec.Command("git", "branch", "-r", "--format=%(refname:short)")
	cmd.Dir = workDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		// If remote fetch fails, just return local branches
		return branches, nil
	}

	// Parse remote branches and add unique ones
	branchMap := make(map[string]bool)
	for _, branch := range branches {
		branchMap[branch] = true
	}

	lines = strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		branch := strings.TrimSpace(line)
		// Remove "origin/" prefix if present
		branch = strings.TrimPrefix(branch, "origin/")
		// Skip HEAD reference
		if branch != "" && branch != "HEAD" && !branchMap[branch] {
			branches = append(branches, branch)
			branchMap[branch] = true
		}
	}

	return branches, nil
}
