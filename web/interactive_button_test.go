// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewSessionButton(t *testing.T) {
	setupTest(t)
	
	// Test that the button exists and has correct href
	t.Run("New Session Button Exists", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/interactive", nil)
		w := httptest.NewRecorder()
		
		handleDashboard(w, req)
		
		resp := w.Result()
		body := w.Body.String()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
		
		// Check for the button with proper href
		if !strings.Contains(body, `href="/interactive?modal=create"`) {
			t.Error("New Session button with correct href not found")
			
			// Log part of the body to help debug
			idx := strings.Index(body, "New Session")
			if idx > 0 {
				start := idx - 100
				if start < 0 {
					start = 0
				}
				end := idx + 100
				if end > len(body) {
					end = len(body)
				}
				t.Logf("Context around 'New Session': %s", body[start:end])
			}
		}
		
		// Also check that the button text exists
		if !strings.Contains(body, "New Session") {
			t.Error("'New Session' text not found in page")
		}
	})
	
	// Test that clicking the button (following the href) shows the modal
	t.Run("Clicking New Session Shows Modal", func(t *testing.T) {
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
			t.Error("Modal title 'New Interactive Session' not found")
		}
		
		if !strings.Contains(body, "Working Directory") {
			t.Error("Working Directory field not found in modal")
		}
		
		if !strings.Contains(body, "Start Session") {
			t.Error("Start Session button not found in modal")
		}
		
		// Check the form action
		if !strings.Contains(body, `action="/api/interactive"`) {
			t.Error("Form with correct action not found")
		}
	})
}

func TestInteractivePageStructure(t *testing.T) {
	setupTest(t)
	
	// Test the overall page structure
	t.Run("Page Has Correct Structure", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/interactive", nil)
		w := httptest.NewRecorder()
		
		handleDashboard(w, req)
		
		body := w.Body.String()
		
		// Check for key structural elements
		checks := []struct {
			name     string
			contains string
		}{
			{"Title", "Interactive Sessions"},
			{"Navigation Link", `href="/interactive"`},
			{"Container", `class="container"`},
			{"Button Class", `class="btn btn-primary"`},
		}
		
		for _, check := range checks {
			if !strings.Contains(body, check.contains) {
				t.Errorf("%s: expected to find '%s'", check.name, check.contains)
			}
		}
	})
}