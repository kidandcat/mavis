// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"mavis/codeagent"
)

// agentUserMap tracks which user launched which agent
var (
	agentUserMap = make(map[string]int64) // agentID -> userID
	agentUserMu  sync.RWMutex
)

// RegisterAgentForUser associates an agent with the user who launched it
func RegisterAgentForUser(agentID string, userID int64) {
	agentUserMu.Lock()
	defer agentUserMu.Unlock()
	agentUserMap[agentID] = userID
	log.Printf("Registered agent %s for user %d", agentID, userID)
}

// MonitorAgentsProcess continuously monitors agents and sends notifications when they complete
func MonitorAgentsProcess(ctx context.Context, b *bot.Bot) {
	log.Println("Starting agent monitoring process...")

	// Keep track of agents we've already notified about
	notifiedAgents := make(map[string]bool)

	// Log initial state
	log.Printf("Agent monitor started. Checking agents every 5 seconds...")

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second): // Check every 5 seconds
			agents := agentManager.ListAgents()

			for _, agent := range agents {
				// Skip if we've already notified about this agent
				if notifiedAgents[agent.ID] {
					continue
				}

				// Check if agent has completed (finished, failed, or killed)
				if agent.Status == codeagent.StatusFinished ||
					agent.Status == codeagent.StatusFailed ||
					agent.Status == codeagent.StatusKilled {

					// Find the user who launched this agent
					agentUserMu.RLock()
					userID, exists := agentUserMap[agent.ID]
					agentUserMu.RUnlock()

					if !exists {
						// If we don't know who launched it, skip
						continue
					}

					// Mark as notified BEFORE sending to prevent any race condition
					notifiedAgents[agent.ID] = true
					log.Printf("[AgentMonitor] Marking agent %s as notified for user %d", agent.ID, userID)

					// Send notification using SendLongMessage for full output
					notification := formatAgentCompletionNotification(agent)
					log.Printf("[AgentMonitor] Sending completion notification for agent %s, status: %s", agent.ID, agent.Status)
					SendLongMessage(ctx, b, userID, notification)

					log.Printf("[AgentMonitor] Sent completion notification for agent %s to user %d", agent.ID, userID)

					// Remove the agent from the manager now that notification is sent
					if err := agentManager.RemoveAgent(agent.ID); err != nil {
						log.Printf("Failed to remove agent %s: %v", agent.ID, err)
					} else {
						log.Printf("Removed agent %s after notification", agent.ID)
					}
				}
			}

			// Clean up old agents from our tracking maps
			// Remove agents that have been completed for more than 1 hour
			for agentID, notified := range notifiedAgents {
				if notified {
					// Check if this agent still exists in the manager
					found := false
					for _, agent := range agents {
						if agent.ID == agentID {
							found = true
							break
						}
					}
					if !found {
						// Agent no longer exists, clean up
						delete(notifiedAgents, agentID)
						agentUserMu.Lock()
						delete(agentUserMap, agentID)
						agentUserMu.Unlock()
					}
				}
			}
		}
	}
}

// formatAgentCompletionNotification creates a formatted notification message for agent completion
func formatAgentCompletionNotification(agent codeagent.AgentInfo) string {
	var sb strings.Builder

	// Check if this is a usage limit error
	isUsageLimitError := false
	if agent.Status == codeagent.StatusFailed && 
		(strings.Contains(agent.Output, "Max usage limit reached") || 
		 strings.Contains(agent.Error, "Max usage limit reached")) {
		isUsageLimitError = true
	}

	// Determine status emoji and message
	statusEmoji := ""
	statusText := ""
	switch agent.Status {
	case codeagent.StatusFinished:
		statusEmoji = "✅"
		statusText = "Successfully completed"
	case codeagent.StatusFailed:
		if isUsageLimitError {
			statusEmoji = "⏰"
			statusText = "Usage limit reached"
		} else {
			statusEmoji = "❌"
			statusText = "Failed"
		}
	case codeagent.StatusKilled:
		statusEmoji = "🛑"
		statusText = "Killed"
	}

	sb.WriteString(fmt.Sprintf("%s *Code Agent Completed*\n\n", statusEmoji))
	sb.WriteString(fmt.Sprintf("🆔 ID: `%s`\n", agent.ID))
	sb.WriteString(fmt.Sprintf("📊 Status: %s\n", statusText))
	sb.WriteString(fmt.Sprintf("📝 Task: %s\n", truncateString(agent.Prompt, 100)))
	sb.WriteString(fmt.Sprintf("📁 Directory: %s\n", agent.Folder))

	if !agent.StartTime.IsZero() && !agent.EndTime.IsZero() {
		sb.WriteString(fmt.Sprintf("⏱️ Duration: %s\n", agent.Duration.Round(time.Second)))
	}

	// Special handling for usage limit errors
	if isUsageLimitError {
		sb.WriteString("\n⏰ *Usage Limit Reached*\n\n")
		sb.WriteString("Your Claude API usage limit has been reached.\n\n")
		sb.WriteString("*When do limits reset?*\n")
		sb.WriteString("• Daily limits: Reset at midnight UTC\n")
		sb.WriteString("• Monthly limits: Reset on the 1st of each month\n\n")
		sb.WriteString("*Current time (UTC):* ")
		sb.WriteString(time.Now().UTC().Format("2006-01-02 15:04:05"))
		sb.WriteString("\n\n")
		sb.WriteString("*Time until daily reset:* ")
		sb.WriteString(formatTimeUntilReset())
		sb.WriteString("\n\n")
		sb.WriteString("💡 *Tips:*\n")
		sb.WriteString("• Check your usage at https://console.anthropic.com\n")
		sb.WriteString("• Consider upgrading your plan for higher limits\n")
		sb.WriteString("• Try again after the reset time\n")
	} else {
		// Add full output if available
		if agent.Output != "" {
			sb.WriteString(fmt.Sprintf("\n📄 *Output:*\n```\n%s\n```", agent.Output))
		}

		// Add full error if available
		if agent.Error != "" {
			sb.WriteString(fmt.Sprintf("\n❌ *Error:*\n```\n%s\n```", agent.Error))
		}
	}

	// Note: Agent will be removed after this notification

	return sb.String()
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// formatTimeUntilReset calculates and formats the time until midnight UTC
func formatTimeUntilReset() string {
	now := time.Now().UTC()
	
	// Calculate next midnight UTC
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	
	// Calculate duration
	duration := nextMidnight.Sub(now)
	
	// Format the duration in a human-readable way
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	
	if hours > 0 {
		return fmt.Sprintf("%d hours %d minutes", hours, minutes)
	}
	return fmt.Sprintf("%d minutes", minutes)
}

