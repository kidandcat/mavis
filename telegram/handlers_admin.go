// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package telegram

import (
	"context"
	"os"

	"mavis/core"

	"github.com/go-telegram/bot/models"
)

func handleRestartCommand(ctx context.Context, message *models.Message) {
	// Only admin can restart the bot
	if message.From.ID != AdminUserID {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Only admin can restart the bot.")
		return
	}

	core.SendMessage(ctx, b, message.Chat.ID, "🔄 Restarting bot...")

	// Exit the process
	os.Exit(0)
}
