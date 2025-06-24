// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package telegram

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"mavis/core"

	"github.com/go-telegram/bot/models"
)

func handleStartCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 4 {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Please provide workdir, port, and build command.\nUsage: /start <workdir> <port> <build command...>\n\nExample: /start ~/reservas_rb 3000 rails s")
		return
	}

	workdir := strings.TrimSpace(parts[1])
	port := strings.TrimSpace(parts[2])
	buildCmdStr := strings.Join(parts[3:], " ")

	// Validate port
	if _, err := strconv.Atoi(port); err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Invalid port number: %s", port))
		return
	}

	// Resolve the workdir path
	absWorkdir, err := core.ResolvePath(workdir)
	if err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error resolving workdir path: %v", err))
		return
	}

	// Check if workdir exists
	info, err := os.Stat(absWorkdir)
	if err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Directory not found: %s", absWorkdir))
		return
	}
	if !info.IsDir() {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Path is not a directory: %s", absWorkdir))
		return
	}

	lanServerMutex.Lock()
	defer lanServerMutex.Unlock()

	// Check if server is already running
	if lanServerProcess != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ LAN server is already running!\n📁 Workdir: %s\n🔌 Port: %s\n🛠️ Command: %s\n\nUse /stop to stop it first.", lanServerWorkDir, lanServerPort, lanServerCmd))
		return
	}

	// Check if port is in use and find an available one if needed
	if core.IsPortInUse(port) {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("⚠️ Port %s is already in use. Finding an available port...", port))

		availablePort, err := core.FindAvailablePort(port)
		if err != nil {
			core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Could not find an available port: %v", err))
			return
		}
		port = availablePort
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ Using port %s instead", port))
	}

	core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🚀 Starting LAN server...\n📁 Workdir: %s\n🔌 Port: %s\n🛠️ Build command: %s", absWorkdir, port, buildCmdStr))

	// Start the build command in the workdir
	buildCmd := exec.Command("sh", "-c", buildCmdStr)
	buildCmd.Dir = absWorkdir

	// Set environment variables including the PORT
	buildCmd.Env = append(os.Environ(), fmt.Sprintf("PORT=%s", port))

	// Capture output for error reporting
	buildOutput := &strings.Builder{}
	buildCmd.Stdout = io.MultiWriter(os.Stdout, buildOutput)
	buildCmd.Stderr = io.MultiWriter(os.Stderr, buildOutput)

	if err := buildCmd.Start(); err != nil {
		// Try to get more detailed error output
		output := buildOutput.String()
		if output != "" {
			core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to start build command: %v\n\n📋 *Output:*\n```\n%s\n```", err, output))
		} else {
			core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to start build command: %v", err))
		}
		return
	}

	// Give the build command a moment to start and check if it's still running
	time.Sleep(2 * time.Second)

	// Check if build process already exited (failed to start properly)
	if buildCmd.ProcessState != nil {
		output := buildOutput.String()
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Build command failed to start properly.\n\n📋 *Output:*\n```\n%s\n```", output))
		return
	}

	// Store the process info
	lanServerProcess = buildCmd.Process
	lanServerPort = port
	lanServerWorkDir = absWorkdir
	lanServerCmd = buildCmdStr

	// Get local IP addresses
	var ipAddresses []string
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ipAddresses = append(ipAddresses, ipnet.IP.String())
				}
			}
		}
	}

	// Try to set up UPnP port mapping
	portInt, _ := strconv.Atoi(port)

	// Attempt UPnP mapping in a goroutine to not block startup
	go func() {
		core.SendMessage(ctx, b, message.Chat.ID, "🔌 Attempting UPnP port mapping...")

		err := upnpManager.MapPort(portInt, portInt, "TCP", fmt.Sprintf("Mavis Server - %s", buildCmdStr))
		if err != nil {
			log.Printf("UPnP mapping failed: %v", err)
			core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("⚠️ UPnP port mapping failed: %v\n\nServer is still accessible on LAN.", err))
		} else {
			// Get public IP
			publicIP, err := core.GetPublicIP(ctx)
			if err != nil {
				log.Printf("Failed to get public IP: %v", err)
				core.SendMessage(ctx, b, message.Chat.ID, "⚠️ UPnP succeeded but couldn't get public IP. Server is accessible on LAN.")
			} else {
				// Send success message with public URL
				publicURL := fmt.Sprintf("http://%s:%s", publicIP, port)
				core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ UPnP mapping successful!\n\n🌍 *Public URL:* %s\n\n⚠️ *Important:* This URL is accessible from the internet!", publicURL))
			}
		}
	}()

	// Build access URLs
	var accessURLs strings.Builder
	accessURLs.WriteString("\n🌐 *Access URLs:*\n")
	accessURLs.WriteString(fmt.Sprintf("  🏠 Local: http://localhost:%s\n", port))
	for _, ip := range ipAddresses {
		accessURLs.WriteString(fmt.Sprintf("  📡 LAN: http://%s:%s\n", ip, port))
	}
	accessURLs.WriteString(fmt.Sprintf("  🎯 mDNS: http://%s:%s (if available)\n", lanDomainName, port))

	// Success message
	successMsg := fmt.Sprintf("✅ LAN server started successfully!\n📁 Workdir: %s\n🔌 Port: %s\n🛠️ Build command: %s\n%s\n💡 *Note:* Attempting to expose to internet via UPnP...", absWorkdir, port, buildCmdStr, accessURLs.String())

	core.SendMessage(ctx, b, message.Chat.ID, successMsg)

	// Monitor the process in a goroutine
	go func() {
		// Wait for process to exit and capture the error
		err := buildCmd.Wait()

		lanServerMutex.Lock()
		if lanServerProcess != nil {
			// Clean up UPnP mapping
			if lanServerPort != "" {
				portInt, _ := strconv.Atoi(lanServerPort)
				upnpManager.UnmapPort(portInt)
			}

			// Clean up
			lanServerProcess = nil
			lanServerPort = ""
			lanServerWorkDir = ""
			lanServerCmd = ""
			lanServerMutex.Unlock()

			// Build error message with reason
			errorMsg := "⚠️ LAN server has stopped"
			if err != nil {
				// Get the output that was captured
				output := buildOutput.String()
				if output != "" {
					errorMsg = fmt.Sprintf("⚠️ LAN server has stopped.\n❌ *Reason:* %v\n\n📋 *Output:*\n```\n%s\n```", err, output)
				} else {
					errorMsg = fmt.Sprintf("⚠️ LAN server has stopped.\n❌ *Reason:* %v", err)
				}
			}

			core.SendMessage(ctx, b, message.Chat.ID, errorMsg)
		} else {
			lanServerMutex.Unlock()
		}
	}()
}

func handleStopLANCommand(ctx context.Context, message *models.Message) {
	lanServerMutex.Lock()
	defer lanServerMutex.Unlock()

	if lanServerProcess == nil && lanHTTPServer == nil {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ No LAN server is currently running.")
		return
	}

	workdir := lanServerWorkDir
	port := lanServerPort
	cmd := lanServerCmd

	// Stop process-based server if running
	if lanServerProcess != nil {
		// Kill the server process
		if err := lanServerProcess.Kill(); err != nil {
			core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to stop LAN server process: %v", err))
		}

		// Also try to kill any process using the port
		if lanServerPort != "" {
			killPortCmd := exec.Command("sh", "-c", fmt.Sprintf("lsof -ti:%s | xargs kill -9 2>/dev/null || true", lanServerPort))
			killPortCmd.Run()
		}
	}

	// Stop Go HTTP server if running
	if lanHTTPServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := lanHTTPServer.Shutdown(shutdownCtx); err != nil {
			core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("⚠️ Warning: HTTP server shutdown error: %v", err))
		}
	}

	// Clean up UPnP mapping
	if lanServerPort != "" {
		portInt, _ := strconv.Atoi(lanServerPort)
		upnpManager.UnmapPort(portInt)
	}

	// Clean up
	lanServerProcess = nil
	lanHTTPServer = nil
	lanServerPort = ""
	lanServerWorkDir = ""
	lanServerCmd = ""

	core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🛑 LAN server stopped.\n📁 Workdir: %s\n🔌 Port: %s\n🛠️ Command: %s", workdir, port, cmd))
}

func handleServeCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Please provide a directory to serve.\nUsage: /serve <directory> [port]\n\nExample: /serve ~/myproject 8080\n\nIf port is not specified, it defaults to 8080.")
		return
	}

	workdir := strings.TrimSpace(parts[1])
	port := "8080" // Default port

	// Check if port was specified
	if len(parts) >= 3 {
		port = strings.TrimSpace(parts[2])
		// Validate port
		if _, err := strconv.Atoi(port); err != nil {
			core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Invalid port number: %s", port))
			return
		}
	}

	// Resolve the workdir path
	absWorkdir, err := core.ResolvePath(workdir)
	if err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if workdir exists
	info, err := os.Stat(absWorkdir)
	if err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Directory not found: %s", absWorkdir))
		return
	}
	if !info.IsDir() {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Path is not a directory: %s", absWorkdir))
		return
	}

	lanServerMutex.Lock()
	defer lanServerMutex.Unlock()

	// Check if server is already running
	if lanServerProcess != nil || lanHTTPServer != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ LAN server is already running!\n📁 Workdir: %s\n🔌 Port: %s\n🛠️ Command: %s\n\nUse /stop to stop it first.", lanServerWorkDir, lanServerPort, lanServerCmd))
		return
	}

	// Check if port is in use and find an available one if needed
	if core.IsPortInUse(port) {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("⚠️ Port %s is already in use. Finding an available port...", port))

		availablePort, err := core.FindAvailablePort(port)
		if err != nil {
			core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Could not find an available port: %v", err))
			return
		}
		port = availablePort
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ Using port %s instead", port))
	}

	core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🚀 Starting LAN file server...\n📁 Directory: %s\n🔌 Port: %s\n🛠️ Server: Go HTTP Server", absWorkdir, port))

	// Start the Go file server
	_, err = StartFileServer(absWorkdir, port)
	if err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to start LAN file server: %v", err))
		return
	}

	// Store the server info
	lanServerPort = port
	lanServerWorkDir = absWorkdir
	lanServerCmd = fmt.Sprintf("Go file server on port %s", port)

	// Get local IP addresses
	var ipAddresses []string
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ipAddresses = append(ipAddresses, ipnet.IP.String())
				}
			}
		}
	}

	// Try to set up UPnP port mapping
	portInt, _ := strconv.Atoi(port)

	// Attempt UPnP mapping in a goroutine to not block startup
	go func() {
		core.SendMessage(ctx, b, message.Chat.ID, "🔌 Attempting UPnP port mapping...")

		err := upnpManager.MapPort(portInt, portInt, "TCP", "Mavis File Server")
		if err != nil {
			log.Printf("UPnP mapping failed: %v", err)
			core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("⚠️ UPnP port mapping failed: %v\n\nServer is still accessible on LAN.", err))
		} else {
			// Get public IP
			publicIP, err := core.GetPublicIP(ctx)
			if err != nil {
				log.Printf("Failed to get public IP: %v", err)
				core.SendMessage(ctx, b, message.Chat.ID, "⚠️ UPnP succeeded but couldn't get public IP. Server is accessible on LAN.")
			} else {
				// Send success message with public URL
				publicURL := fmt.Sprintf("http://%s:%s", publicIP, port)
				core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("✅ UPnP mapping successful!\n\n🌍 *Public URL:* %s\n\n⚠️ *Important:* This URL is accessible from the internet!", publicURL))
			}
		}
	}()

	// Build access URLs
	var accessURLs strings.Builder
	accessURLs.WriteString("\n🌐 *Access URLs:*\n")
	accessURLs.WriteString(fmt.Sprintf("  🏠 Local: http://localhost:%s\n", port))
	for _, ip := range ipAddresses {
		accessURLs.WriteString(fmt.Sprintf("  📡 LAN: http://%s:%s\n", ip, port))
	}
	accessURLs.WriteString(fmt.Sprintf("  🎯 mDNS: http://%s:%s (if available)\n", lanDomainName, port))

	// Success message
	successMsg := fmt.Sprintf("✅ LAN file server started successfully!\n📁 Serving: %s\n🔌 Port: %s\n📄 Server: Go HTTP Server\n%s\n💡 *Note:* Attempting to expose to internet via UPnP...", absWorkdir, port, accessURLs.String())

	core.SendMessage(ctx, b, message.Chat.ID, successMsg)
}
