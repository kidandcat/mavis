package web

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func SystemControlsForm() g.Node {
	return h.Form(
		h.Method("post"),
		h.Action("/api/system/restart"),
		h.Button(
			h.Type("submit"),
			h.Class("btn btn-danger"),
			g.Text("Restart Mavis"),
		),
		h.Div(h.ID("restart-message"), h.Class("message")),
	)
}

func SystemSection() g.Node {
	return h.Div(h.ID("system-section"), h.Class("section"),
		h.Div(h.Class("section-header"),
			h.H2(g.Text("System Management")),
		),

		h.Div(h.Class("subsection"),
			h.H3(g.Text("System Controls")),
			SystemControlsForm(),
		),

		h.Div(h.Class("subsection"),
			h.H3(g.Text("Run Command")),
			CommandForm(),
		),

		h.Div(h.Class("subsection"),
			h.H3(g.Text("Upload Image")),
			ImageUploadForm(),
		),
	)
}

func CommandForm() g.Node {
	return h.Form(
		h.ID("command-form"),
		h.Method("post"),
		h.Action("/api/command/run"),

		h.Div(h.Class("form-group"),
			h.Input(
				h.Type("text"),
				h.Name("command"),
				h.Placeholder("Enter command to run..."),
				h.Required(),
			),
		),

		h.Button(
			h.Type("submit"),
			h.Class("btn btn-primary"),
			g.Text("Run Command"),
		),

		h.Div(h.ID("command-output"), h.Class("terminal-output")),
	)
}

func CommandOutput(output string, success bool) g.Node {
	class := "error"
	if success {
		class = "success"
	}
	return h.Pre(
		h.Class(class),
		g.Text(output),
	)
}

func ImageUploadForm() g.Node {
	return h.Form(
		g.Attr("enctype", "multipart/form-data"),
		h.ID("image-upload-form"),
		h.Method("post"),
		h.Action("/api/images"),

		h.Div(h.Class("form-group"),
			h.Input(
				h.Type("file"),
				h.Name("image"),
				h.Accept("image/*"),
				h.Required(),
			),
		),

		h.Button(
			h.Type("submit"),
			h.Class("btn btn-primary"),
			g.Text("Upload Image"),
		),

		h.Div(h.ID("upload-result")),
	)
}

func ImageUploadResult(success bool, message string, imageURL string) g.Node {
	class := "error"
	if success {
		class = "success"
	}

	return h.Div(
		h.Class("upload-result "+class),
		h.P(g.Text(message)),
		g.If(success && imageURL != "",
			h.Div(
				h.Img(h.Src(imageURL), h.Alt("Uploaded image"), h.Style("max-width: 300px;")),
				h.P(h.A(h.Href(imageURL), h.Target("_blank"), g.Text("View full size"))),
			),
		),
	)
}