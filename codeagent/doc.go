// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

// Package codeagent provides functionality for launching and managing
// Claude code agents using the command-line interface.
//
// This package allows you to:
//   - Launch code agents with specific prompts and working directories
//   - Track the status of running agents (pending, running, finished, failed, killed)
//   - Retrieve agent output and error messages
//   - Manage multiple agents concurrently
//   - Wait for agent completion
//   - Kill running agents
//
// Basic usage:
//
//	manager := codeagent.NewManager()
//	ctx := context.Background()
//
//	// Launch an agent
//	agentID, err := manager.LaunchAgent(ctx, "/project/path", "Fix the bug in main.go")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Wait for completion
//	info, err := manager.WaitForAgent(ctx, agentID)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	fmt.Printf("Status: %s\n", info.Status)
//	fmt.Printf("Output: %s\n", info.Output)
//
// The package is designed to be used by AI agents and tools, providing
// a clean API for programmatic control of Claude code agents.
package codeagent

