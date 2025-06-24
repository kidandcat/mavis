// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"mavis/codeagent"
	"mavis/core"
	"mavis/telegram"
	"mavis/web"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/joho/godotenv"
)

var (
	AdminUserID  int64
	Bot          *bot.Bot // Exported for use by other packages
	agentManager *codeagent.Manager
	ProjectDir   string
)

// LAN server tracking
var (
	lanServerProcess *os.Process
	lanHTTPServer    *http.Server
	lanServerPort    string
	lanServerWorkDir string
	lanServerCmd     string
	lanServerMutex   sync.Mutex
	lanDomainName    = "mavis.local"
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

	// Store the project directory before changing working directory
	ProjectDir, err = os.Getwd()
	if err != nil {
		log.Fatal("Failed to get current working directory:", err)
	}
	log.Printf("Project directory: %s", ProjectDir)

	// Change working directory to user home
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Failed to get user home directory:", err)
	}
	if err := os.Chdir(homeDir); err != nil {
		log.Fatal("Failed to change to home directory:", err)
	}
	log.Printf("Changed working directory to: %s", homeDir)

	// Create data directory if it doesn't exist
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatal("Failed to create data directory:", err)
	}

	// User store and authorization removed - single user mode

	// Initialize code agent manager
	agentManager = codeagent.NewManager()

	// Set callback for when queued agents start
	agentManager.SetAgentStartCallback(func(agentID, folder, prompt, queueID string) {
		log.Printf("[StartCallback] Called for agent %s, queueID: %s", agentID, queueID)
		// Get the queued agent info to find the user
		if queueInfo, exists := core.GetQueueTracker().GetQueuedAgentInfo(queueID); exists {
			// Register the agent for the user who queued it
			telegram.RegisterAgentForUser(agentID, queueInfo.UserID)
			log.Printf("[StartCallback] Successfully registered agent %s for user %d", agentID, queueInfo.UserID)

			// Notify the user that their queued agent has started
			core.SendMessage(ctx, Bot, queueInfo.UserID, fmt.Sprintf("🏃 Queued agent started!\n🆔 ID: `%s`\n📁 Directory: %s\n📝 Task: %s\n\nUse `/status %s` to check status.",
				agentID, folder, prompt, agentID))

			// Remove from queue tracker
			core.GetQueueTracker().RemoveQueuedAgent(queueID)

			log.Printf("Queued agent started: ID=%s, Folder=%s, User=%d", agentID, folder, queueInfo.UserID)
		} else {
			log.Printf("WARNING: Queued agent started but no queue info found: ID=%s, QueueID=%s", agentID, queueID)
			log.Printf("WARNING: This agent will NOT receive completion notifications!")
		}
	})

	// Create bot with custom error handler
	Bot, err = bot.New(telegramBotToken,
		bot.WithDefaultHandler(handler),
		bot.WithErrorsHandler(handleBotError))
	if err != nil {
		panic("Error creating bot (telegram token: " + telegramBotToken + "): " + err.Error())
	}
	Bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, helloHandler)
	// Code agent commands are handled in handleMessage function

	// Initialize global variables in packages
	telegram.InitializeGlobals(Bot, agentManager, AdminUserID)
	web.InitializeGlobals(Bot, agentManager, AdminUserID, ProjectDir)
	core.InitializeGlobals(AdminUserID)

	go telegram.MonitorAgentsProcess(ctx, Bot)
	go telegram.RecoveryCheck(ctx, Bot)
	go cleanupOldTempFiles(ctx)

	// Start web server if enabled
	webPort := os.Getenv("WEB_PORT")
	if webPort != "" {
		go func() {
			log.Printf("Starting web server on port %s", webPort)
			if err := web.StartWebServer(webPort); err != nil && err != http.ErrServerClosed {
				log.Printf("Web server error: %v", err)
			}
		}()
	}

	// Send startup notification to admin
	startupMsg := "🚀 Mavis ready"
	if webPort != "" {
		startupMsg += fmt.Sprintf("\n🌐 Web interface: http://localhost:%s", webPort)
	}
	_, err = Bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: AdminUserID,
		Text:   startupMsg,
	})
	if err != nil {
		log.Printf("Failed to send startup notification: %v", err)
	}

	log.Println("Ready")

	// Start bot with error handling
	if err := startBotWithErrorHandling(ctx); err != nil {
		log.Printf("[TGBOT] [ERROR] Bot stopped with error: %v", err)
	}
}

func handleBotError(err error) {
	if err == nil {
		return
	}

	errStr := err.Error()
	// Check if it's the specific conflict error
	if contains(errStr, "error get updates") && contains(errStr, "conflict") &&
		contains(errStr, "Conflict: terminated by other getUpdates request") {
		log.Printf("[TGBOT] [ERROR] %s", errStr)
		// Send danger message
		ctx := context.Background()
		sendDangerMessage(ctx, "⚠️ DANGER: Another Telegram bot instance is running!\n\nThe bot detected a conflict - another instance is already polling for updates. Only one bot instance can run at a time.\n\nPlease stop the other instance and restart this bot.")
		// Do nothing else - the bot will handle this error internally
		return
	}

	// Log other errors
	log.Printf("[TGBOT] [ERROR] Bot error: %v", err)
}

func startBotWithErrorHandling(ctx context.Context) error {
	// Use a channel to capture panic from bot.Start
	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errChan <- fmt.Errorf("bot panicked: %v", r)
			}
		}()

		// Start the bot
		Bot.Start(ctx)
		errChan <- nil
	}()

	// Monitor for errors
	for {
		select {
		case err := <-errChan:
			if err != nil {
				return err
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && strings.Contains(s, substr))
}

func sendDangerMessage(ctx context.Context, message string) {
	// Try to send a danger message to admin
	// We can't use the bot since it's in error state, so we'll log it prominently
	log.Printf("\n"+
		"════════════════════════════════════════════════\n"+
		"⚠️  DANGER - BOT CONFLICT DETECTED ⚠️\n"+
		"════════════════════════════════════════════════\n"+
		"%s\n"+
		"════════════════════════════════════════════════\n", message)

	// If we can, try to send via bot (might fail)
	if Bot != nil {
		_, _ = Bot.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: AdminUserID,
			Text:   message,
		})
	}
}

func handler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message != nil {
		// Check if user is the authorized admin
		if update.Message.From.ID != AdminUserID {
			// Send message to unauthorized user
			core.SendMessage(ctx, Bot, update.Message.Chat.ID,
				"❌ You are not authorized to use this bot.")
			return
		}

		// Handle photo messages
		if update.Message.Photo != nil && len(update.Message.Photo) > 0 {
			telegram.HandlePhotoMessage(ctx, update.Message)
			return
		}

		// Handle document messages (for non-photo image files)
		if update.Message.Document != nil {
			telegram.HandleDocumentMessage(ctx, update.Message)
			return
		}

		telegram.HandleMessage(ctx, update.Message)
	}
}

func helloHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	Bot.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Hello %s! I'm Mavis, a code agent manager. Use /help to see available commands.", update.Message.Chat.FirstName),
	})
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
