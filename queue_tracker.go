// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"sync"
)

// QueuedAgentInfo tracks information about queued agents
type QueuedAgentInfo struct {
	QueueID string
	UserID  int64
	Folder  string
	Prompt  string
}

// QueueTracker tracks queued agents and their associated users
type QueueTracker struct {
	queuedAgents map[string]QueuedAgentInfo // queueID -> info
	mu           sync.RWMutex
}

// Global queue tracker instance
var queueTracker = &QueueTracker{
	queuedAgents: make(map[string]QueuedAgentInfo),
}

// RegisterQueuedAgent registers a queued agent with its user
func (qt *QueueTracker) RegisterQueuedAgent(queueID string, userID int64, folder, prompt string) {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	qt.queuedAgents[queueID] = QueuedAgentInfo{
		QueueID: queueID,
		UserID:  userID,
		Folder:  folder,
		Prompt:  prompt,
	}
}

// GetQueuedAgentInfo retrieves information about a queued agent
func (qt *QueueTracker) GetQueuedAgentInfo(queueID string) (QueuedAgentInfo, bool) {
	qt.mu.RLock()
	defer qt.mu.RUnlock()

	info, exists := qt.queuedAgents[queueID]
	return info, exists
}

// RemoveQueuedAgent removes a queued agent from tracking
func (qt *QueueTracker) RemoveQueuedAgent(queueID string) {
	qt.mu.Lock()
	defer qt.mu.Unlock()

	delete(qt.queuedAgents, queueID)
}

// GetQueuedAgentByFolder finds queued agents for a specific folder
func (qt *QueueTracker) GetQueuedAgentByFolder(folder string) []QueuedAgentInfo {
	qt.mu.RLock()
	defer qt.mu.RUnlock()

	var agents []QueuedAgentInfo
	for _, info := range qt.queuedAgents {
		if info.Folder == folder {
			agents = append(agents, info)
		}
	}

	return agents
}
