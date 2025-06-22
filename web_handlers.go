// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

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
)

// WebResponse is a standard response format for the web API
type WebResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func sendJSONResponse(w http.ResponseWriter, data interface{}, err error) {
	w.Header().Set("Content-Type", "application/json")
	
	response := WebResponse{
		Success: err == nil,
		Data:    data,
	}
	
	if err != nil {
		response.Error = err.Error()
		w.WriteHeader(http.StatusBadRequest)
	}
	
	json.NewEncoder(w).Encode(response)
}

// handleWebAgents returns list of all agents
func handleWebAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	agents := GetAllAgentsStatus()
	sendJSONResponse(w, agents, nil)
}

// handleWebAgent handles individual agent operations
func handleWebAgent(w http.ResponseWriter, r *http.Request) {
	// Extract agent ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/agent/")
	parts := strings.Split(path, "/")
	if len(parts) < 1 {
		sendJSONResponse(w, nil, fmt.Errorf("invalid agent path"))
		return
	}
	
	agentID := parts[0]
	
	switch r.Method {
	case "GET":
		// Get agent status
		if len(parts) > 1 && parts[1] == "status" {
			status := GetAgentDetailedStatus(agentID)
			sendJSONResponse(w, status, nil)
			return
		}
		
	case "DELETE":
		// Stop agent
		err := StopAgent(agentID)
		sendJSONResponse(w, map[string]string{"message": "Agent stopped"}, err)
		return
	}
	
	http.Error(w, "Not found", http.StatusNotFound)
}

// handleWebCode creates a new code agent
func handleWebCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		Directory  string   `json:"directory"`
		Task       string   `json:"task"`
		NewBranch  bool     `json:"new_branch"`
		Branch     string   `json:"branch"`
		Images     []string `json:"images"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONResponse(w, nil, err)
		return
	}
	
	// Resolve directory path
	workDir := resolvePath(req.Directory)
	
	// Validate directory
	if !isGitRepo(workDir) {
		sendJSONResponse(w, nil, fmt.Errorf("not a git repository"))
		return
	}
	
	// Create agent
	ctx := context.Background()
	var agentID string
	var err error
	
	if req.NewBranch {
		agentID, err = CreateNewBranchAgent(ctx, workDir, req.Task, req.Images, 0) // 0 for web user
	} else if req.Branch != "" {
		agentID, err = CreateEditBranchAgent(ctx, workDir, req.Branch, req.Task, req.Images, 0)
	} else {
		agentID, err = CreateCodeAgent(ctx, workDir, req.Task, req.Images, 0)
	}
	
	if err != nil {
		sendJSONResponse(w, nil, err)
		return
	}
	
	sendJSONResponse(w, map[string]string{"agent_id": agentID}, nil)
}

// handleWebGitDiff returns git diff for a path
func handleWebGitDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	path := r.URL.Query().Get("path")
	if path == "" {
		sendJSONResponse(w, nil, fmt.Errorf("path parameter required"))
		return
	}
	
	// Resolve path
	resolvedPath := resolvePath(path)
	
	// Get git diff
	diff, err := getGitDiff(resolvedPath)
	if err != nil {
		sendJSONResponse(w, nil, err)
		return
	}
	
	sendJSONResponse(w, map[string]string{"diff": diff}, nil)
}

// handleWebGitCommit commits and pushes changes
func handleWebGitCommit(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		Directory string `json:"directory"`
		Message   string `json:"message"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONResponse(w, nil, err)
		return
	}
	
	// Resolve directory
	workDir := resolvePath(req.Directory)
	
	// Validate git repo
	if !isGitRepo(workDir) {
		sendJSONResponse(w, nil, fmt.Errorf("not a git repository"))
		return
	}
	
	// Perform commit and push
	output, err := commitAndPush(workDir, req.Message)
	if err != nil {
		sendJSONResponse(w, nil, err)
		return
	}
	
	sendJSONResponse(w, map[string]string{"output": output}, nil)
}

// handleWebLS lists directory contents
func handleWebLS(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}
	
	// Resolve path
	resolvedPath := resolvePath(path)
	
	// List directory
	entries, err := os.ReadDir(resolvedPath)
	if err != nil {
		sendJSONResponse(w, nil, err)
		return
	}
	
	var files []map[string]interface{}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		files = append(files, map[string]interface{}{
			"name":    entry.Name(),
			"is_dir":  entry.IsDir(),
			"size":    info.Size(),
			"mode":    info.Mode().String(),
			"modtime": info.ModTime().Format(time.RFC3339),
		})
	}
	
	sendJSONResponse(w, files, nil)
}

// handleWebDownload downloads a file
func handleWebDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "path parameter required", http.StatusBadRequest)
		return
	}
	
	// Resolve path
	resolvedPath := resolvePath(filePath)
	
	// Check file exists and size
	info, err := os.Stat(resolvedPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	
	if info.IsDir() {
		http.Error(w, "cannot download directory", http.StatusBadRequest)
		return
	}
	
	if info.Size() > 50*1024*1024 {
		http.Error(w, "file too large (max 50MB)", http.StatusBadRequest)
		return
	}
	
	// Open file
	file, err := os.Open(resolvedPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()
	
	// Set headers
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(resolvedPath)))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	
	// Copy file to response
	io.Copy(w, file)
}

// handleWebRun runs a command in a directory
func handleWebRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		Directory string   `json:"directory"`
		Command   string   `json:"command"`
		Args      []string `json:"args"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONResponse(w, nil, err)
		return
	}
	
	// Resolve directory
	workDir := resolvePath(req.Directory)
	
	// Run command
	output, err := runCommand(workDir, req.Command, req.Args...)
	if err != nil {
		sendJSONResponse(w, nil, err)
		return
	}
	
	sendJSONResponse(w, map[string]string{"output": output}, nil)
}

// handleWebUsers manages authorized users
func handleWebUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// List users
		userMap := authorizedUsers.ListUsers()
		userList := make([]map[string]interface{}, 0, len(userMap))
		for username, userID := range userMap {
			userList = append(userList, map[string]interface{}{
				"username": username,
				"user_id": userID,
				"is_admin": userID == AdminUserID,
			})
		}
		sendJSONResponse(w, userList, nil)
		
	case "POST":
		// Add user
		var req struct {
			Username string `json:"username"`
			UserID   int64  `json:"user_id"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSONResponse(w, nil, err)
			return
		}
		
		if err := authorizedUsers.AddUser(req.Username, req.UserID); err != nil {
			sendJSONResponse(w, nil, err)
			return
		}
		
		sendJSONResponse(w, map[string]string{"message": "User added"}, nil)
		
	case "DELETE":
		// Remove user
		username := r.URL.Query().Get("username")
		if username == "" {
			sendJSONResponse(w, nil, fmt.Errorf("username parameter required"))
			return
		}
		
		if err := authorizedUsers.RemoveUser(username); err != nil {
			sendJSONResponse(w, nil, err)
			return
		}
		
		sendJSONResponse(w, map[string]string{"message": "User removed"}, nil)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleWebImages manages pending images
func handleWebImages(w http.ResponseWriter, r *http.Request) {
	// For web interface, we use a fixed user ID (0)
	webUserID := int64(0)
	
	switch r.Method {
	case "GET":
		// Get pending images
		images := getPendingImages(webUserID)
		sendJSONResponse(w, images, nil)
		
	case "POST":
		// Upload image
		err := r.ParseMultipartForm(10 << 20) // 10MB max
		if err != nil {
			sendJSONResponse(w, nil, err)
			return
		}
		
		file, header, err := r.FormFile("image")
		if err != nil {
			sendJSONResponse(w, nil, err)
			return
		}
		defer file.Close()
		
		// Create temp directory
		userTempDir := filepath.Join("data", "temp", fmt.Sprintf("user_%d", webUserID))
		if err := os.MkdirAll(userTempDir, 0755); err != nil {
			sendJSONResponse(w, nil, err)
			return
		}
		
		// Save file
		filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
		localPath := filepath.Join(userTempDir, filename)
		
		out, err := os.Create(localPath)
		if err != nil {
			sendJSONResponse(w, nil, err)
			return
		}
		defer out.Close()
		
		_, err = io.Copy(out, file)
		if err != nil {
			sendJSONResponse(w, nil, err)
			return
		}
		
		// Add to pending images
		addPendingImage(webUserID, localPath)
		
		sendJSONResponse(w, map[string]string{"path": localPath}, nil)
		
	case "DELETE":
		// Clear images
		clearPendingImages(webUserID)
		sendJSONResponse(w, map[string]string{"message": "Images cleared"}, nil)
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}