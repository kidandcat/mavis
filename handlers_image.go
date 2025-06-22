// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/go-telegram/bot/models"
)

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