// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"mavis/codeagent"
	"mavis/core"
	"mavis/soul"
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
	soulManager  *soul.ManagerSQLite
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
	log.Println("[STARTUP] Starting Mavis application...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.Println("[STARTUP] Loading environment variables...")
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system environment variables")
	} else {
		log.Println("[STARTUP] .env file loaded successfully")
	}

	log.Println("[STARTUP] Validating required environment variables...")
	// Load environment variables
	telegramBotToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if telegramBotToken == "" {
		log.Fatal("[STARTUP] TELEGRAM_BOT_TOKEN environment variable is required")
	}
	log.Println("[STARTUP] TELEGRAM_BOT_TOKEN found")

	adminUserIDStr := os.Getenv("ADMIN_USER_ID")
	if adminUserIDStr == "" {
		log.Fatal("[STARTUP] ADMIN_USER_ID environment variable is required")
	}
	log.Println("[STARTUP] ADMIN_USER_ID found")

	var err error
	AdminUserID, err = strconv.ParseInt(adminUserIDStr, 10, 64)
	if err != nil {
		log.Fatal("[STARTUP] ADMIN_USER_ID must be a valid integer:", err)
	}
	log.Printf("[STARTUP] Admin User ID parsed: %d", AdminUserID)

	log.Println("[STARTUP] Setting up working directories...")
	// Store the project directory before changing working directory
	ProjectDir, err = os.Getwd()
	if err != nil {
		log.Fatal("[STARTUP] Failed to get current working directory:", err)
	}
	log.Printf("[STARTUP] Project directory: %s", ProjectDir)

	// Change working directory to user home
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("[STARTUP] Failed to get user home directory:", err)
	}
	log.Printf("[STARTUP] User home directory: %s", homeDir)

	if err := os.Chdir(homeDir); err != nil {
		log.Fatal("[STARTUP] Failed to change to home directory:", err)
	}
	log.Printf("[STARTUP] Changed working directory to: %s", homeDir)

	log.Println("[STARTUP] Creating data directory...")
	// Create data directory if it doesn't exist
	if err := os.MkdirAll("data", 0755); err != nil {
		log.Fatal("[STARTUP] Failed to create data directory:", err)
	}
	log.Println("[STARTUP] Data directory created/verified")

	// User store and authorization removed - single user mode

	log.Println("[STARTUP] Initializing code agent manager...")
	// Initialize code agent manager
	agentManager = codeagent.NewManager()
	log.Println("[STARTUP] Code agent manager initialized")

	log.Println("[STARTUP] Initializing soul manager...")
	// Initialize soul manager
	configDir := filepath.Join(homeDir, ".config", "mavis")
	log.Printf("[STARTUP] Soul manager config directory: %s", configDir)

	log.Println("[STARTUP] Creating soul manager...")
	soulManager, err = soul.NewManagerSQLite(configDir)
	if err != nil {
		log.Fatal("[STARTUP] Failed to initialize soul manager:", err)
	}
	log.Println("[STARTUP] Soul manager initialized")

	log.Println("[STARTUP] Setting up agent callbacks...")
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
	log.Println("[STARTUP] Agent callbacks configured")

	log.Println("[STARTUP] Creating Telegram bot...")
	// Create bot with custom error handler
	Bot, err = bot.New(telegramBotToken,
		bot.WithDefaultHandler(handler),
		bot.WithErrorsHandler(handleBotError))
	if err != nil {
		log.Fatal("[STARTUP] Error creating bot (telegram token: " + telegramBotToken + "): " + err.Error())
	}
	log.Println("[STARTUP] Telegram bot created successfully")

	log.Println("[STARTUP] Registering bot handlers...")
	Bot.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeExact, helloHandler)
	// Code agent commands are handled in handleMessage function
	log.Println("[STARTUP] Bot handlers registered")

	log.Println("[STARTUP] Initializing package globals...")
	// Initialize global variables in packages
	telegram.InitializeGlobals(Bot, agentManager, AdminUserID)
	log.Println("[STARTUP] Telegram globals initialized")

	web.InitializeGlobals(Bot, agentManager, soulManager, AdminUserID, ProjectDir)
	log.Println("[STARTUP] Web globals initialized")

	core.InitializeGlobals(AdminUserID)
	log.Println("[STARTUP] Core globals initialized")

	// Initialize UPnP (optional feature)
	log.Println("[STARTUP] Initializing UPnP support...")
	telegram.InitializeUPnP()
	log.Println("[STARTUP] UPnP initialization complete")

	log.Println("[STARTUP] Starting background processes...")
	go telegram.MonitorAgentsProcess(ctx, Bot)
	log.Println("[STARTUP] Agent monitor process started")

	go telegram.RecoveryCheck(ctx, Bot)
	log.Println("[STARTUP] Recovery check process started")

	go cleanupOldTempFiles(ctx)
	log.Println("[STARTUP] Cleanup process started")

	// Start web server if enabled
	webPort := os.Getenv("WEB_PORT")
	if webPort != "" {
		log.Printf("[STARTUP] Web server enabled on port %s", webPort)
		go func() {
			log.Printf("[WEB] Starting web server on port %s", webPort)
			if err := web.StartWebServer(webPort); err != nil && err != http.ErrServerClosed {
				log.Printf("[WEB] [ERROR] Web server error: %v", err)
			}
		}()
	} else {
		log.Println("[STARTUP] Web server disabled (WEB_PORT not set)")
	}

	log.Println("[STARTUP] Sending startup notification to admin...")
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
		log.Printf("[STARTUP] [WARNING] Failed to send startup notification: %v", err)
	} else {
		log.Println("[STARTUP] Startup notification sent successfully")
	}

	log.Println("[STARTUP] All initialization complete - starting bot...")

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, 
		syscall.SIGINT,    // Interrupt (Ctrl+C)
		syscall.SIGTERM,   // Termination
		syscall.SIGQUIT,   // Quit
		syscall.SIGHUP,    // Hangup
		syscall.SIGUSR1,   // User-defined signal 1
		syscall.SIGUSR2,   // User-defined signal 2
	)

	// Start signal handler goroutine
	go func() {
		sig := <-sigChan
		log.Printf("[SIGNAL] Received signal: %s (%d)", sig.String(), sig)
		
		// Send shutdown notification to admin
		shutdownMsg := fmt.Sprintf("🛑 Mavis shutting down\n📡 Signal: %s", sig.String())
		_, _ = Bot.SendMessage(context.Background(), &bot.SendMessageParams{
			ChatID: AdminUserID,
			Text:   shutdownMsg,
		})

		// Cancel context to trigger graceful shutdown
		cancel()
		
		// Give some time for cleanup
		time.Sleep(2 * time.Second)
		
		log.Printf("[SIGNAL] Exiting due to signal: %s", sig.String())
		os.Exit(0)
	}()

	// Start bot with error handling
	if err := startBotWithErrorHandling(ctx); err != nil {
		log.Printf("[TGBOT] [ERROR] Bot stopped with error: %v", err)
	}
}

func handleBotError(err error) {
	if err == nil {
		return
	}

	log.Printf("[TGBOT] [ERROR] Bot error occurred: %v", err)

	errStr := err.Error()
	// Check if it's the specific conflict error
	if contains(errStr, "error get updates") && contains(errStr, "conflict") &&
		contains(errStr, "Conflict: terminated by other getUpdates request") {
		log.Printf("[TGBOT] [CONFLICT] Detected bot instance conflict!")
		log.Printf("[TGBOT] [CONFLICT] Full error: %s", errStr)
		// Send danger message
		ctx := context.Background()
		sendDangerMessage(ctx, "⚠️ DANGER: Another Telegram bot instance is running!\n\nThe bot detected a conflict - another instance is already polling for updates. Only one bot instance can run at a time.\n\nPlease stop the other instance and restart this bot.")
		// Do nothing else - the bot will handle this error internally
		return
	}

	// Log other errors with more detail
	log.Printf("[TGBOT] [ERROR] Unhandled bot error: %v", err)
	log.Printf("[TGBOT] [ERROR] Error type: %T", err)
}

func startBotWithErrorHandling(ctx context.Context) error {
	log.Println("[TGBOT] Initializing bot startup with error handling...")

	// Use a channel to capture panic from bot.Start
	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[TGBOT] [PANIC] Bot panicked during startup: %v", r)
				errChan <- fmt.Errorf("bot panicked: %v", r)
			}
		}()

		log.Println("[TGBOT] Starting bot polling for updates...")
		// Start the bot
		Bot.Start(ctx)
		log.Println("[TGBOT] Bot polling stopped normally")
		errChan <- nil
	}()

	log.Println("[TGBOT] Monitoring for bot errors and context cancellation...")
	// Monitor for errors
	for {
		select {
		case err := <-errChan:
			if err != nil {
				log.Printf("[TGBOT] [ERROR] Bot error received: %v", err)
				return err
			}
			log.Println("[TGBOT] Bot startup completed successfully")
			return nil
		case <-ctx.Done():
			log.Printf("[TGBOT] Context cancelled: %v", ctx.Err())
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
