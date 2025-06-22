// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
)

// generateUniquePlanFilename creates a unique plan filename for different command types
func generateUniquePlanFilename(commandType string) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_PLAN_%d.md", strings.ToUpper(commandType), timestamp)
}

func handleMessage(ctx context.Context, message *models.Message) {
	// Handle Telegram commands for code agents
	if strings.HasPrefix(message.Text, "/") {
		// Check if it's a command
		parts := strings.Fields(message.Text)
		if len(parts) > 0 {
			command := parts[0]
			switch command {
			case "/help":
				handleHelpCommand(ctx, message)
				return
			case "/code":
				handleCodeCommand(ctx, message)
				return
			case "/ps":
				handleAgentsCommand(ctx, message)
				return
			case "/status":
				handleStatusCommand(ctx, message)
				return
			case "/stop":
				// Check if it's the LAN stop command or agent stop command
				if len(parts) == 1 {
					// No arguments, it's LAN stop
					handleStopLANCommand(ctx, message)
				} else {
					// Has arguments, it's agent stop
					handleStopCommand(ctx, message)
				}
				return
			case "/start":
				handleStartCommand(ctx, message)
				return
			case "/new_branch":
				handleGitCodeCommand(ctx, message)
				return
			case "/edit_branch":
				handleGitBranchCommand(ctx, message)
				return
			case "/review":
				handleReviewCommand(ctx, message)
				return
			case "/pr":
				handlePRCommand(ctx, message)
				return
			case "/restart":
				handleRestartCommand(ctx, message)
				return
			case "/download":
				handleDownloadCommand(ctx, message)
				return
			case "/ls":
				handleLsCommand(ctx, message)
				return
			case "/mkdir":
				handleMkdirCommand(ctx, message)
				return
			case "/adduser":
				handleAddUserCommand(ctx, message)
				return
			case "/removeuser":
				handleRemoveUserCommand(ctx, message)
				return
			case "/users":
				handleUsersCommand(ctx, message)
				return
			case "/commit":
				handleCommitCommand(ctx, message)
				return
			case "/serve":
				handleServeCommand(ctx, message)
				return
			case "/diff":
				handleDiffCommand(ctx, message)
				return
			case "/run":
				handleRunCommand(ctx, message)
				return
			case "/images":
				handleImagesCommand(ctx, message)
				return
			case "/clear_images":
				handleClearImagesCommand(ctx, message)
				return
			case "/cleanup":
				handleCleanupCommand(ctx, message)
				return
			}
		}
		return
	}

	// For non-command messages, just show help
	SendMessage(ctx, b, message.Chat.ID, "I'm Mavis, a code agent manager. Use /help to see available commands.")
}

func launchCodeAgentCommand(ctx context.Context, chatID int64, directory, task string) {
	// Use AdminUserID for single-user app
	chatID = AdminUserID
	// Resolve the directory path relative to home directory
	absDir, err := ResolvePath(directory)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check for pending images
	pendingImages := getPendingImages(AdminUserID)
	if len(pendingImages) > 0 {
		// Append image information to the task
		task += fmt.Sprintf("\n\nThe user has provided %d image(s) for this task:", len(pendingImages))
		for i, imagePath := range pendingImages {
			task += fmt.Sprintf("\n- Image %d: %s", i+1, imagePath)
		}
		task += "\n\nPlease analyze these images as part of the task. You can read them using the Read tool."

		SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching code agent in %s...\n📸 Including %d pending image(s)", absDir, len(pendingImages)))
	} else {
		SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching code agent in %s...", absDir))
	}

	// Launch the agent
	agentID, err := agentManager.LaunchAgent(ctx, absDir, task)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Check if the agent was queued
	if strings.HasPrefix(agentID, "queued-") {
		// Extract queue position and queue ID from the ID
		parts := strings.Split(agentID, "-")
		var queuePos, queueID string
		for i := 0; i < len(parts); i++ {
			if parts[i] == "pos" && i+1 < len(parts) {
				queuePos = parts[i+1]
			} else if parts[i] == "qid" && i+1 < len(parts) {
				queueID = parts[i+1]
			}
		}

		// Register the queued agent for tracking
		if queueID != "" {
			queueTracker.RegisterQueuedAgent(queueID, AdminUserID, absDir, task)
		}

		queuedTasks := agentManager.GetQueuedTasksForFolder(absDir)
		SendMessage(ctx, b, chatID, fmt.Sprintf("⏳ Agent queued!\n📁 Directory: %s\n📝 Task: %s\n🔢 Queue position: %s\n📊 Total queued tasks for this folder: %d\n\nThe agent will start automatically when the current agent in this folder completes.",
			directory, task, queuePos, queuedTasks))

		// Clear pending images even for queued agents
		if len(pendingImages) > 0 {
			clearPendingImages(AdminUserID)
		}
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	// Clear pending images after using them
	if len(pendingImages) > 0 {
		clearPendingImages(AdminUserID)
	}

	SendMessage(ctx, b, chatID, fmt.Sprintf("✅ Code agent launched!\n🆔 ID: `%s`\n📝 Task: %s\n📁 Directory: %s\n\nUse `/status %s` to check status.",
		agentID, task, directory, agentID))
}

func listCodeAgentsCommand(ctx context.Context, chatID int64) {
	agents := agentManager.ListAgents()

	if len(agents) == 0 {
		SendMessage(ctx, b, chatID, "📋 No code agents running.")
		return
	}

	message := "📋 *Code Agents:*\n\n"
	for _, agent := range agents {
		status := "⏳"
		switch agent.Status {
		case "running":
			status = "🟢"
		case "finished":
			status = "✅"
		case "failed":
			status = "❌"
		case "killed":
			status = "🔴"
		}

		message += fmt.Sprintf("%s `%s` - %s\n", status, agent.ID, string(agent.Status))
		if agent.Folder != "" {
			message += fmt.Sprintf("   📁 %s\n", agent.Folder)
		}
		if agent.Prompt != "" {
			// Truncate prompt if too long
			prompt := agent.Prompt
			if len(prompt) > 50 {
				prompt = prompt[:50] + "..."
			}
			message += fmt.Sprintf("   📝 %s\n", prompt)
		}
		message += "\n"
	}

	// Add detailed queue status
	detailedQueueStatus := agentManager.GetDetailedQueueStatus()
	if len(detailedQueueStatus) > 0 {
		message += "\n📊 *Queued Tasks:*\n"
		for folder, tasks := range detailedQueueStatus {
			message += fmt.Sprintf("\n📁 *%s* (%d tasks):\n", folder, len(tasks))
			for i, task := range tasks {
				// Truncate prompt if too long
				prompt := task.Prompt
				if len(prompt) > 60 {
					prompt = prompt[:60] + "..."
				}
				message += fmt.Sprintf("   %d. 📝 %s\n", i+1, prompt)
				message += fmt.Sprintf("      🆔 Queue ID: %s\n", task.QueueID)
			}
		}
	}

	SendMessage(ctx, b, chatID, message)
}

func getCodeAgentDetailsCommand(ctx context.Context, chatID int64, agentID string) {
	agentInfo, err := agentManager.GetAgentInfo(agentID)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Agent not found: %s", agentID))
		return
	}

	status := "⏳"
	switch agentInfo.Status {
	case "running":
		status = "🟢 Running"
	case "finished":
		status = "✅ Finished"
	case "failed":
		status = "❌ Failed"
	case "killed":
		status = "🔴 Killed"
	default:
		status = "⏳ " + string(agentInfo.Status)
	}

	message := fmt.Sprintf("*Code Agent Details*\n\n🆔 ID: `%s`\n📊 Status: %s\n📝 Task: %s\n📁 Directory: %s\n🕐 Started: %s\n",
		agentInfo.ID, status, agentInfo.Prompt, agentInfo.Folder, agentInfo.StartTime.Format("15:04:05"))

	if !agentInfo.EndTime.IsZero() {
		message += fmt.Sprintf("🏁 Ended: %s\n", agentInfo.EndTime.Format("15:04:05"))
		message += fmt.Sprintf("⏱️ Duration: %s\n", agentInfo.Duration.Round(time.Second))
	}

	// Add full output if available
	if agentInfo.Output != "" {
		message += fmt.Sprintf("\n📄 *Output:*\n```\n%s\n```", agentInfo.Output)
	}

	// Add full error if available
	if agentInfo.Error != "" {
		message += fmt.Sprintf("\n❌ *Error:*\n```\n%s\n```", agentInfo.Error)
	}

	// Use SendLongMessage to handle potentially long output
	SendLongMessage(ctx, b, chatID, message)
}

func killCodeAgentCommand(ctx context.Context, chatID int64, agentID string) {
	err := agentManager.KillAgent(agentID)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to stop agent: %v", err))
		return
	}

	SendMessage(ctx, b, chatID, fmt.Sprintf("🔴 Agent %s has been stopped.", agentID))
}

func handleDownloadCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Please provide a file path.\nUsage: /download <path>\n\nExample: /download ~/Downloads/app.apk")
		return
	}

	// Join all parts after the command in case the path has spaces
	path := strings.Join(parts[1:], " ")

	// Resolve the path relative to home directory
	absPath, err := ResolvePath(path)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error resolving path: %v", err))
		return
	}

	// Check if file exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ File not found: %s", absPath))
		} else {
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error accessing file: %v", err))
		}
		return
	}

	// Check if it's a file (not a directory)
	if info.IsDir() {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Path is a directory, not a file: %s", absPath))
		return
	}

	// Check file size (Telegram has a 50MB limit for bots)
	const maxFileSize = 50 * 1024 * 1024 // 50MB
	if info.Size() > maxFileSize {
		// Inform user about the file size limitation
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ File too large: %s (%.2f MB)\n\n📋 *File Size Limitations:*\n• Standard Bot API: 50MB maximum\n• Self-hosted Bot API Server: 2GB maximum\n\n💡 *Solution:* To send files up to 2GB, you need to set up a self-hosted Telegram Bot API server.\nLearn more: https://github.com/tdlib/telegram-bot-api",
			info.Name(), float64(info.Size())/(1024*1024)))
		return
	}

	// Prepare caption with sending status
	caption := fmt.Sprintf("📤 *Sending file:* `%s`\n📏 *Size:* %.2f MB\n📍 *Path:* `%s`",
		info.Name(), float64(info.Size())/(1024*1024), path)

	err = SendFile(ctx, b, message.Chat.ID, absPath, caption)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to send file: %v", err))
		return
	}
}

func handleLsCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	path := "~" // Default to home directory

	if len(parts) >= 2 {
		// Join all parts after the command in case the path has spaces
		path = strings.Join(parts[1:], " ")
	}

	// Resolve the path relative to home directory
	absPath, err := ResolvePath(path)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error resolving path: %v", err))
		return
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Path not found: %s", absPath))
		} else {
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error accessing path: %v", err))
		}
		return
	}

	// If it's a file, show file info
	if !info.IsDir() {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("📄 *File:* `%s`\n📏 *Size:* %.2f KB\n📅 *Modified:* %s",
			info.Name(), float64(info.Size())/1024, info.ModTime().Format("2006-01-02 15:04:05")))
		return
	}

	// List directory contents
	entries, err := os.ReadDir(absPath)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error reading directory: %v", err))
		return
	}

	if len(entries) == 0 {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("📁 *Directory:* `%s`\n\n(empty)", path))
		return
	}

	// Build directory listing
	var dirs []string
	var files []string

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if entry.IsDir() {
			dirs = append(dirs, fmt.Sprintf("📁 `%s/`", entry.Name()))
		} else {
			size := float64(info.Size()) / 1024
			sizeUnit := "KB"
			if size > 1024 {
				size = size / 1024
				sizeUnit = "MB"
			}
			files = append(files, fmt.Sprintf("📄 `%s` (%.1f %s)", entry.Name(), size, sizeUnit))
		}
	}

	responseMsg := fmt.Sprintf("📁 *Directory:* `%s`\n\n", path)

	if len(dirs) > 0 {
		responseMsg += "*Directories:*\n"
		for _, dir := range dirs {
			responseMsg += dir + "\n"
		}
		responseMsg += "\n"
	}

	if len(files) > 0 {
		responseMsg += "*Files:*\n"
		for _, file := range files {
			responseMsg += file + "\n"
		}
	}

	// Truncate if message is too long
	if len(responseMsg) > 4000 {
		responseMsg = responseMsg[:3997] + "..."
	}

	SendMessage(ctx, b, message.Chat.ID, responseMsg)
}

func handleMkdirCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Please provide a directory path.\nUsage: /mkdir <path>\n\nExample: /mkdir ~/projects/newapp")
		return
	}

	// Join all parts after the command in case the path has spaces
	path := strings.Join(parts[1:], " ")

	// Resolve the path relative to home directory
	absPath, err := ResolvePath(path)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error resolving path: %v", err))
		return
	}

	// Check if path already exists
	_, err = os.Stat(absPath)
	if err == nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Path already exists: %s", absPath))
		return
	}

	// Create the directory (including parent directories)
	err = os.MkdirAll(absPath, 0755)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error creating directory: %v", err))
		return
	}

	SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ Directory created: `%s`", path))
}

func handleGitCodeCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 3 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Please provide directory and task.\nUsage: /new_branch <directory> <task>\n\nExample: /new_branch ~/myproject implement new feature")
		return
	}

	directory := strings.TrimSpace(parts[1])
	task := strings.TrimSpace(strings.Join(parts[2:], " "))

	if directory == "" || task == "" {
		SendMessage(ctx, b, message.Chat.ID, "❌ Both directory and task are required.\n\nExample: /new_branch ~/myproject implement new feature")
		return
	}

	launchGitCodeAgent(ctx, message.Chat.ID, directory, task)
}

func launchGitCodeAgent(ctx context.Context, chatID int64, directory, task string) {
	// Resolve the directory path relative to home directory
	absDir, err := ResolvePath(directory)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	SendMessage(ctx, b, chatID, fmt.Sprintf("🔍 Checking git repository status in %s...", absDir))

	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "git-agent-*")
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to create temp directory: %v", err))
		return
	}

	// Copy the repository to temp directory
	SendMessage(ctx, b, chatID, "📋 Copying repository to temporary workspace...")

	// Use rsync to copy the directory, excluding .git if needed
	cmd := exec.Command("rsync", "-av", "--exclude=node_modules", "--exclude=.DS_Store", absDir+"/", tempDir+"/")
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tempDir)
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to copy repository: %v\nOutput: %s", err, string(output)))
		return
	}

	// Prepare the git-specific prompt
	gitPrompt := fmt.Sprintf(`IMPORTANT GIT WORKFLOW INSTRUCTIONS:
You are working on a git repository. You MUST follow these steps:

1. First, create a new branch for your changes using: git checkout -b feature/<descriptive-name>
2. Make all the necessary changes to complete the task: %s
3. Stage and commit your changes with a descriptive commit message
   IMPORTANT: When staging files, NEVER include *_PLAN_*.md files. Use commands like:
   - git add . && git reset *_PLAN_*.md  (to add all except plan files)
   - Or stage files individually, explicitly excluding *_PLAN_*.md files
4. Try to push the branch to the remote repository using: git push -u origin <branch-name>
5. If the push fails due to authentication or permissions, that's okay - just report the status

Remember:
- Always work on a new branch, never directly on main/master
- Make atomic, well-described commits
- Include a clear commit message explaining what was changed and why
- NEVER commit *_PLAN_*.md files - they're for your planning only

Task: %s`, task, task)

	SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching git-aware code agent in temporary workspace...\n📁 Original: %s\n📁 Workspace: %s", absDir, tempDir))

	// Launch the agent with the git-specific prompt
	agentID, err := agentManager.LaunchAgent(ctx, tempDir, gitPrompt)
	if err != nil {
		os.RemoveAll(tempDir)
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	SendMessage(ctx, b, chatID, fmt.Sprintf("✅ Git-aware code agent launched!\n🆔 ID: `%s`\n📝 Task: %s\n📁 Original Directory: %s\n📁 Working Directory: %s\n🌿 The agent will create a new branch and attempt to push changes\n\nUse `/status %s` to check status.",
		agentID, task, directory, tempDir, agentID))
}

func handleGitBranchCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 4 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Please provide directory, branch name, and task.\nUsage: /edit_branch <directory> <branch> <task>\n\nExample: /edit_branch ~/myproject feature/add-auth implement authentication system")
		return
	}

	directory := strings.TrimSpace(parts[1])
	branch := strings.TrimSpace(parts[2])
	task := strings.TrimSpace(strings.Join(parts[3:], " "))

	if directory == "" || branch == "" || task == "" {
		SendMessage(ctx, b, message.Chat.ID, "❌ Directory, branch name, and task are all required.\n\nExample: /edit_branch ~/myproject feature/add-auth implement authentication system")
		return
	}

	launchGitBranchAgent(ctx, message.Chat.ID, directory, branch, task)
}

func launchGitBranchAgent(ctx context.Context, chatID int64, directory, branch, task string) {
	// Resolve the directory path relative to home directory
	absDir, err := ResolvePath(directory)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	SendMessage(ctx, b, chatID, fmt.Sprintf("🔍 Checking git repository and branch status in %s...", absDir))

	// Check if the branch exists
	checkBranchCmd := exec.Command("git", "branch", "--list", branch)
	checkBranchCmd.Dir = absDir
	branchOutput, _ := checkBranchCmd.CombinedOutput()

	// Check remote branches too
	checkRemoteBranchCmd := exec.Command("git", "branch", "-r", "--list", fmt.Sprintf("origin/%s", branch))
	checkRemoteBranchCmd.Dir = absDir
	remoteBranchOutput, _ := checkRemoteBranchCmd.CombinedOutput()

	branchExists := len(strings.TrimSpace(string(branchOutput))) > 0 || len(strings.TrimSpace(string(remoteBranchOutput))) > 0

	if !branchExists {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Branch '%s' does not exist locally or remotely.\n\n💡 Use `/new_branch` to create a new branch, or check the branch name and try again.", branch))
		return
	}

	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "git-branch-agent-*")
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to create temp directory: %v", err))
		return
	}

	// Copy the repository to temp directory
	SendMessage(ctx, b, chatID, "📋 Copying repository to temporary workspace...")

	// Use rsync to copy the directory, excluding .git if needed
	cmd := exec.Command("rsync", "-av", "--exclude=node_modules", "--exclude=.DS_Store", absDir+"/", tempDir+"/")
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tempDir)
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to copy repository: %v\nOutput: %s", err, string(output)))
		return
	}

	// Prepare the git-specific prompt for existing branch
	gitBranchPrompt := fmt.Sprintf(`IMPORTANT GIT WORKFLOW INSTRUCTIONS:
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

	SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching git-aware code agent for existing branch...\n📁 Original: %s\n📁 Workspace: %s\n🌿 Branch: %s", absDir, tempDir, branch))

	// Launch the agent with the git branch-specific prompt
	agentID, err := agentManager.LaunchAgent(ctx, tempDir, gitBranchPrompt)
	if err != nil {
		os.RemoveAll(tempDir)
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	SendMessage(ctx, b, chatID, fmt.Sprintf("✅ Git-aware code agent launched for existing branch!\n🆔 ID: `%s`\n📝 Task: %s\n🌿 Branch: %s\n📁 Original Directory: %s\n📁 Working Directory: %s\n\nThe agent will work on the existing branch and attempt to push changes.\n\nUse `/status %s` to check status.",
		agentID, task, branch, directory, tempDir, agentID))
}

func handleHelpCommand(ctx context.Context, message *models.Message) {
	helpText := "📚 *Available Commands*\n\n" +
		"*LAN Server Commands:*\n" +
		"• `/start <workdir> <port> <command...>` - Start LAN server with build command\n" +
		"• `/serve <directory> [port]` - Serve static files on LAN (default port: 8080)\n" +
		"• `/stop` - Stop LAN server\n\n" +
		"*Code Agent Commands:*\n" +
		"• `/code <directory> <task>` - Launch a new code agent\n" +
		"• `/new_branch <directory> <task>` - Launch git-aware agent (creates branch & pushes)\n" +
		"• `/edit_branch <directory> <branch> <task>` - Launch git-aware agent on existing branch\n" +
		"• `/commit <directory>` - Commit and push current changes\n" +
		"• `/diff [path]` - Show git diffs (directory: all files, file: single diff)\n" +
		"• `/review <directory>` - Review pending changes in workspace\n" +
		"• `/review <directory> <pr_url>` - Review PR and send result to Telegram\n" +
		"• `/pr <directory> <pr_url>` - Review PR, post comment, and approve if ready\n" +
		"• `/ps` - List all active code agents\n" +
		"• `/status <agent_id>` - Get details of a specific agent\n" +
		"• `/stop <agent_id>` - Kill a running agent\n\n" +
		"*Image Commands:*\n" +
		"• Send images directly to include them in the next `/code` command\n" +
		"• `/images` - Show pending images\n" +
		"• `/clear_images` - Clear all pending images\n\n" +
		"*File & Directory Commands:*\n" +
		"• `/download <file_path>` - Download a file (up to 50MB)\n" +
		"• `/ls [directory]` - List directory contents\n" +
		"• `/mkdir <directory>` - Create a new directory\n" +
		"• `/run <workspace> <command> [args...]` - Run command in workspace\n\n"

	// Add admin commands if user is admin
	if message.From.ID == AdminUserID {
		helpText += "*Admin Commands:*\n" +
			"• `/adduser <username> <user_id>` - Add authorized user\n" +
			"• `/removeuser <username>` - Remove authorized user\n" +
			"• `/users` - List all authorized users\n" +
			"• `/cleanup` - Force cleanup of stuck finished agents\n" +
			"• `/restart` - Restart bot with green deployment\n\n"
	}

	helpText += "*Other Commands:*\n" +
		"• `/help` - Show this help message\n\n" +
		"*Examples:*\n" +
		"• `/start ~/reservas_rb 3000 rails s` - Start Rails app on LAN\n" +
		"• `/serve ~/public_html` - Serve static files on port 8080\n" +
		"• `/serve ~/docs 3000` - Serve static files on port 3000\n" +
		"• `/stop` - Stop LAN server\n" +
		"• `/code /home/project \"fix the bug in main.py\"`\n" +
		"• `/ps`\n" +
		"• `/status abc123`\n" +
		"• `/stop abc123` - Stop specific agent\n" +
		"• `/new_branch /my/repo \"add error handling to API\"`\n" +
		"• `/edit_branch ~/myproject feature/auth \"fix authentication bug\"`\n" +
		"• `/commit ~/myproject` - Commit and push changes\n" +
		"• `/diff ~/myproject` - Show all git diffs in project\n" +
		"• `/diff ~/myproject/main.go` - Show diff for single file\n" +
		"• `/review ~/myproject` - Review pending changes\n" +
		"• `/review ~/myproject https://github.com/owner/repo/pull/123` - Review PR\n" +
		"• `/pr ~/myproject https://github.com/owner/repo/pull/123` - Review PR & post comment\n" +
		"• `/run ~/myapp npm test` - Run tests in myapp workspace\n" +
		"• `/run . python script.py --verbose` - Run Python script in current dir"

	SendMessage(ctx, b, message.Chat.ID, helpText)
}

func handleCodeCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)

	if len(parts) < 3 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Usage: `/code <directory> <task>`\n\nExample: `/code /home/project \"fix the bug in main.py\"`")
		return
	}

	// Extract directory and task
	directory := parts[1]
	task := strings.Join(parts[2:], " ")

	// Call the existing launch function
	launchCodeAgentCommand(ctx, message.Chat.ID, directory, task)
}

func handleAgentsCommand(ctx context.Context, message *models.Message) {
	listCodeAgentsCommand(ctx, message.Chat.ID)
}

func handleStatusCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)

	if len(parts) < 2 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Usage: `/status <agent_id>`\n\nExample: `/status abc123`")
		return
	}

	agentID := parts[1]
	getCodeAgentDetailsCommand(ctx, message.Chat.ID, agentID)
}

func handleStopCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)

	if len(parts) < 2 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Usage: `/stop <agent_id>`\n\nExample: `/stop abc123`")
		return
	}

	agentID := parts[1]
	killCodeAgentCommand(ctx, message.Chat.ID, agentID)
}

func handleAddUserCommand(ctx context.Context, message *models.Message) {
	// Only admin can add users
	if message.From.ID != AdminUserID {
		SendMessage(ctx, b, message.Chat.ID, "❌ Only admin can add users.")
		return
	}

	parts := strings.Fields(message.Text)
	if len(parts) < 3 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Usage: `/adduser <username> <user_id>`\n\nExample: `/adduser john123 987654321`")
		return
	}

	username := parts[1]
	userIDStr := parts[2]

	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Invalid user ID: %s", userIDStr))
		return
	}

	if err := authorizedUsers.AddUser(username, userID); err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to add user: %v", err))
		return
	}

	SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ User `%s` (ID: `%d`) has been authorized.", username, userID))
}

func handleRemoveUserCommand(ctx context.Context, message *models.Message) {
	// Only admin can remove users
	if message.From.ID != AdminUserID {
		SendMessage(ctx, b, message.Chat.ID, "❌ Only admin can remove users.")
		return
	}

	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Usage: `/removeuser <username>`\n\nExample: `/removeuser john123`")
		return
	}

	username := parts[1]

	if err := authorizedUsers.RemoveUser(username); err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to remove user: %v", err))
		return
	}

	SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ User `%s` has been removed from authorized users.", username))
}

func handleUsersCommand(ctx context.Context, message *models.Message) {
	// Only admin can list users
	if message.From.ID != AdminUserID {
		SendMessage(ctx, b, message.Chat.ID, "❌ Only admin can list users.")
		return
	}

	users := authorizedUsers.ListUsers()

	if len(users) == 0 {
		SendMessage(ctx, b, message.Chat.ID, "📋 No authorized users.")
		return
	}

	var sb strings.Builder
	sb.WriteString("📋 *Authorized Users:*\n\n")

	for username, userID := range users {
		sb.WriteString(fmt.Sprintf("👤 `%s` - ID: `%d`\n", username, userID))
	}

	SendMessage(ctx, b, message.Chat.ID, sb.String())
}

func handleReviewCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Please provide workspace directory.\nUsage:\n• `/review <directory>` - Review pending changes\n• `/review <directory> <pr_url>` - Review PR\n\nExamples:\n• `/review ~/myproject`\n• `/review ~/myproject https://github.com/owner/repo/pull/123`")
		return
	}

	directory := strings.TrimSpace(parts[1])

	// If only directory is provided, review pending changes
	if len(parts) == 2 {
		launchPendingChangesReviewAgent(ctx, message.Chat.ID, directory)
		return
	}

	// If PR URL is provided, review the PR
	prURL := strings.TrimSpace(parts[2])
	if directory == "" || prURL == "" {
		SendMessage(ctx, b, message.Chat.ID, "❌ Both directory and PR URL are required for PR review.\n\nExample: /review ~/myproject https://github.com/owner/repo/pull/123")
		return
	}

	launchPRReviewAgent(ctx, message.Chat.ID, directory, prURL)
}

func launchPRReviewAgent(ctx context.Context, chatID int64, directory, prURL string) {
	// Resolve the directory path relative to home directory
	absDir, err := ResolvePath(directory)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	SendMessage(ctx, b, chatID, fmt.Sprintf("🔍 Preparing PR review for %s...", prURL))

	// Prepare the PR review prompt
	prReviewPrompt := fmt.Sprintf(`IMPORTANT PR REVIEW INSTRUCTIONS:
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

4. Write a concise PR review and send it to Telegram:
   - DO NOT use gh pr comment or post anything to the PR
   - Structure your review as follows:
     * List any bugs or issues found (if any)
     * Code improvement suggestions (following the project conventions and best practices) (if any)
     * Final verdict: Approve, Request Changes, or Needs Discussion

Remember:
- Be concise
- Point out specific line numbers or files when mentioning issues
- If everything looks good, don't say anything
- Send your review message directly to the output (it will be sent to Telegram)

PR URL: %s`, prURL, prURL, prURL, prURL, prURL)

	SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching PR review agent...\n📁 Repository: %s\n🔗 PR: %s", absDir, prURL))

	// Launch the agent with the PR review prompt and unique plan file
	planFilename := generateUniquePlanFilename("PR_REVIEW")
	agentID, err := agentManager.LaunchAgentWithPlanFile(ctx, absDir, prReviewPrompt, planFilename)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	SendMessage(ctx, b, chatID, fmt.Sprintf("✅ PR review agent launched!\n🆔 ID: `%s`\n🔗 PR: %s\n📁 Repository: %s\n\nThe agent will:\n• Analyze the PR changes\n• Review code quality and bugs\n• Send the review to this Telegram chat\n\nUse `/status %s` to check status.",
		agentID, prURL, directory, agentID))
}

func launchPendingChangesReviewAgent(ctx context.Context, chatID int64, directory string) {
	// Resolve the directory path relative to home directory
	absDir, err := ResolvePath(directory)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	SendMessage(ctx, b, chatID, "🔍 Preparing to review pending changes...")

	// Prepare the pending changes review prompt
	pendingChangesPrompt := `IMPORTANT PENDING CHANGES REVIEW INSTRUCTIONS:
You are tasked with reviewing the pending changes in this git repository. Follow these steps carefully:

1. First, check the current git status:
   - Run: git status
   - Run: git diff --staged
   - Run: git diff

2. Analyze the pending changes:
   - Look for potential bugs, security issues, or performance problems
   - Check if the implementation aligns with the requirements
   - Verify that the code follows project conventions and best practices
   - Check for missing tests or documentation
   - Review both staged and unstaged changes

4. Write a concise review of the pending changes:
   - DO NOT make any commits or push changes
   - Structure your review as follows:
     * Summary of changes (files modified, added, deleted)
     * List any bugs or issues found (if any)
     * Code improvement suggestions (following the project conventions and best practices) (if any)
     * Recommendations for next steps (e.g., ready to commit, needs more work, etc.)

Remember:
- Be concise and focused on the actual changes
- Point out specific files and line numbers when mentioning issues
- If everything looks good, say so briefly
- Send your review message directly to the output (it will be sent to Telegram)`

	SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching pending changes review agent...\n📁 Repository: %s", absDir))

	// Launch the agent with the pending changes review prompt and unique plan file
	planFilename := generateUniquePlanFilename("REVIEW")
	agentID, err := agentManager.LaunchAgentWithPlanFile(ctx, absDir, pendingChangesPrompt, planFilename)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	SendMessage(ctx, b, chatID, fmt.Sprintf("✅ Pending changes review agent launched!\n🆔 ID: `%s`\n📁 Repository: %s\n\nThe agent will:\n• Check git status and diffs\n• Review code quality and bugs\n• Send the review to this Telegram chat\n\nUse `/status %s` to check status.",
		agentID, directory, agentID))
}

func handlePRCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 3 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Please provide workspace directory and PR URL.\nUsage: `/pr <directory> <pr_url>`\n\nExample: `/pr ~/myproject https://github.com/owner/repo/pull/123`")
		return
	}

	directory := strings.TrimSpace(parts[1])
	prURL := strings.TrimSpace(parts[2])

	if directory == "" || prURL == "" {
		SendMessage(ctx, b, message.Chat.ID, "❌ Both directory and PR URL are required.\n\nExample: /pr ~/myproject https://github.com/owner/repo/pull/123")
		return
	}

	launchPRCommentAgent(ctx, message.Chat.ID, directory, prURL)
}

func launchPRCommentAgent(ctx context.Context, chatID int64, directory, prURL string) {
	// Resolve the directory path relative to home directory
	absDir, err := ResolvePath(directory)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	SendMessage(ctx, b, chatID, fmt.Sprintf("🔍 Preparing PR review for %s...", prURL))

	// Prepare the PR review and comment prompt
	prCommentPrompt := fmt.Sprintf(`IMPORTANT PR REVIEW AND COMMENT INSTRUCTIONS:
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

	SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching PR review agent...\n📁 Repository: %s\n🔗 PR: %s", absDir, prURL))

	// Launch the agent with the PR comment prompt and unique plan file
	planFilename := generateUniquePlanFilename("PR_COMMENT")
	agentID, err := agentManager.LaunchAgentWithPlanFile(ctx, absDir, prCommentPrompt, planFilename)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	SendMessage(ctx, b, chatID, fmt.Sprintf("✅ PR review agent launched!\n🆔 ID: `%s`\n🔗 PR: %s\n📁 Repository: %s\n\nThe agent will:\n• Analyze the PR changes\n• Post a review comment on the PR\n• Approve the PR if it's ready to merge\n\nUse `/status %s` to check status.",
		agentID, prURL, directory, agentID))
}

func handleStartCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 4 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Please provide workdir, port, and build command.\nUsage: /start <workdir> <port> <build command...>\n\nExample: /start ~/reservas_rb 3000 rails s")
		return
	}

	workdir := strings.TrimSpace(parts[1])
	port := strings.TrimSpace(parts[2])
	buildCmdStr := strings.Join(parts[3:], " ")

	// Validate port
	if _, err := strconv.Atoi(port); err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Invalid port number: %s", port))
		return
	}

	// Resolve the workdir path
	absWorkdir, err := ResolvePath(workdir)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error resolving workdir path: %v", err))
		return
	}

	// Check if workdir exists
	info, err := os.Stat(absWorkdir)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Directory not found: %s", absWorkdir))
		return
	}
	if !info.IsDir() {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Path is not a directory: %s", absWorkdir))
		return
	}

	lanServerMutex.Lock()
	defer lanServerMutex.Unlock()

	// Check if server is already running
	if lanServerProcess != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ LAN server is already running!\n📁 Workdir: %s\n🔌 Port: %s\n🛠️ Command: %s\n\nUse /stop to stop it first.", lanServerWorkDir, lanServerPort, lanServerCmd))
		return
	}

	// Check if port is in use and find an available one if needed
	if IsPortInUse(port) {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("⚠️ Port %s is already in use. Finding an available port...", port))

		availablePort, err := FindAvailablePort(port)
		if err != nil {
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Could not find an available port: %v", err))
			return
		}
		port = availablePort
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ Using port %s instead", port))
	}

	SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🚀 Starting LAN server...\n📁 Workdir: %s\n🔌 Port: %s\n🛠️ Build command: %s", absWorkdir, port, buildCmdStr))

	// Start the build command in the workdir
	buildCmd := exec.Command("sh", "-c", buildCmdStr)
	buildCmd.Dir = absWorkdir

	// Set environment variables including the PORT
	buildCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%s", port))

	// Capture output for error reporting
	buildOutput := &strings.Builder{}
	buildCmd.Stdout = io.MultiWriter(os.Stdout, buildOutput)
	buildCmd.Stderr = io.MultiWriter(os.Stderr, buildOutput)

	if err := buildCmd.Start(); err != nil {
		// Try to get more detailed error output
		output := buildOutput.String()
		if output != "" {
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to start build command: %v\n\n📋 *Output:*\n```\n%s\n```", err, output))
		} else {
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to start build command: %v", err))
		}
		return
	}

	// Give the build command a moment to start and check if it's still running
	time.Sleep(2 * time.Second)

	// Check if build process already exited (failed to start properly)
	if buildCmd.ProcessState != nil {
		output := buildOutput.String()
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Build command failed to start properly.\n\n📋 *Output:*\n```\n%s\n```", output))
		return
	}

	// Store the process info
	lanServerProcess = buildCmd.Process
	lanServerPort = port
	lanServerWorkDir = absWorkdir
	lanServerCmd = buildCmdStr

	// Get local IP addresses
	var ipAddresses []string
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ipAddresses = append(ipAddresses, ipnet.IP.String())
				}
			}
		}
	}

	// Try to set up UPnP port mapping
	portInt, _ := strconv.Atoi(port)

	// Attempt UPnP mapping in a goroutine to not block startup
	go func() {
		SendMessage(ctx, b, message.Chat.ID, "🔌 Attempting UPnP port mapping...")

		err := upnpManager.MapPort(portInt, portInt, "TCP", fmt.Sprintf("Mavis Server - %s", buildCmdStr))
		if err != nil {
			log.Printf("UPnP mapping failed: %v", err)
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("⚠️ UPnP port mapping failed: %v\n\nServer is still accessible on LAN.", err))
		} else {
			// Get public IP
			publicIP, err := GetPublicIP(ctx)
			if err != nil {
				log.Printf("Failed to get public IP: %v", err)
				SendMessage(ctx, b, message.Chat.ID, "⚠️ UPnP succeeded but couldn't get public IP. Server is accessible on LAN.")
			} else {
				// Send success message with public URL
				publicURL := fmt.Sprintf("http://%s:%s", publicIP, port)
				SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ UPnP mapping successful!\n\n🌍 *Public URL:* %s\n\n⚠️ *Important:* This URL is accessible from the internet!", publicURL))
			}
		}
	}()

	// Build access URLs
	var accessURLs strings.Builder
	accessURLs.WriteString("\n🌐 *Access URLs:*\n")
	accessURLs.WriteString(fmt.Sprintf("  🏠 Local: http://localhost:%s\n", port))
	for _, ip := range ipAddresses {
		accessURLs.WriteString(fmt.Sprintf("  📡 LAN: http://%s:%s\n", ip, port))
	}
	accessURLs.WriteString(fmt.Sprintf("  🎯 mDNS: http://%s:%s (if available)\n", lanDomainName, port))

	// Success message
	successMsg := fmt.Sprintf("✅ LAN server started successfully!\n📁 Workdir: %s\n🔌 Port: %s\n🛠️ Build command: %s\n%s\n💡 *Note:* Attempting to expose to internet via UPnP...", absWorkdir, port, buildCmdStr, accessURLs.String())

	SendMessage(ctx, b, message.Chat.ID, successMsg)

	// Monitor the process in a goroutine
	go func() {
		// Wait for process to exit and capture the error
		err := buildCmd.Wait()

		lanServerMutex.Lock()
		if lanServerProcess != nil {
			// Clean up UPnP mapping
			if lanServerPort != "" {
				portInt, _ := strconv.Atoi(lanServerPort)
				upnpManager.UnmapPort(portInt)
			}

			// Clean up
			lanServerProcess = nil
			lanServerPort = ""
			lanServerWorkDir = ""
			lanServerCmd = ""
			lanServerMutex.Unlock()

			// Build error message with reason
			errorMsg := "⚠️ LAN server has stopped"
			if err != nil {
				// Get the output that was captured
				output := buildOutput.String()
				if output != "" {
					errorMsg = fmt.Sprintf("⚠️ LAN server has stopped.\n❌ *Reason:* %v\n\n📋 *Output:*\n```\n%s\n```", err, output)
				} else {
					errorMsg = fmt.Sprintf("⚠️ LAN server has stopped.\n❌ *Reason:* %v", err)
				}
			}

			SendMessage(ctx, b, message.Chat.ID, errorMsg)
		} else {
			lanServerMutex.Unlock()
		}
	}()
}

func handleStopLANCommand(ctx context.Context, message *models.Message) {
	lanServerMutex.Lock()
	defer lanServerMutex.Unlock()

	if lanServerProcess == nil && lanHTTPServer == nil {
		SendMessage(ctx, b, message.Chat.ID, "❌ No LAN server is currently running.")
		return
	}

	workdir := lanServerWorkDir
	port := lanServerPort
	cmd := lanServerCmd

	// Stop process-based server if running
	if lanServerProcess != nil {
		// Kill the server process
		if err := lanServerProcess.Kill(); err != nil {
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to stop LAN server process: %v", err))
		}

		// Also try to kill any process using the port
		if lanServerPort != "" {
			killPortCmd := exec.Command("sh", "-c", fmt.Sprintf("lsof -ti:%s | xargs kill -9 2>/dev/null || true", lanServerPort))
			killPortCmd.Run()
		}
	}

	// Stop Go HTTP server if running
	if lanHTTPServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := lanHTTPServer.Shutdown(shutdownCtx); err != nil {
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("⚠️ Warning: HTTP server shutdown error: %v", err))
		}
	}

	// Clean up UPnP mapping
	if lanServerPort != "" {
		portInt, _ := strconv.Atoi(lanServerPort)
		upnpManager.UnmapPort(portInt)
	}

	// Clean up
	lanServerProcess = nil
	lanHTTPServer = nil
	lanServerPort = ""
	lanServerWorkDir = ""
	lanServerCmd = ""

	SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🛑 LAN server stopped.\n📁 Workdir: %s\n🔌 Port: %s\n🛠️ Command: %s", workdir, port, cmd))
}

func handleServeCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Please provide a directory to serve.\nUsage: /serve <directory> [port]\n\nExample: /serve ~/myproject 8080\n\nIf port is not specified, it defaults to 8080.")
		return
	}

	workdir := strings.TrimSpace(parts[1])
	port := "8080" // Default port

	// Check if port was specified
	if len(parts) >= 3 {
		port = strings.TrimSpace(parts[2])
		// Validate port
		if _, err := strconv.Atoi(port); err != nil {
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Invalid port number: %s", port))
			return
		}
	}

	// Resolve the workdir path
	absWorkdir, err := ResolvePath(workdir)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if workdir exists
	info, err := os.Stat(absWorkdir)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Directory not found: %s", absWorkdir))
		return
	}
	if !info.IsDir() {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Path is not a directory: %s", absWorkdir))
		return
	}

	lanServerMutex.Lock()
	defer lanServerMutex.Unlock()

	// Check if server is already running
	if lanServerProcess != nil || lanHTTPServer != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ LAN server is already running!\n📁 Workdir: %s\n🔌 Port: %s\n🛠️ Command: %s\n\nUse /stop to stop it first.", lanServerWorkDir, lanServerPort, lanServerCmd))
		return
	}

	// Check if port is in use and find an available one if needed
	if IsPortInUse(port) {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("⚠️ Port %s is already in use. Finding an available port...", port))

		availablePort, err := FindAvailablePort(port)
		if err != nil {
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Could not find an available port: %v", err))
			return
		}
		port = availablePort
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ Using port %s instead", port))
	}

	SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🚀 Starting LAN file server...\n📁 Directory: %s\n🔌 Port: %s\n🛠️ Server: Go HTTP Server", absWorkdir, port))

	// Start the Go file server
	server, err := StartFileServer(absWorkdir, port)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to start LAN file server: %v", err))
		return
	}

	// Store the server reference (we'll need to modify the global variables)
	lanHTTPServer = server
	lanServerPort = port
	lanServerWorkDir = absWorkdir
	lanServerCmd = fmt.Sprintf("Go file server on port %s", port)

	// Get local IP addresses
	var ipAddresses []string
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ipAddresses = append(ipAddresses, ipnet.IP.String())
				}
			}
		}
	}

	// Try to set up UPnP port mapping
	portInt, _ := strconv.Atoi(port)

	// Attempt UPnP mapping in a goroutine to not block startup
	go func() {
		SendMessage(ctx, b, message.Chat.ID, "🔌 Attempting UPnP port mapping...")

		err := upnpManager.MapPort(portInt, portInt, "TCP", "Mavis File Server")
		if err != nil {
			log.Printf("UPnP mapping failed: %v", err)
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("⚠️ UPnP port mapping failed: %v\n\nServer is still accessible on LAN.", err))
		} else {
			// Get public IP
			publicIP, err := GetPublicIP(ctx)
			if err != nil {
				log.Printf("Failed to get public IP: %v", err)
				SendMessage(ctx, b, message.Chat.ID, "⚠️ UPnP succeeded but couldn't get public IP. Server is accessible on LAN.")
			} else {
				// Send success message with public URL
				publicURL := fmt.Sprintf("http://%s:%s", publicIP, port)
				SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ UPnP mapping successful!\n\n🌍 *Public URL:* %s\n\n⚠️ *Important:* This URL is accessible from the internet!", publicURL))
			}
		}
	}()

	// Build access URLs
	var accessURLs strings.Builder
	accessURLs.WriteString("\n🌐 *Access URLs:*\n")
	accessURLs.WriteString(fmt.Sprintf("  🏠 Local: http://localhost:%s\n", port))
	for _, ip := range ipAddresses {
		accessURLs.WriteString(fmt.Sprintf("  📡 LAN: http://%s:%s\n", ip, port))
	}
	accessURLs.WriteString(fmt.Sprintf("  🎯 mDNS: http://%s:%s (if available)\n", lanDomainName, port))

	// Success message
	successMsg := fmt.Sprintf("✅ LAN file server started successfully!\n📁 Serving: %s\n🔌 Port: %s\n📄 Server: Go HTTP Server\n%s\n💡 *Note:* Attempting to expose to internet via UPnP...", absWorkdir, port, accessURLs.String())

	SendMessage(ctx, b, message.Chat.ID, successMsg)
}

func handleCommitCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		SendMessage(ctx, b, message.Chat.ID, "❌ Please provide a directory.\nUsage: /commit <directory>\n\nExample: /commit ~/myproject")
		return
	}

	directory := strings.TrimSpace(strings.Join(parts[1:], " "))

	if directory == "" {
		SendMessage(ctx, b, message.Chat.ID, "❌ Directory is required.\n\nExample: /commit ~/myproject")
		return
	}

	launchCommitAgent(ctx, message.Chat.ID, directory)
}

func launchCommitAgent(ctx context.Context, chatID int64, directory string) {
	// Resolve the directory path relative to home directory
	absDir, err := ResolvePath(directory)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	SendMessage(ctx, b, chatID, fmt.Sprintf("🔍 Checking git repository status in %s...", absDir))

	// Prepare the commit-specific prompt
	commitPrompt := `IMPORTANT COMMIT INSTRUCTIONS:
You are tasked with committing the current changes in a git repository and pushing to remote. Follow these steps:

1. First, check the current git status using: git status
2. Review all uncommitted changes using: git diff
3. Stage all relevant changes (be careful not to stage files that shouldn't be committed)
   IMPORTANT: NEVER stage *_PLAN_*.md files. Use commands like:
   - git add . && git reset *_PLAN_*.md  (to add all except plan files)
   - Or stage files individually, explicitly excluding *_PLAN_*.md files
4. Create a meaningful commit with a descriptive message based on the changes
5. Push the commit to the remote repository using: git push
6. If the push fails due to authentication, that's okay - just report the status

Remember:
- Write clear, descriptive commit messages
- Don't commit files that shouldn't be in version control (like .env, node_modules, *_PLAN_*.md files, etc.)
- Make sure the commit message accurately describes what was changed
- If there are no changes to commit, report that clearly

Your task: Review the changes, commit them with an appropriate message, and push to remote.`

	SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching Claude Code to commit changes...\n📁 Directory: %s", absDir))

	// Launch the agent with the commit-specific prompt
	agentID, err := agentManager.LaunchAgent(ctx, absDir, commitPrompt)
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	SendMessage(ctx, b, chatID, fmt.Sprintf("✅ Commit agent launched!\n🆔 ID: `%s`\n📁 Directory: %s\n\nThe agent will:\n• Review uncommitted changes\n• Create a meaningful commit\n• Push to the remote repository\n\nUse `/status %s` to check status.",
		agentID, directory, agentID))
}

func handleDiffCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	path := "." // Default to current directory

	if len(parts) >= 2 {
		// Join all parts after the command in case the path has spaces
		path = strings.Join(parts[1:], " ")
	}

	// Resolve the path
	absPath, err := ResolvePath(path)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error resolving path: %v", err))
		return
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Path not found: %s", absPath))
		return
	}

	// Determine if it's a file or directory
	if !info.IsDir() {
		// Handle single file diff
		handleFileDiff(ctx, message.Chat.ID, absPath, path)
		return
	}

	// It's a directory - check if it's a git repository
	gitDir := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Directory is not a git repository: %s", absPath))
		return
	}

	SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("📊 Checking git status in %s...", path))

	// Run git status to get modified files
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = absPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to get git status: %v\nOutput: %s", err, string(output)))
		return
	}

	if len(output) == 0 {
		SendMessage(ctx, b, message.Chat.ID, "✅ Working directory is clean. No modified files.")
		return
	}

	// Parse the git status output
	lines := strings.Split(string(output), "\n")
	var staged []string
	var modified []string
	var untracked []string
	var deleted []string

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Git status format: XY filename
		// X = staged status, Y = unstaged status
		if len(line) < 3 {
			continue
		}

		status := line[:2]
		filename := strings.TrimSpace(line[2:])

		switch {
		case status[0] == 'A' || status[0] == 'M':
			staged = append(staged, filename)
		case status[1] == 'M':
			modified = append(modified, filename)
		case status[1] == 'D':
			deleted = append(deleted, filename)
		case status == "??":
			untracked = append(untracked, filename)
		}
	}

	// Build the summary message
	var responseMsg strings.Builder
	responseMsg.WriteString(fmt.Sprintf("📊 *Git Status - %s*\n\n", path))

	totalFiles := 0
	if len(staged) > 0 {
		responseMsg.WriteString(fmt.Sprintf("✅ *Staged files:* %d\n", len(staged)))
		totalFiles += len(staged)
	}

	if len(modified) > 0 {
		responseMsg.WriteString(fmt.Sprintf("📝 *Modified files:* %d\n", len(modified)))
		totalFiles += len(modified)
	}

	if len(deleted) > 0 {
		responseMsg.WriteString(fmt.Sprintf("🗑️ *Deleted files:* %d\n", len(deleted)))
		totalFiles += len(deleted)
	}

	if len(untracked) > 0 {
		responseMsg.WriteString(fmt.Sprintf("❓ *Untracked files:* %d\n", len(untracked)))
		totalFiles += len(untracked)
	}

	responseMsg.WriteString(fmt.Sprintf("\n📈 *Total:* %d file(s) with changes", totalFiles))

	// Send the summary message
	SendMessage(ctx, b, message.Chat.ID, responseMsg.String())

	// Send diffs for each modified or staged file
	processedFiles := make(map[string]bool)

	// Process staged files
	for _, file := range staged {
		if !processedFiles[file] {
			processedFiles[file] = true
			sendGitDiff(ctx, message.Chat.ID, absPath, file, true)
			time.Sleep(100 * time.Millisecond) // Small delay to avoid rate limiting
		}
	}

	// Process modified files
	for _, file := range modified {
		if !processedFiles[file] {
			processedFiles[file] = true
			sendGitDiff(ctx, message.Chat.ID, absPath, file, false)
			time.Sleep(100 * time.Millisecond) // Small delay to avoid rate limiting
		}
	}

	// For deleted files, just send a notification
	for _, file := range deleted {
		if !processedFiles[file] {
			processedFiles[file] = true
			SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🗑️ *Deleted:* `%s`", file))
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// handleFileDiff handles diff for a single file
func handleFileDiff(ctx context.Context, chatID int64, absPath, displayPath string) {
	// Get the directory and filename
	dir := filepath.Dir(absPath)
	filename := filepath.Base(absPath)

	// Check if the parent directory is a git repository
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ File is not in a git repository: %s", displayPath))
		return
	}

	// Check git status for this specific file
	cmd := exec.Command("git", "status", "--porcelain", filename)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to get git status for file: %v", err))
		return
	}

	if len(output) == 0 {
		SendMessage(ctx, b, chatID, fmt.Sprintf("✅ File has no changes: `%s`", displayPath))
		return
	}

	// Parse the status
	line := strings.TrimSpace(string(output))
	if len(line) < 3 {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Invalid git status output for file: %s", displayPath))
		return
	}

	status := line[:2]
	staged := status[0] == 'A' || status[0] == 'M'

	// Send the diff
	sendGitDiff(ctx, chatID, dir, filename, staged)
}

// sendGitDiff sends the git diff for a specific file
func sendGitDiff(ctx context.Context, chatID int64, repoDir, filename string, staged bool) {
	var cmd *exec.Cmd
	if staged {
		// For staged files, use --cached
		cmd = exec.Command("git", "diff", "--cached", "--", filename)
	} else {
		// For unstaged files
		cmd = exec.Command("git", "diff", "--", filename)
	}
	cmd.Dir = repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to get diff for %s: %v", filename, err))
		return
	}

	// If no diff output (e.g., for new untracked files), try to show the file content
	if len(output) == 0 {
		// Check if it's a new file
		statusCmd := exec.Command("git", "status", "--porcelain", filename)
		statusCmd.Dir = repoDir
		statusOutput, _ := statusCmd.CombinedOutput()

		if strings.HasPrefix(string(statusOutput), "??") {
			// It's an untracked file, show its content
			content, err := os.ReadFile(filepath.Join(repoDir, filename))
			if err != nil {
				SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to read file %s: %v", filename, err))
				return
			}

			// Prepare the message
			var msg strings.Builder
			msg.WriteString(fmt.Sprintf("📄 *New file:* `%s`\n\n", filename))
			msg.WriteString("```\n")

			// Truncate content if too long
			contentStr := string(content)
			if len(contentStr) > 3000 {
				contentStr = contentStr[:2997] + "..."
			}
			msg.WriteString(contentStr)
			msg.WriteString("\n```")

			SendMessage(ctx, b, chatID, msg.String())
			return
		}

		// No changes to show
		return
	}

	// Prepare the diff message
	var msg strings.Builder
	statusIcon := "📝"
	if staged {
		statusIcon = "✅"
	}
	msg.WriteString(fmt.Sprintf("%s *File:* `%s`\n\n", statusIcon, filename))
	msg.WriteString("```diff\n")

	// Truncate diff if too long
	diffStr := string(output)
	if len(diffStr) > 3500 {
		diffStr = diffStr[:3497] + "..."
	}
	msg.WriteString(diffStr)
	msg.WriteString("\n```")

	// Send the diff as a separate message
	SendLongMessage(ctx, b, chatID, msg.String())
}

func handleRunCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)

	// Check if we have at least a workspace and a command
	if len(parts) < 3 {
		SendMessage(ctx, b, message.Chat.ID,
			"❌ Usage: /run <workspace> <command> [args...]\n\n"+
				"Example: /run ~/projects/myapp npm test\n"+
				"Example: /run . python script.py --verbose")
		return
	}

	// Extract workspace and command
	workspace := parts[1]
	command := parts[2]
	args := parts[3:]

	// Resolve the workspace path
	absWorkspace, err := ResolvePath(workspace)
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error resolving workspace path: %v", err))
		return
	}

	// Check if workspace exists
	if _, err := os.Stat(absWorkspace); os.IsNotExist(err) {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Workspace directory does not exist: %s", absWorkspace))
		return
	}

	// Send initial message
	cmdStr := strings.Join(parts[2:], " ")
	SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🚀 Running command in workspace: %s\n```\n%s\n```", absWorkspace, cmdStr))

	// Create and execute the command
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = absWorkspace

	// Capture both stdout and stderr
	output, err := cmd.CombinedOutput()

	// Format the response
	var responseMsg strings.Builder
	responseMsg.WriteString(fmt.Sprintf("📁 *Workspace:* `%s`\n", absWorkspace))
	responseMsg.WriteString(fmt.Sprintf("💻 *Command:* `%s`\n", cmdStr))
	responseMsg.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	if err != nil {
		responseMsg.WriteString(fmt.Sprintf("❌ *Error:* %v\n\n", err))
	} else {
		responseMsg.WriteString("✅ *Command completed successfully*\n\n")
	}

	// Add output if any
	if len(output) > 0 {
		responseMsg.WriteString("📄 *Output:*\n```\n")
		outputStr := string(output)
		// Truncate output if too long
		if len(outputStr) > 3000 {
			outputStr = outputStr[:2997] + "..."
		}
		responseMsg.WriteString(outputStr)
		responseMsg.WriteString("\n```")
	} else {
		responseMsg.WriteString("ℹ️ *No output produced*")
	}

	// Send the response
	response := responseMsg.String()
	if len(response) > 4000 {
		response = response[:3997] + "..."
	}

	SendMessage(ctx, b, message.Chat.ID, response)
}

func handleRestartCommand(ctx context.Context, message *models.Message) {
	// Only admin can restart the bot
	if message.From.ID != AdminUserID {
		SendMessage(ctx, b, message.Chat.ID, "❌ Only admin can restart the bot.")
		return
	}

	SendMessage(ctx, b, message.Chat.ID, "🔄 Restarting bot...")

	// Exit the process
	os.Exit(0)
}

func handleImagesCommand(ctx context.Context, message *models.Message) {
	userID := AdminUserID
	pendingImages := getPendingImages(userID)

	if len(pendingImages) == 0 {
		SendMessage(ctx, b, message.Chat.ID, "📸 You have no pending images.\n\nSend images to the chat and they will be included in your next `/code` command.")
		return
	}

	msg := fmt.Sprintf("📸 *Pending Images: %d*\n\n", len(pendingImages))
	for i, imagePath := range pendingImages {
		filename := filepath.Base(imagePath)
		msg += fmt.Sprintf("%d. `%s`\n", i+1, filename)
	}
	msg += "\nThese images will be included in your next `/code` command.\nUse `/clear_images` to remove them."

	SendMessage(ctx, b, message.Chat.ID, msg)
}

func handleClearImagesCommand(ctx context.Context, message *models.Message) {
	// Only admin can clear images
	if message.From.ID != AdminUserID {
		SendMessage(ctx, b, message.Chat.ID, "❌ Only admin can clear images.")
		return
	}

	count := getPendingImageCount(AdminUserID)
	if count > 0 {
		clearPendingImages(AdminUserID)
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🗑️ Cleared %d pending image(s).", count))
	} else {
		SendMessage(ctx, b, message.Chat.ID, "📋 No pending images to clear.")
	}
}

func handleCleanupCommand(ctx context.Context, message *models.Message) {
	// Only admin can cleanup stuck agents
	if message.From.ID != AdminUserID {
		SendMessage(ctx, b, message.Chat.ID, "❌ Only admin can cleanup stuck agents.")
		return
	}

	SendMessage(ctx, b, message.Chat.ID, "🔧 Starting cleanup of stuck agents...")

	cleanedCount := ForceCleanupStuckAgents()

	if cleanedCount > 0 {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ Cleanup complete! Removed %d stuck agent(s).\n\n💡 Queued tasks should now start processing automatically.", cleanedCount))

		// Show updated status
		time.Sleep(1 * time.Second) // Give a moment for queue processing
		listCodeAgentsCommand(ctx, message.Chat.ID)
	} else {
		SendMessage(ctx, b, message.Chat.ID, "📋 No stuck agents found. All agents are running normally.")
	}
}
