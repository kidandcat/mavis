// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"sync"
	"time"

	"mavis/codeagent"
)

// TrackedAgent represents a single source of truth for agent tracking
type TrackedAgent struct {
	Agent    *codeagent.Agent
	UserID   int64
	QueueID  string // If this was queued
	Folder   string
	Started  time.Time
	Notified bool // Whether completion has been notified
}

// UnifiedAgentTracker manages all agent tracking in one place
type UnifiedAgentTracker struct {
	agents map[string]*TrackedAgent // agentID -> TrackedAgent
	mu     sync.RWMutex
}

// NewUnifiedAgentTracker creates a new unified tracker
func NewUnifiedAgentTracker() *UnifiedAgentTracker {
	return &UnifiedAgentTracker{
		agents: make(map[string]*TrackedAgent),
	}
}

// RegisterAgent registers a new agent with all its metadata
func (t *UnifiedAgentTracker) RegisterAgent(agentID string, agent *codeagent.Agent, userID int64, queueID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.agents[agentID] = &TrackedAgent{
		Agent:    agent,
		UserID:   userID,
		QueueID:  queueID,
		Folder:   agent.Folder,
		Started:  time.Now(),
		Notified: false,
	}
}

// GetAgent retrieves a tracked agent by ID
func (t *UnifiedAgentTracker) GetAgent(agentID string) (*TrackedAgent, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	agent, exists := t.agents[agentID]
	return agent, exists
}

// RemoveAgent removes an agent from tracking
func (t *UnifiedAgentTracker) RemoveAgent(agentID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.agents, agentID)
}

// MarkNotified marks an agent as having sent completion notification
func (t *UnifiedAgentTracker) MarkNotified(agentID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if agent, exists := t.agents[agentID]; exists {
		agent.Notified = true
	}
}

// GetUnnotifiedCompletedAgents returns agents that have completed but not been notified
func (t *UnifiedAgentTracker) GetUnnotifiedCompletedAgents() []*TrackedAgent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var unnotified []*TrackedAgent
	for _, tracked := range t.agents {
		if !tracked.Notified && tracked.Agent != nil {
			status := tracked.Agent.GetStatus()
			if status == codeagent.StatusFinished ||
				status == codeagent.StatusFailed ||
				status == codeagent.StatusKilled {
				unnotified = append(unnotified, tracked)
			}
		}
	}

	return unnotified
}

// GetAgentsByUser returns all agents for a specific user
func (t *UnifiedAgentTracker) GetAgentsByUser(userID int64) []*TrackedAgent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var userAgents []*TrackedAgent
	for _, tracked := range t.agents {
		if tracked.UserID == userID {
			userAgents = append(userAgents, tracked)
		}
	}

	return userAgents
}

// GetRunningAgentInFolder returns the running agent in a specific folder
func (t *UnifiedAgentTracker) GetRunningAgentInFolder(folder string) (*TrackedAgent, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	for _, tracked := range t.agents {
		if tracked.Folder == folder && tracked.Agent != nil &&
			tracked.Agent.GetStatus() == codeagent.StatusRunning {
			return tracked, true
		}
	}

	return nil, false
}

// GetOrphanedAgents returns agents without user association
func (t *UnifiedAgentTracker) GetOrphanedAgents() []*TrackedAgent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var orphaned []*TrackedAgent
	for _, tracked := range t.agents {
		if tracked.UserID == 0 {
			orphaned = append(orphaned, tracked)
		}
	}

	return orphaned
}

// GetOldCompletedAgents returns completed agents older than the specified duration
func (t *UnifiedAgentTracker) GetOldCompletedAgents(olderThan time.Duration) []*TrackedAgent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var old []*TrackedAgent
	now := time.Now()

	for _, tracked := range t.agents {
		if tracked.Agent != nil {
			status := tracked.Agent.GetStatus()
			if (status == codeagent.StatusFinished ||
				status == codeagent.StatusFailed ||
				status == codeagent.StatusKilled) &&
				!tracked.Agent.EndTime.IsZero() &&
				now.Sub(tracked.Agent.EndTime) > olderThan {
				old = append(old, tracked)
			}
		}
	}

	return old
}

// GetAllAgents returns all tracked agents
func (t *UnifiedAgentTracker) GetAllAgents() map[string]*TrackedAgent {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Create a copy to avoid holding the lock
	copy := make(map[string]*TrackedAgent)
	for id, agent := range t.agents {
		copy[id] = agent
	}

	return copy
}