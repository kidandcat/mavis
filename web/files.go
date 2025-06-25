package web

import (
	"fmt"
	"path/filepath"
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type FileInfo struct {
	Name  string
	IsDir bool
	Size  int64
	Mode  string
}

func FilesSection(currentPath string, files []FileInfo) g.Node {
	breadcrumbs := generateBreadcrumbs(currentPath)

	return h.Div(h.ID("files-section"), h.Class("section"),
		h.Div(h.Class("section-header"),
			h.H2(g.Text("File Browser")),
		),

		h.Div(h.Class("file-browser"),
			h.Div(h.Class("file-path"),
				g.Group(g.Map(breadcrumbs, func(crumb breadcrumb) g.Node {
					if crumb.IsLast {
						return h.Span(g.Text(crumb.Name))
					}
					return g.Group([]g.Node{
						h.A(
							h.Href("/files?path="+crumb.Path),
							g.Text(crumb.Name),
						),
						h.Span(g.Text(" / ")),
					})
				})),
			),

			h.Table(h.Class("file-list"),
				h.THead(
					h.Tr(
						h.Th(g.Text("Name")),
						h.Th(g.Text("Size")),
						h.Th(g.Text("Permissions")),
						h.Th(g.Text("Actions")),
					),
				),
				h.TBody(
					g.If(currentPath != "/",
						h.Tr(
							h.Td(
								h.A(
									h.Href("/files?path="+filepath.Dir(currentPath)),
									g.Text(".."),
								),
							),
							h.Td(g.Text("")),
							h.Td(g.Text("")),
							h.Td(g.Text("")),
						),
					),
					g.Group(g.Map(files, func(file FileInfo) g.Node {
						return FileRow(currentPath, file)
					})),
				),
			),
		),
	)
}

func FileRow(currentPath string, file FileInfo) g.Node {
	fullPath := filepath.Join(currentPath, file.Name)

	return h.Tr(
		h.Td(
			g.If(file.IsDir,
				h.A(
					h.Class("file-dir"),
					h.Href(fmt.Sprintf("/files?path=%s", fullPath)),
					g.Text(file.Name+"/"),
				),
			),
			g.If(!file.IsDir,
				h.Span(h.Class("file-name"), g.Text(file.Name)),
			),
		),
		h.Td(g.Text(formatFileSize(file.Size))),
		h.Td(h.Class("file-mode"), g.Text(file.Mode)),
		h.Td(
			g.If(!file.IsDir,
				h.A(
					h.Class("btn btn-sm"),
					h.Href(fmt.Sprintf("/api/files/download?path=%s", fullPath)),
					h.Download(file.Name),
					g.Text("Download"),
				),
			),
		),
	)
}

type breadcrumb struct {
	Name   string
	Path   string
	IsLast bool
}

func generateBreadcrumbs(path string) []breadcrumb {
	if path == "/" || path == "" {
		return []breadcrumb{{Name: "Root", Path: "/", IsLast: true}}
	}

	parts := strings.Split(strings.Trim(path, "/"), "/")
	crumbs := []breadcrumb{{Name: "Root", Path: "/", IsLast: false}}

	currentPath := ""
	for i, part := range parts {
		currentPath = filepath.Join(currentPath, part)
		crumbs = append(crumbs, breadcrumb{
			Name:   part,
			Path:   "/" + currentPath,
			IsLast: i == len(parts)-1,
		})
	}

	return crumbs
}

func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
