package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMCPModal(t *testing.T) {
	// Test MCPModal rendering
	tests := []struct {
		name      string
		action    string
		mcp       *MCP
		wantError bool
	}{
		{
			name:      "Add modal",
			action:    "add",
			mcp:       nil,
			wantError: false,
		},
		{
			name:   "Edit modal",
			action: "edit",
			mcp: &MCP{
				ID:      "test-id",
				Name:    "test-mcp",
				Command: "/usr/bin/test",
				Args:    []string{"--arg1", "--arg2"},
				Env:     map[string]string{"KEY": "value"},
			},
			wantError: false,
		},
		{
			name:   "Delete modal",
			action: "delete",
			mcp: &MCP{
				ID:      "test-id",
				Name:    "test-mcp",
				Command: "/usr/bin/test",
			},
			wantError: false,
		},
		{
			name:      "Invalid action",
			action:    "invalid",
			mcp:       nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			modal := MCPModal(tt.action, tt.mcp)
			if tt.wantError && modal != nil {
				t.Errorf("Expected nil modal for invalid action, got modal")
			}
			if !tt.wantError && modal == nil {
				t.Errorf("Expected modal for action %s, got nil", tt.action)
			}
		})
	}
}

func TestMCPStore(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "mcp_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store := NewMCPStore(tmpFile.Name())

	// Test Add
	mcp1 := &MCP{
		Name:    "test-mcp-1",
		Command: "/usr/bin/test1",
		Args:    []string{"--arg1"},
		Env:     map[string]string{"KEY1": "value1"},
	}
	err = store.Add(mcp1)
	if err != nil {
		t.Errorf("Failed to add MCP: %v", err)
	}
	if mcp1.ID == "" {
		t.Error("MCP ID was not generated")
	}

	// Test List
	mcps := store.List()
	if len(mcps) != 1 {
		t.Errorf("Expected 1 MCP, got %d", len(mcps))
	}

	// Test Get
	retrieved, ok := store.Get(mcp1.ID)
	if !ok {
		t.Error("Failed to get MCP")
	}
	if retrieved.Name != mcp1.Name {
		t.Errorf("Expected name %s, got %s", mcp1.Name, retrieved.Name)
	}

	// Test Update
	updatedMCP := &MCP{
		Name:    "updated-mcp",
		Command: "/usr/bin/updated",
		Args:    []string{"--updated"},
		Env:     map[string]string{"UPDATED": "true"},
	}
	err = store.Update(mcp1.ID, updatedMCP)
	if err != nil {
		t.Errorf("Failed to update MCP: %v", err)
	}

	// Verify update
	retrieved, _ = store.Get(mcp1.ID)
	if retrieved.Name != updatedMCP.Name {
		t.Errorf("Expected updated name %s, got %s", updatedMCP.Name, retrieved.Name)
	}

	// Test Delete
	err = store.Delete(mcp1.ID)
	if err != nil {
		t.Errorf("Failed to delete MCP: %v", err)
	}

	// Verify deletion
	_, ok = store.Get(mcp1.ID)
	if ok {
		t.Error("MCP was not deleted")
	}
}

func TestHandleMCPRoutes(t *testing.T) {
	// Create a temporary file for testing
	tmpFile, err := os.CreateTemp("", "mcp_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Initialize the global mcpStore for testing
	mcpStore = NewMCPStore(tmpFile.Name())

	// Test POST (Create) with form data
	t.Run("Create MCP with form data", func(t *testing.T) {
		form := url.Values{}
		form.Add("name", "test-mcp")
		form.Add("command", "/usr/bin/test")
		form.Add("args", "arg1, arg2, arg3")
		form.Add("env", "KEY1=value1, KEY2=value2")

		req := httptest.NewRequest("POST", "/api/mcps", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		rr := httptest.NewRecorder()
		handleMCPRoutes(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Errorf("Expected status %d, got %d", http.StatusSeeOther, rr.Code)
		}

		// Check if MCP was created
		mcps := mcpStore.List()
		if len(mcps) != 1 {
			t.Errorf("Expected 1 MCP, got %d", len(mcps))
		}
		if mcps[0].Name != "test-mcp" {
			t.Errorf("Expected name 'test-mcp', got '%s'", mcps[0].Name)
		}
		if len(mcps[0].Args) != 3 {
			t.Errorf("Expected 3 args, got %d", len(mcps[0].Args))
		}
		if len(mcps[0].Env) != 2 {
			t.Errorf("Expected 2 env vars, got %d", len(mcps[0].Env))
		}
	})

	// Test POST (Create) with JSON
	t.Run("Create MCP with JSON", func(t *testing.T) {
		mcp := MCP{
			Name:    "json-mcp",
			Command: "/usr/bin/json",
			Args:    []string{"--json"},
			Env:     map[string]string{"JSON": "true"},
		}
		body, _ := json.Marshal(mcp)

		req := httptest.NewRequest("POST", "/api/mcps", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handleMCPRoutes(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var response MCP
		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}
		if response.Name != "json-mcp" {
			t.Errorf("Expected name 'json-mcp', got '%s'", response.Name)
		}
	})

	// Test GET (List)
	t.Run("List MCPs", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/mcps", nil)
		rr := httptest.NewRecorder()
		handleMCPRoutes(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, rr.Code)
		}

		var mcps []*MCP
		err := json.NewDecoder(rr.Body).Decode(&mcps)
		if err != nil {
			t.Errorf("Failed to decode response: %v", err)
		}
		if len(mcps) != 2 {
			t.Errorf("Expected 2 MCPs, got %d", len(mcps))
		}
	})

	// Test PUT (Update) with form data
	t.Run("Update MCP with form data", func(t *testing.T) {
		// Get an existing MCP ID
		mcps := mcpStore.List()
		if len(mcps) == 0 {
			t.Fatal("No MCPs available for update test")
		}
		mcpID := mcps[0].ID

		form := url.Values{}
		form.Add("_method", "PUT")
		form.Add("name", "updated-mcp")
		form.Add("command", "/usr/bin/updated")
		form.Add("args", "updated-arg")
		form.Add("env", "UPDATED=true")

		req := httptest.NewRequest("POST", "/api/mcps?id="+mcpID, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		rr := httptest.NewRecorder()
		handleMCPRoutes(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Errorf("Expected status %d, got %d", http.StatusSeeOther, rr.Code)
		}

		// Verify update
		updated, ok := mcpStore.Get(mcpID)
		if !ok {
			t.Error("Failed to get updated MCP")
		}
		if updated.Name != "updated-mcp" {
			t.Errorf("Expected name 'updated-mcp', got '%s'", updated.Name)
		}
	})

	// Test DELETE with form data
	t.Run("Delete MCP with form data", func(t *testing.T) {
		// Get an existing MCP ID
		mcps := mcpStore.List()
		if len(mcps) == 0 {
			t.Fatal("No MCPs available for delete test")
		}
		mcpID := mcps[0].ID

		form := url.Values{}
		form.Add("_method", "DELETE")

		req := httptest.NewRequest("POST", "/api/mcps?id="+mcpID, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		rr := httptest.NewRecorder()
		handleMCPRoutes(rr, req)

		if rr.Code != http.StatusSeeOther {
			t.Errorf("Expected status %d, got %d", http.StatusSeeOther, rr.Code)
		}

		// Verify deletion
		_, ok := mcpStore.Get(mcpID)
		if ok {
			t.Error("MCP was not deleted")
		}
	})
}

func TestMCPModalFormParsing(t *testing.T) {
	// Test parsing of args and env from form input
	tests := []struct {
		name     string
		argsStr  string
		envStr   string
		wantArgs []string
		wantEnv  map[string]string
	}{
		{
			name:     "Simple args and env",
			argsStr:  "arg1, arg2, arg3",
			envStr:   "KEY1=value1, KEY2=value2",
			wantArgs: []string{"arg1", "arg2", "arg3"},
			wantEnv:  map[string]string{"KEY1": "value1", "KEY2": "value2"},
		},
		{
			name:     "Args with spaces",
			argsStr:  " arg1 , arg2 , arg3 ",
			envStr:   " KEY1 = value1 , KEY2 = value2 ",
			wantArgs: []string{"arg1", "arg2", "arg3"},
			wantEnv:  map[string]string{"KEY1": "value1", "KEY2": "value2"},
		},
		{
			name:     "Empty args and env",
			argsStr:  "",
			envStr:   "",
			wantArgs: []string{},
			wantEnv:  map[string]string{},
		},
		{
			name:     "Complex env values",
			argsStr:  "--port, 8080, --verbose",
			envStr:   "API_KEY=abc123def456, DEBUG=true, PATH=/usr/bin:/usr/local/bin",
			wantArgs: []string{"--port", "8080", "--verbose"},
			wantEnv:  map[string]string{"API_KEY": "abc123def456", "DEBUG": "true", "PATH": "/usr/bin:/usr/local/bin"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse args
			var args []string
			if tt.argsStr != "" {
				for _, arg := range strings.Split(tt.argsStr, ",") {
					if trimmed := strings.TrimSpace(arg); trimmed != "" {
						args = append(args, trimmed)
					}
				}
			}

			// Parse env
			env := make(map[string]string)
			if tt.envStr != "" {
				for _, pair := range strings.Split(tt.envStr, ",") {
					parts := strings.SplitN(strings.TrimSpace(pair), "=", 2)
					if len(parts) == 2 {
						env[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
					}
				}
			}

			// Check args
			if len(args) != len(tt.wantArgs) {
				t.Errorf("Expected %d args, got %d", len(tt.wantArgs), len(args))
			}
			for i, arg := range args {
				if i < len(tt.wantArgs) && arg != tt.wantArgs[i] {
					t.Errorf("Expected arg[%d] = '%s', got '%s'", i, tt.wantArgs[i], arg)
				}
			}

			// Check env
			if len(env) != len(tt.wantEnv) {
				t.Errorf("Expected %d env vars, got %d", len(tt.wantEnv), len(env))
			}
			for k, v := range tt.wantEnv {
				if env[k] != v {
					t.Errorf("Expected env[%s] = '%s', got '%s'", k, v, env[k])
				}
			}
		})
	}
}

// TestMCPAgentIntegration tests MCP integration with agents
func TestMCPAgentIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "mcp_agent_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary MCP store
	tmpFile, err := os.CreateTemp("", "mcp_store_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store := NewMCPStore(tmpFile.Name())

	// Add a test MCP that will fail (invalid command)
	failingMCP := &MCP{
		ID:      "test-failing-mcp",
		Name:    "failing-mcp-server",
		Command: "/nonexistent/mcp/server",
		Args:    []string{"--test"},
		Env:     map[string]string{"TEST": "true"},
	}
	err = store.Add(failingMCP)
	if err != nil {
		t.Errorf("Failed to add test MCP: %v", err)
	}

	// Test 1: Create MCP config file
	t.Run("CreateMCPConfigFile", func(t *testing.T) {
		selectedMCPs := []string{failingMCP.ID}
		backupFile, err := CreateMCPConfigFile(tmpDir, selectedMCPs, store)
		if err != nil {
			t.Errorf("Failed to create MCP config: %v", err)
		}

		// Verify .mcp.json was created
		mcpFile := filepath.Join(tmpDir, ".mcp.json")
		if _, err := os.Stat(mcpFile); os.IsNotExist(err) {
			t.Error(".mcp.json was not created")
		}

		// Read and verify content
		data, err := os.ReadFile(mcpFile)
		if err != nil {
			t.Errorf("Failed to read .mcp.json: %v", err)
		}

		var config MCPConfig
		err = json.Unmarshal(data, &config)
		if err != nil {
			t.Errorf("Failed to parse .mcp.json: %v", err)
		}

		if len(config.MCPServers) != 1 {
			t.Errorf("Expected 1 MCP server, got %d", len(config.MCPServers))
		}

		if _, ok := config.MCPServers[failingMCP.Name]; !ok {
			t.Errorf("Expected MCP server '%s' not found", failingMCP.Name)
		}

		// Cleanup
		RestoreMCPConfigFile(tmpDir, backupFile)
	})

	// Test 2: Verify MCP server availability (this should fail)
	t.Run("VerifyMCPServerAvailability", func(t *testing.T) {
		// Test the new MCP verification functionality

		// Test with failing MCP
		err := VerifyMCPServer(failingMCP, tmpDir)
		if err == nil {
			t.Error("Expected error for non-existent MCP server, got nil")
		} else {
			t.Logf("Got expected error: %v", err)
		}

		// Test VerifyMCPServers function
		selectedMCPs := []string{failingMCP.ID}
		err = VerifyMCPServers(selectedMCPs, store, tmpDir)
		if err == nil {
			t.Error("Expected error for non-existent MCP servers, got nil")
		} else {
			t.Logf("Got expected error: %v", err)
		}

		// Add a valid MCP (using a common command)
		validMCP := &MCP{
			ID:      "test-valid-mcp",
			Name:    "valid-mcp-server",
			Command: "echo", // Echo should exist on most systems
			Args:    []string{"test"},
			Env:     map[string]string{},
		}
		err = store.Add(validMCP)
		if err != nil {
			t.Errorf("Failed to add valid MCP: %v", err)
		}

		// Test with valid MCP
		err = VerifyMCPServer(validMCP, tmpDir)
		if err != nil {
			t.Errorf("Expected no error for valid MCP server, got: %v", err)
		}
	})

	// Test 3: Agent should fail if MCP server is not available
	t.Run("AgentShouldFailWithoutMCP", func(t *testing.T) {
		// Test that agent creation fails when MCP server is not available

		// Save the global mcpStore temporarily
		oldStore := mcpStore
		mcpStore = store
		defer func() { mcpStore = oldStore }()

		// Try to create agent with failing MCP
		selectedMCPs := []string{failingMCP.ID}
		_, err := createCodeAgent("test task", tmpDir, selectedMCPs)

		if err == nil {
			t.Error("Expected error when creating agent with failing MCP, got nil")
		} else if !strings.Contains(err.Error(), "MCP server verification failed") {
			t.Errorf("Expected MCP verification error, got: %v", err)
		} else {
			t.Logf("Got expected error: %v", err)
		}

		// Verify no .mcp.json was left behind
		mcpFile := filepath.Join(tmpDir, ".mcp.json")
		if _, err := os.Stat(mcpFile); err == nil {
			t.Error(".mcp.json file was created even though MCP verification failed")
		}
	})
}
