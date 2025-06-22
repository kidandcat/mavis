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

	"mavis/codeagent"

	"github.com/go-telegram/bot"
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
	// Track agents that failed to be removed (for retry)
	failedRemovals := make(map[string]bool)

	// Log initial state
	log.Printf("Agent monitor started. Checking agents every 5 seconds...")

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second): // Check every 5 seconds
			// Log monitoring cycle start
			log.Printf("[AgentMonitor] Starting monitoring cycle...")
			agents := agentManager.ListAgents()
			log.Printf("[AgentMonitor] Found %d agents to check", len(agents))

			for _, agent := range agents {
				runningDuration := ""
				if !agent.StartTime.IsZero() {
					runningDuration = fmt.Sprintf(", running for: %v", time.Since(agent.StartTime).Round(time.Second))
				}
				log.Printf("[AgentMonitor] Checking agent %s, status: %s, notified: %v%s", agent.ID, agent.Status, notifiedAgents[agent.ID], runningDuration)
				// Skip if we've already notified about this agent (unless removal failed)
				if notifiedAgents[agent.ID] && !failedRemovals[agent.ID] {
					log.Printf("[AgentMonitor] Agent %s already notified and not in failed removals, skipping", agent.ID)
					continue
				}

				// Note: Zombie process detection removed - agents use CombinedOutput()
				// which runs synchronously, so they cannot become zombie processes

				// Check if agent has completed (finished, failed, or killed)
				if agent.Status == codeagent.StatusFinished ||
					agent.Status == codeagent.StatusFailed ||
					agent.Status == codeagent.StatusKilled {

					// Find the user who launched this agent
					agentUserMu.RLock()
					userID, exists := agentUserMap[agent.ID]
					agentUserMu.RUnlock()

					if !exists {
						// If we don't know who launched it, skip notification but still try to remove
						log.Printf("[AgentMonitor] WARNING: No user found for agent %s, skipping notification but will try removal", agent.ID)
						// Still try to remove the agent to prevent it from being stuck
						log.Printf("[AgentMonitor] Attempting to remove orphaned agent %s from manager (folder: %s)", agent.ID, agent.Folder)
						if err := agentManager.RemoveAgent(agent.ID); err != nil {
							log.Printf("[AgentMonitor] ERROR: Failed to remove orphaned agent %s: %v", agent.ID, err)
							failedRemovals[agent.ID] = true
						} else {
							log.Printf("[AgentMonitor] Successfully removed orphaned agent %s", agent.ID)
							delete(failedRemovals, agent.ID)
							delete(notifiedAgents, agent.ID)
						}
						continue
					}

					// Mark as notified BEFORE sending to prevent any race condition
					notifiedAgents[agent.ID] = true
					log.Printf("[AgentMonitor] Marking agent %s as notified for user %d", agent.ID, userID)

					// Send notification using SendLongMessage for full output (only if not retrying)
					if !failedRemovals[agent.ID] {
						notification := formatAgentCompletionNotification(agent)
						log.Printf("[AgentMonitor] Sending completion notification for agent %s, status: %s", agent.ID, agent.Status)
						SendLongMessage(ctx, b, userID, notification)
						log.Printf("[AgentMonitor] Sent completion notification for agent %s to user %d", agent.ID, userID)
					} else {
						log.Printf("[AgentMonitor] Skipping notification for agent %s (failed removal retry)", agent.ID)
					}

					// Remove the agent from the manager now that notification is sent
					log.Printf("[AgentMonitor] Attempting to remove agent %s from manager (folder: %s)", agent.ID, agent.Folder)
					if err := agentManager.RemoveAgent(agent.ID); err != nil {
						log.Printf("[AgentMonitor] ERROR: Failed to remove agent %s: %v", agent.ID, err)
						failedRemovals[agent.ID] = true
						// Don't give up - we'll retry next cycle
						log.Printf("[AgentMonitor] Agent %s marked for removal retry", agent.ID)
					} else {
						log.Printf("[AgentMonitor] Successfully removed agent %s after notification", agent.ID)
						delete(failedRemovals, agent.ID)
						delete(notifiedAgents, agent.ID) // Clear notification flag since agent is removed

						// Clean up user tracking immediately
						agentUserMu.Lock()
						delete(agentUserMap, agent.ID)
						agentUserMu.Unlock()
						log.Printf("[AgentMonitor] Cleaned up tracking for agent %s", agent.ID)
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

// ForceCleanupStuckAgents manually removes all finished/failed/killed agents that might be stuck
// This is a recovery function for when the automatic monitor fails
func ForceCleanupStuckAgents() int {
	log.Printf("[ForceCleanup] Starting manual cleanup of stuck agents...")

	agents := agentManager.ListAgents()
	cleanedCount := 0

	for _, agent := range agents {
		// Only clean up completed agents
		if agent.Status == codeagent.StatusFinished ||
			agent.Status == codeagent.StatusFailed ||
			agent.Status == codeagent.StatusKilled {

			log.Printf("[ForceCleanup] Found stuck agent %s with status %s in folder %s", agent.ID, agent.Status, agent.Folder)

			if err := agentManager.RemoveAgent(agent.ID); err != nil {
				log.Printf("[ForceCleanup] ERROR: Failed to remove stuck agent %s: %v", agent.ID, err)
			} else {
				log.Printf("[ForceCleanup] Successfully removed stuck agent %s", agent.ID)
				cleanedCount++

				// Clean up tracking
				agentUserMu.Lock()
				delete(agentUserMap, agent.ID)
				agentUserMu.Unlock()
			}
		}
	}

	log.Printf("[ForceCleanup] Cleanup complete. Removed %d stuck agents", cleanedCount)
	return cleanedCount
}
