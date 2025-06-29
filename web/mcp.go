package web

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Global MCP store instance
var mcpStore *MCPStore

// MCP represents a Model Context Protocol server configuration
type MCP struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// MCPConfig represents the .mcp.json file format
type MCPConfig struct {
	MCPServers map[string]MCPServer `json:"mcpServers"`
}

// MCPServer represents a server configuration in .mcp.json
type MCPServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// MCPStore manages MCP configurations
type MCPStore struct {
	mu   sync.RWMutex
	mcps map[string]*MCP
	file string
}

// NewMCPStore creates a new MCP store
func NewMCPStore(configFile string) *MCPStore {
	store := &MCPStore{
		mcps: make(map[string]*MCP),
		file: configFile,
	}
	store.load()
	return store
}

// load reads MCPs from the config file
func (s *MCPStore) load() error {
	data, err := os.ReadFile(s.file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist yet, that's fine
		}
		return err
	}

	var mcps []*MCP
	if err := json.Unmarshal(data, &mcps); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.mcps = make(map[string]*MCP)
	for _, mcp := range mcps {
		s.mcps[mcp.ID] = mcp
	}
	return nil
}

// save writes MCPs to the config file
func (s *MCPStore) save() error {
	s.mu.RLock()
	mcps := make([]*MCP, 0, len(s.mcps))
	for _, mcp := range s.mcps {
		mcps = append(mcps, mcp)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(mcps, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.file, data, 0644)
}

// List returns all MCPs
func (s *MCPStore) List() []*MCP {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	mcps := make([]*MCP, 0, len(s.mcps))
	for _, mcp := range s.mcps {
		mcps = append(mcps, mcp)
	}
	return mcps
}

// Get returns an MCP by ID
func (s *MCPStore) Get(id string) (*MCP, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	mcp, ok := s.mcps[id]
	return mcp, ok
}

// Add creates a new MCP
func (s *MCPStore) Add(mcp *MCP) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if mcp.ID == "" {
		mcp.ID = generateID()
	}
	
	s.mcps[mcp.ID] = mcp
	return s.save()
}

// Update modifies an existing MCP
func (s *MCPStore) Update(id string, mcp *MCP) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	if _, ok := s.mcps[id]; !ok {
		return fmt.Errorf("MCP not found: %s", id)
	}
	
	mcp.ID = id
	s.mcps[id] = mcp
	return s.save()
}

// Delete removes an MCP
func (s *MCPStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	delete(s.mcps, id)
	return s.save()
}

// CreateMCPConfigFile creates a .mcp.json file in the working directory
func CreateMCPConfigFile(workDir string, selectedMCPs []string, store *MCPStore) (string, error) {
	mcpFile := filepath.Join(workDir, ".mcp.json")
	
	// Check if file already exists and back it up
	backupFile := ""
	if _, err := os.Stat(mcpFile); err == nil {
		backupFile = mcpFile + ".backup"
		if err := os.Rename(mcpFile, backupFile); err != nil {
			return "", fmt.Errorf("failed to backup existing .mcp.json: %w", err)
		}
	}
	
	// Create new config
	config := MCPConfig{
		MCPServers: make(map[string]MCPServer),
	}
	
	for _, mcpID := range selectedMCPs {
		mcp, ok := store.Get(mcpID)
		if !ok {
			continue
		}
		
		config.MCPServers[mcp.Name] = MCPServer{
			Command: mcp.Command,
			Args:    mcp.Args,
			Env:     mcp.Env,
		}
	}
	
	// Write config file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	
	if err := os.WriteFile(mcpFile, data, 0644); err != nil {
		return "", err
	}
	
	return backupFile, nil
}

// RestoreMCPConfigFile restores the original .mcp.json file
func RestoreMCPConfigFile(workDir string, backupFile string) error {
	mcpFile := filepath.Join(workDir, ".mcp.json")
	
	// Remove the temporary file
	os.Remove(mcpFile)
	
	// Restore backup if it exists
	if backupFile != "" {
		return os.Rename(backupFile, mcpFile)
	}
	
	return nil
}

// generateID creates a unique ID for MCPs
func generateID() string {
	return fmt.Sprintf("mcp-%d", time.Now().UnixNano())
}