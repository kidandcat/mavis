// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package telegram

import (
	"context"
	"os"
	"os/exec"

	"mavis/core"

	"github.com/go-telegram/bot/models"
)

func handleRestartCommand(ctx context.Context, message *models.Message) {
	// Only admin can restart the bot
	if message.From.ID != AdminUserID {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Only admin can restart the bot.")
		return
	}

	core.SendMessage(ctx, b, message.Chat.ID, "🔄 Building and restarting bot...")

	// Run go build
	cmd := exec.Command("go", "build", ".")
	if err := cmd.Run(); err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Build failed: "+err.Error())
		return
	}

	// Exit the process
	os.Exit(0)
}
