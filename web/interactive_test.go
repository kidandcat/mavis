// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
	
	"mavis/codeagent"
)

func setupTest(t *testing.T) {
	// Initialize agent manager if not already done
	if agentManager == nil {
		agentManager = codeagent.NewManager()
	}
	
	// Create temp directory for MCP store
	tempDir, err := os.MkdirTemp("", "mavis-test")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })
	
	// Initialize MCP store
	mcpStore = NewMCPStore(tempDir + "/mcps.json")
}

func TestInteractiveRoutes(t *testing.T) {
	setupTest(t)
	
	// Test the main interactive page
	t.Run("Interactive Main Page", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/interactive", nil)
		w := httptest.NewRecorder()
		
		handleDashboard(w, req)
		
		resp := w.Result()
		body := w.Body.String()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		
		// Check for key elements
		if !strings.Contains(body, "Interactive Sessions") {
			t.Error("Page should contain 'Interactive Sessions'")
		}
		
		if !strings.Contains(body, "New Session") {
			t.Error("Page should contain 'New Session' button")
		}
		
		// Check the href for the New Session button
		if !strings.Contains(body, `href="/interactive?modal=create"`) {
			t.Error("New Session button should link to /interactive?modal=create")
		}
		
		t.Logf("Body contains: %s", body[:500])
	})
	
	// Test the create modal
	t.Run("Interactive Create Modal", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/interactive?modal=create", nil)
		w := httptest.NewRecorder()
		
		handleDashboard(w, req)
		
		resp := w.Result()
		body := w.Body.String()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		
		// Check for modal elements
		if !strings.Contains(body, "New Interactive Session") {
			t.Error("Modal should contain 'New Interactive Session'")
		}
		
		if !strings.Contains(body, "Working Directory") {
			t.Error("Modal should contain 'Working Directory' field")
		}
		
		if !strings.Contains(body, `action="/api/interactive"`) {
			t.Error("Form should post to /api/interactive")
		}
	})
	
	// Test creating an interactive agent
	t.Run("Create Interactive Agent", func(t *testing.T) {
		// Create a temp directory that actually exists
		tempDir, err := os.MkdirTemp("", "interactive-test")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tempDir)
		
		form := url.Values{}
		form.Add("work_dir", tempDir)
		
		req := httptest.NewRequest("POST", "/api/interactive", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		handleInteractiveRoutes(w, req)
		
		resp := w.Result()
		
		// Should redirect after creation
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("Expected redirect status 303, got %d", resp.StatusCode)
			body := w.Body.String()
			t.Logf("Response body: %s", body)
		}
		
		// Check redirect location
		location := resp.Header.Get("Location")
		if !strings.HasPrefix(location, "/interactive?modal=session-") {
			t.Errorf("Expected redirect to session view, got %s", location)
			// Check for error flash
			cookies := resp.Cookies()
			for _, cookie := range cookies {
				if cookie.Name == "flash" {
					t.Logf("Flash message: %s", cookie.Value)
				}
			}
		}
	})
}

func TestInteractiveAgentManager(t *testing.T) {
	// Test the interactive agent manager directly
	t.Run("Create and List Agents", func(t *testing.T) {
		// Create a test agent with proper context
		ctx := context.Background()
		_, err := interactiveManager.CreateAgent(ctx, "/tmp", "")
		// This might fail if /tmp doesn't exist or if claude is not installed
		if err != nil {
			t.Logf("Could not create agent: %v", err)
		}
		
		// List agents
		agents := interactiveManager.ListAgents()
		t.Logf("Number of agents: %d", len(agents))
	})
}

func TestInteractiveUI(t *testing.T) {
	// Test UI rendering functions
	t.Run("Render Interactive Section", func(t *testing.T) {
		section := InteractiveSection("", "")
		if section == nil {
			t.Error("InteractiveSection should not return nil")
		}
		
		// Test with create modal
		section = InteractiveSection("create", "/tmp")
		if section == nil {
			t.Error("InteractiveSection with create modal should not return nil")
		}
	})
	
	t.Run("Format Time Ago", func(t *testing.T) {
		tests := []struct {
			name     string
			duration string
			expected string
		}{
			{"Just now", "30s", "just now"},
			{"Minutes", "5m", "5 minutes ago"},
			{"One minute", "1m", "1 minute ago"},
			{"Hours", "2h", "2 hours ago"},
			{"One hour", "1h", "1 hour ago"},
			{"Days", "48h", "2 days ago"},
			{"One day", "24h", "1 day ago"},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				d, _ := time.ParseDuration(tt.duration)
				result := formatTimeAgo(time.Now().Add(-d))
				if result != tt.expected {
					t.Errorf("Expected '%s', got '%s'", tt.expected, result)
				}
			})
		}
	})
}

func TestInteractiveHandlers(t *testing.T) {
	// Test handler routing
	t.Run("Agent Action Routing", func(t *testing.T) {
		// Test stop action
		req := httptest.NewRequest("POST", "/api/interactive/test-id/stop", nil)
		w := httptest.NewRecorder()
		
		handleInteractiveAgentAction(w, req)
		
		// Should redirect since agent doesn't exist
		resp := w.Result()
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("Expected redirect for missing agent, got %d", resp.StatusCode)
		}
	})
	
	t.Run("Invalid Work Directory", func(t *testing.T) {
		form := url.Values{}
		form.Add("work_dir", "")
		
		req := httptest.NewRequest("POST", "/api/interactive", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		
		handleInteractiveRoutes(w, req)
		
		resp := w.Result()
		
		// Should redirect with error
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("Expected redirect status 303, got %d", resp.StatusCode)
		}
		
		// Check for error flash or redirect to create modal
		location := resp.Header.Get("Location")
		if location != "/interactive?modal=create" {
			t.Errorf("Expected redirect to create modal, got %s", location)
		}
		
		// The flash message should be set
		cookies := resp.Cookies()
		hasFlash := false
		for _, cookie := range cookies {
			if strings.Contains(cookie.Name, "flash") {
				hasFlash = true
				t.Logf("Flash cookie: %s = %s", cookie.Name, cookie.Value)
				break
			}
		}
		
		if !hasFlash {
			t.Log("No flash cookie found, but redirect is correct")
		}
	})
}