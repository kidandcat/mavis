// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package web

import (
	"fmt"
	"strings"
	"time"

	"mavis/codeagent"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Type alias for convenience
type InteractiveAgent = codeagent.InteractiveAgent

// formatTimeAgo converts a time to a human-readable "X ago" format
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)
	
	switch {
	case duration < time.Minute:
		return "just now"
	case duration < time.Hour:
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	case duration < 24*time.Hour:
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

// InteractiveSection renders the interactive agents page
func InteractiveSection(modalParam, dirParam string) g.Node {
	// Get interactive agents
	agents := interactiveManager.ListAgents()
	
	// Check if we're viewing a specific session
	sessionID := ""
	inputModal := false
	if strings.HasPrefix(modalParam, "session-") {
		sessionID = strings.TrimPrefix(modalParam, "session-")
		if strings.HasSuffix(sessionID, "-input") {
			sessionID = strings.TrimSuffix(sessionID, "-input")
			inputModal = true
		}
	}
	
	// Auto-refresh for session view when not in input modal
	shouldRefresh := sessionID != "" && !inputModal
	
	return h.Div(h.Class("container"),
		// Add refresh meta tag if viewing a session (but not when input modal is open)
		g.If(shouldRefresh,
			h.Meta(g.Attr("http-equiv", "refresh"), h.Content("2")),
		),
		
		// Header with create button
		h.Div(h.Class("header-actions"),
			h.H1(g.Text("Interactive Sessions")),
			h.A(
				h.Href("/interactive?modal=create"),
				h.Class("btn btn-primary"),
				g.Text("New Session"),
			),
		),
		
		// Description
		h.P(h.Class("subtitle"), g.Text("Chat with Claude in real-time interactive sessions")),
		
		// Sessions grid
		h.Div(h.Class("interactive-grid"),
			g.If(len(agents) == 0,
				h.Div(h.Class("empty-state"),
					h.P(g.Text("No interactive sessions active")),
					h.P(h.Class("help-text"), g.Text("Click 'New Session' to start chatting with Claude")),
				),
			),
			g.Group(g.Map(agents, func(agent *InteractiveAgent) g.Node {
				return renderInteractiveAgentCard(agent)
			})),
		),
		
		// Modals
		g.If(modalParam == "create",
			InteractiveCreateModal(dirParam),
		),
		g.If(sessionID != "" && !inputModal,
			InteractiveSessionView(sessionID),
		),
		g.If(inputModal,
			InteractiveInputModal(sessionID),
		),
	)
}

func renderInteractiveAgentCard(agent *InteractiveAgent) g.Node {
	statusClass := "running"
	statusText := "Running"
	
	switch agent.Status {
	case "failed":
		statusClass = "failed"
		statusText = "Failed"
	case "killed":
		statusClass = "killed"
		statusText = "Stopped"
	case "finished":
		statusClass = "finished"
		statusText = "Finished"
	}
	
	return h.Div(h.Class("interactive-card " + statusClass),
		// Header
		h.Div(h.Class("card-header"),
			h.H3(g.Text(fmt.Sprintf("Session %s", agent.ID[:8]))),
			h.Span(h.Class("status-badge"), g.Text(statusText)),
		),
		
		// Info
		h.Div(h.Class("card-body"),
			h.P(h.Class("folder-path"), g.Text(agent.Folder)),
			h.Div(h.Class("time-info"),
				h.Span(g.Text(fmt.Sprintf("Started: %s", agent.StartTime.Format("3:04 PM")))),
				h.Br(),
				h.Span(g.Text(fmt.Sprintf("Last active: %s", formatTimeAgo(agent.LastActive)))),
				g.If(agent.Status == "running",
					g.Group([]g.Node{
						h.Br(),
						h.Span(g.Text(fmt.Sprintf("Duration: %s", formatDuration(time.Since(agent.StartTime))))),
					}),
				),
			),
		),
		
		// Error if any
		g.If(agent.Error != "",
			h.Div(h.Class("error-message"),
				h.Pre(g.Text(agent.Error)),
			),
		),
		
		// Actions
		h.Div(h.Class("card-actions"),
			g.If(agent.Status == "running",
				g.Group([]g.Node{
					h.A(
						h.Href(fmt.Sprintf("/interactive?modal=session-%s", agent.ID)),
						h.Class("btn btn-primary btn-sm"),
						g.Text("Open"),
					),
					h.Form(
						h.Method("POST"),
						h.Action(fmt.Sprintf("/api/interactive/%s/stop", agent.ID)),
						h.Style("display: inline-block; margin-left: 0.5rem;"),
						h.Button(
							h.Type("submit"),
							h.Class("btn btn-sm"),
							g.Text("Stop"),
						),
					),
				}),
			),
			g.If(agent.Status != "running",
				h.Form(
					h.Method("POST"),
					h.Action(fmt.Sprintf("/api/interactive/%s/delete", agent.ID)),
					h.Input(h.Type("hidden"), h.Name("_method"), h.Value("DELETE")),
					h.Button(
						h.Type("submit"),
						h.Class("btn btn-danger btn-sm"),
						g.Text("Delete"),
					),
				),
			),
		),
	)
}

// InteractiveCreateModal renders the create interactive agent modal
func InteractiveCreateModal(dirParam string) g.Node {
	return h.Div(h.ID("create-interactive-modal"), h.Class("modal"), h.Style("display: flex;"),
		h.Div(h.Class("modal-content"),
			h.Div(h.Class("modal-header"),
				h.H3(g.Text("New Interactive Session")),
				h.A(h.Href("/interactive"), h.Class("close-btn"), g.Text("×")),
			),
			
			h.Form(
				h.Method("POST"),
				h.Action("/api/interactive"),
				
				// Working directory
				h.Div(h.Class("form-group"),
					h.Label(h.For("work_dir"), g.Text("Working Directory")),
					h.Input(
						h.Type("text"),
						h.ID("work_dir"),
						h.Name("work_dir"),
						h.Value(dirParam),
						h.Placeholder("/path/to/project"),
						g.Attr("required", ""),
						h.AutoFocus(),
					),
					h.Small(h.Class("help-text"), g.Text("The directory where Claude will work")),
				),
				
				// MCP selection
				h.Div(h.Class("form-group"),
					h.Label(g.Text("Model Context Protocol Servers (optional)")),
					h.Div(h.Class("mcp-checkboxes"),
						g.Group(g.Map(mcpStore.List(), func(mcp *MCP) g.Node {
							return h.Div(h.Class("checkbox-wrapper"),
								h.Input(
									h.Type("checkbox"),
									h.ID(fmt.Sprintf("mcp-%s", mcp.ID)),
									h.Name("selected_mcps"),
									h.Value(mcp.Name),
								),
								h.Label(
									h.For(fmt.Sprintf("mcp-%s", mcp.ID)),
									g.Text(" " + mcp.Name),
								),
							)
						})),
						g.If(len(mcpStore.List()) == 0,
							h.P(h.Class("help-text"), g.Text("No MCPs configured")),
						),
					),
				),
				
				// Info
				h.Div(h.Class("info-box"),
					h.P(g.Text("This will start an interactive Claude session in the selected directory.")),
					h.P(g.Text("You'll be able to chat with Claude and see the output in real-time.")),
				),
				
				// Actions
				h.Div(h.Class("form-actions"),
					h.Button(
						h.Type("submit"),
						h.Class("btn btn-primary"),
						g.Text("Start Session"),
					),
					h.A(h.Href("/interactive"), h.Class("btn btn-secondary"), g.Text("Cancel")),
				),
			),
		),
	)
}

// InteractiveSessionView renders the interactive session interface
func InteractiveSessionView(sessionID string) g.Node {
	agent := interactiveManager.GetAgent(sessionID)
	if agent == nil {
		return h.Div(h.Class("error-state"),
			h.P(g.Text("Session not found")),
			h.A(h.Href("/interactive"), g.Text("Back to sessions")),
		)
	}
	
	// Get output - this is already the terminal screen buffer
	output := agent.GetOutput()
	
	// Build the terminal display with proper line structure
	var outputHTML string
	if len(output) > 0 {
		// The output is already a fixed-size screen buffer
		// Join all lines (including empty ones) to maintain screen structure
		outputHTML = strings.Join(output, "\n")
	} else {
		// Handle different states with plain text
		var statusMsg string
		switch agent.Status {
		case "failed":
			if agent.Error != "" {
				statusMsg = fmt.Sprintf("Error: %s", agent.Error)
			} else {
				statusMsg = "Process failed to start. Check logs for details."
			}
		case "finished":
			statusMsg = "Process completed with no output."
		case "killed":
			statusMsg = "Process was stopped."
		default: // running or pending
			statusMsg = "Waiting for Claude to start..."
		}
		outputHTML = statusMsg
	}
	
	return h.Div(h.ID("session-modal"), h.Class("modal"), h.Style("display: flex;"),
		h.Div(h.Class("modal-content modal-large"),
			h.Div(h.Class("modal-header"),
				h.H3(g.Text(fmt.Sprintf("Session %s - %s", sessionID[:8], agent.Status))),
				h.A(h.Href("/interactive"), h.Class("close-btn"), g.Text("×")),
			),
			
			
			// Folder info
			h.P(h.Class("folder-info"), g.Text(fmt.Sprintf("Working in: %s", agent.Folder))),
			
			// Show error prominently if failed
			g.If(agent.Status == "failed" && agent.Error != "",
				h.Div(h.Class("error-box"),
					h.Strong(g.Text("Error: ")),
					h.Pre(g.Text(agent.Error)),
				),
			),
			
			// Output area - terminal style
			h.Div(h.Class("session-output"),
				h.Pre(h.Style("margin: 0; padding: 0; background: transparent; border: none;"),
					g.Raw(outputHTML),
				),
			),
			
			// Actions (only if running)
			g.If(agent.Status == "running",
				h.Div(h.Class("session-actions"),
					h.A(
						h.Href(fmt.Sprintf("/interactive?modal=session-%s-input", sessionID)),
						h.Class("btn btn-primary"),
						g.Text("Send Message"),
					),
					h.Form(
						h.Method("POST"),
						h.Action(fmt.Sprintf("/api/interactive/%s/stop", sessionID)),
						h.Style("display: inline-block; margin-left: 0.5rem;"),
						h.Button(
							h.Type("submit"),
							h.Class("btn btn-secondary"),
							g.Text("Stop Session"),
						),
					),
				),
			),
		),
	)
}

// InteractiveInputModal renders the input form for sending messages
func InteractiveInputModal(sessionID string) g.Node {
	agent := interactiveManager.GetAgent(sessionID)
	if agent == nil {
		return h.Div(h.Class("error-state"),
			h.P(g.Text("Session not found")),
			h.A(h.Href("/interactive"), g.Text("Back to sessions")),
		)
	}
	
	return h.Div(h.ID("input-modal"), h.Class("modal"), h.Style("display: flex;"),
		h.Div(h.Class("modal-content"),
			h.Div(h.Class("modal-header"),
				h.H3(g.Text("Send Message")),
				h.A(h.Href(fmt.Sprintf("/interactive?modal=session-%s", sessionID)), h.Class("close-btn"), g.Text("×")),
			),
			
			h.Form(
				h.Method("POST"),
				h.Action(fmt.Sprintf("/api/interactive/%s/input", sessionID)),
				
				h.Div(h.Class("form-group"),
					h.Label(h.For("input"), g.Text("Your message to Claude:")),
					h.Textarea(
						h.ID("input"),
						h.Name("input"),
						h.Class("form-control"),
						h.Rows("4"),
						h.Placeholder("Type your message..."),
						g.Attr("autofocus", ""),
						g.Attr("required", ""),
					),
				),
				
				h.Div(h.Class("form-actions"),
					h.Button(
						h.Type("submit"),
						h.Class("btn btn-primary"),
						g.Text("Send"),
					),
					h.A(
						h.Href(fmt.Sprintf("/interactive?modal=session-%s", sessionID)),
						h.Class("btn btn-secondary"),
						g.Text("Cancel"),
					),
				),
			),
		),
	)
}