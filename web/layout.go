package web

import (
	g "maragu.dev/gomponents"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

func Layout(title string, children ...g.Node) g.Node {
	return c.HTML5(
		c.HTML5Props{
			Title:    title,
			Language: "en",
			Head: []g.Node{
				h.Meta(h.Charset("UTF-8")),
				h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1.0")),
				h.Link(h.Rel("stylesheet"), h.Href("/static/css/minimal.css")),
			},
			Body: children,
		},
	)
}

func DashboardLayout(children ...g.Node) g.Node {
	return DashboardLayoutWithRefresh(children, true)
}

func DashboardLayoutNoRefresh(children ...g.Node) g.Node {
	return DashboardLayoutWithRefresh(children, false)
}

func DashboardLayoutWithRefresh(children []g.Node, autoRefresh bool) g.Node {
	headNodes := []g.Node{
		h.Meta(h.Charset("UTF-8")),
		h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1.0")),
		h.Link(h.Rel("stylesheet"), h.Href("/static/css/minimal.css")),
	}

	// Add meta refresh tag for auto-refresh every 5 seconds
	if autoRefresh {
		headNodes = append(headNodes, h.Meta(g.Attr("http-equiv", "refresh"), h.Content("5")))
	}

	return c.HTML5(
		c.HTML5Props{
			Title:    "Mavis Dashboard",
			Language: "en",
			Head:     headNodes,
			Body: []g.Node{
				h.Div(h.Class("navbar"),
					h.Div(h.Class("navbar-brand"),
						h.H1(g.Text("Mavis")),
					),
					h.Nav(h.Class("navbar-menu"),
						h.A(h.Href("/agents"), h.Class("navbar-item"), g.Text("Agents")),
						h.A(h.Href("/files"), h.Class("navbar-item"), g.Text("Files")),
						h.A(h.Href("/git"), h.Class("navbar-item"), g.Text("Git")),
						h.A(h.Href("/system"), h.Class("navbar-item"), g.Text("System")),
					),
				),
				h.Div(h.Class("main-content"),
					h.Main(h.ID("main-content"), h.Class("section"), g.Group(children)),
				),
				// Minimal JavaScript for scroll position preservation only
				h.Script(h.Src("/static/js/minimal.js")),
			},
		},
	)
}
