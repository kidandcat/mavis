// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package telegram

import (
	"fmt"
	"net/http"
	"os"
	"sync"

	"mavis/codeagent"

	"github.com/go-telegram/bot"
)

var (
	// Global references set during initialization
	b            *bot.Bot
	agentManager *codeagent.Manager
	AdminUserID  int64

	// Image tracking for users
	userPendingImages  = make(map[int64][]string) // userID -> array of image paths
	pendingImagesMutex sync.RWMutex

	// LAN server tracking
	lanServerProcess *os.Process
	lanHTTPServer    *http.Server
	lanServerPort    string
	lanServerWorkDir string
	lanServerCmd     string
	lanServerMutex   sync.Mutex
	
	// LAN domain name for mDNS
	lanDomainName = "mavis.local"
)

// InitializeGlobals sets up the global references needed by the telegram package
func InitializeGlobals(botInstance *bot.Bot, manager *codeagent.Manager, adminID int64) {
	b = botInstance
	agentManager = manager
	AdminUserID = adminID
}

func addPendingImage(userID int64, imagePath string) {
	pendingImagesMutex.Lock()
	defer pendingImagesMutex.Unlock()

	userPendingImages[userID] = append(userPendingImages[userID], imagePath)
}

func getPendingImageCount(userID int64) int {
	pendingImagesMutex.RLock()
	defer pendingImagesMutex.RUnlock()

	return len(userPendingImages[userID])
}

func getPendingImages(userID int64) []string {
	pendingImagesMutex.RLock()
	defer pendingImagesMutex.RUnlock()

	images := make([]string, len(userPendingImages[userID]))
	copy(images, userPendingImages[userID])
	return images
}

func clearPendingImages(userID int64) {
	pendingImagesMutex.Lock()
	defer pendingImagesMutex.Unlock()

	// Delete the image files
	if images, exists := userPendingImages[userID]; exists {
		for _, imagePath := range images {
			os.Remove(imagePath)
		}
	}

	delete(userPendingImages, userID)
}

// UnregisterAgent - no longer needed in single-user mode
func UnregisterAgent(agentID string) {
	// No-op in single-user mode
}

// Stub for UPnP manager - TODO: Move to proper package
type upnpManagerStub struct{}

func (u *upnpManagerStub) MapPort(internal, external int, protocol, description string) error {
	return fmt.Errorf("UPnP not implemented")
}

func (u *upnpManagerStub) UnmapPort(port int) {
	// No-op
}

var upnpManager = &upnpManagerStub{}

