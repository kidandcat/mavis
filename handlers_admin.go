// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"os"

	"github.com/go-telegram/bot/models"
)


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
