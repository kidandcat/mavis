package codeagent_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"mavis/codeagent"
)

func ExampleManager() {
	// Create a new manager
	manager := codeagent.NewManager()

	// Launch an agent
	ctx := context.Background()
	agentID, err := manager.LaunchAgent(ctx, "/path/to/project", "Fix the bug in main.go")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Launched agent: %s\n", agentID)

	// Check status
	info, err := manager.GetAgentInfo(agentID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Status: %s\n", info.Status)

	// Wait for completion
	finalInfo, err := manager.WaitForAgent(ctx, agentID)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Final status: %s\n", finalInfo.Status)
	fmt.Printf("Output: %s\n", finalInfo.Output)
	if finalInfo.Error != "" {
		fmt.Printf("Error: %s\n", finalInfo.Error)
	}
}

func ExampleManager_multiple() {
	manager := codeagent.NewManager()
	ctx := context.Background()

	// Launch multiple agents
	var agentIDs []string

	tasks := []struct {
		folder string
		prompt string
	}{
		{"/project1", "Add unit tests for user.go"},
		{"/project2", "Refactor database connection code"},
		{"/project3", "Update documentation"},
	}

	for _, task := range tasks {
		id, err := manager.LaunchAgent(ctx, task.folder, task.prompt)
		if err != nil {
			log.Fatal(err)
		}
		agentIDs = append(agentIDs, id)
	}

	// Monitor progress
	for {
		running := manager.GetRunningCount()
		total := manager.GetTotalCount()

		fmt.Printf("Agents: %d running, %d total\n", running, total)

		if running == 0 {
			break
		}

		time.Sleep(1 * time.Second)
	}

	// Get results
	for _, id := range agentIDs {
		info, _ := manager.GetAgentInfo(id)
		fmt.Printf("Agent %s: %s\n", id, info.Status)
	}

	// Cleanup
	cleaned := manager.CleanupFinishedAgents()
	fmt.Printf("Cleaned up %d agents\n", cleaned)
}

func ExampleAgent_Kill() {
	manager := codeagent.NewManager()
	ctx := context.Background()

	// Launch a long-running agent
	agentID, err := manager.LaunchAgent(ctx, "/path/to/project", "Perform comprehensive code review")
	if err != nil {
		log.Fatal(err)
	}

	// Wait a bit
	time.Sleep(2 * time.Second)

	// Kill the agent
	err = manager.KillAgent(agentID)
	if err != nil {
		log.Fatal(err)
	}

	info, _ := manager.GetAgentInfo(agentID)
	fmt.Printf("Agent status after kill: %s\n", info.Status)
}
