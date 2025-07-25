// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package telegram

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"mavis/codeagent"
	"mavis/core"

	"github.com/go-telegram/bot"
)

// Single-user mode - no need for agent-user mapping

// RegisterAgentForUser - no longer needed in single-user mode
func RegisterAgentForUser(agentID string, userID int64) {
	// No-op in single-user mode
	log.Printf("Agent %s started (single-user mode)", agentID)
}

// MonitorAgentsProcess continuously monitors agents and sends notifications when they complete
func MonitorAgentsProcess(ctx context.Context, b *bot.Bot) {
	log.Println("Starting agent monitoring process...")

	// Keep track of agents we've already notified about
	notifiedAgents := make(map[string]bool)
	// Track agents that failed to be removed (for retry)
	failedRemovals := make(map[string]bool)
	// Track when agents finished for 10-minute cleanup
	finishedAgentTimes := make(map[string]time.Time)

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

					// In single-user mode, all agents belong to AdminUserID
					userID := AdminUserID

					// Mark as notified BEFORE sending to prevent any race condition
					notifiedAgents[agent.ID] = true
					log.Printf("[AgentMonitor] Marking agent %s as notified for user %d", agent.ID, userID)

					// Send notification using SendLongMessage for full output (only if not retrying)
					if !failedRemovals[agent.ID] {
						notification := formatAgentCompletionNotification(agent, userID)
						log.Printf("[AgentMonitor] Sending completion notification for agent %s, status: %s", agent.ID, agent.Status)

						// For web users (userID 0), send to admin's Telegram
						telegramUserID := userID
						if userID == 0 {
							telegramUserID = AdminUserID
							log.Printf("[AgentMonitor] Web-launched agent %s, sending notification to admin (ID: %d)", agent.ID, AdminUserID)
						}

						core.SendLongMessage(ctx, b, telegramUserID, notification)
						log.Printf("[AgentMonitor] Sent completion notification for agent %s to user %d", agent.ID, telegramUserID)

						// TODO: Broadcast SSE event for web interface
						// eventType := "agent_completed"
						// if agent.Status == "failed" {
						// 	eventType = "agent_failed"
						// }
						// BroadcastSSEEvent(eventType, map[string]interface{}{
						// 	"agent_id":     agent.ID,
						// 	"status":       agent.Status,
						// 	"directory":    agent.Folder,
						// 	"output":       agent.Output, // Include full output
						// 	"error":        agent.Error,  // Include error if any
						// 	"notification": notification, // Include formatted notification
						// })
					} else {
						log.Printf("[AgentMonitor] Skipping notification for agent %s (failed removal retry)", agent.ID)
					}

					// MODIFIED: Track completion time for 10-minute delayed cleanup
					log.Printf("[AgentMonitor] Agent %s completed, will be removed after 10 minutes", agent.ID)
					finishedAgentTimes[agent.ID] = time.Now()
					
					// We need to clear the running status for this folder to allow queue processing
					// This is normally done by RemoveAgent, but since we're not removing, do it manually
					if agent.Folder != "" {
						log.Printf("[AgentMonitor] Clearing running status and processing queue for folder: %s", agent.Folder)
						// Call ProcessQueueForFolder which will clear runningPerFolder and start next queued task
						agentManager.ProcessQueueForFolder(agent.Folder)
					}
					
					// Comment out the automatic removal
					/*
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

						// Clean up tracking immediately
						UnregisterAgent(agent.ID)
						log.Printf("[AgentMonitor] Cleaned up tracking for agent %s", agent.ID)
					}
					*/
				}
			}

			// Clean up agents that have been finished for more than 10 minutes
			now := time.Now()
			for agentID, finishTime := range finishedAgentTimes {
				if now.Sub(finishTime) > 10*time.Minute {
					log.Printf("[AgentMonitor] Agent %s has been finished for > 10 minutes, removing", agentID)
					
					// Remove the agent from the manager
					if err := agentManager.RemoveAgent(agentID); err != nil {
						log.Printf("[AgentMonitor] ERROR: Failed to remove agent %s: %v", agentID, err)
						// Don't retry infinitely - if it fails after 10 minutes, remove from tracking
						if now.Sub(finishTime) > 15*time.Minute {
							log.Printf("[AgentMonitor] Giving up on removing agent %s after 15 minutes", agentID)
							delete(finishedAgentTimes, agentID)
							delete(notifiedAgents, agentID)
							UnregisterAgent(agentID)
						}
					} else {
						log.Printf("[AgentMonitor] Successfully removed agent %s after 10 minutes", agentID)
						delete(finishedAgentTimes, agentID)
						delete(notifiedAgents, agentID)
						delete(failedRemovals, agentID)
						UnregisterAgent(agentID)
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
						delete(finishedAgentTimes, agentID)
						UnregisterAgent(agentID)
					}
				}
			}
		}
	}
}

// formatAgentCompletionNotification creates a formatted notification message for agent completion
func formatAgentCompletionNotification(agent codeagent.AgentInfo, userID int64) string {
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
	if userID == 0 {
		sb.WriteString("🌐 Source: Web Interface\n")
	}
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
				UnregisterAgent(agent.ID)
			}
		}
	}

	log.Printf("[ForceCleanup] Cleanup complete. Removed %d stuck agents", cleanedCount)
	return cleanedCount
}

// RecoveryCheck performs periodic checks for stuck agents and queues
func RecoveryCheck(ctx context.Context, b *bot.Bot) {
	log.Println("[Recovery] Starting recovery check process...")

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[Recovery] Recovery check process stopped")
			return
		case <-ticker.C:
			performRecoveryCheck(b)
		}
	}
}

// performRecoveryCheck does the actual recovery work
func performRecoveryCheck(b *bot.Bot) {
	log.Printf("[Recovery] Performing recovery check...")

	// 1. Check for agents marked as running but process is dead
	agents := agentManager.ListAgents()
	deadAgents := 0

	for _, agent := range agents {
		if agent.Status == codeagent.StatusRunning {
			// Check if the agent has been running for too long (e.g., > 30 minutes)
			if !agent.StartTime.IsZero() && time.Since(agent.StartTime) > 30*time.Minute {
				log.Printf("[Recovery] WARNING: Agent %s has been running for %v", agent.ID, time.Since(agent.StartTime))

				// Get the actual agent to check if process is alive
				actualAgent, err := agentManager.GetAgent(agent.ID)
				if err != nil {
					log.Printf("[Recovery] ERROR: Failed to get agent %s: %v", agent.ID, err)
					continue
				}

				if !actualAgent.IsProcessAlive() {
					log.Printf("[Recovery] DETECTED: Agent %s process is dead, marking as failed", agent.ID)
					
					// Capture more context about the failure
					errorDetails := fmt.Sprintf("Process died unexpectedly (detected by recovery check)\n\n"+
						"Agent ID: %s\n"+
						"Running Duration: %v\n"+
						"Last Known Status: %s",
						agent.ID, 
						time.Since(agent.StartTime).Round(time.Second),
						agent.Status)
					
					actualAgent.MarkAsFailedWithDetails(errorDetails)
					deadAgents++

					// The monitor will pick up the failed status and handle notification/removal
				}
			}
		}
	}

	if deadAgents > 0 {
		log.Printf("[Recovery] Found %d dead agents", deadAgents)
	}

	// 2. Check for folders with queues but no running agent
	detailedQueueStatus := agentManager.GetDetailedQueueStatus()
	stuckQueues := 0

	for folder, tasks := range detailedQueueStatus {
		if len(tasks) > 0 {
			// Check if there's a running agent for this folder
			hasRunning, agentID := agentManager.IsAgentRunningInFolder(folder)
			if !hasRunning {
				log.Printf("[Recovery] WARNING: Folder %s has %d queued tasks but no running agent", folder, len(tasks))
				stuckQueues++

				// Send notification about stuck queue
				if b != nil && len(tasks) > 0 {
					// Find user ID from first queued task
					firstTask := tasks[0]
					if queueInfo, exists := core.GetQueueTracker().GetQueuedAgentInfo(firstTask.QueueID); exists {
						notification := fmt.Sprintf("⚠️ *Stuck Queue Detected*\n\n"+
							"📁 Folder: %s\n"+
							"📊 Queued tasks: %d\n"+
							"📝 First task: %s\n\n"+
							"No agent is running in this folder. Attempting to process the queue...",
							folder, len(tasks), truncateString(firstTask.Prompt, 100))
						core.SendMessage(context.Background(), b, queueInfo.UserID, notification)
					}
				}

				// Try to process the queue for this folder
				log.Printf("[Recovery] Attempting to process stuck queue for folder %s", folder)
				agentManager.ProcessQueueForFolder(folder)
			} else {
				// Check if the running agent actually exists
				if _, err := agentManager.GetAgent(agentID); err != nil {
					log.Printf("[Recovery] ERROR: Folder %s claims agent %s is running but it doesn't exist", folder, agentID)
					stuckQueues++

					// Clear the invalid running agent and process queue
					log.Printf("[Recovery] Clearing invalid agent reference and processing queue for folder %s", folder)
					agentManager.ProcessQueueForFolder(folder)
				}
			}
		}
	}

	if stuckQueues > 0 {
		log.Printf("[Recovery] Found %d stuck queues", stuckQueues)
	}

	// 3. Check for orphaned agents without user association
	orphanedAgents := 0
	for _, agent := range agents {
		// In single-user mode, all agents have a user
		hasUser := true

		if !hasUser && agent.Status == codeagent.StatusRunning {
			log.Printf("[Recovery] WARNING: Running agent %s has no user association", agent.ID)
			orphanedAgents++

			// Assign the orphaned agent to the admin user
			RegisterAgentForUser(agent.ID, AdminUserID)
			log.Printf("[Recovery] Assigned orphaned agent %s to admin user (ID: %d)", agent.ID, AdminUserID)

			// Send notification to admin about the orphaned agent
			if b != nil {
				notification := fmt.Sprintf("⚠️ *Orphaned Agent Recovered*\n\n"+
					"🆔 Agent ID: `%s`\n"+
					"📁 Folder: %s\n"+
					"📝 Task: %s\n"+
					"🕐 Running since: %s\n\n"+
					"This agent had no user association and has been assigned to you.",
					agent.ID, agent.Folder, truncateString(agent.Prompt, 100),
					agent.StartTime.Format("15:04:05"))
				core.SendMessage(context.Background(), b, AdminUserID, notification)
			}
		}
	}

	if orphanedAgents > 0 {
		log.Printf("[Recovery] Found %d orphaned agents", orphanedAgents)
	}

	// 4. Clean up completed agents that have been around for too long
	oldCompletedAgents := 0
	for _, agent := range agents {
		if (agent.Status == codeagent.StatusFinished ||
			agent.Status == codeagent.StatusFailed ||
			agent.Status == codeagent.StatusKilled) &&
			!agent.EndTime.IsZero() &&
			time.Since(agent.EndTime) > 1*time.Hour {

			log.Printf("[Recovery] Cleaning up old completed agent %s (ended %v ago)", agent.ID, time.Since(agent.EndTime))
			if err := agentManager.RemoveAgent(agent.ID); err != nil {
				log.Printf("[Recovery] ERROR: Failed to remove old agent %s: %v", agent.ID, err)
			} else {
				oldCompletedAgents++

				// Clean up tracking
				UnregisterAgent(agent.ID)
			}
		}
	}

	if oldCompletedAgents > 0 {
		log.Printf("[Recovery] Cleaned up %d old completed agents", oldCompletedAgents)
	}

	totalIssues := deadAgents + stuckQueues + orphanedAgents + oldCompletedAgents
	if totalIssues > 0 {
		log.Printf("[Recovery] Recovery check complete. Found and handled %d issues", totalIssues)
	}
}
