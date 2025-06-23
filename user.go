// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

// GetUserForAgent returns the chat ID of the user who should be notified about an agent
// In single-user mode, this always returns the AdminUserID
func GetUserForAgent(agentID string) (int64, bool) {
	return AdminUserID, true
}

// UnregisterAgent - no longer needed in single-user mode
func UnregisterAgent(agentID string) {
	// No-op in single-user mode
}
