package web

import (
	"fmt"
	"net/http"
	
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

func FlashMessageComponent(flash *FlashMessage) g.Node {
	if flash == nil {
		return nil
	}
	
	return h.Div(
		h.Class(fmt.Sprintf("notification %s", flash.Type)),
		g.Text(flash.Message),
	)
}

func DashboardLayout(w http.ResponseWriter, r *http.Request, children ...g.Node) g.Node {
	return DashboardLayoutWithRefresh(w, r, children, true)
}

func DashboardLayoutNoRefresh(w http.ResponseWriter, r *http.Request, children ...g.Node) g.Node {
	return DashboardLayoutWithRefresh(w, r, children, false)
}

func DashboardLayoutWithRefresh(w http.ResponseWriter, r *http.Request, children []g.Node, autoRefresh bool) g.Node {
	headNodes := []g.Node{
		h.Meta(h.Charset("UTF-8")),
		h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1.0")),
		h.Link(h.Rel("stylesheet"), h.Href("/static/css/minimal.css")),
	}

	// Add meta refresh tag for auto-refresh every 5 seconds
	if autoRefresh {
		headNodes = append(headNodes, h.Meta(g.Attr("http-equiv", "refresh"), h.Content("5")))
	}

	// Get flash message
	flash := GetFlash(w, r)

	return c.HTML5(
		c.HTML5Props{
			Title:    "Mavis Dashboard",
			Language: "en",
			Head:     headNodes,
			Body: []g.Node{
				h.Div(h.Class("navbar"),
					h.Div(h.Class("navbar-brand"),
						h.H1(g.Text("Mavis")),
						// Souls pause/resume toggle
						h.Div(h.Class("souls-pause-toggle"),
							h.Form(h.Action("/api/souls/toggle-pause"), h.Method("POST"), h.ID("souls-pause-form"),
								h.Button(
									h.Type("submit"),
									h.Class(fmt.Sprintf("toggle-button %s", map[bool]string{true: "paused", false: "active"}[soulManager.IsPaused()])),
									h.Title(fmt.Sprintf("Click to %s souls", map[bool]string{true: "resume", false: "pause"}[soulManager.IsPaused()])),
									g.Text(fmt.Sprintf("Souls: %s", map[bool]string{true: "⏸ Paused", false: "▶ Running"}[soulManager.IsPaused()])),
								),
							),
						),
					),
					h.Nav(h.Class("navbar-menu"),
						h.A(h.Href("/agents"), h.Class("navbar-item"), g.Text("Agents")),
						h.A(h.Href("/souls"), h.Class("navbar-item"), g.Text("Souls")),
						h.A(h.Href("/files"), h.Class("navbar-item"), g.Text("Files")),
						h.A(h.Href("/git"), h.Class("navbar-item"), g.Text("Git")),
						h.A(h.Href("/mcps"), h.Class("navbar-item"), g.Text("MCPs")),
						h.A(h.Href("/system"), h.Class("navbar-item"), g.Text("System")),
					),
				),
				h.Div(h.Class("main-content"),
					// Add flash message if present
					g.If(flash != nil,
						h.Div(h.Class("container"),
							FlashMessageComponent(flash),
						),
					),
					h.Main(h.ID("main-content"), h.Class("section"), g.Group(children)),
				),
				// JavaScript removed - all functionality is server-side now
			},
		},
	)
}
