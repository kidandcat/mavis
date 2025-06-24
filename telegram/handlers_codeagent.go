// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mavis/core"

	"github.com/go-telegram/bot/models"
)

func handleCodeCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)

	if len(parts) < 3 {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Usage: `/code <directory> <task>`\n\nExample: `/code /home/project \"fix the bug in main.py\"`")
		return
	}

	// Extract directory and task
	directory := parts[1]
	task := strings.Join(parts[2:], " ")

	// Call the existing launch function
	launchCodeAgentCommand(ctx, directory, task)
}

func handleAgentsCommand(ctx context.Context, message *models.Message) {
	listCodeAgentsCommand(ctx)
}

func handleStatusCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)

	if len(parts) < 2 {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Usage: `/status <agent_id>`\n\nExample: `/status abc123`")
		return
	}

	agentID := parts[1]
	getCodeAgentDetailsCommand(ctx, agentID)
}

func handleStopCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)

	if len(parts) < 2 {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Usage: `/stop <agent_id>`\n\nExample: `/stop abc123`")
		return
	}

	agentID := parts[1]
	killCodeAgentCommand(ctx, agentID)
}

func handleCleanupCommand(ctx context.Context, message *models.Message) {
	// Only admin can cleanup stuck agents
	if message.From.ID != AdminUserID {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Only admin can cleanup stuck agents.")
		return
	}

	core.SendMessage(ctx, b, message.Chat.ID, "🔧 Starting cleanup of stuck agents...")

	cleanedCount := ForceCleanupStuckAgents()

	if cleanedCount > 0 {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ Cleanup complete! Removed %d stuck agent(s).\n\n💡 Queued tasks should now start processing automatically.", cleanedCount))

		// Show updated status
		time.Sleep(1 * time.Second) // Give a moment for queue processing
		listCodeAgentsCommand(ctx)
	} else {
		core.SendMessage(ctx, b, message.Chat.ID, "📋 No stuck agents found. All agents are running normally.")
	}
}

func launchCodeAgentCommand(ctx context.Context, directory, task string) {
	// Use AdminUserID for single-user app
	chatID := AdminUserID
	// Resolve the directory path relative to home directory
	absDir, err := core.ResolvePath(directory)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check for pending images
	pendingImages := getPendingImages(AdminUserID)
	if len(pendingImages) > 0 {
		// Append image information to the task
		task += fmt.Sprintf("\n\nThe user has provided %d image(s) for this task:", len(pendingImages))
		for i, imagePath := range pendingImages {
			task += fmt.Sprintf("\n- Image %d: %s", i+1, imagePath)
		}
		task += "\n\nPlease analyze these images as part of the task. You can read them using the Read tool."

		core.SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching code agent in %s...\n📸 Including %d pending image(s)", absDir, len(pendingImages)))
	} else {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching code agent in %s...", absDir))
	}

	// Launch the agent
	agentID, err := agentManager.LaunchAgent(ctx, absDir, task)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Check if the agent was queued
	if strings.HasPrefix(agentID, "queued-") {
		// Extract queue position and queue ID from the ID
		parts := strings.Split(agentID, "-")
		var queuePos, queueID string
		for i := 0; i < len(parts); i++ {
			if parts[i] == "pos" && i+1 < len(parts) {
				queuePos = parts[i+1]
			} else if parts[i] == "qid" && i+1 < len(parts) {
				// The queue ID includes everything after "qid-"
				queueIDParts := []string{}
				for j := i + 1; j < len(parts); j++ {
					queueIDParts = append(queueIDParts, parts[j])
				}
				queueID = strings.Join(queueIDParts, "-")
				break
			}
		}

		// Register the queued agent for tracking
		if queueID != "" {
			core.GetQueueTracker().RegisterQueuedAgent(queueID, AdminUserID, absDir, task)
		}

		queuedTasks := agentManager.GetQueuedTasksForFolder(absDir)

		// TODO: Broadcast SSE event for queue update
		// BroadcastSSEEvent("queue_update", map[string]interface{}{
		// 	"directory":      absDir,
		// 	"queue_position": queuePos,
		// 	"total_queued":   queuedTasks,
		// })

		core.SendMessage(ctx, b, chatID, fmt.Sprintf("⏳ Agent queued!\n📁 Directory: %s\n📝 Task: %s\n🔢 Queue position: %s\n📊 Total queued tasks for this folder: %d\n\nThe agent will start automatically when the current agent in this folder completes.",
			directory, task, queuePos, queuedTasks))

		// Clear pending images even for queued agents
		if len(pendingImages) > 0 {
			clearPendingImages(AdminUserID)
		}
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	// Clear pending images after using them
	if len(pendingImages) > 0 {
		clearPendingImages(AdminUserID)
	}

	// TODO: Broadcast SSE event
	// BroadcastSSEEvent("agent_started", map[string]interface{}{
	// 	"agent_id":  agentID,
	// 	"directory": absDir,
	// 	"task":      task,
	// })

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("✅ Code agent launched!\n🆔 ID: `%s`\n📝 Task: %s\n📁 Directory: %s\n\nUse `/status %s` to check status.",
		agentID, task, directory, agentID))
}

func listCodeAgentsCommand(ctx context.Context) {
	chatID := AdminUserID
	agents := agentManager.ListAgents()

	if len(agents) == 0 {
		core.SendMessage(ctx, b, chatID, "📋 No code agents running.")
		return
	}

	message := "📋 *Code Agents:*\n\n"
	for _, agent := range agents {
		status := "⏳"
		switch agent.Status {
		case "running":
			status = "🟢"
		case "finished":
			status = "✅"
		case "failed":
			status = "❌"
		case "killed":
			status = "🔴"
		}

		message += fmt.Sprintf("%s `%s` - %s\n", status, agent.ID, string(agent.Status))
		if agent.Folder != "" {
			message += fmt.Sprintf("   📁 %s\n", agent.Folder)
		}
		if agent.Prompt != "" {
			// Truncate prompt if too long
			prompt := agent.Prompt
			if len(prompt) > 50 {
				prompt = prompt[:50] + "..."
			}
			message += fmt.Sprintf("   📝 %s\n", prompt)
		}
		message += "\n"
	}

	// Add detailed queue status
	detailedQueueStatus := agentManager.GetDetailedQueueStatus()
	if len(detailedQueueStatus) > 0 {
		message += "\n📊 *Queued Tasks:*\n"
		for folder, tasks := range detailedQueueStatus {
			message += fmt.Sprintf("\n📁 *%s* (%d tasks):\n", folder, len(tasks))
			for i, task := range tasks {
				// Truncate prompt if too long
				prompt := task.Prompt
				if len(prompt) > 60 {
					prompt = prompt[:60] + "..."
				}
				message += fmt.Sprintf("   %d. 📝 %s\n", i+1, prompt)
				message += fmt.Sprintf("      🆔 Queue ID: %s\n", task.QueueID)
			}
		}
	}

	core.SendMessage(ctx, b, chatID, message)
}

func getCodeAgentDetailsCommand(ctx context.Context, agentID string) {
	chatID := AdminUserID
	agentInfo, err := agentManager.GetAgentInfo(agentID)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Agent not found: %s", agentID))
		return
	}

	status := "⏳"
	switch agentInfo.Status {
	case "running":
		status = "🟢 Running"
	case "finished":
		status = "✅ Finished"
	case "failed":
		status = "❌ Failed"
	case "killed":
		status = "🔴 Killed"
	default:
		status = "⏳ " + string(agentInfo.Status)
	}

	message := fmt.Sprintf("*Code Agent Details*\n\n🆔 ID: `%s`\n📊 Status: %s\n📝 Task: %s\n📁 Directory: %s\n🕐 Started: %s\n",
		agentInfo.ID, status, agentInfo.Prompt, agentInfo.Folder, agentInfo.StartTime.Format("15:04:05"))

	if !agentInfo.EndTime.IsZero() {
		message += fmt.Sprintf("🏁 Ended: %s\n", agentInfo.EndTime.Format("15:04:05"))
		message += fmt.Sprintf("⏱️ Duration: %s\n", agentInfo.Duration.Round(time.Second))
	}

	// Check for CURRENT_PLAN.md in the agent's working directory
	if agentInfo.Status == "running" {
		planPath := filepath.Join(agentInfo.Folder, "CURRENT_PLAN.md")
		if planContent, err := os.ReadFile(planPath); err == nil {
			message += fmt.Sprintf("\n📋 *Current Plan:*\n```\n%s\n```", string(planContent))
		}
	}

	// Add full output if available
	if agentInfo.Output != "" {
		message += fmt.Sprintf("\n📄 *Output:*\n```\n%s\n```", agentInfo.Output)
	}

	// Add full error if available
	if agentInfo.Error != "" {
		message += fmt.Sprintf("\n❌ *Error:*\n```\n%s\n```", agentInfo.Error)
	}

	// Use SendLongMessage to handle potentially long output
	core.SendLongMessage(ctx, b, chatID, message)
}

func killCodeAgentCommand(ctx context.Context, agentID string) {
	chatID := AdminUserID
	err := agentManager.KillAgent(agentID)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to stop agent: %v", err))
		return
	}

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🔴 Agent %s has been stopped.", agentID))
}
