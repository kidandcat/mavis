package web

import (
	"fmt"
	"net/http"
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// MCPsSection renders the MCPs management section
func MCPsSection(r *http.Request) g.Node {
	mcps := mcpStore.List()

	// Get query parameters for modal state
	modalAction := r.URL.Query().Get("modal")
	mcpID := r.URL.Query().Get("id")

	// Find MCP for editing if ID is provided
	var editingMCP *MCP
	if modalAction == "edit" && mcpID != "" {
		for _, mcp := range mcps {
			if mcp.ID == mcpID {
				editingMCP = mcp
				break
			}
		}
	}

	return h.Div(
		h.Class("section"),
		h.H2(h.Class("section-title"), g.Text("Model Context Protocol Servers")),

		// Add MCP button
		h.Div(
			h.Class("mb-4"),
			h.A(
				h.Href("/mcps?modal=add"),
				h.Class("btn btn-primary"),
				g.Text("Add MCP Server"),
			),
		),

		// MCPs table
		h.Div(
			h.Class("table-container"),
			h.Table(
				h.Class("data-table"),
				// Header row
				h.Tr(
					h.Th(g.Text("Name")),
					h.Th(g.Text("Command")),
					h.Th(g.Text("Args")),
					h.Th(g.Text("Environment")),
					h.Th(g.Text("Actions")),
				),
				// Data rows
				g.Group(g.Map(mcps, func(mcp *MCP) g.Node {
					return h.Tr(
						h.Td(g.Text(mcp.Name)),
						h.Td(h.Code(g.Text(mcp.Command))),
						h.Td(
							g.If(len(mcp.Args) > 0,
								h.Code(g.Text(fmt.Sprintf("%v", mcp.Args))),
							),
							g.If(len(mcp.Args) == 0,
								g.Text("-"),
							),
						),
						h.Td(
							g.If(len(mcp.Env) > 0,
								h.Code(g.Text(fmt.Sprintf("%v", mcp.Env))),
							),
							g.If(len(mcp.Env) == 0,
								g.Text("-"),
							),
						),
						h.Td(
							h.A(
								h.Href(fmt.Sprintf("/mcps?modal=edit&id=%s", mcp.ID)),
								h.Class("btn btn-sm btn-secondary"),
								g.Text("Edit"),
							),
							g.Text(" "),
							h.A(
								h.Href(fmt.Sprintf("/mcps?modal=delete&id=%s", mcp.ID)),
								h.Class("btn btn-sm btn-danger"),
								g.Text("Delete"),
							),
						),
					)
				})),
			),
		),

		// Show modal if requested
		g.If(modalAction != "",
			MCPModal(modalAction, editingMCP),
		),
	)
}

// MCPModal renders the modal for adding/editing/deleting MCPs
func MCPModal(action string, mcp *MCP) g.Node {
	var modalTitle string
	var formAction string

	switch action {
	case "add":
		modalTitle = "Add MCP Server"
		formAction = "/api/mcps"
	case "edit":
		modalTitle = "Edit MCP Server"
		formAction = fmt.Sprintf("/api/mcps?id=%s", mcp.ID)
	case "delete":
		if mcp == nil {
			return nil
		}
		// Render delete confirmation
		return h.Div(
			h.Class("modal"),
			h.Style("display: flex;"),
			h.Div(
				h.Class("modal-content"),
				h.Div(
					h.Class("modal-header"),
					h.H3(g.Text("Confirm Delete")),
					h.A(h.Href("/mcps"), h.Class("close-btn"), g.Text("×")),
				),
				h.Div(
					h.Class("modal-body"),
					h.P(g.Text(fmt.Sprintf("Are you sure you want to delete MCP \"%s\"?", mcp.Name))),
				),
				h.Div(
					h.Class("modal-footer"),
					h.A(h.Href("/mcps"), h.Class("btn btn-secondary"), g.Text("Cancel")),
					h.Form(
						h.Method("post"),
						h.Action(fmt.Sprintf("/api/mcps?id=%s", mcp.ID)),
						h.Style("display: inline;"),
						h.Input(h.Type("hidden"), h.Name("_method"), h.Value("DELETE")),
						h.Button(
							h.Type("submit"),
							h.Class("btn btn-danger"),
							g.Text("Delete"),
						),
					),
				),
			),
		)
	default:
		return nil
	}

	// Parse existing values for editing
	var argsStr, envStr string
	var nameValue, commandValue string
	if mcp != nil {
		nameValue = mcp.Name
		commandValue = mcp.Command
		argsStr = strings.Join(mcp.Args, ", ")
		envPairs := []string{}
		for k, v := range mcp.Env {
			envPairs = append(envPairs, fmt.Sprintf("%s=%s", k, v))
		}
		envStr = strings.Join(envPairs, ", ")
	}

	return h.Div(
		h.Class("modal"),
		h.Style("display: flex;"),
		h.Div(
			h.Class("modal-content"),
			h.Div(
				h.Class("modal-header"),
				h.H3(g.Text(modalTitle)),
				h.A(h.Href("/mcps"), h.Class("close-btn"), g.Text("×")),
			),
			h.Div(
				h.Class("modal-body"),
				h.Form(
					h.Method("post"),
					h.Action(formAction),
					g.If(action == "edit",
						h.Input(h.Type("hidden"), h.Name("_method"), h.Value("PUT")),
					),
					h.Div(
						h.Class("form-group"),
						h.Label(h.For("mcp-name"), g.Text("Name")),
						h.Input(
							h.Type("text"),
							h.ID("mcp-name"),
							h.Name("name"),
							h.Class("form-control"),
							h.Required(),
							h.Placeholder("e.g., filesystem-server"),
							h.Value(nameValue),
						),
					),
					h.Div(
						h.Class("form-group"),
						h.Label(h.For("mcp-command"), g.Text("Command")),
						h.Input(
							h.Type("text"),
							h.ID("mcp-command"),
							h.Name("command"),
							h.Class("form-control"),
							h.Required(),
							h.Placeholder("e.g., /usr/local/bin/mcp-server"),
							h.Value(commandValue),
						),
					),
					h.Div(
						h.Class("form-group"),
						h.Label(h.For("mcp-args"), g.Text("Arguments (comma-separated)")),
						h.Input(
							h.Type("text"),
							h.ID("mcp-args"),
							h.Name("args"),
							h.Class("form-control"),
							h.Placeholder("e.g., --port, 8080"),
							h.Value(argsStr),
						),
					),
					h.Div(
						h.Class("form-group"),
						h.Label(h.For("mcp-env"), g.Text("Environment Variables (KEY=VALUE, comma-separated)")),
						h.Textarea(
							h.ID("mcp-env"),
							h.Name("env"),
							h.Class("form-control"),
							h.Rows("3"),
							h.Placeholder("e.g., API_KEY=abc123, DEBUG=true"),
							g.Text(envStr),
						),
					),
					h.Div(
						h.Class("modal-footer"),
						h.A(h.Href("/mcps"), h.Class("btn btn-secondary"), g.Text("Cancel")),
						h.Button(
							h.Type("submit"),
							h.Class("btn btn-primary"),
							g.Text("Save"),
						),
					),
				),
			),
		),
	)
}
