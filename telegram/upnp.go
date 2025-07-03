// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package telegram

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/huin/goupnp"
	"github.com/huin/goupnp/dcps/internetgateway1"
	"github.com/huin/goupnp/dcps/internetgateway2"
)

// UPnPManager handles UPnP port forwarding
type UPnPManager struct {
	client       upnpClient
	mappedPorts  map[int]portMapping
	mu           sync.Mutex
	externalIP   string
	internalIP   string
}

type portMapping struct {
	internalPort int
	externalPort int
	protocol     string
	description  string
}

// upnpClient interface to abstract the specific IGD client
type upnpClient interface {
	AddPortMapping(newRemoteHost string, newExternalPort uint16, newProtocol string,
		newInternalPort uint16, newInternalClient string, newEnabled bool,
		newPortMappingDescription string, newLeaseDuration uint32) error
	DeletePortMapping(newRemoteHost string, newExternalPort uint16, newProtocol string) error
	GetExternalIPAddress() (string, error)
}

// NewUPnPManager creates a new UPnP manager
func NewUPnPManager() (*UPnPManager, error) {
	manager := &UPnPManager{
		mappedPorts: make(map[int]portMapping),
	}

	// Try to discover UPnP devices
	if err := manager.discoverAndConnect(); err != nil {
		return nil, err
	}

	// Get internal IP
	internalIP, err := getInternalIP()
	if err != nil {
		return nil, fmt.Errorf("failed to get internal IP: %w", err)
	}
	manager.internalIP = internalIP

	// Get external IP
	if manager.client != nil {
		externalIP, err := manager.client.GetExternalIPAddress()
		if err != nil {
			log.Printf("Warning: Failed to get external IP: %v", err)
		} else {
			manager.externalIP = externalIP
		}
	}

	return manager, nil
}

// discoverAndConnect discovers UPnP devices and connects to the first available one
func (m *UPnPManager) discoverAndConnect() error {
	// Try IGD2 first (newer protocol)
	clients2, _, err := internetgateway2.NewWANIPConnection1Clients()
	if err == nil && len(clients2) > 0 {
		m.client = clients2[0]
		log.Println("Connected to UPnP IGD2 device")
		return nil
	}

	// Fall back to IGD1
	clients1, _, err := internetgateway1.NewWANIPConnection1Clients()
	if err == nil && len(clients1) > 0 {
		m.client = clients1[0]
		log.Println("Connected to UPnP IGD1 device")
		return nil
	}

	// Try alternative IGD2 connection type
	clients2Alt, _, err := internetgateway2.NewWANPPPConnection1Clients()
	if err == nil && len(clients2Alt) > 0 {
		m.client = clients2Alt[0]
		log.Println("Connected to UPnP IGD2 PPP device")
		return nil
	}

	// Try alternative IGD1 connection type
	clients1Alt, _, err := internetgateway1.NewWANPPPConnection1Clients()
	if err == nil && len(clients1Alt) > 0 {
		m.client = clients1Alt[0]
		log.Println("Connected to UPnP IGD1 PPP device")
		return nil
	}

	return fmt.Errorf("no UPnP devices found")
}

// MapPort maps an internal port to an external port
func (m *UPnPManager) MapPort(internal, external int, protocol, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil {
		return fmt.Errorf("UPnP client not initialized")
	}

	// Default protocol to TCP if not specified
	if protocol == "" {
		protocol = "TCP"
	}

	// Use internal IP
	internalClient := m.internalIP
	if internalClient == "" {
		return fmt.Errorf("internal IP not available")
	}

	// Add port mapping with 0 lease duration (permanent until removed)
	err := m.client.AddPortMapping(
		"",                    // remoteHost (empty = any)
		uint16(external),      // externalPort
		protocol,              // protocol
		uint16(internal),      // internalPort
		internalClient,        // internalClient
		true,                  // enabled
		description,           // description
		0,                     // leaseDuration (0 = permanent)
	)

	if err != nil {
		return fmt.Errorf("failed to add port mapping: %w", err)
	}

	// Store the mapping
	m.mappedPorts[external] = portMapping{
		internalPort: internal,
		externalPort: external,
		protocol:     protocol,
		description:  description,
	}

	log.Printf("Successfully mapped port %d:%d (%s) - %s", external, internal, protocol, description)
	return nil
}

// UnmapPort removes a port mapping
func (m *UPnPManager) UnmapPort(port int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil {
		return
	}

	mapping, exists := m.mappedPorts[port]
	if !exists {
		return
	}

	err := m.client.DeletePortMapping(
		"",                   // remoteHost
		uint16(port),         // externalPort
		mapping.protocol,     // protocol
	)

	if err != nil {
		log.Printf("Failed to unmap port %d: %v", port, err)
	} else {
		log.Printf("Successfully unmapped port %d", port)
		delete(m.mappedPorts, port)
	}
}

// UnmapAllPorts removes all port mappings
func (m *UPnPManager) UnmapAllPorts() {
	m.mu.Lock()
	ports := make([]int, 0, len(m.mappedPorts))
	for port := range m.mappedPorts {
		ports = append(ports, port)
	}
	m.mu.Unlock()

	for _, port := range ports {
		m.UnmapPort(port)
	}
}

// GetExternalIP returns the external IP address
func (m *UPnPManager) GetExternalIP() string {
	return m.externalIP
}

// GetInternalIP returns the internal IP address
func (m *UPnPManager) GetInternalIP() string {
	return m.internalIP
}

// RefreshMappings refreshes all port mappings (useful for routers that timeout mappings)
func (m *UPnPManager) RefreshMappings() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.client == nil {
		return fmt.Errorf("UPnP client not initialized")
	}

	for _, mapping := range m.mappedPorts {
		err := m.client.AddPortMapping(
			"",
			uint16(mapping.externalPort),
			mapping.protocol,
			uint16(mapping.internalPort),
			m.internalIP,
			true,
			mapping.description,
			0,
		)
		if err != nil {
			log.Printf("Failed to refresh port mapping %d: %v", mapping.externalPort, err)
		}
	}

	return nil
}

// StartRefreshTimer starts a timer to periodically refresh mappings
func (m *UPnPManager) StartRefreshTimer() {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			if err := m.RefreshMappings(); err != nil {
				log.Printf("Failed to refresh UPnP mappings: %v", err)
			}
		}
	}()
}

// getInternalIP gets the internal IP address
func getInternalIP() (string, error) {
	// Try to connect to a public DNS server to determine our local IP
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

// discoverUPnPDevices discovers all available UPnP devices (for debugging)
func discoverUPnPDevices() {
	devices, err := goupnp.DiscoverDevices(internetgateway1.URN_WANIPConnection_1)
	if err != nil {
		log.Printf("Failed to discover IGD1 devices: %v", err)
	} else {
		log.Printf("Found %d IGD1 WANIPConnection devices", len(devices))
	}

	devices, err = goupnp.DiscoverDevices(internetgateway2.URN_WANIPConnection_2)
	if err != nil {
		log.Printf("Failed to discover IGD2 devices: %v", err)
	} else {
		log.Printf("Found %d IGD2 WANIPConnection devices", len(devices))
	}
}