// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"

	"github.com/go-telegram/bot/models"
)

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
