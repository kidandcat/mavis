package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"mavis/codeagent"
	"mavis/soul"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func filterSoulsByStatus(souls []*soul.Soul, status soul.SoulStatus) []*soul.Soul {
	var filtered []*soul.Soul
	for _, s := range souls {
		if s.Status == status {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func handleSoulRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/souls/" || path == "/souls" {
		handleSoulsList(w, r)
		return
	}

	if path == "/souls/scan" {
		handleSoulsScan(w, r)
		return
	}

	// Extract soul ID from path
	parts := strings.Split(strings.TrimPrefix(path, "/souls/"), "/")
	if len(parts) == 0 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Route based on the rest of the path
	if len(parts) == 1 {
		// /souls/{id}
		handleSoulDetails(w, r)
	} else {
		switch parts[1] {
		case "objectives":
			handleUpdateObjectives(w, r)
		case "requirements":
			handleUpdateRequirements(w, r)
		case "delete":
			handleDeleteSoul(w, r)
		case "launch-agent":
			handleLaunchAgentForm(w, r)
		case "test":
			handleTestSoul(w, r)
		case "run-again":
			handleRunAgain(w, r)
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}
}

func handleSoulsList(w http.ResponseWriter, r *http.Request) {
	souls, err := soulManager.ListSouls()
	if err != nil {
		http.Error(w, "Failed to list souls", http.StatusInternalServerError)
		return
	}

	// Use no-refresh layout when modal is open
	layoutFunc := DashboardLayout
	if r.URL.Query().Get("modal") == "create" {
		layoutFunc = DashboardLayoutNoRefresh
	}

	page := layoutFunc(w, r,
		h.Div(h.Class("container"),
			h.H2(g.Text("Souls")),
			h.Div(h.Class("soul-actions"),
				h.A(
					h.Href("/souls?modal=create"),
					h.Class("button primary"),
					g.Text("Create Soul"),
				),
			),
			h.Div(h.Class("souls-list"),
				g.If(len(souls) == 0,
					h.P(g.Text("No souls created yet. Create one to get started!")),
				),
				g.If(len(souls) > 0,
					h.Div(h.Class("souls-columns"),
						// Working column
						h.Div(h.Class("soul-column working-column"),
							h.H3(g.Text("🔄 Working")),
							h.Div(h.Class("soul-cards"),
								g.Group(g.Map(filterSoulsByStatus(souls, soul.SoulStatusWorking), func(s *soul.Soul) g.Node {
									return soulCard(s)
								})),
							),
							g.If(len(filterSoulsByStatus(souls, soul.SoulStatusWorking)) == 0,
								h.P(h.Class("empty-column"), g.Text("No souls currently working")),
							),
						),
						// Standby column
						h.Div(h.Class("soul-column standby-column"),
							h.H3(g.Text("⏸ Standby")),
							h.Div(h.Class("soul-cards"),
								g.Group(g.Map(filterSoulsByStatus(souls, soul.SoulStatusStandby), func(s *soul.Soul) g.Node {
									return soulCard(s)
								})),
							),
							g.If(len(filterSoulsByStatus(souls, soul.SoulStatusStandby)) == 0,
								h.P(h.Class("empty-column"), g.Text("No souls in standby")),
							),
						),
					),
				),
			),
		),
		// Create Soul Modal - show if modal=create in URL
		g.If(r.URL.Query().Get("modal") == "create",
			h.Div(h.ID("createSoulModal"), h.Class("modal"), h.Style("display: block;"),
				h.Div(h.Class("modal-content"),
					h.Div(h.Class("modal-header"),
						h.H3(g.Text("Create New Soul")),
						h.A(h.Href("/souls"), h.Class("close"), g.Text("×")),
					),
					h.Form(h.ID("createSoulForm"), h.Action("/souls/create"), h.Method("POST"),
						h.Div(h.Class("form-group"),
							h.Label(h.For("project_path"), g.Text("Project Path")),
							h.Input(h.Type("text"), h.ID("project_path"), h.Name("project_path"), h.Required(), h.Placeholder("/path/to/project")),
							h.Small(h.Class("help-text"), g.Text("The soul will use the folder name as its name")),
						),
						h.Div(h.Class("form-group"),
							h.Label(h.For("objectives"), g.Text("Objectives (one per line)")),
							h.Textarea(h.ID("objectives"), h.Name("objectives"), h.Rows("5"), h.Placeholder("Build a REST API\nImplement authentication\nAdd comprehensive tests")),
						),
						h.Div(h.Class("form-group"),
							h.Label(h.For("requirements"), g.Text("Requirements (one per line)")),
							h.Textarea(h.ID("requirements"), h.Name("requirements"), h.Rows("5"), h.Placeholder("Must use Go 1.21+\nPostgreSQL database\n95% test coverage")),
						),
						h.Div(h.Class("form-actions"),
							h.Button(h.Type("submit"), h.Class("button primary"), g.Text("Create Soul")),
							h.A(h.Href("/souls"), h.Class("button"), g.Text("Cancel")),
						),
					),
				),
			),
		),
	)

	page.Render(w)
}

func soulCard(s *soul.Soul) g.Node {
	return h.Div(h.Class("soul-card"),
		h.Div(h.Class("soul-header"),
			h.H3(g.Text(s.Name)),
			h.Span(h.Class(fmt.Sprintf("status %s", s.Status)), g.Text(string(s.Status))),
		),
		h.Div(h.Class("soul-info"),
			h.P(h.Strong(g.Text("Project: ")), g.Text(s.ProjectPath)),
			h.P(h.Strong(g.Text("Created: ")), g.Text(s.CreatedAt.Format("2006-01-02 15:04"))),
			g.If(len(s.Iterations) > 0,
				h.P(h.Strong(g.Text("Iterations: ")), g.Text(fmt.Sprintf("%d", len(s.Iterations)))),
			),
		),
		h.Div(h.Class("soul-objectives"),
			h.H4(g.Text("Objectives")),
			g.If(len(s.Objectives) == 0,
				h.P(h.Class("muted"), g.Text("No objectives set")),
			),
			g.If(len(s.Objectives) > 0,
				h.Ul(
					g.Group(g.Map(s.Objectives, func(obj string) g.Node {
						return h.Li(g.Text(obj))
					})),
				),
			),
		),
		h.Div(h.Class("soul-feedback"),
			h.H4(g.Text("Feedback")),
			h.Div(h.Class("feedback-stats"),
				h.Span(h.Class("stat features"), g.Text(fmt.Sprintf("✅ %d Features", len(s.Feedback.ImplementedFeatures)))),
				h.Span(h.Class("stat bugs"), g.Text(fmt.Sprintf("🐛 %d Bugs", len(s.Feedback.KnownBugs)))),
				h.Span(h.Class("stat tests"), g.Text(fmt.Sprintf("🧪 %d Tests", len(s.Feedback.TestResults)))),
			),
		),
		h.Div(h.Class("soul-actions"),
			h.A(h.Href(fmt.Sprintf("/souls/%s", s.ID)), h.Class("button small"), g.Text("View Details")),
			g.If(s.Status == soul.SoulStatusStandby,
				h.Form(h.Action(fmt.Sprintf("/souls/%s/run-again", s.ID)), h.Method("POST"), h.Style("display: inline;"),
					h.Button(h.Type("submit"), h.Class("button small primary"), g.Text("Run Again")),
				),
			),
			h.Form(h.Action(fmt.Sprintf("/souls/%s/delete", s.ID)), h.Method("POST"), h.Style("display: inline;"),
				h.Button(h.Type("submit"), h.Class("button small danger"), g.Text("Delete")),
			),
		),
	)
}

func handleCreateSoul(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	projectPath := r.FormValue("project_path")
	objectivesStr := r.FormValue("objectives")
	requirementsStr := r.FormValue("requirements")

	if projectPath == "" {
		SetFlash(w, "error", "Project path is required")
		http.Redirect(w, r, "/souls", http.StatusSeeOther)
		return
	}

	// Create the soul (name will be extracted from project path)
	newSoul, err := soulManager.CreateSoul("", projectPath)
	if err != nil {
		SetFlash(w, "error", fmt.Sprintf("Failed to create soul: %v", err))
		http.Redirect(w, r, "/souls", http.StatusSeeOther)
		return
	}

	// Parse objectives and requirements
	if objectivesStr != "" {
		objectives := strings.Split(objectivesStr, "\n")
		for _, obj := range objectives {
			obj = strings.TrimSpace(obj)
			if obj != "" {
				newSoul.AddObjective(obj)
			}
		}
	}

	if requirementsStr != "" {
		requirements := strings.Split(requirementsStr, "\n")
		for _, req := range requirements {
			req = strings.TrimSpace(req)
			if req != "" {
				newSoul.AddRequirement(req)
			}
		}
	}

	// Update the soul with objectives and requirements
	if err := soulManager.UpdateSoul(newSoul); err != nil {
		SetFlash(w, "error", fmt.Sprintf("Failed to update soul: %v", err))
		http.Redirect(w, r, "/souls", http.StatusSeeOther)
		return
	}

	SetFlash(w, "success", fmt.Sprintf("Soul '%s' created successfully", filepath.Base(projectPath)))

	// Automatically launch the first test agent for the new soul
	go func() {
		// Small delay to ensure the soul is fully saved
		time.Sleep(1 * time.Second)
		launchTestAgent(newSoul.ID)
	}()

	http.Redirect(w, r, "/souls", http.StatusSeeOther)
}

func handleSoulDetails(w http.ResponseWriter, r *http.Request) {
	soulID := strings.TrimPrefix(r.URL.Path, "/souls/")
	if soulID == "" {
		http.Error(w, "Soul ID required", http.StatusBadRequest)
		return
	}

	s, err := soulManager.GetSoul(soulID)
	if err != nil {
		http.Error(w, "Soul not found", http.StatusNotFound)
		return
	}

	page := DashboardLayoutNoRefresh(w, r,
		h.Div(h.Class("container"),
			h.Div(h.Class("soul-detail-header"),
				h.H2(g.Text(s.Name)),
				h.A(h.Href("/souls"), h.Class("button"), g.Text("← Back to Souls")),
			),
			h.Div(h.Class("soul-detail-content"),
				// Basic Info
				h.Div(h.Class("info-section"),
					h.H3(g.Text("Basic Information")),
					h.Dl(
						h.Dt(g.Text("ID:")), h.Dd(g.Text(s.ID)),
						h.Dt(g.Text("Project Path:")), h.Dd(g.Text(s.ProjectPath)),
						h.Dt(g.Text("Status:")), h.Dd(h.Span(h.Class(fmt.Sprintf("status %s", s.Status)), g.Text(string(s.Status)))),
						h.Dt(g.Text("Created:")), h.Dd(g.Text(s.CreatedAt.Format(time.RFC3339))),
						h.Dt(g.Text("Updated:")), h.Dd(g.Text(s.UpdatedAt.Format(time.RFC3339))),
					),
				),
				// Objectives & Requirements
				h.Div(h.Class("objectives-requirements"),
					h.Div(h.Class("objectives-section"),
						h.H3(g.Text("Objectives")),
						h.Form(h.Action(fmt.Sprintf("/souls/%s/objectives", s.ID)), h.Method("POST"),
							h.Textarea(h.Name("objectives"), h.Rows("6"),
								g.Text(strings.Join(s.Objectives, "\n")),
							),
							h.Button(h.Type("submit"), h.Class("button primary"), g.Text("Update Objectives")),
						),
					),
					h.Div(h.Class("requirements-section"),
						h.H3(g.Text("Requirements")),
						h.Form(h.Action(fmt.Sprintf("/souls/%s/requirements", s.ID)), h.Method("POST"),
							h.Textarea(h.Name("requirements"), h.Rows("6"),
								g.Text(strings.Join(s.Requirements, "\n")),
							),
							h.Button(h.Type("submit"), h.Class("button primary"), g.Text("Update Requirements")),
						),
					),
				),
				// Iterations
				h.Div(h.Class("iterations-section"),
					h.H3(g.Text("Iterations")),
					g.If(len(s.Iterations) == 0,
						h.P(h.Class("muted"), g.Text("No iterations yet")),
					),
					g.If(len(s.Iterations) > 0,
						h.Table(h.Class("iterations-table"),
							h.Tr(
								h.Th(g.Text("#")),
								h.Th(g.Text("Agent ID")),
								h.Th(g.Text("Purpose")),
								h.Th(g.Text("Started")),
								h.Th(g.Text("Completed")),
								h.Th(g.Text("Result")),
							),
							g.Group(g.Map(s.Iterations, func(iter soul.SoulIteration) g.Node {
								return h.Tr(
									h.Td(g.Text(fmt.Sprintf("%d", iter.Number))),
									h.Td(h.Code(g.Text(iter.AgentID))),
									h.Td(g.Text(iter.Purpose)),
									h.Td(g.Text(iter.StartedAt.Format("15:04:05"))),
									h.Td(
										func() g.Node {
											if iter.CompletedAt != nil {
												return g.Text(iter.CompletedAt.Format("15:04:05"))
											}
											return g.Text("In Progress")
										}(),
									),
									h.Td(g.Text(iter.Result)),
								)
							})),
						),
					),
				),
				// Feedback
				h.Div(h.Class("feedback-section"),
					h.H3(g.Text("Feedback")),
					// Implemented Features
					h.Div(h.Class("features-subsection"),
						h.H4(g.Text(fmt.Sprintf("Implemented Features (%d)", len(s.Feedback.ImplementedFeatures)))),
						g.If(len(s.Feedback.ImplementedFeatures) == 0,
							h.P(h.Class("muted"), g.Text("No features implemented yet")),
						),
						g.If(len(s.Feedback.ImplementedFeatures) > 0,
							h.Ul(h.Class("features-list"),
								g.Group(g.Map(s.Feedback.ImplementedFeatures, func(f soul.Feature) g.Node {
									return h.Li(
										h.Strong(g.Text(f.Name)),
										g.Text(" - "),
										g.Text(f.Description),
										h.Span(h.Class("timestamp"), g.Text(fmt.Sprintf(" (%s)", f.ImplementedAt.Format("2006-01-02")))),
									)
								})),
							),
						),
					),
					// Known Bugs
					h.Div(h.Class("bugs-subsection"),
						h.H4(g.Text(fmt.Sprintf("Known Bugs (%d)", len(s.Feedback.KnownBugs)))),
						g.If(len(s.Feedback.KnownBugs) == 0,
							h.P(h.Class("muted"), g.Text("No bugs reported")),
						),
						g.If(len(s.Feedback.KnownBugs) > 0,
							h.Ul(h.Class("bugs-list"),
								g.Group(g.Map(s.Feedback.KnownBugs, func(b soul.Bug) g.Node {
									return h.Li(
										h.Span(h.Class(fmt.Sprintf("severity %s", b.Severity)), g.Text(b.Severity)),
										g.Text(" "),
										g.Text(b.Description),
										h.Span(h.Class("bug-status"), g.Text(fmt.Sprintf(" [%s]", b.Status))),
									)
								})),
							),
						),
					),
					// Test Results
					h.Div(h.Class("tests-subsection"),
						h.H4(g.Text(fmt.Sprintf("Test Results (%d)", len(s.Feedback.TestResults)))),
						g.If(len(s.Feedback.TestResults) == 0,
							h.P(h.Class("muted"), g.Text("No test results")),
						),
						g.If(len(s.Feedback.TestResults) > 0,
							h.Ul(h.Class("tests-list"),
								g.Group(g.Map(s.Feedback.TestResults, func(t soul.TestResult) g.Node {
									return h.Li(
										g.If(t.Passed,
											h.Span(h.Class("test-pass"), g.Text("✅")),
										),
										g.If(!t.Passed,
											h.Span(h.Class("test-fail"), g.Text("❌")),
										),
										g.Text(" "),
										g.Text(t.TestName),
										g.If(t.Message != "",
											g.Group([]g.Node{
												g.Text(" - "),
												h.Span(h.Class("test-message"), g.Text(t.Message)),
											}),
										),
									)
								})),
							),
						),
					),
				),
			),
		),
	)

	page.Render(w)
}

func handleUpdateObjectives(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	soulID := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/objectives"), "/souls/")
	objectivesStr := r.FormValue("objectives")

	objectives := []string{}
	if objectivesStr != "" {
		for _, obj := range strings.Split(objectivesStr, "\n") {
			obj = strings.TrimSpace(obj)
			if obj != "" {
				objectives = append(objectives, obj)
			}
		}
	}

	if err := soulManager.UpdateObjectives(soulID, objectives); err != nil {
		SetFlash(w, "error", fmt.Sprintf("Failed to update objectives: %v", err))
	} else {
		SetFlash(w, "success", "Objectives updated successfully")
	}

	http.Redirect(w, r, fmt.Sprintf("/souls/%s", soulID), http.StatusSeeOther)
}

func handleUpdateRequirements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	soulID := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/requirements"), "/souls/")
	requirementsStr := r.FormValue("requirements")

	requirements := []string{}
	if requirementsStr != "" {
		for _, req := range strings.Split(requirementsStr, "\n") {
			req = strings.TrimSpace(req)
			if req != "" {
				requirements = append(requirements, req)
			}
		}
	}

	if err := soulManager.UpdateRequirements(soulID, requirements); err != nil {
		SetFlash(w, "error", fmt.Sprintf("Failed to update requirements: %v", err))
	} else {
		SetFlash(w, "success", "Requirements updated successfully")
	}

	http.Redirect(w, r, fmt.Sprintf("/souls/%s", soulID), http.StatusSeeOther)
}

func handleDeleteSoul(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	soulID := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/delete"), "/souls/")

	s, err := soulManager.GetSoul(soulID)
	if err != nil {
		SetFlash(w, "error", "Soul not found")
		http.Redirect(w, r, "/souls", http.StatusSeeOther)
		return
	}

	if err := soulManager.DeleteSoul(soulID); err != nil {
		SetFlash(w, "error", fmt.Sprintf("Failed to delete soul: %v", err))
	} else {
		SetFlash(w, "success", fmt.Sprintf("Soul '%s' deleted successfully", s.Name))
	}

	http.Redirect(w, r, "/souls", http.StatusSeeOther)
}

func handleSoulsScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Since we're using SQLite now, scan is instant
	SetFlash(w, "info", "✅ Souls are now stored in a database and don't require scanning.")
	http.Redirect(w, r, "/souls", http.StatusSeeOther)
}

// handleTestSoul handles the test soul action
func handleTestSoul(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	soulID := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/test"), "/souls/")

	// Launch a test agent
	launchTestAgent(soulID)

	SetFlash(w, "success", "Test agent launched! Check the agents page to monitor progress.")
	http.Redirect(w, r, "/agents", http.StatusSeeOther)
}

// handleRunAgain handles running a standby soul again
func handleRunAgain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	soulID := strings.TrimPrefix(strings.TrimSuffix(r.URL.Path, "/run-again"), "/souls/")

	// Get the soul to verify it exists and is in standby
	s, err := soulManager.GetSoul(soulID)
	if err != nil {
		SetFlash(w, "error", "Soul not found")
		http.Redirect(w, r, "/souls", http.StatusSeeOther)
		return
	}

	if s.Status != soul.SoulStatusStandby {
		SetFlash(w, "error", "Soul is not in standby status")
		http.Redirect(w, r, "/souls", http.StatusSeeOther)
		return
	}

	// Update soul status to working
	s.Status = soul.SoulStatusWorking
	if err := soulManager.UpdateSoul(s); err != nil {
		SetFlash(w, "error", fmt.Sprintf("Failed to update soul status: %v", err))
		http.Redirect(w, r, "/souls", http.StatusSeeOther)
		return
	}

	// Launch a test agent to restart the soul's development cycle
	launchTestAgent(soulID)

	SetFlash(w, "success", fmt.Sprintf("Soul '%s' is running again! Check the agents page to monitor progress.", s.Name))
	http.Redirect(w, r, "/agents", http.StatusSeeOther)
}

// Handle the launch agent form display and submission
func handleLaunchAgentForm(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/souls/"), "/")
	if len(parts) < 2 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	soulID := parts[0]

	if r.Method == http.MethodGet {
		// Display the form
		s, err := soulManager.GetSoul(soulID)
		if err != nil {
			http.Error(w, "Soul not found", http.StatusNotFound)
			return
		}

		page := DashboardLayoutNoRefresh(w, r,
			h.Div(h.Class("container"),
				h.H2(g.Text("Launch Agent for Soul: "+s.Name)),
				h.Form(h.Action(fmt.Sprintf("/souls/%s/launch-agent", soulID)), h.Method("POST"),
					h.Div(h.Class("form-group"),
						h.Label(h.For("purpose"), g.Text("What should this agent iteration do?")),
						h.Input(
							h.Type("text"),
							h.ID("purpose"),
							h.Name("purpose"),
							h.Required(),
							h.Value("Continue working on project objectives"),
							h.Style("width: 100%;"),
						),
					),
					h.Div(h.Class("form-actions"),
						h.Button(h.Type("submit"), h.Class("button primary"), g.Text("Launch Agent")),
						h.A(h.Href("/souls"), h.Class("button"), g.Text("Cancel")),
					),
				),
			),
		)
		page.Render(w)
		return
	}

	if r.Method == http.MethodPost {
		// Handle form submission
		purpose := r.FormValue("purpose")
		if purpose == "" {
			purpose = "Continue working on project objectives"
		}

		// Call the API endpoint
		reqBody := map[string]string{
			"soul_id": soulID,
			"purpose": purpose,
		}
		jsonData, _ := json.Marshal(reqBody)

		req, err := http.NewRequest("POST", "/api/souls/launch-agent", bytes.NewReader(jsonData))
		if err != nil {
			SetFlash(w, "error", "Failed to create request")
			http.Redirect(w, r, "/souls", http.StatusSeeOther)
			return
		}
		req.Header.Set("Content-Type", "application/json")

		// Use the existing handler
		rr := httptest.NewRecorder()
		handleLaunchAgentForSoul(rr, req)

		if rr.Code == http.StatusOK {
			var response map[string]string
			json.Unmarshal(rr.Body.Bytes(), &response)
			SetFlash(w, "success", fmt.Sprintf("Agent launched successfully! ID: %s", response["agent_id"]))
			http.Redirect(w, r, "/agents", http.StatusSeeOther)
		} else {
			SetFlash(w, "error", "Failed to launch agent")
			http.Redirect(w, r, "/souls", http.StatusSeeOther)
		}
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// API endpoint for launching an agent from a soul
// launchNextAgent launches a new agent in the soul loop
func launchNextAgent(s *soul.Soul, testFeedback string) {
	// Build prompt based on test feedback
	prompt := fmt.Sprintf("Soul: %s\n\n", s.Name)
	prompt += "IMPORTANT: Focus on ONE specific task at a time. The development process is iterative - you don't need to fix everything in one go.\n\n"
	prompt += "Purpose: Address ONE of the most critical issues from the test feedback below\n\n"

	if len(s.Objectives) > 0 {
		prompt += "Objectives:\n"
		for _, obj := range s.Objectives {
			prompt += fmt.Sprintf("- %s\n", obj)
		}
		prompt += "\n"
	}

	if len(s.Requirements) > 0 {
		prompt += "Requirements:\n"
		for _, req := range s.Requirements {
			prompt += fmt.Sprintf("- %s\n", req)
		}
		prompt += "\n"
	}

	prompt += "Test Feedback:\n"
	prompt += testFeedback
	prompt += "\n\n"
	prompt += "INSTRUCTIONS:\n"
	prompt += "1. Read through all the feedback\n"
	prompt += "2. Pick ONE specific issue or feature to work on\n"
	prompt += "3. Focus only on that single task\n"
	prompt += "4. Make sure your changes are complete and working\n"
	prompt += "5. After you finish, another agent will be launched to continue with the remaining work\n\n"
	prompt += "Remember: Quality over quantity. It's better to fully complete one task than to partially complete many."

	// Launch the agent
	agentID, err := agentManager.LaunchAgent(context.Background(), s.ProjectPath, prompt)
	if err != nil {
		log.Printf("Failed to launch next agent for soul %s: %v", s.ID, err)
		return
	}

	// Start soul iteration
	if err := soulManager.StartSoulIteration(s.ID, agentID, "Address test feedback and continue development"); err != nil {
		log.Printf("Failed to start soul iteration: %v", err)
	}

	// Set completion callback that will test again
	if agent, err := agentManager.GetAgent(agentID); err == nil && agent != nil {
		agent.SetCompletionCallback(func(completedAgent *codeagent.Agent) {
			// Complete the iteration
			result := completedAgent.Output
			if completedAgent.Error != "" {
				result = fmt.Sprintf("Error: %s\n\n%s", completedAgent.Error, completedAgent.Output)
			}

			if err := soulManager.CompleteSoulIteration(s.ID, completedAgent.ID, result); err != nil {
				log.Printf("Failed to complete soul iteration: %v", err)
			}

			// After development, launch a test agent
			go func() {
				time.Sleep(2 * time.Second)
				// Check if souls are paused
				if soulManager.IsPaused() {
					log.Printf("Souls are paused. Not launching test agent for soul %s", s.ID)
					return
				}
				launchTestAgent(s.ID)
			}()
		})
	}
}

// launchTestAgent launches a test agent to check if the application is production ready
func launchTestAgent(soulID string) {
	req := struct {
		SoulID  string `json:"soul_id"`
		Purpose string `json:"purpose"`
	}{
		SoulID:  soulID,
		Purpose: "Test that the application meets all objectives",
	}

	jsonData, _ := json.Marshal(req)
	r, _ := http.NewRequest("POST", "/api/souls/launch-agent", bytes.NewReader(jsonData))
	r.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handleLaunchAgentForSoul(rr, r)

	if rr.Code != http.StatusOK {
		log.Printf("Failed to launch test agent for soul %s", soulID)
	}
}

func handleLaunchAgentForSoul(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SoulID  string `json:"soul_id"`
		Purpose string `json:"purpose"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	s, err := soulManager.GetSoul(req.SoulID)
	if err != nil {
		http.Error(w, "Soul not found", http.StatusNotFound)
		return
	}

	// Set soul status to working when launching an agent
	s.Status = soul.SoulStatusWorking
	if err := soulManager.UpdateSoul(s); err != nil {
		log.Printf("Failed to update soul status to working: %v", err)
	}

	// Build the agent prompt from soul context
	prompt := fmt.Sprintf("Soul: %s\n\n", s.Name)

	// Check if this is a test iteration
	isTestIteration := req.Purpose == "Test that the application meets all objectives"

	if isTestIteration {
		// Special prompt for test iterations
		prompt += "IMPORTANT: You are testing whether this application is production ready.\n\n"
		prompt += "Your task is to thoroughly test that the application meets ALL the objectives and requirements listed below.\n"
		prompt += "At the end of your testing, you MUST output one of the following:\n"
		prompt += "1. The exact text \"PRODUCTION READY\" (without quotes) ONLY if:\n"
		prompt += "   - ALL objectives are met\n"
		prompt += "   - ALL requirements are satisfied\n"
		prompt += "   - There are NO bugs or errors in the application\n"
		prompt += "   - Everything is working correctly\n"
		prompt += "2. If NOT ready, provide a PRIORITIZED list of issues. Start with the MOST CRITICAL issue that blocks production readiness.\n\n"
		prompt += "When listing issues:\n"
		prompt += "- List them in order of priority (most critical first)\n"
		prompt += "- Be specific about what needs to be fixed\n"
		prompt += "- Include any bugs or errors you find\n"
		prompt += "- Focus on functional issues rather than nice-to-haves\n"
		prompt += "- Remember: The next agent will work on ONE issue at a time\n\n"
	} else if req.Purpose != "" {
		prompt += fmt.Sprintf("Purpose: %s\n\n", req.Purpose)
	}

	if len(s.Objectives) > 0 {
		prompt += "Objectives:\n"
		for _, obj := range s.Objectives {
			prompt += fmt.Sprintf("- %s\n", obj)
		}
		prompt += "\n"
	}

	if len(s.Requirements) > 0 {
		prompt += "Requirements:\n"
		for _, req := range s.Requirements {
			prompt += fmt.Sprintf("- %s\n", req)
		}
		prompt += "\n"
	}

	// Add feedback context if available
	if len(s.Feedback.ImplementedFeatures) > 0 {
		prompt += fmt.Sprintf("\nAlready implemented %d features:\n", len(s.Feedback.ImplementedFeatures))
		for _, feature := range s.Feedback.ImplementedFeatures {
			prompt += fmt.Sprintf("- %s: %s\n", feature.Name, feature.Description)
		}
	}
	if len(s.Feedback.KnownBugs) > 0 {
		prompt += fmt.Sprintf("\nThere are %d known bugs:\n", len(s.Feedback.KnownBugs))
		for _, bug := range s.Feedback.KnownBugs {
			prompt += fmt.Sprintf("- [%s] %s (Status: %s)\n", bug.Severity, bug.Description, bug.Status)
		}
	}
	if len(s.Feedback.TestResults) > 0 {
		passedTests := 0
		for _, test := range s.Feedback.TestResults {
			if test.Passed {
				passedTests++
			}
		}
		prompt += fmt.Sprintf("\nTest Results: %d/%d tests passed\n", passedTests, len(s.Feedback.TestResults))
	}

	if isTestIteration {
		prompt += "\nRemember: After testing, output either \"PRODUCTION READY\" or a prioritized list of issues (most critical first).\n"
		prompt += "The development process is iterative - the next agent will tackle one issue at a time.\n"
	}

	// Create a completion callback for the agent
	completionCallback := func(agent *codeagent.Agent) {
		// Complete the soul iteration with the agent's result
		result := agent.Output
		if agent.Error != "" {
			result = fmt.Sprintf("Error: %s\n\n%s", agent.Error, agent.Output)
		}

		if err := soulManager.CompleteSoulIteration(req.SoulID, agent.ID, result); err != nil {
			log.Printf("Failed to complete soul iteration: %v", err)
		}

		// If agent failed or was killed, continue with the next iteration
		if agent.Status == codeagent.StatusFailed || agent.Status == codeagent.StatusKilled {
			log.Printf("Agent %s for soul %s failed/killed (status: %s), continuing with next iteration", agent.ID, req.SoulID, agent.Status)
			
			// Check if souls are paused before launching next iteration
			if soulManager.IsPaused() {
				log.Printf("Souls are paused. Not launching next iteration for soul %s", req.SoulID)
				return
			}
			
			// Launch a test agent to assess the current state
			go func() {
				time.Sleep(2 * time.Second) // Wait a bit to ensure cleanup
				launchTestAgent(req.SoulID)
			}()
			return
		}

		// Check if this was a test iteration
		if isTestIteration && agent.Status == codeagent.StatusFinished {
			// Check if the output is exactly "PRODUCTION READY"
			if strings.TrimSpace(result) == "PRODUCTION READY" {
				// Before declaring production ready, verify there are no bugs
				s, err := soulManager.GetSoul(req.SoulID)
				if err != nil {
					log.Printf("Failed to get soul to check bugs: %v", err)
					return
				}
				
				// Count unfixed bugs
				unfixedBugs := 0
				for _, bug := range s.Feedback.KnownBugs {
					if bug.Status != "fixed" && bug.FixedAt == nil {
						unfixedBugs++
					}
				}
				
				if unfixedBugs > 0 {
					log.Printf("Soul %s claims PRODUCTION READY but has %d unfixed bugs. Continuing iterations.", s.ID, unfixedBugs)
					
					// Create a prompt to fix the bugs
					bugPrompt := fmt.Sprintf("The test agent reported PRODUCTION READY, but there are still %d unfixed bugs:\n\n", unfixedBugs)
					for _, bug := range s.Feedback.KnownBugs {
						if bug.Status != "fixed" && bug.FixedAt == nil {
							bugPrompt += fmt.Sprintf("- [%s] %s\n", bug.Severity, bug.Description)
						}
					}
					bugPrompt += "\nPlease fix these bugs before the application can be considered production ready."
					
					// Launch next agent to fix bugs
					go func() {
						time.Sleep(2 * time.Second)
						if !soulManager.IsPaused() {
							launchNextAgent(s, bugPrompt)
						}
					}()
					return
				}
				
				// No bugs found, proceed with marking as production ready
				s.Status = soul.SoulStatusStandby
				if err := soulManager.UpdateSoul(s); err != nil {
					log.Printf("Failed to update soul status to standby: %v", err)
				}
				log.Printf("Soul %s is PRODUCTION READY! No bugs remaining. Moving to standby.", s.ID)

				// Execute git push ai command in the project folder
				go func() {
					log.Printf("Executing git push for PRODUCTION READY soul %s at %s", s.ID, s.ProjectPath)

					// Create a new context for the git command
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					defer cancel()

					// Execute git push ai command
					cmd := exec.CommandContext(ctx, "bash", "-c", fmt.Sprintf("cd '%s' && git push ai", s.ProjectPath))
					output, err := cmd.CombinedOutput()

					if err != nil {
						log.Printf("Failed to execute git push for soul %s: %v, output: %s", s.ID, err, string(output))
					} else {
						log.Printf("Successfully executed git push for soul %s, output: %s", s.ID, string(output))
					}
				}()
			} else {
				// The output should contain a prompt for the next agent
				// Extract the prompt (everything after the test results)
				// For now, we'll use the entire output as the next prompt
				log.Printf("Soul %s is not production ready yet, launching next agent...", s.ID)

				// Launch a new agent with the output as the prompt
				go func() {
					// Wait a bit to ensure the current agent is fully processed
					time.Sleep(2 * time.Second)

					// Check if souls are paused
					if soulManager.IsPaused() {
						log.Printf("Souls are paused. Not launching next iteration for soul %s", req.SoulID)
						return
					}

					// The result contains the prompt for what needs to be done
					// We'll pass it as part of the context
					s, err := soulManager.GetSoul(req.SoulID)
					if err != nil {
						log.Printf("Failed to get soul for next iteration: %v", err)
						return
					}

					// Add the test result as feedback
					testResult := soul.TestResult{
						TestName:   "Production Readiness Test",
						Passed:     false,
						Message:    result,
						ExecutedAt: time.Now(),
						AgentID:    agent.ID,
					}
					if err := soulManager.AddTestResult(req.SoulID, testResult); err != nil {
						log.Printf("Failed to add test result: %v", err)
					}

					// Launch the next development agent
					launchNextAgent(s, result)
				}()
			}
		}

		// Parse agent output to extract features, bugs, and test results
		features, bugs, testResults := soul.ParseAgentOutput(result, agent.ID)

		// Add extracted features to soul feedback
		for _, feature := range features {
			if err := soulManager.AddFeature(req.SoulID, feature); err != nil {
				log.Printf("Failed to add feature: %v", err)
			}
		}

		// Add extracted bugs to soul feedback
		for _, bug := range bugs {
			if err := soulManager.AddBug(req.SoulID, bug); err != nil {
				log.Printf("Failed to add bug: %v", err)
			}
		}

		// Add extracted test results to soul feedback
		for _, testResult := range testResults {
			if err := soulManager.AddTestResult(req.SoulID, testResult); err != nil {
				log.Printf("Failed to add test result: %v", err)
			}
		}
	}

	// Launch the agent
	agentID, err := agentManager.LaunchAgent(context.Background(), s.ProjectPath, prompt)
	if err != nil {
		log.Printf("Failed to launch agent for soul %s: %v", req.SoulID, err)
		http.Error(w, fmt.Sprintf("Failed to launch agent: %v", err), http.StatusInternalServerError)
		return
	}

	// Start soul iteration
	if err := soulManager.StartSoulIteration(req.SoulID, agentID, req.Purpose); err != nil {
		log.Printf("Failed to start soul iteration: %v", err)
	}

	// Set completion callback for the agent
	if agent, err := agentManager.GetAgent(agentID); err == nil && agent != nil {
		agent.SetCompletionCallback(completionCallback)
	}

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"agent_id": agentID,
		"status":   "launched",
	})
}

// handleSoulsPauseState returns the current pause state
func handleSoulsPauseState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"paused": soulManager.IsPaused(),
	})
}

// handleSoulsTogglePause toggles the pause state
func handleSoulsTogglePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	newState, err := soulManager.TogglePause()
	if err != nil {
		// Check if this is a form submission
		if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" || r.FormValue("_form") != "" {
			SetFlash(w, "error", fmt.Sprintf("Failed to toggle pause state: %v", err))
			http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to toggle pause state: %v", err), http.StatusInternalServerError)
		return
	}

	// If we're resuming (newState = false), spawn agents for working souls without agents
	if !newState {
		go func() {
			log.Printf("Souls resumed, checking for working souls without agents...")

			// Get all souls
			souls, err := soulManager.ListSouls()
			if err != nil {
				log.Printf("Failed to list souls: %v", err)
				return
			}

			// Check each working soul
			for _, s := range souls {
				if s.Status == soul.SoulStatusWorking {
					// Check if an agent is already running for this soul's project
					if running, _ := agentManager.IsAgentRunningInFolder(s.ProjectPath); !running {
						log.Printf("Soul %s (%s) is working but has no agent, launching test agent...", s.ID, s.Name)
						// Launch a test agent for this soul
						launchTestAgent(s.ID)
					}
				}
			}
		}()
	}

	// Check if this is a form submission
	if r.Header.Get("Content-Type") == "application/x-www-form-urlencoded" || r.FormValue("_form") != "" {
		message := fmt.Sprintf("Souls %s", map[bool]string{true: "paused", false: "resumed"}[newState])
		SetFlash(w, "success", message)
		// Redirect back to the referrer page
		referer := r.Referer()
		if referer == "" {
			referer = "/souls"
		}
		http.Redirect(w, r, referer, http.StatusSeeOther)
		return
	}

	// JSON response for API calls
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"paused":  newState,
		"message": fmt.Sprintf("Souls %s", map[bool]string{true: "paused", false: "resumed"}[newState]),
	})
}
