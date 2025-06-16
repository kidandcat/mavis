// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"

	"mavis/codeagent"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/joho/godotenv"
)

var (
	AdminUserID     int64
	b               *bot.Bot
	agentManager    *codeagent.Manager
	OnlineServerURL string
)

// Tunnel process tracking
var (
	ngrokProcess  *os.Process
	buildProcess  *os.Process
	ngrokPort     string
	ngrokWorkDir  string
	ngrokBuildCmd string
	ngrokMutex    sync.Mutex
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

	// Load optional online server URL
	OnlineServerURL = os.Getenv("ONLINE_SERVER_URL")
	if OnlineServerURL == "" {
		OnlineServerURL = "" // Use default server
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

		handleMessage(ctx, update.Message)
	}
}

func helloHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: update.Message.Chat.ID,
		Text:   fmt.Sprintf("Hello %s! I'm Mavis, a code agent manager. Use /help to see available commands.", update.Message.Chat.FirstName),
	})
}
