// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-telegram/bot/models"
)

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