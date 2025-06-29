package web

import (
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// MCPsSection renders the MCPs management section
func MCPsSection() g.Node {
	mcps := mcpStore.List()

	return h.Div(
		h.Class("section"),
		h.H2(h.Class("section-title"), g.Text("Model Context Protocol Servers")),

		// Add MCP button
		h.Div(
			h.Class("mb-4"),
			h.Button(
				h.Class("btn btn-primary"),
				h.ID("add-mcp-btn"),
				h.Type("button"),
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
							h.Button(
								h.Class("btn btn-sm btn-secondary"),
								h.Type("button"),
								h.Data("mcp-id", mcp.ID),
								h.Data("mcp-name", mcp.Name),
								h.Data("mcp-command", mcp.Command),
								g.Attr("onclick", fmt.Sprintf("editMCP('%s')", mcp.ID)),
								g.Text("Edit"),
							),
							g.Text(" "),
							h.Button(
								h.Class("btn btn-sm btn-danger"),
								h.Type("button"),
								g.Attr("onclick", fmt.Sprintf("deleteMCP('%s', '%s')", mcp.ID, mcp.Name)),
								g.Text("Delete"),
							),
						),
					)
				})),
			),
		),

		// Add/Edit MCP Modal
		h.Div(
			h.ID("mcp-modal"),
			h.Class("modal"),
			h.Style("display: none;"),
			h.Div(
				h.Class("modal-content"),
				h.Div(
					h.Class("modal-header"),
					h.H3(h.ID("mcp-modal-title"), g.Text("Add MCP Server")),
					h.Button(
						h.Class("close-btn"),
						h.Type("button"),
						g.Attr("onclick", "closeMCPModal()"),
						g.Text("×"),
					),
				),
				h.Div(
					h.Class("modal-body"),
					h.Form(
						h.ID("mcp-form"),
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
							),
						),
						h.Input(h.Type("hidden"), h.ID("mcp-id"), h.Name("id")),
					),
				),
				h.Div(
					h.Class("modal-footer"),
					h.Button(
						h.Type("button"),
						h.Class("btn btn-secondary"),
						g.Attr("onclick", "closeMCPModal()"),
						g.Text("Cancel"),
					),
					h.Button(
						h.Type("button"),
						h.Class("btn btn-primary"),
						h.ID("save-mcp-btn"),
						g.Attr("onclick", "saveMCP()"),
						g.Text("Save"),
					),
				),
			),
		),

		// JavaScript for MCP management
		h.Script(g.Raw(`
			// MCP Management Functions
			document.getElementById('add-mcp-btn').addEventListener('click', function() {
				document.getElementById('mcp-modal-title').textContent = 'Add MCP Server';
				document.getElementById('mcp-form').reset();
				document.getElementById('mcp-id').value = '';
				document.getElementById('mcp-modal').style.display = 'block';
			});
			
			function closeMCPModal() {
				document.getElementById('mcp-modal').style.display = 'none';
			}
			
			function editMCP(id) {
				// Find the MCP data from the table row
				const row = document.querySelector('[data-mcp-id="' + id + '"]').closest('tr');
				const name = row.cells[0].textContent;
				const command = row.cells[1].textContent;
				const args = row.cells[2].textContent === '-' ? '' : row.cells[2].textContent;
				const env = row.cells[3].textContent === '-' ? '' : row.cells[3].textContent;
				
				// Populate the form
				document.getElementById('mcp-modal-title').textContent = 'Edit MCP Server';
				document.getElementById('mcp-id').value = id;
				document.getElementById('mcp-name').value = name;
				document.getElementById('mcp-command').value = command;
				document.getElementById('mcp-args').value = args.replace(/[\[\]]/g, '');
				document.getElementById('mcp-env').value = env.replace(/[{}]/g, '').replace(/:/g, '=');
				document.getElementById('mcp-modal').style.display = 'block';
			}
			
			function saveMCP() {
				const form = document.getElementById('mcp-form');
				const id = form.elements['id'].value;
				const name = form.elements['name'].value;
				const command = form.elements['command'].value;
				const argsStr = form.elements['args'].value;
				const envStr = form.elements['env'].value;
				
				// Parse args
				const args = argsStr ? argsStr.split(',').map(arg => arg.trim()).filter(arg => arg) : [];
				
				// Parse env
				const env = {};
				if (envStr) {
					envStr.split(',').forEach(pair => {
						const [key, value] = pair.trim().split('=');
						if (key && value) {
							env[key] = value;
						}
					});
				}
				
				const mcp = { name, command, args, env };
				
				const method = id ? 'PUT' : 'POST';
				const url = id ? '/api/mcps?id=' + id : '/api/mcps';
				
				fetch(url, {
					method: method,
					headers: {
						'Content-Type': 'application/json',
					},
					body: JSON.stringify(mcp),
				})
				.then(response => {
					if (!response.ok) {
						throw new Error('Failed to save MCP');
					}
					return response.json();
				})
				.then(data => {
					closeMCPModal();
					window.location.reload();
				})
				.catch(error => {
					alert('Error saving MCP: ' + error.message);
				});
			}
			
			function deleteMCP(id, name) {
				if (!confirm('Are you sure you want to delete MCP "' + name + '"?')) {
					return;
				}
				
				fetch('/api/mcps?id=' + id, {
					method: 'DELETE',
				})
				.then(response => {
					if (!response.ok) {
						throw new Error('Failed to delete MCP');
					}
					window.location.reload();
				})
				.catch(error => {
					alert('Error deleting MCP: ' + error.message);
				});
			}
			
			// Close modal when clicking outside
			window.onclick = function(event) {
				const modal = document.getElementById('mcp-modal');
				if (event.target == modal) {
					closeMCPModal();
				}
			}
		`)),
	)
}
