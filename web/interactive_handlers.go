// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"mavis/codeagent"
)

var interactiveManager = codeagent.NewInteractiveAgentManager()

// createTempMCPConfig creates a temporary MCP config file and returns the path
func createTempMCPConfig(workDir string, selectedMCPs []string) string {
	if len(selectedMCPs) == 0 {
		return ""
	}
	
	// Create MCP config file
	_, err := CreateMCPConfigFile(workDir, selectedMCPs, mcpStore)
	if err != nil {
		// Log error but continue without MCP
		fmt.Printf("Failed to create MCP config: %v\n", err)
		return ""
	}
	
	// Return the path to the config file
	return ".mcp.json"
}

type InteractiveAgentStatus struct {
	ID         string    `json:"id"`
	Folder     string    `json:"folder"`
	Status     string    `json:"status"`
	StartTime  time.Time `json:"start_time"`
	LastActive time.Time `json:"last_active"`
	Error      string    `json:"error,omitempty"`
	Output     []string  `json:"output,omitempty"`
}

type CreateInteractiveRequest struct {
	WorkDir      string   `json:"work_dir"`
	SelectedMCPs []string `json:"selected_mcps"`
}

func handleInteractiveRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// List all interactive agents (JSON for API calls)
		agents := interactiveManager.ListAgents()
		statuses := make([]InteractiveAgentStatus, len(agents))
		
		for i, agent := range agents {
			statuses[i] = InteractiveAgentStatus{
				ID:         agent.ID,
				Folder:     agent.Folder,
				Status:     agent.Status,
				StartTime:  agent.StartTime,
				LastActive: agent.LastActive,
				Error:      agent.Error,
			}
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(statuses)
		
	case http.MethodPost:
		// Create new interactive agent
		var req CreateInteractiveRequest
		
		// Handle form data
		req.WorkDir = r.FormValue("work_dir")
		r.ParseForm()
		req.SelectedMCPs = r.Form["selected_mcps"]
		
		if req.WorkDir == "" {
			SetErrorFlash(w, "Work directory is required")
			http.Redirect(w, r, "/interactive?modal=create", http.StatusSeeOther)
			return
		}
		
		// Resolve path
		absDir, err := ResolvePath(req.WorkDir)
		if err != nil {
			SetErrorFlash(w, fmt.Sprintf("Invalid directory: %v", err))
			http.Redirect(w, r, "/interactive?modal=create", http.StatusSeeOther)
			return
		}
		
		// Create MCP config if MCPs selected
		mcpConfig := ""
		if len(req.SelectedMCPs) > 0 {
			mcpConfig = createTempMCPConfig(absDir, req.SelectedMCPs)
		}
		
		// Create and start agent
		agent, err := interactiveManager.CreateAgent(context.Background(), absDir, mcpConfig)
		if err != nil {
			SetErrorFlash(w, fmt.Sprintf("Failed to create interactive agent: %v", err))
			http.Redirect(w, r, "/interactive", http.StatusSeeOther)
			return
		}
		
		// Redirect to session view
		SetSuccessFlash(w, fmt.Sprintf("Interactive session %s started", agent.ID[:8]))
		http.Redirect(w, r, fmt.Sprintf("/interactive?modal=session-%s", agent.ID), http.StatusSeeOther)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Handle individual agent actions
func handleInteractiveAgentAction(w http.ResponseWriter, r *http.Request) {
	// Extract agent ID and action from URL
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	
	agentID := pathParts[3]
	
	// Handle different actions
	if len(pathParts) >= 5 {
		action := pathParts[4]
		switch action {
		case "stop":
			handleInteractiveStop(w, r, agentID)
		case "delete":
			handleInteractiveDelete(w, r, agentID)
		case "input":
			handleInteractiveInput(w, r, agentID)
		default:
			http.Error(w, "Unknown action", http.StatusBadRequest)
		}
	} else {
		// Handle method-based actions for backward compatibility
		switch r.Method {
		case http.MethodDelete:
			handleInteractiveDelete(w, r, agentID)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func handleInteractiveStop(w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	agent := interactiveManager.GetAgent(agentID)
	if agent == nil {
		SetErrorFlash(w, "Session not found")
		http.Redirect(w, r, "/interactive", http.StatusSeeOther)
		return
	}
	
	if err := agent.Stop(); err != nil {
		SetErrorFlash(w, fmt.Sprintf("Failed to stop session: %v", err))
	} else {
		SetSuccessFlash(w, "Session stopped")
	}
	
	http.Redirect(w, r, "/interactive", http.StatusSeeOther)
}

func handleInteractiveDelete(w http.ResponseWriter, r *http.Request, agentID string) {
	// Support both DELETE method and POST with _method=DELETE
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Check for method override
	if r.Method == http.MethodPost && r.FormValue("_method") != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	if err := interactiveManager.RemoveAgent(agentID); err != nil {
		SetErrorFlash(w, fmt.Sprintf("Failed to delete session: %v", err))
	} else {
		SetSuccessFlash(w, "Session deleted")
	}
	
	http.Redirect(w, r, "/interactive", http.StatusSeeOther)
}

func handleInteractiveInput(w http.ResponseWriter, r *http.Request, agentID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	agent := interactiveManager.GetAgent(agentID)
	if agent == nil {
		SetErrorFlash(w, "Session not found")
		http.Redirect(w, r, "/interactive", http.StatusSeeOther)
		return
	}
	
	input := r.FormValue("input")
	if input == "" {
		// Redirect back to session view
		http.Redirect(w, r, fmt.Sprintf("/interactive?modal=session-%s", agentID), http.StatusSeeOther)
		return
	}
	
	if err := agent.SendInput(input); err != nil {
		SetErrorFlash(w, fmt.Sprintf("Failed to send input: %v", err))
	}
	
	// Redirect back to session view
	http.Redirect(w, r, fmt.Sprintf("/interactive?modal=session-%s", agentID), http.StatusSeeOther)
}

// This handler is no longer needed for SSE but kept for potential future use
func handleInteractiveStream(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Streaming not supported - use page refresh", http.StatusNotImplemented)
}