package components

import (
	"fmt"
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
}

// safeSubstring safely extracts a substring, handling cases where the string is shorter than requested
func safeSubstring(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func AgentsSection(agents []AgentStatus) g.Node {
	return h.Div(h.ID("agents-section"), h.Class("section agents-container"),
		h.Div(h.Class("agents-panel"),
			h.Div(h.Class("section-header"),
				h.H2(g.Text("Active Agents")),
				h.Button(
					h.Class("btn btn-primary ms-3"),
					g.Attr("onclick", "showCreateAgentModal()"),
					g.Text("+ New Agent"),
				),
			),
			h.Div(h.ID("agents-grid"), h.Class("agents-grid"),
				g.Group(g.Map(agents, func(agent AgentStatus) g.Node {
					return AgentCard(agent)
				})),
			),
			CreateAgentModal(),
		),
		h.Div(h.ID("agent-status-panel"), h.Class("agent-status-panel"),
			h.Div(h.Class("status-panel-header"),
				h.H3(g.Text("Agent Status")),
				h.Span(h.Class("status-panel-subtitle"), g.Text("Select an agent to view status")),
			),
			h.Div(h.ID("agent-status-content"), h.Class("agent-status-content"),
				h.Div(h.Class("status-placeholder"),
					h.P(g.Text("No agent selected")),
				),
			),
		),
	)
}

func AgentCard(agent AgentStatus) g.Node {
	statusClass := "status-active"
	if agent.Status == "stopped" || agent.Status == "error" {
		statusClass = "status-stopped"
	} else if agent.IsStale {
		statusClass = "status-stale"
	}

	return h.Div(
		h.ID(fmt.Sprintf("agent-%s", agent.ID)),
		h.Class("agent-card "+statusClass),
		g.Attr("data-agent-id", agent.ID),
		g.Attr("onclick", fmt.Sprintf("selectAgent('%s')", agent.ID)),

		h.Div(h.Class("agent-header"),
			h.H3(g.Text(fmt.Sprintf("Agent %s", safeSubstring(agent.ID, 8)))),
			h.Span(h.Class("agent-status"), g.Text(agent.Status)),
		),

		h.Div(h.Class("agent-task"),
			h.P(g.Text(agent.Task)),
		),

		h.Div(h.Class("agent-stats"),
			h.Div(h.Class("stat"),
				h.Span(h.Class("stat-label"), g.Text("Started:")),
				h.Span(h.Class("stat-value"), g.Text(agent.StartTime.Format("15:04:05"))),
			),
			h.Div(h.Class("stat"),
				h.Span(h.Class("stat-label"), g.Text("Messages:")),
				h.Span(h.Class("stat-value stat-messages"), g.Text(fmt.Sprintf("%d", agent.MessagesSent))),
			),
			h.Div(h.Class("stat"),
				h.Span(h.Class("stat-label"), g.Text("Queue:")),
				h.Span(h.Class("stat-value stat-queue"), g.Text(agent.QueueStatus)),
			),
		),

		h.Div(h.Class("agent-actions"),
			g.If(agent.Status == "active",
				h.Button(
					h.Class("btn btn-sm btn-danger"),
					g.Attr("onclick", fmt.Sprintf("event.stopPropagation(); stopAgent('%s')", agent.ID)),
					g.Text("Stop"),
				),
			),
		),
	)
}

func CreateAgentModal() g.Node {
	return h.Div(h.ID("create-agent-modal"), h.Class("modal"), h.Style("display: none;"),
		h.Div(h.Class("modal-content"),
			h.Div(h.Class("modal-header"),
				h.H3(g.Text("Create New Agent")),
				h.Button(h.Class("close-btn"), g.Attr("onclick", "hideCreateAgentModal()"), g.Text("×")),
			),
			h.Form(
				h.ID("create-agent-form"),
				g.Attr("onsubmit", "event.preventDefault(); createAgent();"),

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

				h.Div(h.Class("form-group"),
					h.Label(h.For("work_dir"), g.Text("Working Directory (optional)")),
					h.Input(
						h.Type("text"),
						h.ID("work_dir"),
						h.Name("work_dir"),
						h.Placeholder("Leave empty for current dir or use . or /absolute/path"),
					),
				),

				h.Div(h.Class("form-actions"),
					h.Button(h.Type("submit"), h.Class("btn btn-primary"), g.Text("Create Agent")),
					h.Button(h.Type("button"), h.Class("btn btn-secondary"), g.Attr("onclick", "hideCreateAgentModal()"), g.Text("Cancel")),
				),
			),
		),
	)
}

func AgentStatusModal(agentID string, content string) g.Node {
	return h.Div(h.Class("modal-content"),
		h.Div(h.Class("modal-header"),
			h.H3(g.Text(fmt.Sprintf("Agent %s Status", safeSubstring(agentID, 8)))),
			h.Button(h.Class("close-btn"), g.Attr("onclick", "closeModal()"), g.Text("×")),
		),
		h.Div(h.Class("terminal-output"),
			h.Pre(h.ID("agent-status-content"), g.Text(content)),
		),
	)
}
