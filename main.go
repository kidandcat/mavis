// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"mavis/codeagent"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/joho/godotenv"
)

var (
	AdminUserID     int64
	b               *bot.Bot
	agentManager    *codeagent.Manager
)

// LAN server tracking
var (
	lanServerProcess *os.Process
	lanServerPort    string
	lanServerWorkDir string
	lanServerCmd     string
	lanServerMutex   sync.Mutex
	lanDomainName    = "mavis.local"
)

// Image tracking for users
var (
	userPendingImages = make(map[int64][]string) // userID -> array of image paths
	pendingImagesMutex sync.RWMutex
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	}

	// Load environment variables
	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if telegramBotToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	adminUserIDStr := os.Getenv("ADMIN_USER_ID")
	if adminUserIDStr == "" {
		log.Fatal("ADMIN_USER_ID environment variable is required")
	}

	var err error
	AdminUserID, err = strconv.ParseInt(adminUserIDStr, 10, 64)
	if err != nil {
		log.Fatal("ADMIN_USER_ID must be a valid integer:", err)
	}


	// Create data directory if it doesn't exist
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatal("Failed to create data directory:", err)
	}

	// Initialize authorization system
	if err := InitAuthorization(); err != nil {
		log.Fatal("Failed to initialize authorization system:", err)
	}

	// Initialize code agent manager
	agentManager = codeagent.NewManager()

	b, err = bot.New(telegramBotToken, bot.WithDefaultHandler(handler))
	if err != nil {
		panic("Error creating bot (telegram token: " + telegramBotToken + "): " + err.Error())
	}
	b.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, helloHandler)
	// Code agent commands are handled in handleMessage function

	go MonitorAgentsProcess(ctx, b)
	go cleanupOldTempFiles(ctx)

	// Send startup notification to admin
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: AdminUserID,
		Text:   "🚀 Mavis ready",
	})
	if err != nil {
		log.Printf("Failed to send startup notification: %v", err)
	}

	log.Println("Ready")
	b.Start(ctx)
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil {
		// Check if user is authorized
		if !authorizedUsers.IsAuthorized(update.Message.From.ID) {
			// Notify admin about unauthorized access attempt
			username := update.Message.From.Username
			if username == "" {
				username = fmt.Sprintf("%s %s", update.Message.From.FirstName, update.Message.From.LastName)
			}

			adminNotification := fmt.Sprintf("⚠️ *Unauthorized Access Attempt*\n\n"+
				"👤 User: %s\n"+
				"🆔 User ID: `%d`\n"+
				"💬 Message: %s\n\n"+
				"To authorize this user, use:\n`/adduser %s %d`",
				username, update.Message.From.ID, update.Message.Text, username, update.Message.From.ID)

			SendMessage(ctx, b, AdminUserID, adminNotification)

			// Send message to unauthorized user
			SendMessage(ctx, b, update.Message.Chat.ID,
				"❌ You are not authorized to use this bot. The admin has been notified of your request.")
			return
		}

		// Handle photo messages
		if update.Message.Photo != nil && len(update.Message.Photo) > 0 {
			handlePhotoMessage(ctx, update.Message)
			return
		}

		// Handle document messages (for non-photo image files)
		if update.Message.Document != nil {
			handleDocumentMessage(ctx, update.Message)
			return
		}

		handleMessage(ctx, update.Message)
	}
}

func helloHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Hello %s! I'm Mavis, a code agent manager. Use /help to see available commands.", update.Message.Chat.FirstName),
	})
}

func handlePhotoMessage(ctx context.Context, message *models.Message) {
	userID := message.From.ID
	
	// Get the largest photo size
	photo := message.Photo[len(message.Photo)-1]
	
	// Download the photo
	filePath, err := downloadTelegramFile(ctx, photo.FileID, userID, "jpg")
	if err != nil {
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to download photo: %v", err))
		return
	}
	
	// Add to pending images
	addPendingImage(userID, filePath)
	
	// Get pending count
	count := getPendingImageCount(userID)
	
	SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("📸 Photo saved! You have %d pending image(s).\n\nThese images will be included in your next `/code` command.", count))
}

func handleDocumentMessage(ctx context.Context, message *models.Message) {
	userID := message.From.ID
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
		SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to download image: %v", err))
		return
	}
	
	// Add to pending images
	addPendingImage(userID, filePath)
	
	// Get pending count
	count := getPendingImageCount(userID)
	
	SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🖼️ Image saved! You have %d pending image(s).\n\nThese images will be included in your next `/code` command.", count))
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

func addPendingImage(userID int64, imagePath string) {
	pendingImagesMutex.Lock()
	defer pendingImagesMutex.Unlock()
	
	userPendingImages[userID] = append(userPendingImages[userID], imagePath)
}

func getPendingImageCount(userID int64) int {
	pendingImagesMutex.RLock()
	defer pendingImagesMutex.RUnlock()
	
	return len(userPendingImages[userID])
}

func getPendingImages(userID int64) []string {
	pendingImagesMutex.RLock()
	defer pendingImagesMutex.RUnlock()
	
	images := make([]string, len(userPendingImages[userID]))
	copy(images, userPendingImages[userID])
	return images
}

func clearPendingImages(userID int64) {
	pendingImagesMutex.Lock()
	defer pendingImagesMutex.Unlock()
	
	// Delete the image files
	if images, exists := userPendingImages[userID]; exists {
		for _, imagePath := range images {
			os.Remove(imagePath)
		}
	}
	
	delete(userPendingImages, userID)
}

func cleanupOldTempFiles(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour) // Run cleanup every hour
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Clean up temp directory
			tempDir := filepath.Join("data", "temp")
			entries, err := os.ReadDir(tempDir)
			if err != nil {
				continue
			}
			
			now := time.Now()
			for _, entry := range entries {
				if entry.IsDir() {
					dirPath := filepath.Join(tempDir, entry.Name())
					info, err := entry.Info()
					if err != nil {
						continue
					}
					
					// Remove directories older than 24 hours
					if now.Sub(info.ModTime()) > 24*time.Hour {
						os.RemoveAll(dirPath)
						log.Printf("Cleaned up old temp directory: %s", dirPath)
					}
				}
			}
		case <-ctx.Done():
			return
		}
	}
}
