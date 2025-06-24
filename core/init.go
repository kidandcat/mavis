// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package core

var (
	// Global reference to admin user ID
	AdminUserID int64
)

// InitializeGlobals sets up the global references needed by the core package
func InitializeGlobals(adminID int64) {
	AdminUserID = adminID
}

// GetQueueTracker returns the global queue tracker instance
func GetQueueTracker() *QueueTracker {
	return queueTracker
}