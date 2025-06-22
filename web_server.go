// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	webServer *http.Server
	sseClients = make(map[chan SSEEvent]bool)
	sseBroadcast = make(chan SSEEvent)
)

type SSEEvent struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func StartWebServer(port string) error {
	mux := http.NewServeMux()
	
	// Static files
	mux.HandleFunc("/static/", serveStatic)
	
	// Main dashboard
	mux.HandleFunc("/", requireAuth(handleWebDashboard))
	
	// API endpoints
	mux.HandleFunc("/api/agents", requireAuth(handleWebAgents))
	mux.HandleFunc("/api/agent/", requireAuth(handleWebAgent))
	mux.HandleFunc("/api/code", requireAuth(handleWebCode))
	mux.HandleFunc("/api/git/diff", requireAuth(handleWebGitDiff))
	mux.HandleFunc("/api/git/commit", requireAuth(handleWebGitCommit))
	mux.HandleFunc("/api/files/ls", requireAuth(handleWebLS))
	mux.HandleFunc("/api/files/download", requireAuth(handleWebDownload))
	mux.HandleFunc("/api/command/run", requireAuth(handleWebRun))
	mux.HandleFunc("/api/users", requireAuth(handleWebUsers))
	mux.HandleFunc("/api/images", requireAuth(handleWebImages))
	
	// SSE endpoint for real-time updates
	mux.HandleFunc("/events", requireAuth(handleSSE))
	
	// Authentication
	mux.HandleFunc("/login", handleWebLogin)
	mux.HandleFunc("/logout", handleWebLogout)
	
	webServer = &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	
	// Start SSE broadcaster
	go sseEventBroadcaster()
	
	log.Printf("Starting web server on port %s", port)
	return webServer.ListenAndServe()
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

func handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	
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
	
	// Listen for events
	for {
		select {
		case event := <-clientChan:
			data, _ := json.Marshal(event.Data)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func requireAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// For now, we'll implement a simple session-based auth
		// In production, this should use proper session management
		cookie, err := r.Cookie("mavis_session")
		if err != nil || cookie.Value == "" {
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		
		// TODO: Validate session
		handler(w, r)
	}
}

func handleWebLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		tmpl := `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Mavis - Login</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background-color: #f5f5f5;
            display: flex;
            align-items: center;
            justify-content: center;
            height: 100vh;
            margin: 0;
        }
        .login-container {
            background: white;
            padding: 2rem;
            border-radius: 8px;
            box-shadow: 0 2px 10px rgba(0,0,0,0.1);
            width: 100%;
            max-width: 400px;
        }
        h1 {
            text-align: center;
            color: #333;
            margin-bottom: 2rem;
        }
        .form-group {
            margin-bottom: 1rem;
        }
        label {
            display: block;
            margin-bottom: 0.5rem;
            color: #555;
        }
        input {
            width: 100%;
            padding: 0.75rem;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 1rem;
            box-sizing: border-box;
        }
        button {
            width: 100%;
            padding: 0.75rem;
            background-color: #007bff;
            color: white;
            border: none;
            border-radius: 4px;
            font-size: 1rem;
            cursor: pointer;
            transition: background-color 0.2s;
        }
        button:hover {
            background-color: #0056b3;
        }
        .error {
            color: #dc3545;
            margin-top: 1rem;
            text-align: center;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <h1>🤖 Mavis</h1>
        <form method="POST">
            <div class="form-group">
                <label for="password">Access Token</label>
                <input type="password" id="password" name="password" required autofocus>
            </div>
            <button type="submit">Login</button>
        </form>
    </div>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(tmpl))
		return
	}
	
	if r.Method == "POST" {
		password := r.FormValue("password")
		// TODO: Check against configured web password
		expectedPassword := os.Getenv("WEB_PASSWORD")
		if expectedPassword == "" {
			expectedPassword = "mavis" // Default password
		}
		
		if password == expectedPassword {
			// Set session cookie
			http.SetCookie(w, &http.Cookie{
				Name:     "mavis_session",
				Value:    "authenticated", // In production, use proper session ID
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteStrictMode,
			})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		
		// Invalid password
		http.Error(w, "Invalid password", http.StatusUnauthorized)
	}
}

func handleWebLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "mavis_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func handleWebDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl := `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Mavis Dashboard</title>
    <link rel="stylesheet" href="/static/css/style.css">
</head>
<body>
    <div id="app">
        <nav class="navbar">
            <div class="navbar-brand">
                <h1>🤖 Mavis</h1>
            </div>
            <div class="navbar-menu">
                <a href="#agents" class="navbar-item active">Agents</a>
                <a href="#files" class="navbar-item">Files</a>
                <a href="#git" class="navbar-item">Git</a>
                <a href="#system" class="navbar-item">System</a>
                <a href="/logout" class="navbar-item">Logout</a>
            </div>
        </nav>
        
        <main class="main-content">
            <section id="agents-section" class="section">
                <h2>Code Agents</h2>
                <div class="agent-controls">
                    <button id="new-agent-btn" class="btn btn-primary">New Agent</button>
                    <button id="refresh-agents-btn" class="btn">Refresh</button>
                </div>
                <div id="agents-list" class="agents-list">
                    <!-- Agents will be loaded here -->
                </div>
            </section>
            
            <section id="terminal" class="terminal-section">
                <h3>Output</h3>
                <div id="terminal-output" class="terminal">
                    <!-- Terminal output will appear here -->
                </div>
            </section>
        </main>
        
        <!-- New Agent Modal -->
        <div id="new-agent-modal" class="modal">
            <div class="modal-content">
                <h2>New Code Agent</h2>
                <form id="new-agent-form">
                    <div class="form-group">
                        <label for="agent-directory">Directory</label>
                        <input type="text" id="agent-directory" name="directory" required>
                    </div>
                    <div class="form-group">
                        <label for="agent-task">Task</label>
                        <textarea id="agent-task" name="task" rows="4" required></textarea>
                    </div>
                    <div class="form-group">
                        <label>
                            <input type="checkbox" name="new_branch"> Create new branch
                        </label>
                    </div>
                    <div class="form-actions">
                        <button type="submit" class="btn btn-primary">Create Agent</button>
                        <button type="button" class="btn" onclick="closeModal()">Cancel</button>
                    </div>
                </form>
            </div>
        </div>
    </div>
    
    <script src="/static/js/app.js"></script>
</body>
</html>`
	
	t := template.Must(template.New("dashboard").Parse(tmpl))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t.Execute(w, nil)
}

func serveStatic(w http.ResponseWriter, r *http.Request) {
	// Serve static files from data/web/static directory
	http.StripPrefix("/static/", http.FileServer(http.Dir("data/web/static"))).ServeHTTP(w, r)
}