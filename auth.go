// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

type AuthorizedUsers struct {
	Users map[string]int64 `json:"users"` // username -> userID mapping
	mu    sync.RWMutex
}

var (
	authorizedUsers *AuthorizedUsers
	authFilePath    = "data/authorized_users.json"
)

func InitAuthorization() error {
	authorizedUsers = &AuthorizedUsers{
		Users: make(map[string]int64),
	}

	// Load existing authorized users from disk
	if err := authorizedUsers.Load(); err != nil {
		// If file doesn't exist, create with admin as the only authorized user
		if os.IsNotExist(err) {
			// Admin is always authorized
			authorizedUsers.Users["admin"] = AdminUserID
			return authorizedUsers.Save()
		}
		return err
	}

	// Ensure admin is always authorized
	authorizedUsers.mu.Lock()
	authorizedUsers.Users["admin"] = AdminUserID
	authorizedUsers.mu.Unlock()

	return authorizedUsers.Save()
}

func (au *AuthorizedUsers) Load() error {
	au.mu.Lock()
	defer au.mu.Unlock()

	data, err := os.ReadFile(authFilePath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, au)
}

func (au *AuthorizedUsers) Save() error {
	au.mu.RLock()
	defer au.mu.RUnlock()

	data, err := json.MarshalIndent(au, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(authFilePath, data, 0644)
}

func (au *AuthorizedUsers) IsAuthorized(userID int64) bool {
	au.mu.RLock()
	defer au.mu.RUnlock()

	// Admin is always authorized
	if userID == AdminUserID {
		return true
	}

	// Check if user ID is in the authorized list
	for _, id := range au.Users {
		if id == userID {
			return true
		}
	}

	return false
}

func (au *AuthorizedUsers) IsAuthorizedByUsername(username string) bool {
	au.mu.RLock()
	defer au.mu.RUnlock()

	_, exists := au.Users[username]
	return exists
}

func (au *AuthorizedUsers) AddUser(username string, userID int64) error {
	au.mu.Lock()
	defer au.mu.Unlock()

	au.Users[username] = userID
	return au.Save()
}

func (au *AuthorizedUsers) RemoveUser(username string) error {
	au.mu.Lock()
	defer au.mu.Unlock()

	// Don't allow removing admin
	if username == "admin" {
		return fmt.Errorf("cannot remove admin user")
	}

	delete(au.Users, username)
	return au.Save()
}

func (au *AuthorizedUsers) ListUsers() map[string]int64 {
	au.mu.RLock()
	defer au.mu.RUnlock()

	// Return a copy of the map
	users := make(map[string]int64)
	for k, v := range au.Users {
		users[k] = v
	}
	return users
}
