// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package web

import (
	"mavis/codeagent"
	"os"
	"path/filepath"

	"github.com/go-telegram/bot"
)

var (
	// Global references set during initialization
	b            *bot.Bot
	agentManager *codeagent.Manager
	AdminUserID  int64
	ProjectDir   string
)

// InitializeGlobals sets up the global references needed by the web package
func InitializeGlobals(botInstance *bot.Bot, manager *codeagent.Manager, adminID int64, projectDir string) {
	b = botInstance
	agentManager = manager
	AdminUserID = adminID
	ProjectDir = projectDir

	// Initialize MCP store with user-level config
	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".config", "mavis")
	mcpConfigFile := filepath.Join(configDir, "mcps.json")
	mcpStore = NewMCPStore(mcpConfigFile)
}
