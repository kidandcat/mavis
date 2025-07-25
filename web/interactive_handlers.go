// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package web

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
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
	
	// Redirect back to session view (without modal parameter to close it)
	http.Redirect(w, r, fmt.Sprintf("/interactive?modal=session-%s", agentID), http.StatusSeeOther)
}

// handleInteractiveStream provides HTTP streaming of conversation updates
func handleInteractiveStream(w http.ResponseWriter, r *http.Request) {
	// Extract session ID from URL
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	
	sessionID := pathParts[3]
	agent := interactiveManager.GetAgent(sessionID)
	if agent == nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	
	// Set headers for chunked transfer encoding
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	
	// Enable flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}
	
	// Write initial HTML structure
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Interactive Session Stream</title>
<link rel="stylesheet" href="/static/css/minimal.css">
<style>
html, body { 
	margin: 0; 
	padding: 0;
	height: 100vh;
	background: var(--surface);
	font-family: var(--font-family);
	overflow-y: auto;
}
body {
	padding: 20px;
	box-sizing: border-box;
}
.conversation-view {
	display: flex;
	flex-direction: column;
	gap: 1rem;
	min-height: calc(100vh - 40px); /* Account for padding */
	overflow-y: auto;
}
</style>
</head>
<body>
<div class="conversation-view">
`)
	flusher.Flush()
	
	// Subscribe to agent updates
	subID, updates := agent.Subscribe()
	defer agent.Unsubscribe(subID)
	
	// Send current message history in reverse order (newest first)
	messages := agent.GetMessageHistory()
	lastMessageCount := len(messages)
	
	for i := len(messages) - 1; i >= 0; i-- {
		msgHTML := renderMessageHTML(messages[i])
		if msgHTML != "" {
			fmt.Fprintf(w, `<div style="order: %d;">%s</div>`, -i, msgHTML)
		}
	}
	
	flusher.Flush()
	
	// Stream updates
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	ctx := r.Context()
	
	for {
		select {
		case <-ctx.Done():
			return
			
		case <-updates:
			// Get latest messages
			messages = agent.GetMessageHistory()
			
			// Send only new messages at the top
			for i := len(messages) - 1; i >= lastMessageCount; i-- {
				msgHTML := renderMessageHTML(messages[i])
				if msgHTML != "" {
					fmt.Fprintf(w, `<div style="order: %d;">%s</div>`, -i, msgHTML)
				}
			}
			
			lastMessageCount = len(messages)
			
			flusher.Flush()
			
		case <-ticker.C:
			// Send keep-alive comment
			fmt.Fprint(w, "<!-- keepalive -->\n")
			flusher.Flush()
		}
	}
}

// renderMessageHTML renders a message as HTML string
func renderMessageHTML(msg codeagent.Message) string {
	// Skip messages that are just UI elements
	content := strings.TrimSpace(msg.Content)
	if isUIContent(content) {
		return ""
	}
	
	switch msg.Type {
	case "user":
		return fmt.Sprintf(`<div class="message message-user">
<div class="message-header">
<span class="message-type">You</span>
<span class="message-time">%s</span>
</div>
<div class="message-content"><pre>%s</pre></div>
</div>`, msg.Timestamp.Format("3:04 PM"), html.EscapeString(content))
		
	case "assistant":
		formatted := formatAssistantContentHTML(content)
		tokens := ""
		if msg.Metadata["tokens"] != "" {
			tokens = fmt.Sprintf(`<div class="message-tokens"><span>%s</span></div>`, html.EscapeString(msg.Metadata["tokens"]))
		}
		return fmt.Sprintf(`<div class="message message-assistant">
<div class="message-header">
<span class="message-type">Claude</span>
<span class="message-time">%s</span>
</div>
<div class="message-content">%s</div>
%s
</div>`, msg.Timestamp.Format("3:04 PM"), formatted, tokens)
		
	case "tool":
		// Skip tool status updates that are just UI
		if strings.Contains(content, "tokens") || strings.Contains(content, "esc to interrupt") {
			return ""
		}
		return fmt.Sprintf(`<div class="message message-tool">
<div class="message-header">
<span class="message-type tool-indicator">%s</span>
<span class="message-time">%s</span>
</div>
<div class="message-content"><code>%s</code></div>
</div>`, getToolIconHTML(content), msg.Timestamp.Format("3:04 PM"), html.EscapeString(content))
		
	default:
		return fmt.Sprintf(`<div class="message message-system">
<div class="message-header">
<span class="message-type">System</span>
<span class="message-time">%s</span>
</div>
<div class="message-content"><pre>%s</pre></div>
</div>`, msg.Timestamp.Format("3:04 PM"), html.EscapeString(content))
	}
}

// isUIContent checks if content is just UI elements that should be filtered
func isUIContent(content string) bool {
	// Remove any remaining ANSI codes for checking
	cleaned := stripANSICodes(content)
	cleaned = strings.TrimSpace(cleaned)
	
	// Skip empty content
	if cleaned == "" {
		return true
	}
	
	// Skip UI border elements
	if strings.HasPrefix(cleaned, "╭") || strings.HasPrefix(cleaned, "╰") || 
	   strings.HasPrefix(cleaned, "│") || strings.Contains(cleaned, "─────") {
		return true
	}
	
	// Skip common UI messages
	uiPatterns := []string{
		"? for shortcuts",
		"Bypassing Permissions",
		"Auto-update failed",
		"Try claude doctor",
		"npm i -g @anthropic-ai/claude-code",
		"@anthropic-ai/claude-code",
		"✗ Auto-update failed",
		"Try /inst",
		"esc to interrupt",
		"tokens",
		"Update Todos",
		"↑", "↓", "⚒", // Status indicators
	}
	
	for _, pattern := range uiPatterns {
		if strings.Contains(cleaned, pattern) {
			return true
		}
	}
	
	// Skip lines that are ONLY box drawing characters
	if isOnlyBoxDrawing(cleaned) {
		return true
	}
	
	// Skip tool execution patterns like "⏺ Read(file.go)" or "Read(file.go)"
	if isToolExecutionPattern(cleaned) {
		return true
	}
	
	return false
}

// isToolExecutionPattern checks if content matches tool execution patterns
func isToolExecutionPattern(content string) bool {
	// Pattern matches: "⏺ ToolName(params)" or "ToolName(params)"
	// Examples: "⏺ Read(web/interactive_handlers.go)", "Read(web/interactive_handlers.go)"
	toolPattern := regexp.MustCompile(`^(?:⏺\s+)?[A-Za-z][A-Za-z0-9]*\([^)]*\)\s*$`)
	return toolPattern.MatchString(content)
}

// stripANSICodes removes ANSI escape codes from text
func stripANSICodes(text string) string {
	// Remove ANSI escape sequences
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07`)
	return ansiRegex.ReplaceAllString(text, "")
}

// isOnlyBoxDrawing checks if a string contains only box drawing characters
func isOnlyBoxDrawing(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '─' && r != '│' && r != '╭' && r != '╮' && 
		   r != '╯' && r != '╰' && r != '┴' && r != '┬' && r != '├' && 
		   r != '┤' && r != '┼' && r != '═' && r != '║' && r != '╔' && 
		   r != '╗' && r != '╝' && r != '╚' {
			return false
		}
	}
	return true
}

// Helper functions for HTML rendering
func formatAssistantContentHTML(content string) string {
	// Simple markdown-like formatting
	escaped := html.EscapeString(content)
	
	// Convert code blocks
	escaped = strings.ReplaceAll(escaped, "```", "</pre><pre>")
	if strings.Count(escaped, "<pre>")%2 != 0 {
		escaped += "</pre>"
	}
	
	// Convert inline code
	parts := strings.Split(escaped, "`")
	for i := 1; i < len(parts); i += 2 {
		if i < len(parts) {
			parts[i] = "<code>" + parts[i] + "</code>"
		}
	}
	escaped = strings.Join(parts, "")
	
	// Convert line breaks
	escaped = strings.ReplaceAll(escaped, "\n", "<br>")
	
	return escaped
}

func getToolIconHTML(content string) string {
	if strings.Contains(content, "✓") {
		return "✓"
	} else if strings.Contains(content, "✗") {
		return "✗"
	} else if strings.Contains(content, "⏺") {
		return "⏺"
	}
	return "🔧"
}