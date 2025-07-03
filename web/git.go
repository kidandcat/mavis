package web

import (
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func GitSection(folderPath string, diff string, showDiff bool) g.Node {
	if folderPath == "" {
		folderPath = "."
	}

	return h.Div(h.ID("git-section"), h.Class("section"),
		h.Div(h.Class("section-header"),
			h.H2(g.Text("Git Operations")),
		),

		h.Div(h.Class("git-folder-selector"),
			h.Form(h.Class("inline-form"), h.Method("get"), h.Action("/git"),
				h.Div(h.Class("form-group"),
					h.Label(h.For("git-folder"), g.Text("Repository Path:")),
					h.Input(
						h.Type("text"),
						h.ID("git-folder"),
						h.Name("folder"),
						h.Placeholder("Enter repository path (e.g., /path/to/repo)"),
						h.Value(folderPath),
						h.Class("form-control"),
					),
				),
				h.Button(
					h.Type("submit"),
					h.Class("btn btn-primary"),
					g.Text("Load Repository"),
				),
			),
		),

		h.Div(h.ID("git-diff-container"),
			g.If(showDiff,
				GitDiff(diff),
			),
			g.If(!showDiff && folderPath != "",
				h.Div(h.Class("info"), g.Text("Loading repository changes...")),
			),
			g.If(!showDiff && folderPath == "",
				h.Div(h.Class("info"), g.Text("Select a repository path and click 'Load Repository' to view changes")),
			),
		),

		g.If(showDiff,
			GitCommitForm(folderPath),
		),

		h.Div(h.Class("git-pr-section"),
			h.Div(h.Class("section-header"),
				h.H3(g.Text("Pull Request Operations")),
			),

			h.Div(h.Class("pr-operations"),
				// PR Review/Approval Form
				h.Div(h.Class("pr-review-form"),
					h.H4(g.Text("Review/Approve Pull Request")),
					h.Form(
						h.ID("pr-review-form"),
						h.Method("post"),
						h.Action("/api/git/pr/review"),

						h.Input(h.Type("hidden"), h.Name("folder"), h.Value(folderPath)),

						h.Div(h.Class("form-group"),
							h.Label(h.For("pr-url"), g.Text("PR URL:")),
							h.Input(
								h.Type("url"),
								h.ID("pr-url"),
								h.Name("pr_url"),
								h.Placeholder("https://github.com/owner/repo/pull/123"),
								h.Class("form-control"),
								h.Required(),
							),
						),

						h.Div(h.Class("form-group"),
							h.Label(h.For("pr-action"), g.Text("Action:")),
							h.Select(
								h.ID("pr-action"),
								h.Name("action"),
								h.Class("form-control"),
								h.Option(h.Value("approve"), g.Text("Review & Approve")),
								h.Option(h.Value("review"), g.Text("Review Only")),
								h.Option(h.Value("request-changes"), g.Text("Request Changes")),
							),
						),

						h.Button(
							h.Type("submit"),
							h.Class("btn btn-primary"),
							g.Text("Submit Review"),
						),
					),
				),
			),

			h.Div(h.ID("pr-result"), h.Class("pr-result")),
		),
	)
}

func GitDiff(diff string) g.Node {
	if diff == "" {
		return h.Div(h.Class("no-changes"), g.Text("No changes to commit"))
	}

	lines := strings.Split(diff, "\n")
	return h.Div(h.Class("git-diff"),
		h.Pre(
			g.Group(g.Map(lines, func(line string) g.Node {
				class := ""
				if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
					class = "diff-add"
				} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
					class = "diff-remove"
				} else if strings.HasPrefix(line, "@@") {
					class = "diff-hunk"
				} else if strings.HasPrefix(line, "diff --git") {
					class = "diff-header"
				}

				if class != "" {
					return h.Span(h.Class(class), g.Text(line+"\n"))
				}
				return g.Text(line + "\n")
			})),
		),
	)
}

func GitCommitForm(folderPath string) g.Node {
	return h.Div(h.Class("git-commit-form"),
		h.Form(
			h.ID("git-commit-form"),
			h.Method("post"),
			h.Action("/api/git/commit"),

			// Hidden field to pass the folder path
			h.Input(h.Type("hidden"), h.Name("folder"), h.Value(folderPath)),

			h.Div(h.Class("form-info"),
				h.P(g.Text("The AI will analyze your changes and create an appropriate commit message.")),
			),

			h.Div(h.Class("form-actions"),
				h.Button(
					h.Type("submit"),
					h.Class("btn btn-primary"),
					g.Text("Launch AI Commit"),
				),
			),
		),

		h.Div(h.ID("git-result"), h.Class("git-result")),
	)
}

func GitResult(success bool, message string) g.Node {
	class := "success"
	if !success {
		class = "error"
	}

	return h.Div(
		h.Class("notification "+class),
		g.Text(message),
	)
}
