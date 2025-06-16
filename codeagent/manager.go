// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package codeagent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager manages multiple code agents
type Manager struct {
	agents       map[string]*Agent
	mu           sync.RWMutex
	nextID       int
	availableIDs []int // Pool of reusable IDs from cleaned up agents
}

// NewManager creates a new agent manager
func NewManager() *Manager {
	return &Manager{
		agents:       make(map[string]*Agent),
		nextID:       1,
		availableIDs: make([]int, 0),
	}
}

// LaunchAgent creates and starts a new agent
func (m *Manager) LaunchAgent(ctx context.Context, folder, prompt string) (string, error) {
	m.mu.Lock()
	var agentNum int
	if len(m.availableIDs) > 0 {
		// Reuse an available ID
		agentNum = m.availableIDs[0]
		m.availableIDs = m.availableIDs[1:]
	} else {
		// Use next sequential ID
		agentNum = m.nextID
		m.nextID++
	}
	id := fmt.Sprintf("%d", agentNum)
	m.mu.Unlock()

	agent := NewAgent(id, folder, prompt)

	m.mu.Lock()
	m.agents[id] = agent
	m.mu.Unlock()

	agent.StartAsync(ctx)

	return id, nil
}

// LaunchAgentWithID creates and starts a new agent with a custom ID
func (m *Manager) LaunchAgentWithID(ctx context.Context, id, folder, prompt string) error {
	m.mu.Lock()
	if _, exists := m.agents[id]; exists {
		m.mu.Unlock()
		return fmt.Errorf("agent with ID %s already exists", id)
	}
	m.mu.Unlock()

	agent := NewAgent(id, folder, prompt)

	m.mu.Lock()
	m.agents[id] = agent
	m.mu.Unlock()

	agent.StartAsync(ctx)

	return nil
}

// GetAgent returns an agent by ID
func (m *Manager) GetAgent(id string) (*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agents[id]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", id)
	}

	return agent, nil
}

// GetAgentInfo returns information about an agent
func (m *Manager) GetAgentInfo(id string) (AgentInfo, error) {
	agent, err := m.GetAgent(id)
	if err != nil {
		return AgentInfo{}, err
	}

	return agent.ToInfo(), nil
}

// ListAgents returns information about all agents
func (m *Manager) ListAgents() []AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]AgentInfo, 0, len(m.agents))
	for _, agent := range m.agents {
		infos = append(infos, agent.ToInfo())
	}

	return infos
}

// ListAgentsByStatus returns agents with a specific status
func (m *Manager) ListAgentsByStatus(status AgentStatus) []AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var infos []AgentInfo
	for _, agent := range m.agents {
		if agent.GetStatus() == status {
			infos = append(infos, agent.ToInfo())
		}
	}

	return infos
}

// KillAgent terminates a running agent
func (m *Manager) KillAgent(id string) error {
	agent, err := m.GetAgent(id)
	if err != nil {
		return err
	}

	return agent.Kill()
}

// RemoveAgent removes an agent from the manager
func (m *Manager) RemoveAgent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	agent, exists := m.agents[id]
	if !exists {
		return fmt.Errorf("agent %s not found", id)
	}

	// Kill if running
	if agent.GetStatus() == StatusRunning {
		if err := agent.Kill(); err != nil {
			return fmt.Errorf("failed to kill agent: %w", err)
		}
	}

	delete(m.agents, id)

	// Add the ID back to available pool for reuse
	var agentNum int
	if n, _ := fmt.Sscanf(id, "%d", &agentNum); n == 1 {
		m.availableIDs = append(m.availableIDs, agentNum)
	}

	return nil
}

// WaitForAgent waits for an agent to finish
func (m *Manager) WaitForAgent(ctx context.Context, id string) (AgentInfo, error) {
	agent, err := m.GetAgent(id)
	if err != nil {
		return AgentInfo{}, err
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return AgentInfo{}, ctx.Err()
		case <-ticker.C:
			status := agent.GetStatus()
			if status == StatusFinished || status == StatusFailed || status == StatusKilled {
				return agent.ToInfo(), nil
			}
		}
	}
}

// WaitForMultipleAgents waits for multiple agents to finish
func (m *Manager) WaitForMultipleAgents(ctx context.Context, ids []string) ([]AgentInfo, error) {
	var wg sync.WaitGroup
	results := make([]AgentInfo, len(ids))
	errors := make([]error, len(ids))

	for i, id := range ids {
		wg.Add(1)
		go func(index int, agentID string) {
			defer wg.Done()
			info, err := m.WaitForAgent(ctx, agentID)
			results[index] = info
			errors[index] = err
		}(i, id)
	}

	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

// CleanupFinishedAgents removes all finished, failed, or killed agents
func (m *Manager) CleanupFinishedAgents() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, agent := range m.agents {
		status := agent.GetStatus()
		if status == StatusFinished || status == StatusFailed || status == StatusKilled {
			delete(m.agents, id)
			count++

			// Add the ID back to available pool for reuse
			var agentNum int
			if n, _ := fmt.Sscanf(id, "%d", &agentNum); n == 1 {
				m.availableIDs = append(m.availableIDs, agentNum)
			}
		}
	}

	return count
}

// GetRunningCount returns the number of currently running agents
func (m *Manager) GetRunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, agent := range m.agents {
		if agent.GetStatus() == StatusRunning {
			count++
		}
	}

	return count
}

// GetTotalCount returns the total number of agents
func (m *Manager) GetTotalCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.agents)
}

