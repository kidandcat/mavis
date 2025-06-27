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
	Branch  string `json:"branch"`
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
		// Get progress for running agents
		progress := ""
		plan := ""
		if agent.Status == "running" || agent.Status == "active" {
			progress = getAgentProgress(agent.ID)
			plan = getAgentPlan(agent.ID)
		}
		
		agentStatuses[i] = AgentStatus{
			ID:           agent.ID,
			Task:         agent.Task,
			Status:       agent.Status,
			StartTime:    agent.StartTime,
			LastActive:   agent.LastActive,
			MessagesSent: agent.MessagesSent,
			QueueStatus:  agent.QueueStatus,
			IsStale:      agent.IsStale,
			Progress:     progress,
			Plan:         plan,
			Output:       agent.Output,
			Duration:     agent.Duration,
		}
	}

	// Get query parameters
	modalParam := r.URL.Query().Get("modal")
	dirParam := r.URL.Query().Get("dir")
	
	// Check directory and get branches if modal is create and dir is provided
	var branches []string
	if modalParam == "create" && dirParam != "" {
		// Resolve the directory path
		if absDir, err := ResolvePath(dirParam); err == nil {
			// Check if it's a git repository
			if isGitRepo(absDir) {
				// Get branches
				if branchList, err := listGitBranches(absDir); err == nil {
					branches = branchList
				}
			}
		}
	}
	
	// Render the appropriate section based on path
	var content g.Node
	switch path {
	case "/", "/agents":
		content = AgentsSection(agentStatuses, modalParam, dirParam, branches)
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
		folderPath := r.URL.Query().Get("folder")
		if folderPath == "" {
			folderPath = "."
		}
		// Get git diff if folder is specified
		diff := ""
		showDiff := false
		if folderPath != "" {
			diffData, err := getGitDiff(folderPath)
			if err == nil {
				diff = diffData
				showDiff = true
			}
		}
		content = GitSection(folderPath, diff, showDiff)
	case "/system":
		content = SystemSection()
	default:
		content = AgentsSection(agentStatuses, modalParam, dirParam, branches)
	}

	// Only enable auto-refresh on agents page when no modal is open
	shouldAutoRefresh := modalParam != "create" && (path == "/" || path == "/agents")
	if shouldAutoRefresh {
		_ = DashboardLayout(w, r, content).Render(w)
	} else {
		_ = DashboardLayoutNoRefresh(w, r, content).Render(w)
	}
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
		// Check if this is a form submission
		if r.Header.Get("Content-Type") != "application/json" && !strings.Contains(r.Header.Get("Accept"), "application/json") {
			SetErrorFlash(w, fmt.Sprintf("Failed to stop agent: %v", err))
			http.Redirect(w, r, "/agents", http.StatusSeeOther)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Check if this is a form submission (redirect) or API call (JSON)
	if r.Header.Get("Content-Type") != "application/json" && !strings.Contains(r.Header.Get("Accept"), "application/json") {
		SetSuccessFlash(w, fmt.Sprintf("Agent %s stopped successfully", agentID))
		http.Redirect(w, r, "/agents", http.StatusSeeOther)
		return
	}
	
	// Return success JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped", "id": agentID})
}

func handleDeleteAgent(w http.ResponseWriter, r *http.Request) {
	// Accept both DELETE and POST for form compatibility
	if r.Method != "DELETE" && r.Method != "POST" {
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
		// Check if this is a form submission
		if r.Method == "POST" {
			SetErrorFlash(w, fmt.Sprintf("Failed to delete agent: %v", err))
			http.Redirect(w, r, "/agents", http.StatusSeeOther)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Check if this is a form submission (redirect) or API call (JSON)
	if r.Method == "POST" {
		SetSuccessFlash(w, fmt.Sprintf("Agent %s deleted successfully", agentID))
		http.Redirect(w, r, "/agents", http.StatusSeeOther)
		return
	}
	
	// Return success JSON
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
		req.Branch = r.FormValue("branch")
	}

	if req.Task == "" {
		http.Error(w, "Task is required", http.StatusBadRequest)
		return
	}

	agentID, err := createAgentWithBranch(req.Task, req.WorkDir, req.Branch)
	if err != nil {
		// Check if this is a form submission
		if r.Header.Get("Content-Type") != "application/json" {
			SetErrorFlash(w, fmt.Sprintf("Failed to create agent: %v", err))
			http.Redirect(w, r, "/agents", http.StatusSeeOther)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Check if this is a form submission (redirect) or API call (JSON)
	if r.Header.Get("Content-Type") != "application/json" {
		// Form submission - redirect to agents page
		if strings.HasPrefix(agentID, "queued-") {
			SetWarningFlash(w, "Agent queued - another agent is currently running")
		} else {
			SetSuccessFlash(w, fmt.Sprintf("Agent %s created successfully", agentID))
		}
		http.Redirect(w, r, "/agents", http.StatusSeeOther)
		return
	}
	
	// API call - return JSON response
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

	// Check if this is a form submission (redirect) or API call (JSON)
	if r.Header.Get("Content-Type") != "application/json" {
		// Form submission - redirect to agents page
		SetSuccessFlash(w, fmt.Sprintf("Commit agent launched for directory: %s. The AI will review changes and create an appropriate commit.", req.Folder))
		http.Redirect(w, r, "/agents", http.StatusSeeOther)
		return
	}
	
	// API call - return JSON
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

func handlePRCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Folder string `json:"folder"`
		Branch string `json:"branch"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		Base   string `json:"base"`
	}

	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		req.Folder = r.FormValue("folder")
		req.Branch = r.FormValue("branch")
		req.Title = r.FormValue("title")
		req.Body = r.FormValue("body")
		req.Base = r.FormValue("base")
	}

	if req.Folder == "" {
		req.Folder = "."
	}
	if req.Base == "" {
		req.Base = "main"
	}

	// Resolve the directory path
	absDir, err := ResolvePath(req.Folder)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Error resolving directory path: %v", err)})
		return
	}

	// Launch PR creation agent
	ctx := context.Background()
	launchPRCreateAgent(ctx, absDir, req.Branch, req.Title, req.Body, req.Base)

	// Check if this is a form submission (redirect) or API call (JSON)
	if r.Header.Get("Content-Type") != "application/json" {
		// Form submission - redirect back to git page
		http.Redirect(w, r, fmt.Sprintf("/git?folder=%s&success=pr_create_launched", req.Folder), http.StatusSeeOther)
		return
	}
	
	// API call - return JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("PR creation agent launched. The AI will create a pull request from branch '%s' to '%s'.", req.Branch, req.Base),
	})
}

func handlePRReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Folder string `json:"folder"`
		PRURL  string `json:"pr_url"`
		Action string `json:"action"`
	}

	if r.Header.Get("Content-Type") == "application/json" {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		req.Folder = r.FormValue("folder")
		req.PRURL = r.FormValue("pr_url")
		req.Action = r.FormValue("action")
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

	// Launch PR review agent
	ctx := context.Background()
	launchPRReviewAgent(ctx, absDir, req.PRURL, req.Action)

	// Check if this is a form submission (redirect) or API call (JSON)
	if r.Header.Get("Content-Type") != "application/json" {
		// Form submission - redirect back to git page
		http.Redirect(w, r, fmt.Sprintf("/git?folder=%s&success=pr_review_launched", req.Folder), http.StatusSeeOther)
		return
	}
	
	// API call - return JSON
	actionText := "review"
	if req.Action == "approve" {
		actionText = "review and approve"
	} else if req.Action == "request-changes" {
		actionText = "review and request changes on"
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("PR review agent launched. The AI will %s the pull request.", actionText),
	})
}

func handleCheckDirectory(w http.ResponseWriter, r *http.Request) {
	// Get directory from query params
	dir := r.URL.Query().Get("dir")
	if dir == "" {
		dir = "."
	}

	// Resolve the directory path
	absDir, err := ResolvePath(dir)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"exists": false,
			"isGitRepo": false,
			"error": fmt.Sprintf("Error resolving directory path: %v", err),
		})
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	exists := err == nil && info.IsDir()
	
	response := map[string]interface{}{
		"exists": exists,
		"isGitRepo": false,
		"branches": []string{},
	}

	if !exists {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Check if it's a git repository
	isRepo := isGitRepo(absDir)
	response["isGitRepo"] = isRepo

	if isRepo {
		// Get list of branches
		branches, err := listGitBranches(absDir)
		if err == nil {
			response["branches"] = branches
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
