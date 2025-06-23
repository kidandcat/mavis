// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

var (
	webServer    *http.Server
	sseClients   = make(map[chan SSEEvent]bool)
	sseBroadcast = make(chan SSEEvent)
	// Authentication removed - running on local network only
)

type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

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

	// API endpoints for AJAX
	mux.HandleFunc("/api/agent/", handleAgentRoutes)
	mux.HandleFunc("/api/code", handleCreateAgent)
	mux.HandleFunc("/api/git/diff", handleGitDiff)
	mux.HandleFunc("/api/git/commit", handleGitCommit)
	mux.HandleFunc("/api/files/download", handleWebDownload)
	mux.HandleFunc("/api/command/run", handleWebRunCommand)
	mux.HandleFunc("/api/images", handleImageUpload)

	// JSON API endpoints
	mux.HandleFunc("/api/agents", handleWebAgents)

	// SSE endpoint for real-time updates
	mux.HandleFunc("/events", handleSSE)

	// Authentication removed - running on local network only

	webServer = &http.Server{
		Addr:    ":" + port,
		Handler: mux,
		// Longer timeouts for SSE connections
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 5 * time.Minute,
		IdleTimeout:  5 * time.Minute,
	}

	// Start SSE broadcaster
	go sseEventBroadcaster()

	// Start periodic agent status broadcaster
	go agentStatusBroadcaster()

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
	} else {
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

func sseEventBroadcaster() {
	for {
		event := <-sseBroadcast
		for client := range sseClients {
			select {
			case client <- event:
			default:
				// Client's channel is full, close it
				close(client)
				delete(sseClients, client)
			}
		}
	}
}

func BroadcastSSEEvent(eventType string, data interface{}) {
	select {
	case sseBroadcast <- SSEEvent{Type: eventType, Data: data}:
	default:
		// Broadcast channel is full, skip
	}
}

func agentStatusBroadcaster() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		agents := GetAllAgentsStatusJSON()
		// Broadcast agent status updates
		BroadcastSSEEvent("agents-update", agents)
	}
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable Nginx buffering

	// Create client channel
	clientChan := make(chan SSEEvent, 10)
	sseClients[clientChan] = true

	// Remove client on disconnect
	defer func() {
		delete(sseClients, clientChan)
		close(clientChan)
	}()

	// Get flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"message\":\"Connected to Mavis\"}\n\n")
	flusher.Flush()

	// Send initial agents data
	agents := GetAllAgentsStatusJSON()
	data, _ := json.Marshal(agents)
	fmt.Fprintf(w, "event: agents-update\ndata: %s\n\n", data)
	flusher.Flush()

	// Create a ticker for heartbeat
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	// Listen for events
	for {
		select {
		case event := <-clientChan:
			data, _ := json.Marshal(event.Data)
			_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			if err != nil {
				log.Printf("SSE write error: %v", err)
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			// Send heartbeat to keep connection alive
			_, err := fmt.Fprintf(w, ": heartbeat\n\n")
			if err != nil {
				log.Printf("SSE heartbeat error: %v", err)
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			log.Println("SSE client disconnected")
			return
		}
	}
}

// Authentication functions removed - running on local network only
