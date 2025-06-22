// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/huin/goupnp/dcps/internetgateway1"
	"github.com/huin/goupnp/dcps/internetgateway2"
)

// UPnPManager manages UPnP port mappings
type UPnPManager struct {
	mu       sync.Mutex
	mappings map[string]*UPnPMapping // port -> mapping
}

// UPnPMapping represents a single port mapping
type UPnPMapping struct {
	InternalPort int
	ExternalPort int
	Protocol     string
	Description  string
	Client       interface{} // Can be WANIPConnection1 or WANIPConnection2
}

var upnpManager = &UPnPManager{
	mappings: make(map[string]*UPnPMapping),
}

// GetPublicIP fetches the public IP address using multiple services
func GetPublicIP(ctx context.Context) (string, error) {
	services := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://ipinfo.io/ip",
		"https://api.myip.com",
		"https://checkip.amazonaws.com",
	}

	for _, service := range services {
		ip, err := fetchIPFromService(ctx, service)
		if err == nil && ip != "" {
			return ip, nil
		}
		log.Printf("Failed to get IP from %s: %v", service, err)
	}

	return "", fmt.Errorf("failed to get public IP from all services")
}

// fetchIPFromService fetches IP from a single service
func fetchIPFromService(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	ip := strings.TrimSpace(string(body))
	// Basic validation
	if !strings.Contains(ip, ".") || len(ip) > 45 { // Max IPv6 length
		return "", fmt.Errorf("invalid IP format: %s", ip)
	}

	return ip, nil
}

// MapPort creates a UPnP port mapping
func (m *UPnPManager) MapPort(internalPort, externalPort int, protocol, description string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Try IGDv2 first
	clients2, _, err := internetgateway2.NewWANIPConnection2Clients()
	if err == nil && len(clients2) > 0 {
		client := clients2[0]
		err = client.AddPortMapping("", uint16(externalPort), protocol, uint16(internalPort), getLocalIP(), true, description, 0)
		if err == nil {
			m.mappings[fmt.Sprintf("%d", internalPort)] = &UPnPMapping{
				InternalPort: internalPort,
				ExternalPort: externalPort,
				Protocol:     protocol,
				Description:  description,
				Client:       client,
			}
			log.Printf("UPnP: Successfully mapped port %d -> %d using IGDv2", internalPort, externalPort)
			return nil
		}
		log.Printf("UPnP IGDv2 AddPortMapping failed: %v", err)
	}

	// Try IGDv1
	clients1, _, err := internetgateway1.NewWANIPConnection1Clients()
	if err == nil && len(clients1) > 0 {
		client := clients1[0]
		err = client.AddPortMapping("", uint16(externalPort), protocol, uint16(internalPort), getLocalIP(), true, description, 0)
		if err == nil {
			m.mappings[fmt.Sprintf("%d", internalPort)] = &UPnPMapping{
				InternalPort: internalPort,
				ExternalPort: externalPort,
				Protocol:     protocol,
				Description:  description,
				Client:       client,
			}
			log.Printf("UPnP: Successfully mapped port %d -> %d using IGDv1", internalPort, externalPort)
			return nil
		}
		log.Printf("UPnP IGDv1 AddPortMapping failed: %v", err)
	}

	return fmt.Errorf("UPnP port mapping failed: no compatible UPnP device found")
}

// UnmapPort removes a UPnP port mapping
func (m *UPnPManager) UnmapPort(internalPort int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mapping, exists := m.mappings[fmt.Sprintf("%d", internalPort)]
	if !exists {
		return nil // Already unmapped
	}

	var err error
	switch client := mapping.Client.(type) {
	case *internetgateway2.WANIPConnection2:
		err = client.DeletePortMapping("", uint16(mapping.ExternalPort), mapping.Protocol)
	case *internetgateway1.WANIPConnection1:
		err = client.DeletePortMapping("", uint16(mapping.ExternalPort), mapping.Protocol)
	default:
		err = fmt.Errorf("unknown client type")
	}

	if err != nil {
		log.Printf("UPnP: Failed to unmap port %d: %v", internalPort, err)
		// Still remove from our records even if unmapping failed
	} else {
		log.Printf("UPnP: Successfully unmapped port %d", internalPort)
	}

	delete(m.mappings, fmt.Sprintf("%d", internalPort))
	return err
}

// UnmapAllPorts removes all UPnP port mappings
func (m *UPnPManager) UnmapAllPorts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for portStr, mapping := range m.mappings {
		var err error
		switch client := mapping.Client.(type) {
		case *internetgateway2.WANIPConnection2:
			err = client.DeletePortMapping("", uint16(mapping.ExternalPort), mapping.Protocol)
		case *internetgateway1.WANIPConnection1:
			err = client.DeletePortMapping("", uint16(mapping.ExternalPort), mapping.Protocol)
		}

		if err != nil {
			log.Printf("UPnP: Failed to unmap port %s: %v", portStr, err)
		} else {
			log.Printf("UPnP: Successfully unmapped port %s", portStr)
		}
	}

	// Clear all mappings
	m.mappings = make(map[string]*UPnPMapping)
}

// getLocalIP returns the local IP address (simplified version)
func getLocalIP() string {
	// This is a simplified version. In production, you'd want to
	// determine the correct local IP that routes to the gateway
	return ""
}
