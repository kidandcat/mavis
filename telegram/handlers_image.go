// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package telegram

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"mavis/core"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func HandlePhotoMessage(ctx context.Context, message *models.Message) {
	userID := AdminUserID

	// Get the largest photo size
	photo := message.Photo[len(message.Photo)-1]

	// Download the photo
	filePath, err := downloadTelegramFile(ctx, photo.FileID, userID, "jpg")
	if err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to download photo: %v", err))
		return
	}

	// Add to pending images
	addPendingImage(userID, filePath)

	// Get pending count
	count := getPendingImageCount(userID)

	core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("📸 Photo saved! You have %d pending image(s).\n\nThese images will be included in your next `/code` command.", count))
}

func HandleDocumentMessage(ctx context.Context, message *models.Message) {
	userID := AdminUserID
	doc := message.Document

	// Check if it's an image file
	isImage := false
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg"}
	for _, ext := range imageExts {
		if filepath.Ext(doc.FileName) == ext {
			isImage = true
			break
		}
	}

	if !isImage {
		// Not an image, ignore
		return
	}

	// Download the document
	filePath, err := downloadTelegramFile(ctx, doc.FileID, userID, filepath.Ext(doc.FileName)[1:])
	if err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to download image: %v", err))
		return
	}

	// Add to pending images
	addPendingImage(userID, filePath)

	// Get pending count
	count := getPendingImageCount(userID)

	core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🖼️ Image saved! You have %d pending image(s).\n\nThese images will be included in your next `/code` command.", count))
}

func downloadTelegramFile(ctx context.Context, fileID string, userID int64, extension string) (string, error) {
	// Get file info from Telegram
	file, err := b.GetFile(ctx, &bot.GetFileParams{
		FileID: fileID,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	// Create user temp directory
	userTempDir := filepath.Join("data", "temp", fmt.Sprintf("user_%d", userID))
	if err := os.MkdirAll(userTempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Generate unique filename
	filename := fmt.Sprintf("%d.%s", time.Now().UnixNano(), extension)
	localPath := filepath.Join(userTempDir, filename)

	// Download file
	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", b.Token(), file.FilePath)
	resp, err := http.Get(fileURL)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	// Save to local file
	out, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return localPath, nil
}

func handleImagesCommand(ctx context.Context, message *models.Message) {
	userID := AdminUserID
	pendingImages := getPendingImages(userID)

	if len(pendingImages) == 0 {
		core.SendMessage(ctx, b, message.Chat.ID, "📸 You have no pending images.\n\nSend images to the chat and they will be included in your next `/code` command.")
		return
	}

	msg := fmt.Sprintf("📸 *Pending Images: %d*\n\n", len(pendingImages))
	for i, imagePath := range pendingImages {
		filename := filepath.Base(imagePath)
		msg += fmt.Sprintf("%d. `%s`\n", i+1, filename)
	}
	msg += "\nThese images will be included in your next `/code` command.\nUse `/clear_images` to remove them."

	core.SendMessage(ctx, b, message.Chat.ID, msg)
}

func handleClearImagesCommand(ctx context.Context, message *models.Message) {
	// Only admin can clear images
	if message.From.ID != AdminUserID {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Only admin can clear images.")
		return
	}

	count := getPendingImageCount(AdminUserID)
	if count > 0 {
		clearPendingImages(AdminUserID)
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🗑️ Cleared %d pending image(s).", count))
	} else {
		core.SendMessage(ctx, b, message.Chat.ID, "📋 No pending images to clear.")
	}
}
