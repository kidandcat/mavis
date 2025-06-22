// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-telegram/bot/models"
)

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