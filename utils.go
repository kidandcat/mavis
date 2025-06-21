// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/google/uuid"
)

func SendMessage(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	// Add debug logging to track message sending
	log.Printf("[SendMessage] Sending to chat %d, text length: %d, preview: %.50s...", chatID, len(text), text)
	
	_, err := b.SendMessage(ctx, &bot.SendMessageParams{
		Text:      bot.EscapeMarkdownUnescaped(text),
		ChatID:    chatID,
		ParseMode: models.ParseModeMarkdown,
	})
	if err != nil {
		log.Println("Error sending message", err)
	} else {
		log.Printf("[SendMessage] Successfully sent message to chat %d", chatID)
	}
}

// SendLongMessage sends a message that might be too long for Telegram's limit
// It splits the message into multiple parts if necessary
func SendLongMessage(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	const maxMessageLength = 4000 // Telegram's limit is 4096, leave some buffer

	log.Printf("[SendLongMessage] Called for chat %d, text length: %d", chatID, len(text))

	if len(text) <= maxMessageLength {
		SendMessage(ctx, b, chatID, text)
		return
	}

	// Split the message into parts
	parts := splitMessage(text, maxMessageLength)
	log.Printf("[SendLongMessage] Splitting into %d parts", len(parts))

	for i, part := range parts {
		if i > 0 {
			// Add a small delay between messages to avoid rate limiting
			time.Sleep(100 * time.Millisecond)
		}

		// Add continuation markers
		if len(parts) > 1 {
			header := fmt.Sprintf("📄 *Message Part %d/%d*\n\n", i+1, len(parts))
			part = header + part
		}

		SendMessage(ctx, b, chatID, part)
	}
}

// splitMessage intelligently splits a message into parts
func splitMessage(text string, maxLength int) []string {
	if len(text) <= maxLength {
		return []string{text}
	}

	var parts []string

	// Try to split at code block boundaries first
	if strings.Contains(text, "```") {
		parts = splitAtCodeBlocks(text, maxLength)
		if len(parts) > 0 {
			return parts
		}
	}

	// Otherwise, split at newlines
	lines := strings.Split(text, "\n")
	currentPart := ""

	for _, line := range lines {
		// If adding this line would exceed the limit
		if len(currentPart)+len(line)+1 > maxLength {
			if currentPart != "" {
				parts = append(parts, strings.TrimSpace(currentPart))
				currentPart = ""
			}

			// If a single line is too long, split it
			if len(line) > maxLength {
				words := strings.Fields(line)
				for _, word := range words {
					if len(currentPart)+len(word)+1 > maxLength {
						parts = append(parts, strings.TrimSpace(currentPart))
						currentPart = word
					} else {
						if currentPart != "" {
							currentPart += " "
						}
						currentPart += word
					}
				}
			} else {
				currentPart = line
			}
		} else {
			if currentPart != "" {
				currentPart += "\n"
			}
			currentPart += line
		}
	}

	if currentPart != "" {
		parts = append(parts, strings.TrimSpace(currentPart))
	}

	return parts
}

// splitAtCodeBlocks tries to split text at code block boundaries
func splitAtCodeBlocks(text string, maxLength int) []string {
	var parts []string
	remaining := text

	for len(remaining) > maxLength {
		// Find a good split point (prefer code block boundaries)
		splitIdx := findCodeBlockSplitPoint(remaining, maxLength)
		if splitIdx == -1 {
			// No good code block split found
			return nil
		}

		parts = append(parts, remaining[:splitIdx])
		remaining = remaining[splitIdx:]
	}

	if remaining != "" {
		parts = append(parts, remaining)
	}

	return parts
}

// findCodeBlockSplitPoint finds a good point to split at code block boundaries
func findCodeBlockSplitPoint(text string, maxLength int) int {
	// Look for closing code blocks before maxLength
	searchText := text[:min(len(text), maxLength)]
	lastCodeBlockEnd := strings.LastIndex(searchText, "```")

	if lastCodeBlockEnd != -1 {
		// Find the newline after the closing ```
		newlineIdx := strings.Index(text[lastCodeBlockEnd:], "\n")
		if newlineIdx != -1 {
			return lastCodeBlockEnd + newlineIdx + 1
		}
	}

	// Otherwise, try to split at a paragraph break
	lastDoubleNewline := strings.LastIndex(searchText, "\n\n")
	if lastDoubleNewline != -1 {
		return lastDoubleNewline + 2
	}

	// Last resort: split at any newline
	lastNewline := strings.LastIndex(searchText, "\n")
	if lastNewline != -1 {
		return lastNewline + 1
	}

	return -1
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func NewID(length int) string {
	return uuid.New().String()[:length]
}

// ResolvePath resolves a path relative to the user's home directory
// It handles:
// - Paths starting with ~ (replaced with home directory)
// - Relative paths (resolved from home directory)
// - Absolute paths (returned as-is)
func ResolvePath(path string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Handle ~ prefix
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:]), nil
	} else if path == "~" {
		return homeDir, nil
	}

	// If path is already absolute, return it
	if filepath.IsAbs(path) {
		return path, nil
	}

	// Otherwise, treat it as relative to home directory
	return filepath.Join(homeDir, path), nil
}

// SendFile sends a file document to a Telegram chat
func SendFile(ctx context.Context, b *bot.Bot, chatID int64, filePath string, caption string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	params := &bot.SendDocumentParams{
		ChatID: chatID,
		Document: &models.InputFileUpload{
			Filename: fileInfo.Name(),
			Data:     file,
		},
	}

	if caption != "" {
		params.Caption = bot.EscapeMarkdownUnescaped(caption)
		params.ParseMode = models.ParseModeMarkdown
	}

	_, err = b.SendDocument(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to send document: %w", err)
	}

	return nil
}

// GetmDNSInstructions returns instructions for setting up mDNS
func GetmDNSInstructions() string {
	return `To use the .local domain, ensure mDNS/Bonjour is configured:

**macOS**: Built-in support, no configuration needed
**Linux**: Install avahi-daemon: sudo apt-get install avahi-daemon
**Windows**: Install Bonjour Print Services from Apple

For manual configuration, add to /etc/hosts:
<your-server-ip> mavis.local`
}

// IsPortInUse checks if a port is already in use
func IsPortInUse(port string) bool {
	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		// Port is in use or error occurred
		return true
	}
	ln.Close()
	return false
}

// FindAvailablePort finds an available port starting from the given port
// It tries the original port first, then increments until it finds an available one
func FindAvailablePort(startPort string) (string, error) {
	port, err := strconv.Atoi(startPort)
	if err != nil {
		return "", fmt.Errorf("invalid port number: %s", startPort)
	}

	// Try up to 100 ports
	for i := 0; i < 100; i++ {
		currentPort := strconv.Itoa(port + i)
		if !IsPortInUse(currentPort) {
			return currentPort, nil
		}
	}

	return "", fmt.Errorf("no available port found in range %d-%d", port, port+99)
}

