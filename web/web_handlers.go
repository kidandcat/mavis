// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	g "maragu.dev/gomponents"
)

type CreateAgentRequest struct {
	Task    string `json:"task"`
	WorkDir string `json:"work_dir"`
}

// Login handler removed - authentication disabled for local network use

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	// For API calls requesting JSON
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		switch path {
		case "/agents":
			agents := GetAllAgentsStatusJSON()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(agents)
		case "/files":
			// Return file listing as JSON
			dir := r.URL.Query().Get("path")
			if dir == "" {
				// Default to user's home directory
				dir = os.Getenv("HOME")
				if dir == "" {
					dir = "/"
				}
			}
			files, err := listFilesNew(dir)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(files)
		default:
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		}
		return
	}

	// For regular page requests, render the full dashboard
	agents := GetAllAgentsStatusJSON()
	agentStatuses := make([]AgentStatus, len(agents))
	for i, agent := range agents {
		agentStatuses[i] = AgentStatus{
			ID:           agent.ID,
			Task:         agent.Task,
			Status:       agent.Status,
			StartTime:    agent.StartTime,
			LastActive:   agent.LastActive,
			MessagesSent: agent.MessagesSent,
			QueueStatus:  agent.QueueStatus,
			IsStale:      agent.IsStale,
		}
	}

	// Render the appropriate section based on path
	var content g.Node
	switch path {
	case "/", "/agents":
		content = AgentsSection(agentStatuses)
	case "/files":
		dir := r.URL.Query().Get("path")
		if dir == "" {
			// Default to user's home directory
			dir = os.Getenv("HOME")
			if dir == "" {
				dir = "/"
			}
		}
		filesRaw, _ := listFilesNew(dir)
		// Convert to FileInfo
		files := make([]FileInfo, len(filesRaw))
		for i, f := range filesRaw {
			files[i] = FileInfo{
				Name:  f.Name,
				IsDir: f.IsDir,
				Size:  f.Size,
				Mode:  f.Mode,
			}
		}
		content = FilesSection(dir, files)
	case "/git":
		content = GitSection()
	case "/system":
		content = SystemSection()
	default:
		content = AgentsSection(agentStatuses)
	}

	_ = DashboardLayout(content).Render(w)
}

func handleAgentStatus(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	agentID := pathParts[3]
	
	// Check if progress-only is requested
	if r.URL.Query().Get("progress-only") == "true" {
		progress := getAgentProgress(agentID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"id":       agentID,
			"progress": progress,
		})
		return
	}
	
	status := getAgentStatus(agentID)

	if status == "" {
		status = "Agent not found or no status available"
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":     agentID,
		"status": status,
	})
}

func handleStopAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	agentID := pathParts[3]
	err := stopAgent(agentID)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped", "id": agentID})
}

func handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	agentID := pathParts[3]
	
	// Remove the agent from the manager
	err := agentManager.RemoveAgent(agentID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Return success
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": agentID})
}

func handleCreateAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateAgentRequest
	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		// Handle form data
		req.Task = r.FormValue("task")
		req.WorkDir = r.FormValue("work_dir")
	}

	if req.Task == "" {
		http.Error(w, "Task is required", http.StatusBadRequest)
		return
	}

	agentID, err := createCodeAgent(req.Task, req.WorkDir)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Check if agent was queued
	if strings.HasPrefix(agentID, "queued-") {
		// Parse queue information
		parts := strings.Split(agentID, "-")
		var queuePos string
		for i := 0; i < len(parts); i++ {
			if parts[i] == "pos" && i+1 < len(parts) {
				queuePos = parts[i+1]
				break
			}
		}

		// Return queued agent info
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ID":            agentID,
			"Task":          req.Task,
			"Status":        "queued",
			"QueuePosition": queuePos,
			"StartTime":     time.Now(),
			"LastActive":    time.Now(),
			"MessagesSent":  0,
			"QueueStatus":   fmt.Sprintf("Position %s", queuePos),
			"IsStale":       false,
		})
		return
	}

	// Get the new agent status
	agent := getAgentByID(agentID)
	if agent == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get agent status"})
		return
	}

	// Return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

func handleGitDiff(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}

	diff, err := getGitDiff(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"diff": diff})
}

func handleGitCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Folder string `json:"folder"`
	}

	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		req.Folder = r.FormValue("folder")
	}

	if req.Folder == "" {
		req.Folder = "."
	}

	// Resolve the directory path
	absDir, err := ResolvePath(req.Folder)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Error resolving directory path: %v", err)})
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Directory not found: %s", absDir)})
		return
	}
	if !info.IsDir() {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Path is not a directory: %s", absDir)})
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Directory is not a git repository: %s", absDir)})
		return
	}

	// Launch the commit agent
	ctx := context.Background()
	launchCommitAgent(ctx, req.Folder)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Commit agent launched for directory: %s. The AI will review changes and create an appropriate commit.", req.Folder),
	})
}

func handleWebRunCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Command string `json:"command"`
	}

	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		req.Command = r.FormValue("command")
	}

	if req.Command == "" {
		http.Error(w, "Command is required", http.StatusBadRequest)
		return
	}

	// Only allow specific safe commands
	allowedCommands := []string{"df", "uptime", "date", "whoami", "pwd"}
	allowed := false
	for _, cmd := range allowedCommands {
		if strings.HasPrefix(req.Command, cmd) {
			allowed = true
			break
		}
	}

	if !allowed {
		http.Error(w, "Command not allowed", http.StatusForbidden)
		return
	}

	// Parse command into parts
	parts := strings.Fields(req.Command)
	if len(parts) == 0 {
		http.Error(w, "Invalid command", http.StatusBadRequest)
		return
	}

	cmd := parts[0]
	args := parts[1:]
	output, err := runCommand(".", cmd, args...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"output": output})
}

func handleImageUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create uploads directory if it doesn't exist
	uploadsDir := "data/uploads"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%d%s", time.Now().Unix(), ext)
	filepath := filepath.Join(uploadsDir, filename)

	// Save file
	dst, err := os.Create(filepath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return success with file URL
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url":      "/uploads/" + filename,
		"filename": filename,
	})
}

func handleWebRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Send success response before restarting
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "restarting",
		"message": "Mavis is restarting...",
	})

	// Schedule restart after a short delay to allow response to be sent
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
}
