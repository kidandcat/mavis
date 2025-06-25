package web

import (
	"fmt"
	"strings"
	"time"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type AgentStatus struct {
	ID           string
	Task         string
	Status       string
	StartTime    time.Time
	LastActive   time.Time
	MessagesSent int
	QueueStatus  string
	IsStale      bool
	Progress     string
	Output       string
	Duration     time.Duration
}

// safeSubstring safely extracts a substring, handling cases where the string is shorter than requested
func safeSubstring(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// getStatusClass returns the CSS class for an agent based on its status
func getStatusClass(agent AgentStatus) string {
	if agent.Status == "finished" {
		return "finished"
	} else if agent.Status == "stopped" || agent.Status == "error" || agent.Status == "failed" || agent.Status == "killed" {
		return "failed"
	} else if agent.Status == "queued" {
		return "queued"
	} else if agent.Status == "running" || agent.Status == "active" {
		return "running"
	} else if agent.IsStale {
		return "stale"
	}
	return "running"
}

// categorizeAgents sorts agents into their respective columns
func categorizeAgents(agents []AgentStatus) (planning, queued, running, finished []AgentStatus) {
	for _, agent := range agents {
		switch agent.Status {
		case "queued":
			queued = append(queued, agent)
		case "active", "running":
			// Check if agent has progress to determine if it's planning or running
			trimmedProgress := strings.TrimSpace(agent.Progress)
			if trimmedProgress == "" || strings.Contains(trimmedProgress, "(The AI will update progress here as it works)") {
				planning = append(planning, agent)
			} else {
				running = append(running, agent)
			}
		case "finished", "failed", "killed", "stopped", "error":
			finished = append(finished, agent)
		default:
			running = append(running, agent)
		}
	}
	return
}

// formatDuration converts a time.Duration to a readable format
func formatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60

	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

// getRunningTime calculates the running time for active agents
func getRunningTime(agent AgentStatus) string {
	if agent.Status != "active" && agent.Status != "running" {
		return ""
	}

	elapsed := time.Since(agent.StartTime)
	return " for " + formatDuration(elapsed)
}

func AgentsSection(agents []AgentStatus, modalParam string) g.Node {
	// Categorize agents
	planning, queued, running, finished := categorizeAgents(agents)

	// Check if we should show the create modal based on query params
	showModal := modalParam == "create"

	return h.Div(h.ID("agents-section"), h.Class("section"),
		h.Div(h.Class("section-header"),
			h.H2(g.Text("Agents")),
			h.Form(h.Method("get"), h.Action("/agents"), h.Style("display: inline;"),
				h.Input(h.Type("hidden"), h.Name("modal"), h.Value("create")),
				h.Button(
					h.Type("submit"),
					h.Class("btn btn-primary ms-3"),
					g.Text("+ New Agent"),
				),
			),
		),
		h.Div(h.Class("kanban-container"),
			// Queue Column
			h.Div(h.Class("kanban-column"),
				h.Div(h.Class("kanban-header"),
					h.H3(g.Text("Queue")),
					h.Span(h.Class("kanban-count"), g.Text(fmt.Sprintf("(%d)", len(queued)))),
				),
				h.Div(h.ID("queue-column"), h.Class("kanban-cards"),
					g.Group(g.Map(queued, func(agent AgentStatus) g.Node {
						return AgentCard(agent)
					})),
				),
			),
			// Planning Column
			h.Div(h.Class("kanban-column"),
				h.Div(h.Class("kanban-header"),
					h.H3(g.Text("Planning")),
					h.Span(h.Class("kanban-count"), g.Text(fmt.Sprintf("(%d)", len(planning)))),
				),
				h.Div(h.ID("planning-column"), h.Class("kanban-cards"),
					g.Group(g.Map(planning, func(agent AgentStatus) g.Node {
						return AgentCard(agent)
					})),
				),
			),
			// Coding Column
			h.Div(h.Class("kanban-column"),
				h.Div(h.Class("kanban-header"),
					h.H3(g.Text("Coding")),
					h.Span(h.Class("kanban-count"), g.Text(fmt.Sprintf("(%d)", len(running)))),
				),
				h.Div(h.ID("running-column"), h.Class("kanban-cards"),
					g.Group(g.Map(running, func(agent AgentStatus) g.Node {
						return AgentCard(agent)
					})),
				),
			),
			// Finished Column
			h.Div(h.Class("kanban-column"),
				h.Div(h.Class("kanban-header"),
					h.H3(g.Text("Finished")),
					h.Span(h.Class("kanban-count"), g.Text(fmt.Sprintf("(%d)", len(finished)))),
				),
				h.Div(h.ID("finished-column"), h.Class("kanban-cards"),
					g.Group(g.Map(finished, func(agent AgentStatus) g.Node {
						return AgentCard(agent)
					})),
				),
			),
		),
		// Render modal if query param is set
		g.If(showModal,
			CreateAgentModal(),
		),
	)
}

func AgentCard(agent AgentStatus) g.Node {
	statusClass := getStatusClass(agent)
	agentId := agent.ID

	// For finished agents, show output and duration
	if agent.Status == "finished" {
		duration := formatDuration(agent.Duration)
		output := agent.Output
		if output == "" {
			output = "No output available"
		}

		return h.Div(
			h.ID(fmt.Sprintf("agent-%s", agentId)),
			h.Class("agent-card "+statusClass),

			h.Div(h.Class("agent-header"),
				h.H3(g.Text(fmt.Sprintf("Agent %s", safeSubstring(agentId, 8)))),
				h.Span(h.Class("agent-status"), g.Text(agent.Status)),
			),
			h.Div(h.Class("agent-output"),
				h.Pre(g.Text(output)),
			),
			h.Div(h.Class("agent-time"),
				h.Span(h.Class("time-label"), g.Text("Time taken:")),
				h.Span(h.Class("time-value"), g.Text(duration)),
			),
			h.Div(h.Class("agent-actions"),
				h.Form(h.Method("post"), h.Action(fmt.Sprintf("/api/agent/%s/delete", agentId)), h.Style("display: inline;"),
					h.Button(h.Type("submit"), h.Class("btn btn-sm btn-secondary"), g.Text("Delete")),
				),
			),
		)
	}

	// For other statuses
	return h.Div(
		h.ID(fmt.Sprintf("agent-%s", agentId)),
		h.Class("agent-card "+statusClass),

		h.Div(h.Class("agent-header"),
			h.H3(g.Text(fmt.Sprintf("Agent %s", safeSubstring(agentId, 8)))),
			h.Span(h.Class("agent-status"), g.Text(agent.Status+getRunningTime(agent))),
		),
		h.Div(h.Class("agent-task"),
			h.P(g.Text(agent.Task)),
		),
		h.Div(h.Class("agent-stats")),

		// Show progress if available
		g.If(agent.Progress != "" && (agent.Status == "running" || agent.Status == "active"),
			h.Div(h.Class("agent-progress"),
				h.Div(h.Class("progress-header"), g.Text("Progress:")),
				h.Div(h.Class("progress-content"),
					h.Pre(g.Text(agent.Progress)),
				),
			),
		),

		// Show planning indicator for agents without progress
		g.If(agent.Progress == "" && (agent.Status == "running" || agent.Status == "active"),
			h.Div(h.Class("agent-planning"),
				h.Div(h.Class("planning-header"), g.Text("Planning:")),
				h.Div(h.Class("planning-content"), g.Text("Agent is analyzing the task and creating a plan...")),
			),
		),

		h.Div(h.Class("agent-actions"),
			g.If(agent.Status == "active" || agent.Status == "running",
				h.Form(h.Method("post"), h.Action(fmt.Sprintf("/api/agent/%s/stop", agentId)), h.Style("display: inline;"),
					h.Button(h.Type("submit"), h.Class("btn btn-sm btn-danger"), g.Text("Stop")),
				),
			),
			g.If(agent.Status == "finished" || agent.Status == "failed" || agent.Status == "killed" || agent.Status == "stopped",
				h.Form(h.Method("post"), h.Action(fmt.Sprintf("/api/agent/%s/delete", agentId)), h.Style("display: inline;"),
					h.Button(h.Type("submit"), h.Class("btn btn-sm btn-secondary"), g.Text("Delete")),
				),
			),
		),
	)
}

func CreateAgentModal() g.Node {
	return h.Div(h.ID("create-agent-modal"), h.Class("modal"), h.Style("display: flex;"),
		h.Div(h.Class("modal-content"),
			h.Div(h.Class("modal-header"),
				h.H3(g.Text("Create New Agent")),
				h.A(h.Href("/agents"), h.Class("close-btn"), g.Text("×")),
			),
			h.Form(
				h.Method("post"),
				h.Action("/api/code"),
				g.Attr("enctype", "multipart/form-data"),

				h.Div(h.Class("form-group"),
					h.Label(h.For("work_dir"), g.Text("Working Directory (optional)")),
					h.Input(
						h.Type("text"),
						h.ID("work_dir"),
						h.Name("work_dir"),
						h.Placeholder("Leave empty for current dir or use . or /absolute/path"),
						h.AutoFocus(),
					),
				),

				h.Div(h.Class("form-group"),
					h.Label(h.For("task"), g.Text("Task Description")),
					h.Textarea(
						h.ID("task"),
						h.Name("task"),
						h.Rows("4"),
						h.Required(),
						h.Placeholder("Enter the task for the agent..."),
					),
				),

				h.Div(h.Class("form-actions"),
					h.Button(h.Type("submit"), h.Class("btn btn-primary"), g.Text("Create Agent")),
					h.A(h.Href("/agents"), h.Class("btn btn-secondary"), g.Text("Cancel")),
				),
			),
		),
	)
}
