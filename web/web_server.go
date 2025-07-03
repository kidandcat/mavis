// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package web

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

var (
	webServer *http.Server
	// SSE removed - using meta refresh instead
	// sseClients   = make(map[chan SSEEvent]bool)
	// sseBroadcast = make(chan SSEEvent)
	// Authentication removed - running on local network only
)

// SSE removed - using meta refresh instead
// type SSEEvent struct {
// 	Type string      `json:"type"`
// 	Data interface{} `json:"data"`
// }

func StartWebServer(port string) error {
	mux := http.NewServeMux()

	// Static files
	mux.HandleFunc("/static/", serveStatic)
	mux.HandleFunc("/uploads/", serveUploads)

	// Main routes - serve full pages
	mux.HandleFunc("/", handleDashboard)
	mux.HandleFunc("/agents", handleDashboard)
	mux.HandleFunc("/files", handleDashboard)
	mux.HandleFunc("/git", handleDashboard)
	mux.HandleFunc("/system", handleDashboard)
	mux.HandleFunc("/mcps", handleDashboard)

	// API endpoints for AJAX
	mux.HandleFunc("/api/agent/", handleAgentRoutes)
	mux.HandleFunc("/api/code", handleCreateAgent)
	mux.HandleFunc("/api/git/diff", handleGitDiff)
	mux.HandleFunc("/api/git/commit", handleGitCommit)
	mux.HandleFunc("/api/git/pr/create", handlePRCreate)
	mux.HandleFunc("/api/git/pr/review", handlePRReview)
	mux.HandleFunc("/api/git/check-directory", handleCheckDirectory)
	mux.HandleFunc("/api/files/download", handleWebDownload)
	mux.HandleFunc("/api/command/run", handleWebRunCommand)
	mux.HandleFunc("/api/images", handleImageUpload)
	mux.HandleFunc("/api/system/restart", handleWebRestart)

	// JSON API endpoints
	mux.HandleFunc("/api/agents", handleWebAgents)
	mux.HandleFunc("/api/mcps", handleMCPRoutes)

	// SSE removed - using meta refresh instead
	// mux.HandleFunc("/events", handleSSE)

	// Authentication removed - running on local network only

	webServer = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
		// Longer timeouts for SSE connections
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  5 * time.Minute,
	}

	// SSE removed - using meta refresh instead
	// go sseEventBroadcaster()
	// go agentStatusBroadcaster()

	log.Printf("Starting web server on port %s", port)
	return webServer.ListenAndServe()
}

func handleAgentRoutes(w http.ResponseWriter, r *http.Request) {
	// Route to appropriate handler based on path
	path := r.URL.Path
	if strings.HasSuffix(path, "/status") {
		handleAgentStatus(w, r)
	} else if strings.HasSuffix(path, "/stop") {
		handleStopAgent(w, r)
	} else if strings.HasSuffix(path, "/delete") {
		handleDeleteAgent(w, r)
	} else {
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func handleMCPRoutes(w http.ResponseWriter, r *http.Request) {
	// Check for method override from forms
	isFormSubmission := false
	if r.Method == http.MethodPost {
		if method := r.FormValue("_method"); method != "" {
			r.Method = method
			isFormSubmission = true
		}
	}

	switch r.Method {
	case http.MethodGet:
		// List all MCPs
		mcps := mcpStore.List()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mcps)

	case http.MethodPost:
		// Create new MCP
		var mcp MCP

		// Check if this is form data
		if strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			// Parse form data
			if err := r.ParseForm(); err != nil {
				SetErrorFlash(w, "Failed to parse form data")
				http.Redirect(w, r, "/mcps", http.StatusSeeOther)
				return
			}

			mcp.Name = r.FormValue("name")
			mcp.Command = r.FormValue("command")

			// Parse args
			argsStr := r.FormValue("args")
			if argsStr != "" {
				for _, arg := range strings.Split(argsStr, ",") {
					if trimmed := strings.TrimSpace(arg); trimmed != "" {
						mcp.Args = append(mcp.Args, trimmed)
					}
				}
			}

			// Parse env
			envStr := r.FormValue("env")
			mcp.Env = make(map[string]string)
			if envStr != "" {
				for _, pair := range strings.Split(envStr, ",") {
					parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
					if len(parts) == 2 {
						mcp.Env[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				}
			}
		} else {
			// JSON request
			if err := json.NewDecoder(r.Body).Decode(&mcp); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		if err := mcpStore.Add(&mcp); err != nil {
			if strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
				SetErrorFlash(w, "Failed to add MCP: "+err.Error())
				http.Redirect(w, r, "/mcps", http.StatusSeeOther)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Form submission - redirect
		if strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			SetSuccessFlash(w, "MCP server added successfully")
			http.Redirect(w, r, "/mcps", http.StatusSeeOther)
			return
		}

		// JSON response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mcp)

	case http.MethodPut:
		// Update existing MCP
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing MCP ID", http.StatusBadRequest)
			return
		}

		var mcp MCP

		// Check if this is form data
		if strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			// Parse form data
			if err := r.ParseForm(); err != nil {
				SetErrorFlash(w, "Failed to parse form data")
				http.Redirect(w, r, "/mcps", http.StatusSeeOther)
				return
			}

			mcp.Name = r.FormValue("name")
			mcp.Command = r.FormValue("command")

			// Parse args
			argsStr := r.FormValue("args")
			if argsStr != "" {
				for _, arg := range strings.Split(argsStr, ",") {
					if trimmed := strings.TrimSpace(arg); trimmed != "" {
						mcp.Args = append(mcp.Args, trimmed)
					}
				}
			}

			// Parse env
			envStr := r.FormValue("env")
			mcp.Env = make(map[string]string)
			if envStr != "" {
				for _, pair := range strings.Split(envStr, ",") {
					parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
					if len(parts) == 2 {
						mcp.Env[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				}
			}
		} else {
			// JSON request
			if err := json.NewDecoder(r.Body).Decode(&mcp); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}

		if err := mcpStore.Update(id, &mcp); err != nil {
			if strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
				SetErrorFlash(w, "Failed to update MCP: "+err.Error())
				http.Redirect(w, r, "/mcps", http.StatusSeeOther)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Form submission - redirect
		if strings.Contains(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") {
			SetSuccessFlash(w, "MCP server updated successfully")
			http.Redirect(w, r, "/mcps", http.StatusSeeOther)
			return
		}

		// JSON response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mcp)

	case http.MethodDelete:
		// Delete MCP
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing MCP ID", http.StatusBadRequest)
			return
		}

		if err := mcpStore.Delete(id); err != nil {
			if isFormSubmission { // Form submission
				SetErrorFlash(w, "Failed to delete MCP: "+err.Error())
				http.Redirect(w, r, "/mcps", http.StatusSeeOther)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Form submission - redirect
		if isFormSubmission {
			SetSuccessFlash(w, "MCP server deleted successfully")
			http.Redirect(w, r, "/mcps", http.StatusSeeOther)
			return
		}

		// JSON response
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// SSE removed - using meta refresh instead
// func sseEventBroadcaster() {
// 	for {
// 		event := <-sseBroadcast
// 		for client := range sseClients {
// 			select {
// 			case client <- event:
// 			default:
// 				// Client's channel is full, close it
// 				close(client)
// 				delete(sseClients, client)
// 			}
// 		}
// 	}
// }

// SSE removed - using meta refresh instead
// func BroadcastSSEEvent(eventType string, data interface{}) {
// 	select {
// 	case sseBroadcast <- SSEEvent{Type: eventType, Data: data}:
// 	default:
// 		// Broadcast channel is full, skip
// 	}
// }

// SSE removed - using meta refresh instead
// func agentStatusBroadcaster() {
// 	ticker := time.NewTicker(2 * time.Second)
// 	defer ticker.Stop()
//
// 	for range ticker.C {
// 		agents := GetAllAgentsStatusJSON()
// 		// Broadcast agent status updates
// 		BroadcastSSEEvent("agents-update", agents)
// 	}
// }

// SSE removed - using meta refresh instead
// func handleSSE(w http.ResponseWriter, r *http.Request) {
// 	// Set headers for SSE
// 	w.Header().Set("Content-Type", "text/event-stream")
// 	w.Header().Set("Cache-Control", "no-cache")
// 	w.Header().Set("Connection", "keep-alive")
// 	w.Header().Set("X-Accel-Buffering", "no") // Disable Nginx buffering
//
// 	// Create client channel
// 	clientChan := make(chan SSEEvent, 10)
// 	sseClients[clientChan] = true
//
// 	// Remove client on disconnect
// 	defer func() {
// 		delete(sseClients, clientChan)
// 		close(clientChan)
// 	}()
//
// 	// Get flusher
// 	flusher, ok := w.(http.Flusher)
// 	if !ok {
// 		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
// 		return
// 	}
//
// 	// Send initial connection event
// 	fmt.Fprintf(w, "event: connected\ndata: {\"message\":\"Connected to Mavis\"}\n\n")
// 	flusher.Flush()
//
// 	// Send initial agents data
// 	agents := GetAllAgentsStatusJSON()
// 	data, _ := json.Marshal(agents)
// 	fmt.Fprintf(w, "event: agents-update\ndata: %s\n\n", data)
// 	flusher.Flush()
//
// 	// Create a ticker for heartbeat
// 	heartbeat := time.NewTicker(30 * time.Second)
// 	defer heartbeat.Stop()
//
// 	// Listen for events
// 	for {
// 		select {
// 		case event := <-clientChan:
// 			data, _ := json.Marshal(event.Data)
// 			_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
// 			if err != nil {
// 				log.Printf("SSE write error: %v", err)
// 				return
// 			}
// 			flusher.Flush()
// 		case <-heartbeat.C:
// 			// Send heartbeat to keep connection alive
// 			_, err := fmt.Fprintf(w, ": heartbeat\n\n")
// 			if err != nil {
// 				log.Printf("SSE heartbeat error: %v", err)
// 				return
// 			}
// 			flusher.Flush()
// 		case <-r.Context().Done():
// 			log.Println("SSE client disconnected")
// 			return
// 		}
// 	}
// }

// Authentication functions removed - running on local network only
