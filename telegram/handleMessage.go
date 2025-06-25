// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package telegram

import (
	"context"
	"strings"

	"mavis/core"

	"github.com/go-telegram/bot/models"
)

func HandleMessage(ctx context.Context, message *models.Message) {
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
			case "/approve":
				handleApproveCommand(ctx, message)
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
	core.SendMessage(ctx, b, message.Chat.ID, "I'm Mavis, a code agent manager. Use /help to see available commands.")
}
