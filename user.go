// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

// GetUserForAgent returns the chat ID of the user who should be notified about an agent
func GetUserForAgent(agentID string) (int64, bool) {
	agentUserMu.RLock()
	defer agentUserMu.RUnlock()
	chatID, exists := agentUserMap[agentID]
	return chatID, exists
}

// UnregisterAgent removes an agent from the notification map
func UnregisterAgent(agentID string) {
	agentUserMu.Lock()
	defer agentUserMu.Unlock()
	delete(agentUserMap, agentID)
}
