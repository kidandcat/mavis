package components

import (
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func GitSection() g.Node {
	return h.Div(h.ID("git-section"), h.Class("section"),
		h.Div(h.Class("section-header"),
			h.H2(g.Text("Git Operations")),
		),

		h.Div(h.Class("git-folder-selector"),
			h.Form(h.Class("inline-form"),
				h.Div(h.Class("form-group"),
					h.Label(h.For("git-folder"), g.Text("Repository Path:")),
					h.Input(
						h.Type("text"),
						h.ID("git-folder"),
						h.Name("folder"),
						h.Placeholder("Enter repository path (e.g., /path/to/repo)"),
						h.Value("."),
						h.Class("form-control"),
					),
				),
				h.Button(
					h.Type("button"),
					h.Class("btn btn-primary"),
					g.Attr("onclick", "refreshGitDiff(); return false;"),
					g.Text("Load Repository"),
				),
			),
		),

		h.Div(h.ID("git-diff-container"),
			h.Div(h.Class("info"), g.Text("Select a repository path and click 'Load Repository' to view changes")),
		),

		GitCommitForm(),
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

func GitCommitForm() g.Node {
	return h.Div(h.Class("git-commit-form"),
		h.Form(
			h.ID("git-commit-form"),
			g.Attr("onsubmit", "submitGitCommit(event); return false;"),

			h.Div(h.Class("form-group"),
				h.Label(h.For("commit-message"), g.Text("Commit Message")),
				h.Textarea(
					h.ID("commit-message"),
					h.Name("message"),
					h.Rows("3"),
					h.Required(),
					h.Placeholder("Enter commit message..."),
				),
			),

			h.Div(h.Class("form-group checkbox-group"),
				h.Label(
					h.Input(
						h.Type("checkbox"),
						h.Name("push"),
						h.Value("true"),
						h.Checked(),
					),
					g.Text(" Push to remote after commit"),
				),
			),

			h.Div(h.Class("form-actions"),
				h.Button(
					h.Type("submit"),
					h.Class("btn btn-primary"),
					g.Text("Commit & Push"),
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
		g.If(success,
			h.Script(g.Raw(`
				setTimeout(() => {
					refreshGitDiff();
					document.getElementById('commit-message').value = '';
				}, 1000);
			`)),
		),
	)
}
