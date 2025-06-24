// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package web

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// FileServer serves files from a directory with directory listing
type FileServer struct {
	root string
}

// NewFileServer creates a new file server
func NewFileServer(root string) *FileServer {
	// Clean the root path to ensure it's absolute and normalized
	absRoot, err := filepath.Abs(root)
	if err != nil {
		// If we can't get absolute path, use the original
		absRoot = root
	}
	return &FileServer{root: absRoot}
}

// ServeHTTP handles HTTP requests
func (fs *FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Clean the URL path
	urlPath := r.URL.Path
	if urlPath == "" {
		urlPath = "/"
	}

	// Log the request for debugging
	log.Printf("FileServer: Request for URL path: %s", urlPath)

	// Convert URL path to filesystem path
	fsPath := filepath.Join(fs.root, filepath.FromSlash(urlPath))

	// Log the resolved filesystem path
	log.Printf("FileServer: Resolved to filesystem path: %s (root: %s)", fsPath, fs.root)

	// Ensure fs.root ends with separator for proper prefix checking
	rootWithSep := fs.root
	if !strings.HasSuffix(rootWithSep, string(filepath.Separator)) {
		rootWithSep += string(filepath.Separator)
	}

	// Prevent directory traversal attacks
	if !strings.HasPrefix(fsPath, fs.root) && fsPath != fs.root {
		log.Printf("FileServer: Forbidden - path %s is outside root %s", fsPath, fs.root)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Get file info
	info, err := os.Stat(fsPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("FileServer: Not Found - path %s does not exist", fsPath)
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}
		log.Printf("FileServer: Error stating path %s: %v", fsPath, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// If it's a directory, serve directory listing
	if info.IsDir() {
		fs.serveDirectory(w, r, fsPath, urlPath)
		return
	}

	// Serve the file
	fs.serveFile(w, r, fsPath)
}

// serveFile serves a single file with support for range requests
func (fs *FileServer) serveFile(w http.ResponseWriter, r *http.Request, path string) {
	file, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Get file info
	info, err := file.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Set content type based on file extension
	ext := filepath.Ext(path)
	contentType := getContentType(ext)
	w.Header().Set("Content-Type", contentType)

	// Set cache headers
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Last-Modified", info.ModTime().UTC().Format(http.TimeFormat))

	// Handle range requests
	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		// Parse range header
		ranges, err := parseRangeHeader(rangeHeader, info.Size())
		if err != nil || len(ranges) != 1 {
			// Invalid range
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", info.Size()))
			http.Error(w, "Requested Range Not Satisfiable", http.StatusRequestedRangeNotSatisfiable)
			return
		}

		// Serve partial content
		start := ranges[0].start
		end := ranges[0].end
		length := end - start + 1

		// Seek to start position
		_, err = file.Seek(start, 0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Set partial content headers
		w.Header().Set("Content-Length", fmt.Sprintf("%d", length))
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, info.Size()))
		w.WriteHeader(http.StatusPartialContent)

		// Copy partial content with buffer
		buf := make([]byte, 32*1024) // 32KB buffer
		copied := int64(0)
		for copied < length {
			toRead := int64(len(buf))
			if length-copied < toRead {
				toRead = length - copied
			}
			n, err := file.Read(buf[:toRead])
			if err != nil && err != io.EOF {
				return
			}
			if n == 0 {
				break
			}
			_, err = w.Write(buf[:n])
			if err != nil {
				return
			}
			copied += int64(n)
		}
	} else {
		// Serve full file
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))

		// Use buffered copy for better performance
		buf := make([]byte, 32*1024) // 32KB buffer
		_, err = io.CopyBuffer(w, file, buf)
		if err != nil {
			// Log error but don't send another response
			log.Printf("Error copying file: %v", err)
		}
	}
}

// serveDirectory serves a directory listing
func (fs *FileServer) serveDirectory(w http.ResponseWriter, r *http.Request, fsPath, urlPath string) {
	// Read directory
	entries, err := os.ReadDir(fsPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure URL path ends with /
	if !strings.HasSuffix(urlPath, "/") {
		http.Redirect(w, r, urlPath+"/", http.StatusMovedPermanently)
		return
	}

	// Prepare file list
	type FileInfo struct {
		Name    string
		IsDir   bool
		Size    string
		ModTime string
		URL     string
	}

	var files []FileInfo

	// Add parent directory link if not at root
	if urlPath != "/" {
		files = append(files, FileInfo{
			Name:  "..",
			IsDir: true,
			URL:   filepath.Dir(strings.TrimSuffix(urlPath, "/")),
		})
	}

	// Process entries
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fileInfo := FileInfo{
			Name:    entry.Name(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime().Format("2006-01-02 15:04:05"),
			URL:     filepath.Join(urlPath, entry.Name()),
		}

		if entry.IsDir() {
			fileInfo.Name += "/"
			fileInfo.Size = "-"
		} else {
			fileInfo.Size = formatSize(info.Size())
		}

		files = append(files, fileInfo)
	}

	// Sort files (directories first, then by name)
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return files[i].Name < files[j].Name
	})

	// Render HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	tmpl := `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Index of {{.Path}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            margin: 20px;
            line-height: 1.6;
        }
        h1 {
            color: #333;
            border-bottom: 2px solid #007bff;
            padding-bottom: 10px;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 20px;
        }
        th {
            background-color: #f8f9fa;
            text-align: left;
            padding: 10px;
            border-bottom: 2px solid #dee2e6;
        }
        td {
            padding: 8px 10px;
            border-bottom: 1px solid #dee2e6;
        }
        tr:hover {
            background-color: #f8f9fa;
        }
        a {
            color: #007bff;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
        .dir {
            font-weight: bold;
        }
        .size, .date {
            color: #666;
        }
        .footer {
            margin-top: 40px;
            padding-top: 20px;
            border-top: 1px solid #dee2e6;
            color: #666;
            font-size: 0.9em;
        }
    </style>
</head>
<body>
    <h1>Index of {{.Path}}</h1>
    <table>
        <thead>
            <tr>
                <th>Name</th>
                <th>Size</th>
                <th>Modified</th>
            </tr>
        </thead>
        <tbody>
            {{range .Files}}
            <tr>
                <td>
                    <a href="{{.URL}}" {{if .IsDir}}class="dir"{{end}}>{{.Name}}</a>
                </td>
                <td class="size">{{.Size}}</td>
                <td class="date">{{.ModTime}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    <div class="footer">
        Mavis File Server
    </div>
</body>
</html>`

	t := template.Must(template.New("listing").Parse(tmpl))
	err = t.Execute(w, map[string]interface{}{
		"Path":  urlPath,
		"Files": files,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// httpRange represents a single range in a range request
type httpRange struct {
	start, end int64
}

// parseRangeHeader parses the Range header and returns the ranges
func parseRangeHeader(rangeHeader string, fileSize int64) ([]httpRange, error) {
	if !strings.HasPrefix(rangeHeader, "bytes=") {
		return nil, fmt.Errorf("invalid range header")
	}

	rangeSpec := rangeHeader[6:] // Remove "bytes="
	ranges := []httpRange{}

	for _, rangeStr := range strings.Split(rangeSpec, ",") {
		rangeStr = strings.TrimSpace(rangeStr)
		if rangeStr == "" {
			continue
		}

		var start, end int64
		var err error

		if strings.HasPrefix(rangeStr, "-") {
			// Suffix range: last N bytes
			end = fileSize - 1
			start, err = strconv.ParseInt(rangeStr[1:], 10, 64)
			if err != nil {
				return nil, err
			}
			start = fileSize - start
			if start < 0 {
				start = 0
			}
		} else if strings.HasSuffix(rangeStr, "-") {
			// Prefix range: from start to end of file
			start, err = strconv.ParseInt(rangeStr[:len(rangeStr)-1], 10, 64)
			if err != nil {
				return nil, err
			}
			end = fileSize - 1
		} else {
			// Full range: start-end
			parts := strings.Split(rangeStr, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid range")
			}
			start, err = strconv.ParseInt(parts[0], 10, 64)
			if err != nil {
				return nil, err
			}
			end, err = strconv.ParseInt(parts[1], 10, 64)
			if err != nil {
				return nil, err
			}
		}

		// Validate range
		if start < 0 || end < 0 || start > end || start >= fileSize {
			return nil, fmt.Errorf("invalid range")
		}
		if end >= fileSize {
			end = fileSize - 1
		}

		ranges = append(ranges, httpRange{start: start, end: end})
	}

	if len(ranges) == 0 {
		return nil, fmt.Errorf("no valid ranges")
	}

	return ranges, nil
}

// formatSize formats file size in human-readable format
func formatSize(size int64) string {
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

// getContentType returns content type based on file extension
func getContentType(ext string) string {
	switch strings.ToLower(ext) {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".gz":
		return "application/gzip"
	case ".tar":
		return "application/x-tar"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".xml":
		return "text/xml; charset=utf-8"
	case ".go":
		return "text/plain; charset=utf-8"
	case ".py":
		return "text/plain; charset=utf-8"
	case ".java":
		return "text/plain; charset=utf-8"
	case ".c", ".cpp", ".h", ".hpp":
		return "text/plain; charset=utf-8"
	case ".md":
		return "text/markdown; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// StartFileServer starts the file server on the specified port
func StartFileServer(root string, port string) (*http.Server, error) {
	// Create file server
	fs := NewFileServer(root)

	log.Printf("StartFileServer: Starting file server on port %s, serving directory: %s", port, fs.root)

	// Create HTTP server with longer timeouts for large files
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      fs,
		ReadTimeout:  5 * time.Minute,  // Increased from 10 seconds
		WriteTimeout: 30 * time.Minute, // Increased to allow large file downloads
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("StartFileServer: Listening on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Server failed to start or crashed
			log.Printf("StartFileServer: Server error: %v", err)
		}
	}()

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	return server, nil
}
