// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package codeagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// AgentStatus represents the current state of an agent
type AgentStatus string

const (
	StatusPending  AgentStatus = "pending"
	StatusRunning  AgentStatus = "running"
	StatusFinished AgentStatus = "finished"
	StatusFailed   AgentStatus = "failed"
	StatusKilled   AgentStatus = "killed"
)

// CompletionCallback is called when an agent completes (successfully or with error)
type CompletionCallback func(agent *Agent)

// Agent represents a code agent instance
type Agent struct {
	ID                 string
	Folder             string
	Prompt             string
	Status             AgentStatus
	Output             string
	Error              string
	StartTime          time.Time
	EndTime            time.Time
	cmd                *exec.Cmd
	mu                 sync.RWMutex
	PlanFilename       string             // Custom plan filename (defaults to CURRENT_PLAN.md)
	completionCallback CompletionCallback // Called when agent completes
	PlanContent        string             // Content of CURRENT_PLAN.md (preserved on error)
}

// NewAgent creates a new agent instance
func NewAgent(id, folder, prompt string) *Agent {
	return &Agent{
		ID:           id,
		Folder:       folder,
		Prompt:       prompt,
		Status:       StatusPending,
		PlanFilename: "CURRENT_PLAN.md", // Default plan filename
	}
}

// NewAgentWithPlanFile creates a new agent instance with a custom plan filename
func NewAgentWithPlanFile(id, folder, prompt, planFilename string) *Agent {
	return &Agent{
		ID:           id,
		Folder:       folder,
		Prompt:       prompt,
		Status:       StatusPending,
		PlanFilename: planFilename,
	}
}

// SetCompletionCallback sets the callback to be called when the agent completes
func (a *Agent) SetCompletionCallback(callback CompletionCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.completionCallback = callback
}

// Start launches the agent
func (a *Agent) Start(ctx context.Context) error {
	log.Printf("[Agent] Starting agent %s in folder %s", a.ID, a.Folder)
	// Set status to running
	a.mu.Lock()
	a.Status = StatusRunning
	a.StartTime = time.Now()
	a.mu.Unlock()

	// Create plan file
	planFile := fmt.Sprintf("%s/%s", a.Folder, a.PlanFilename)
	planContent := `# Current Task Plan

## Task
` + a.Prompt + `

## Plan
(The AI will write its plan here)

## Progress
(The AI will update progress here as it works)
`
	if err := os.WriteFile(planFile, []byte(planContent), 0644); err != nil {
		return fmt.Errorf("failed to create %s: %v", a.PlanFilename, err)
	}

	// Defer cleanup of the plan file
	defer func() {
		// Read plan content before removal if there was an error
		if a.Status == StatusFailed || a.Status == StatusKilled {
			if content, err := os.ReadFile(planFile); err == nil {
				a.mu.Lock()
				a.PlanContent = string(content)
				a.mu.Unlock()
			}
		}
		os.Remove(planFile)
	}()

	// Modified prompt to instruct the AI to use the plan file
	enhancedPrompt := fmt.Sprintf(`IMPORTANT: Before starting any work, you MUST:
1. Read the file %s in the current directory
2. Write your detailed plan for completing the task in the "## Plan" section
3. As you work, update the "## Progress" section with what you've completed
4. Keep the plan updated as you discover new requirements or change approach

`, a.PlanFilename) + a.Prompt

	// Use a shell to properly execute the claude script
	// Escape single quotes in the prompt to prevent shell injection
	escapedPrompt := strings.ReplaceAll(enhancedPrompt, "'", "'\"'\"'")
	
	// Check if .mcp.json exists in the working directory
	mcpConfigPath := filepath.Join(a.Folder, ".mcp.json")
	hasMCPConfig := false
	if _, err := os.Stat(mcpConfigPath); err == nil {
		hasMCPConfig = true
		log.Printf("[Agent] Found .mcp.json in %s", a.Folder)
		// Read and log the content for debugging
		if content, err := os.ReadFile(mcpConfigPath); err == nil {
			log.Printf("[Agent] .mcp.json content:\n%s", string(content))
			// Parse to check which servers are configured
			var config map[string]interface{}
			if err := json.Unmarshal(content, &config); err == nil {
				if servers, ok := config["mcpServers"].(map[string]interface{}); ok {
					serverNames := make([]string, 0, len(servers))
					for name := range servers {
						serverNames = append(serverNames, name)
					}
					log.Printf("[Agent] MCP servers configured: %v", serverNames)
				}
			}
		}
	} else {
		log.Printf("[Agent] No .mcp.json found in %s", a.Folder)
	}
	
	// Add MCP hint to prompt if MCP config exists
	if hasMCPConfig {
		escapedPrompt = "IMPORTANT: MCP servers have been configured for this session. The available tools from MCP servers should be accessible to you.\n\n" + escapedPrompt
	}
	
	// Build the claude command with MCP config if present
	cmdString := ""
	if hasMCPConfig {
		cmdString = fmt.Sprintf("cd '%s' && claude --dangerously-skip-permissions --mcp-config .mcp.json -p '%s'", a.Folder, escapedPrompt)
	} else {
		cmdString = fmt.Sprintf("cd '%s' && claude --dangerously-skip-permissions -p '%s'", a.Folder, escapedPrompt)
	}
	log.Printf("[Agent] Executing command: %s", cmdString)
	a.cmd = exec.CommandContext(ctx, "/bin/sh", "-c", cmdString)
	
	// Ensure the command inherits the current environment
	a.cmd.Env = os.Environ()

	// Set up pipes for streaming output
	stdout, err := a.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderr, err := a.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := a.cmd.Start(); err != nil {
		a.mu.Lock()
		a.Status = StatusFailed
		
		// Build detailed error message
		var errorBuilder strings.Builder
		errorBuilder.WriteString(fmt.Sprintf("Failed to start command: %v", err))
		
		// Check for common startup errors
		if strings.Contains(err.Error(), "executable file not found") || strings.Contains(err.Error(), "no such file") {
			errorBuilder.WriteString("\n\n⚠️ The 'claude' command was not found. Please ensure:")
			errorBuilder.WriteString("\n1. Claude CLI is installed")
			errorBuilder.WriteString("\n2. Claude CLI is in your PATH") 
			errorBuilder.WriteString("\n3. You can run 'claude --version' from the terminal")
		} else if strings.Contains(err.Error(), "permission denied") {
			errorBuilder.WriteString("\n\n⚠️ Permission denied. Please check:")
			errorBuilder.WriteString("\n1. The claude command is executable")
			errorBuilder.WriteString("\n2. You have permission to execute commands in the working directory")
		}
		
		errorBuilder.WriteString(fmt.Sprintf("\n\nCommand: %s", cmdString))
		errorBuilder.WriteString(fmt.Sprintf("\nWorking Directory: %s", a.Folder))
		errorBuilder.WriteString(fmt.Sprintf("\nFailure Time: %s", time.Now().Format("2006-01-02 15:04:05")))
		
		a.Error = errorBuilder.String()
		a.EndTime = time.Now()
		a.mu.Unlock()
		return err
	}

	// Capture output in a thread-safe way
	var outputBuilder strings.Builder
	var outputMu sync.Mutex

	// Create a wait group for the output readers
	var wg sync.WaitGroup
	wg.Add(2)

	// Read stdout
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				outputMu.Lock()
				outputBuilder.Write(buf[:n])
				outputMu.Unlock()
			}
			if err != nil {
				break
			}
		}
	}()

	// Read stderr
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				outputMu.Lock()
				outputBuilder.Write(buf[:n])
				outputMu.Unlock()
			}
			if err != nil {
				break
			}
		}
	}()

	// Wait for the command to complete
	cmdErr := a.cmd.Wait()

	// Wait for all output to be read
	wg.Wait()

	// Get the final output
	outputMu.Lock()
	output := outputBuilder.String()
	outputMu.Unlock()

	a.mu.Lock()
	a.Output = output
	a.EndTime = time.Now()

	if cmdErr != nil {
		a.Status = StatusFailed
		// Create a detailed error message including command information and output
		var errorBuilder strings.Builder
		errorBuilder.WriteString(fmt.Sprintf("Command failed: %v", cmdErr))

		if a.cmd.ProcessState != nil {
			errorBuilder.WriteString(fmt.Sprintf("\nProcess State: %s", a.cmd.ProcessState.String()))
			if a.cmd.ProcessState.ExitCode() >= 0 {
				errorBuilder.WriteString(fmt.Sprintf("\nExit Code: %d", a.cmd.ProcessState.ExitCode()))
				
				// Add common exit code explanations
				switch a.cmd.ProcessState.ExitCode() {
				case 1:
					errorBuilder.WriteString("\n💡 Exit code 1: General errors (syntax errors, incorrect usage)")
				case 2:
					errorBuilder.WriteString("\n💡 Exit code 2: Misuse of shell builtins")
				case 126:
					errorBuilder.WriteString("\n💡 Exit code 126: Command cannot execute (permission problem or not executable)")
				case 127:
					errorBuilder.WriteString("\n💡 Exit code 127: Command not found")
				case 128:
					errorBuilder.WriteString("\n💡 Exit code 128: Invalid argument to exit")
				case 130:
					errorBuilder.WriteString("\n💡 Exit code 130: Script terminated by Ctrl+C")
				case 137:
					errorBuilder.WriteString("\n💡 Exit code 137: Process killed (SIGKILL) - possibly out of memory")
				case 139:
					errorBuilder.WriteString("\n💡 Exit code 139: Segmentation fault (SIGSEGV)")
				}
			}
			
			// Check for signals
			if a.cmd.ProcessState.Sys() != nil {
				if ws, ok := a.cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
					if ws.Signaled() {
						sig := ws.Signal()
						errorBuilder.WriteString(fmt.Sprintf("\nTerminated by signal: %s", sig))
						
						switch sig {
						case syscall.SIGKILL:
							errorBuilder.WriteString("\n⚠️ Process was forcefully killed - check system resources (memory, disk space)")
						case syscall.SIGSEGV:
							errorBuilder.WriteString("\n⚠️ Segmentation fault - process crashed")
						}
					}
				}
			}
		}

		if len(a.cmd.Args) > 0 {
			errorBuilder.WriteString(fmt.Sprintf("\nCommand: %s", strings.Join(a.cmd.Args, " ")))
		}

		errorBuilder.WriteString(fmt.Sprintf("\nWorking Directory: %s", a.Folder))

		// Check if claude command exists
		if cmdErr.Error() == "exit status 127" || strings.Contains(cmdErr.Error(), "executable file not found") {
			errorBuilder.WriteString("\n\n⚠️ The 'claude' command was not found. Please ensure:")
			errorBuilder.WriteString("\n1. Claude CLI is installed")
			errorBuilder.WriteString("\n2. Claude CLI is in your PATH")
			errorBuilder.WriteString("\n3. You can run 'claude --version' from the terminal")
		}

		if output != "" {
			// Show last part of output if very long
			outputToShow := output
			if len(outputToShow) > 2000 {
				outputToShow = "...(truncated)...\n" + outputToShow[len(outputToShow)-2000:]
			}
			errorBuilder.WriteString(fmt.Sprintf("\nProcess Output:\n%s", outputToShow))
			
			// Check for MCP-related errors in output
			if strings.Contains(output, "mcp") || strings.Contains(output, "MCP") {
				errorBuilder.WriteString("\n\n⚠️ MCP-related error detected. Please check:")
				errorBuilder.WriteString("\n1. MCP server commands are correct and executable")
				errorBuilder.WriteString("\n2. MCP server dependencies are installed")
				errorBuilder.WriteString("\n3. The .mcp.json file format is correct")
			}
		}

		errorBuilder.WriteString(fmt.Sprintf("\nFailure Time: %s", time.Now().Format("2006-01-02 15:04:05")))

		a.Error = errorBuilder.String()
		log.Printf("[Agent] Agent %s failed in folder %s: %v", a.ID, a.Folder, cmdErr)
	} else {
		a.Status = StatusFinished
		log.Printf("[Agent] Agent %s finished successfully in folder %s", a.ID, a.Folder)
	}
	a.mu.Unlock()

	log.Printf("[Agent] Agent %s completed with status %s, waiting for monitor to detect", a.ID, a.Status)

	// Call completion callback if set
	if a.completionCallback != nil {
		log.Printf("[Agent] Calling completion callback for agent %s", a.ID)
		a.completionCallback(a)
	}

	return nil
}

// StartAsync launches the agent asynchronously
func (a *Agent) StartAsync(ctx context.Context) {
	go func() {
		_ = a.Start(ctx)
	}()
}

// Kill terminates the agent if it's running
func (a *Agent) Kill() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.Status != StatusRunning {
		return fmt.Errorf("agent %s is not running", a.ID)
	}

	if a.cmd != nil && a.cmd.Process != nil {
		if err := a.cmd.Process.Kill(); err != nil {
			return err
		}
		a.Status = StatusKilled
		a.EndTime = time.Now()
	}

	return nil
}

// GetStatus returns the current status of the agent
func (a *Agent) GetStatus() AgentStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Status
}

// GetOutput returns the output of the agent
func (a *Agent) GetOutput() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Output
}

// GetError returns any error from the agent execution
func (a *Agent) GetError() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Error
}

// GetDuration returns how long the agent has been running or ran
func (a *Agent) GetDuration() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.StartTime.IsZero() {
		return 0
	}

	if a.EndTime.IsZero() {
		return time.Since(a.StartTime)
	}

	return a.EndTime.Sub(a.StartTime)
}

// ToInfo returns a snapshot of the agent's current state
func (a *Agent) ToInfo() AgentInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// Calculate duration directly to avoid nested lock acquisition
	var duration time.Duration
	if !a.StartTime.IsZero() {
		if a.EndTime.IsZero() {
			duration = time.Since(a.StartTime)
		} else {
			duration = a.EndTime.Sub(a.StartTime)
		}
	}

	return AgentInfo{
		ID:          a.ID,
		Folder:      a.Folder,
		Prompt:      a.Prompt,
		Status:      a.Status,
		Output:      a.Output,
		Error:       a.Error,
		StartTime:   a.StartTime,
		EndTime:     a.EndTime,
		Duration:    duration,
		PlanContent: a.PlanContent,
	}
}

// IsProcessAlive checks if the agent's process is still running
func (a *Agent) IsProcessAlive() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.cmd == nil || a.cmd.Process == nil {
		return false
	}

	// Try to send signal 0 to check if process exists
	err := a.cmd.Process.Signal(nil)
	return err == nil
}

// MarkAsFailed marks the agent as failed with the given error message
func (a *Agent) MarkAsFailed(errorMsg string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.Status = StatusFailed
	a.Error = errorMsg
	if a.EndTime.IsZero() {
		a.EndTime = time.Now()
	}
}

// MarkAsFailedWithDetails marks the agent as failed and includes available process output and error details
func (a *Agent) MarkAsFailedWithDetails(errorMsg string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.Status = StatusFailed
	if a.EndTime.IsZero() {
		a.EndTime = time.Now()
	}

	// Build detailed error message including process information
	var detailedError strings.Builder
	detailedError.WriteString(errorMsg)

	// If we have a command and process, try to get additional details
	if a.cmd != nil {
		if a.cmd.ProcessState != nil {
			detailedError.WriteString(fmt.Sprintf("\nProcess State: %s", a.cmd.ProcessState.String()))
			if a.cmd.ProcessState.ExitCode() >= 0 {
				detailedError.WriteString(fmt.Sprintf("\nExit Code: %d", a.cmd.ProcessState.ExitCode()))
			}
			
			// Check for specific system signals that might indicate why the process died
			if a.cmd.ProcessState.Sys() != nil {
				if ws, ok := a.cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
					if ws.Signaled() {
						sig := ws.Signal()
						detailedError.WriteString(fmt.Sprintf("\nTerminated by signal: %s", sig))
						
						// Add explanations for common signals
						switch sig {
						case syscall.SIGKILL:
							detailedError.WriteString("\n⚠️ Process was forcefully killed (SIGKILL) - possibly due to OOM killer or manual termination")
						case syscall.SIGSEGV:
							detailedError.WriteString("\n⚠️ Segmentation fault (SIGSEGV) - process crashed due to memory access violation")
						case syscall.SIGTERM:
							detailedError.WriteString("\n⚠️ Process was terminated (SIGTERM) - requested to shut down")
						case syscall.SIGABRT:
							detailedError.WriteString("\n⚠️ Process aborted (SIGABRT) - likely due to assertion failure")
						}
					}
				}
			}
		}

		// Include the command that was executed
		if len(a.cmd.Args) > 0 {
			detailedError.WriteString(fmt.Sprintf("\nCommand: %s", strings.Join(a.cmd.Args, " ")))
		}

		// Include working directory
		if a.cmd.Dir != "" {
			detailedError.WriteString(fmt.Sprintf("\nWorking Directory: %s", a.cmd.Dir))
		} else {
			detailedError.WriteString(fmt.Sprintf("\nWorking Directory: %s", a.Folder))
		}
	}

	// Include any output we've captured so far
	if a.Output != "" {
		// Limit output to last 1000 characters if very long
		outputToShow := a.Output
		if len(outputToShow) > 1000 {
			outputToShow = "...(truncated)...\n" + outputToShow[len(outputToShow)-1000:]
		}
		detailedError.WriteString(fmt.Sprintf("\nLast Captured Output:\n%s", outputToShow))
	}

	// Include timestamp
	detailedError.WriteString(fmt.Sprintf("\nFailure Time: %s", time.Now().Format("2006-01-02 15:04:05")))

	a.Error = detailedError.String()
}

// AgentInfo is a snapshot of an agent's state
type AgentInfo struct {
	ID          string
	Folder      string
	Prompt      string
	Status      AgentStatus
	Output      string
	Error       string
	StartTime   time.Time
	EndTime     time.Time
	Duration    time.Duration
	PlanContent string // Content of CURRENT_PLAN.md (preserved on error)
}
