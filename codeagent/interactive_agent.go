// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package codeagent

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
	
	"github.com/creack/pty"
	"mavis/core"
)

// ansiToHTML converts ANSI escape sequences to HTML
func ansiToHTML(input string) string {
	// Pattern to match ANSI escape sequences
	ansiPattern := regexp.MustCompile(`\x1b\[([0-9;]+)m`)
	
	var result strings.Builder
	lastIndex := 0
	openSpan := false
	
	for _, match := range ansiPattern.FindAllStringSubmatchIndex(input, -1) {
		// Add text before the match
		if match[0] > lastIndex {
			result.WriteString(escapeHTML(input[lastIndex:match[0]]))
		}
		
		// Extract the ANSI codes
		codes := input[match[2]:match[3]]
		
		// Close previous span if open
		if openSpan {
			result.WriteString("</span>")
			openSpan = false
		}
		
		// Convert ANSI codes to CSS
		style := ansiCodesToCSS(codes)
		if style != "" {
			result.WriteString(`<span style="` + style + `">`)
			openSpan = true
		}
		
		lastIndex = match[1]
	}
	
	// Add remaining text
	if lastIndex < len(input) {
		result.WriteString(escapeHTML(input[lastIndex:]))
	}
	
	// Close final span if open
	if openSpan {
		result.WriteString("</span>")
	}
	
	return result.String()
}

// escapeHTML escapes HTML special characters
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}

// ansiCodesToCSS converts ANSI codes to CSS styles
func ansiCodesToCSS(codes string) string {
	parts := strings.Split(codes, ";")
	var styles []string
	
	for i := 0; i < len(parts); i++ {
		code := parts[i]
		switch code {
		case "0": // Reset
			return ""
		case "1": // Bold
			styles = append(styles, "font-weight: bold")
		case "3": // Italic
			styles = append(styles, "font-style: italic")
		case "4": // Underline
			styles = append(styles, "text-decoration: underline")
		// Foreground colors
		case "30":
			styles = append(styles, "color: #000000")
		case "31":
			styles = append(styles, "color: #cc0000")
		case "32":
			styles = append(styles, "color: #4e9a06")
		case "33":
			styles = append(styles, "color: #c4a000")
		case "34":
			styles = append(styles, "color: #3465a4")
		case "35":
			styles = append(styles, "color: #75507b")
		case "36":
			styles = append(styles, "color: #06989a")
		case "37":
			styles = append(styles, "color: #d3d7cf")
		// Bright foreground colors
		case "90":
			styles = append(styles, "color: #555753")
		case "91":
			styles = append(styles, "color: #ef2929")
		case "92":
			styles = append(styles, "color: #8ae234")
		case "93":
			styles = append(styles, "color: #fce94f")
		case "94":
			styles = append(styles, "color: #729fcf")
		case "95":
			styles = append(styles, "color: #ad7fa8")
		case "96":
			styles = append(styles, "color: #34e2e2")
		case "97":
			styles = append(styles, "color: #eeeeec")
		// Background colors
		case "40":
			styles = append(styles, "background-color: #000000")
		case "41":
			styles = append(styles, "background-color: #cc0000")
		case "42":
			styles = append(styles, "background-color: #4e9a06")
		case "43":
			styles = append(styles, "background-color: #c4a000")
		case "44":
			styles = append(styles, "background-color: #3465a4")
		case "45":
			styles = append(styles, "background-color: #75507b")
		case "46":
			styles = append(styles, "background-color: #06989a")
		case "47":
			styles = append(styles, "background-color: #d3d7cf")
		// 256 color mode
		case "38":
			if i+2 < len(parts) && parts[i+1] == "5" {
				colorIndex := parts[i+2]
				if color := ansi256ToHex(colorIndex); color != "" {
					styles = append(styles, "color: "+color)
				}
				i += 2
			} else if i+4 < len(parts) && parts[i+1] == "2" {
				// True color (24-bit)
				r, g, b := parts[i+2], parts[i+3], parts[i+4]
				styles = append(styles, fmt.Sprintf("color: rgb(%s, %s, %s)", r, g, b))
				i += 4
			}
		case "48":
			if i+2 < len(parts) && parts[i+1] == "5" {
				colorIndex := parts[i+2]
				if color := ansi256ToHex(colorIndex); color != "" {
					styles = append(styles, "background-color: "+color)
				}
				i += 2
			} else if i+4 < len(parts) && parts[i+1] == "2" {
				// True color (24-bit)
				r, g, b := parts[i+2], parts[i+3], parts[i+4]
				styles = append(styles, fmt.Sprintf("background-color: rgb(%s, %s, %s)", r, g, b))
				i += 4
			}
		}
	}
	
	return strings.Join(styles, "; ")
}

// ansi256ToHex converts 256 color index to hex
func ansi256ToHex(index string) string {
	// This is a simplified version - you could expand with full 256 color palette
	colorMap := map[string]string{
		"0": "#000000", "1": "#800000", "2": "#008000", "3": "#808000",
		"4": "#000080", "5": "#800080", "6": "#008080", "7": "#c0c0c0",
		"8": "#808080", "9": "#ff0000", "10": "#00ff00", "11": "#ffff00",
		"12": "#0000ff", "13": "#ff00ff", "14": "#00ffff", "15": "#ffffff",
	}
	if color, ok := colorMap[index]; ok {
		return color
	}
	return ""
}

// TerminalBuffer simulates a simple terminal buffer for handling cursor movements
type TerminalBuffer struct {
	screen      []string // Fixed size screen buffer
	currentRow  int
	currentCol  int
	width       int
	height      int
	savedRow    int // For save/restore cursor
	savedCol    int
}

// NewTerminalBuffer creates a new terminal buffer
func NewTerminalBuffer(width, height int) *TerminalBuffer {
	screen := make([]string, height)
	// Initialize with empty lines
	for i := range screen {
		screen[i] = ""
	}
	return &TerminalBuffer{
		screen:     screen,
		currentRow: 0,
		currentCol: 0,
		width:      width,
		height:     height,
	}
}

// ProcessOutput processes terminal output and handles control sequences
func (tb *TerminalBuffer) ProcessOutput(output string) {
	// Convert string to bytes for easier handling
	data := []byte(output)
	i := 0
	
	for i < len(data) {
		// Check for ANSI escape sequences
		if i < len(data)-1 && data[i] == 0x1b { // ESC character
			if data[i+1] == '[' {
				// CSI sequence
				j := i + 2
				// Collect parameters and command
				for j < len(data) && !isTerminalCommand(data[j]) {
					j++
				}
				if j < len(data) {
					params := string(data[i+2:j])
					cmd := data[j]
					tb.processCSISequence(params, cmd)
					i = j + 1
					continue
				}
			} else if data[i+1] == ']' {
				// OSC sequence - skip until ST or BEL
				j := i + 2
				for j < len(data) && data[j] != 0x07 && !(j < len(data)-1 && data[j] == 0x1b && data[j+1] == '\\') {
					j++
				}
				if j < len(data) && data[j] == 0x07 {
					i = j + 1
					continue
				} else if j < len(data)-1 && data[j] == 0x1b && data[j+1] == '\\' {
					i = j + 2
					continue
				}
			} else if data[i+1] == '7' {
				// Save cursor position (DECSC)
				tb.savedRow = tb.currentRow
				tb.savedCol = tb.currentCol
				i += 2
				continue
			} else if data[i+1] == '8' {
				// Restore cursor position (DECRC)
				tb.currentRow = tb.savedRow
				tb.currentCol = tb.savedCol
				i += 2
				continue
			} else if data[i+1] == 'c' {
				// Reset terminal (RIS)
				tb.clearScreen()
				tb.currentRow = 0
				tb.currentCol = 0
				i += 2
				continue
			}
		}
		
		// Handle regular characters
		switch data[i] {
		case '\n':
			tb.currentRow++
			tb.currentCol = 0
			if tb.currentRow >= tb.height {
				// Scroll up
				tb.scrollUp()
				tb.currentRow = tb.height - 1
			}
		case '\r':
			tb.currentCol = 0
			// Carriage return often precedes overwriting the current line
		case '\b':
			if tb.currentCol > 0 {
				tb.currentCol--
			}
		case '\t':
			// Move to next tab stop (every 8 columns)
			tb.currentCol = ((tb.currentCol / 8) + 1) * 8
			if tb.currentCol >= tb.width {
				tb.currentCol = tb.width - 1
			}
		case 0x07: // BEL - ignore bell character
			// Do nothing
		default:
			// Regular character - only print if it's printable
			if data[i] >= 32 || data[i] >= 128 {
				// Handle UTF-8 multi-byte sequences
				ch, size := utf8.DecodeRune(data[i:])
				if ch != utf8.RuneError && tb.currentRow < tb.height {
					// Write character at current position
					if tb.currentCol < tb.width {
						tb.setChar(tb.currentRow, tb.currentCol, ch)
						tb.currentCol++
					}
					// Handle line wrap
					if tb.currentCol >= tb.width {
						tb.currentCol = 0
						tb.currentRow++
						if tb.currentRow >= tb.height {
							tb.scrollUp()
							tb.currentRow = tb.height - 1
						}
					}
				}
				// Skip additional bytes of multi-byte sequence
				if size > 1 {
					i += size - 1
				}
			}
		}
		i++
	}
}

// processCSISequence handles CSI (Control Sequence Introducer) sequences
func (tb *TerminalBuffer) processCSISequence(params string, cmd byte) {
	// Parse numeric parameters
	values := []int{}
	if params != "" {
		parts := strings.Split(params, ";")
		for _, part := range parts {
			if v, err := strconv.Atoi(part); err == nil {
				values = append(values, v)
			} else {
				values = append(values, 0)
			}
		}
	}
	
	// Default value for single parameter commands
	n := 1
	if len(values) > 0 && values[0] > 0 {
		n = values[0]
	}
	
	switch cmd {
	case 'A': // Cursor up
		tb.currentRow = max(0, tb.currentRow-n)
	case 'B': // Cursor down
		tb.currentRow = min(tb.height-1, tb.currentRow+n)
	case 'C': // Cursor forward
		tb.currentCol = min(tb.width-1, tb.currentCol+n)
	case 'D': // Cursor back
		tb.currentCol = max(0, tb.currentCol-n)
	case 'E': // Cursor next line
		tb.currentRow = min(tb.height-1, tb.currentRow+n)
		tb.currentCol = 0
	case 'F': // Cursor previous line
		tb.currentRow = max(0, tb.currentRow-n)
		tb.currentCol = 0
	case 'G': // Cursor horizontal absolute
		tb.currentCol = max(0, min(tb.width-1, n-1))
	case 'H', 'f': // Cursor position
		row := 1
		col := 1
		if len(values) > 0 {
			row = values[0]
		}
		if len(values) > 1 {
			col = values[1]
		}
		tb.currentRow = max(0, min(tb.height-1, row-1))
		tb.currentCol = max(0, min(tb.width-1, col-1))
	case 'J': // Erase display
		switch n {
		case 0: // Clear from cursor to end
			tb.clearFromCursor()
		case 1: // Clear from start to cursor
			tb.clearToCursor()
		case 2, 3: // Clear entire screen
			tb.clearScreen()
			// After clearing, cursor typically goes to home position
			tb.currentRow = 0
			tb.currentCol = 0
		}
	case 'K': // Erase line
		switch n {
		case 0: // Clear from cursor to end of line
			tb.clearLineFrom(tb.currentRow, tb.currentCol)
		case 1: // Clear from start of line to cursor
			tb.clearLineTo(tb.currentRow, tb.currentCol)
		case 2: // Clear entire line
			tb.screen[tb.currentRow] = ""
		}
	case 's': // Save cursor position
		tb.savedRow = tb.currentRow
		tb.savedCol = tb.currentCol
	case 'u': // Restore cursor position
		tb.currentRow = tb.savedRow
		tb.currentCol = tb.savedCol
	}
}

// Helper methods for terminal operations
func (tb *TerminalBuffer) setChar(row, col int, ch rune) {
	if row >= 0 && row < tb.height && col >= 0 && col < tb.width {
		line := []rune(tb.screen[row])
		// Extend line with spaces if necessary
		for len(line) <= col {
			line = append(line, ' ')
		}
		// Ensure line is at least width characters for proper overwriting
		for len(line) < tb.width {
			line = append(line, ' ')
		}
		line[col] = ch
		// Trim trailing spaces but keep the line at least as long as the cursor position
		endPos := len(line) - 1
		for endPos > col && line[endPos] == ' ' {
			endPos--
		}
		tb.screen[row] = string(line[:endPos+1])
	}
}

func (tb *TerminalBuffer) clearScreen() {
	for i := range tb.screen {
		tb.screen[i] = ""
	}
}

func (tb *TerminalBuffer) clearFromCursor() {
	// Clear from cursor to end of current line
	tb.clearLineFrom(tb.currentRow, tb.currentCol)
	// Clear all lines below
	for i := tb.currentRow + 1; i < tb.height; i++ {
		tb.screen[i] = ""
	}
}

func (tb *TerminalBuffer) clearToCursor() {
	// Clear all lines above
	for i := 0; i < tb.currentRow; i++ {
		tb.screen[i] = ""
	}
	// Clear from start of current line to cursor
	tb.clearLineTo(tb.currentRow, tb.currentCol)
}

func (tb *TerminalBuffer) clearLineFrom(row, col int) {
	if row >= 0 && row < tb.height {
		line := []rune(tb.screen[row])
		// Ensure line is long enough
		for len(line) < col {
			line = append(line, ' ')
		}
		// Clear from col to end by filling with spaces
		if col < len(line) {
			for i := col; i < len(line); i++ {
				line[i] = ' '
			}
		}
		// Trim trailing spaces
		endPos := len(line) - 1
		for endPos >= 0 && line[endPos] == ' ' {
			endPos--
		}
		if endPos >= 0 {
			tb.screen[row] = string(line[:endPos+1])
		} else {
			tb.screen[row] = ""
		}
	}
}

func (tb *TerminalBuffer) clearLineTo(row, col int) {
	if row >= 0 && row < tb.height {
		line := tb.screen[row]
		if col < len(line) {
			tb.screen[row] = strings.Repeat(" ", col) + line[col:]
		}
	}
}

func (tb *TerminalBuffer) scrollUp() {
	// Move all lines up by one
	copy(tb.screen[0:], tb.screen[1:])
	tb.screen[tb.height-1] = ""
}

// GetScreenLines returns the current screen content
func (tb *TerminalBuffer) GetScreenLines() []string {
	result := make([]string, tb.height)
	for i, line := range tb.screen {
		// Trim trailing spaces but preserve line
		result[i] = strings.TrimRight(line, " ")
	}
	return result
}

// isTerminalCommand checks if a byte is a terminal command character
func isTerminalCommand(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// max returns the maximum of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// InteractiveAgent represents an interactive Claude session
type InteractiveAgent struct {
	ID           string
	Folder       string
	Status       string // "running", "finished", "failed", "killed"
	StartTime    time.Time
	LastActive   time.Time
	Error        string
	CreatedBy    string
	
	// Process management
	cmd          *exec.Cmd
	ptmx         *os.File      // PTY master
	cancel       context.CancelFunc
	
	// Output streaming
	outputBuffer []string
	outputMutex  sync.RWMutex
	subscribers  map[string]chan string
	subMutex     sync.RWMutex
	
	// Terminal emulation
	termBuffer   *TerminalBuffer
	termMutex    sync.RWMutex
}

// NewInteractiveAgent creates a new interactive agent
func NewInteractiveAgent(folder string, mcpConfig string) *InteractiveAgent {
	return &InteractiveAgent{
		ID:          core.NewID(8),
		Folder:      folder,
		Status:      "pending",
		StartTime:   time.Now(),
		LastActive:  time.Now(),
		subscribers: make(map[string]chan string),
		termBuffer:  NewTerminalBuffer(80, 40), // Larger height for better display
	}
}

// Start launches the interactive Claude session
func (ia *InteractiveAgent) Start(ctx context.Context, mcpConfig string) error {
	log.Printf("[InteractiveAgent %s] Starting interactive session in folder: %s", ia.ID, ia.Folder)
	ia.Status = "running"
	ia.LastActive = time.Now()
	
	// Create command context
	cmdCtx, cancel := context.WithCancel(ctx)
	ia.cancel = cancel
	
	// Build command
	cmdParts := []string{"claude", "--dangerously-skip-permissions"}
	
	// Add MCP config if provided
	if mcpConfig != "" {
		cmdParts = append(cmdParts, "--mcp-config", mcpConfig)
		log.Printf("[InteractiveAgent %s] Using MCP config: %s", ia.ID, mcpConfig)
	}
	
	// Log the full command
	log.Printf("[InteractiveAgent %s] Executing command: %s", ia.ID, strings.Join(cmdParts, " "))
	
	// Create command
	ia.cmd = exec.CommandContext(cmdCtx, cmdParts[0], cmdParts[1:]...)
	ia.cmd.Dir = ia.Folder
	log.Printf("[InteractiveAgent %s] Working directory: %s", ia.ID, ia.Folder)
	
	// Start the process with PTY
	var err error
	ia.ptmx, err = pty.Start(ia.cmd)
	if err != nil {
		ia.Status = "failed"
		// Provide more helpful error messages
		if strings.Contains(err.Error(), "executable file not found") {
			ia.Error = "Claude CLI not found. Please ensure 'claude' is installed and in your PATH."
		} else if strings.Contains(err.Error(), "permission denied") {
			ia.Error = "Permission denied. Please check that you have execute permissions for the Claude CLI."
		} else {
			ia.Error = err.Error()
		}
		log.Printf("[InteractiveAgent %s] Failed to start process with PTY: %v", ia.ID, err)
		return fmt.Errorf("failed to start claude: %w", err)
	}
	
	log.Printf("[InteractiveAgent %s] Process started successfully with PID: %d using PTY", ia.ID, ia.cmd.Process.Pid)
	
	// Set PTY size to reasonable defaults
	if err := pty.Setsize(ia.ptmx, &pty.Winsize{
		Rows: 24,
		Cols: 80,
	}); err != nil {
		log.Printf("[InteractiveAgent %s] Failed to set PTY size: %v", ia.ID, err)
	}
	
	// Start output reader for PTY (single reader for both stdout and stderr)
	go ia.readOutput(ia.ptmx, "pty")
	
	// Monitor process
	go ia.monitorProcess()
	
	return nil
}

// readOutput reads from a pipe and broadcasts to subscribers
func (ia *InteractiveAgent) readOutput(reader io.Reader, source string) {
	log.Printf("[InteractiveAgent %s] Starting to read from %s", ia.ID, source)
	
	// Read and process through terminal emulator
	buf := make([]byte, 4096)
	
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			data := string(buf[:n])
			
			// Process through terminal buffer
			ia.termMutex.Lock()
			ia.termBuffer.ProcessOutput(data)
			
			// Get current screen state
			screenLines := ia.termBuffer.GetScreenLines()
			
			// Process and filter the output
			ia.outputMutex.Lock()
			ia.outputBuffer = ia.filterAndProcessOutput(screenLines)
			ia.outputMutex.Unlock()
			ia.termMutex.Unlock()
			
			// Notify subscribers of update
			ia.broadcastScreenUpdate()
			
			// Update last active time
			ia.LastActive = time.Now()
		}
		
		if err != nil {
			if err != io.EOF {
				log.Printf("[InteractiveAgent %s] Error reading from %s: %v", ia.ID, source, err)
			}
			break
		}
	}
	
	log.Printf("[InteractiveAgent %s] Finished reading from %s", ia.ID, source)
}

// filterAndProcessOutput processes the terminal screen and filters out UI elements
func (ia *InteractiveAgent) filterAndProcessOutput(screenLines []string) []string {
	if len(screenLines) == 0 {
		return []string{}
	}
	
	// Find the token status line (it's usually near the bottom)
	var tokenStatusLine string
	var tokenStatusIndex int = -1
	
	// Search from bottom up for the most recent token status
	for i := len(screenLines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(screenLines[i])
		if trimmed != "" && strings.Contains(trimmed, "tokens") && strings.Contains(trimmed, "esc to interrupt") {
			tokenStatusLine = screenLines[i]
			tokenStatusIndex = i
			break
		}
	}
	
	// Build the filtered output
	var result []string
	var seenContent = make(map[string]bool)
	
	// Process all lines except UI elements
	for i := 0; i < len(screenLines); i++ {
		// Skip the token status line itself (we'll add it at the end)
		if i == tokenStatusIndex {
			continue
		}
		
		line := screenLines[i]
		trimmed := strings.TrimSpace(line)
		
		// Skip empty lines initially
		if trimmed == "" && len(result) == 0 {
			continue
		}
		
		// Skip input box UI elements
		if strings.HasPrefix(trimmed, "╭─") || strings.HasPrefix(trimmed, "│") || 
		   strings.HasPrefix(trimmed, "╰─") || strings.HasPrefix(trimmed, "─╮") ||
		   strings.HasPrefix(trimmed, "─╯") || 
		   (trimmed == ">" || (strings.HasPrefix(trimmed, "> ") && len(trimmed) < 80)) ||
		   strings.Contains(trimmed, "? for shortcuts") || 
		   strings.Contains(trimmed, "Bypassing Permissions") ||
		   strings.Contains(trimmed, "Auto-update failed") {
			continue
		}
		
		// Skip token status lines (we only want the last one)
		if strings.Contains(trimmed, "tokens") && strings.Contains(trimmed, "esc to interrupt") {
			continue
		}
		
		// Check for duplicate content (especially tool lines)
		contentKey := trimmed
		// For tool execution lines, use just the command part as key
		if strings.HasPrefix(trimmed, "⏺") || strings.HasPrefix(trimmed, "✓") || 
		   strings.HasPrefix(trimmed, "✗") || strings.HasPrefix(trimmed, "✢") ||
		   strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "*") {
			// Extract just the command part for deduplication
			if idx := strings.Index(trimmed, "("); idx > 0 {
				endIdx := strings.Index(trimmed[idx:], ")")
				if endIdx > 0 {
					contentKey = trimmed[:idx+endIdx+1]
				}
			}
		}
		
		// Skip if we've seen this exact content before
		if contentKey != "" && seenContent[contentKey] {
			continue
		}
		seenContent[contentKey] = true
		
		// Add the line
		result = append(result, ansiToHTML(line))
	}
	
	// Always show the token status line at the bottom if available
	if tokenStatusLine != "" {
		// Add a separator line
		result = append(result, "")
		result = append(result, "─────────────────────────────────────────────────────────────────────────────")
		result = append(result, "")
		// Add the token status line
		result = append(result, ansiToHTML(tokenStatusLine))
	}
	
	return result
}

// isOnlyCursorMovement checks if a line contains only cursor movement sequences
func isOnlyCursorMovement(line string) bool {
	// Remove all ANSI escape sequences
	cleaned := regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`).ReplaceAllString(line, "")
	// If nothing is left, it was only cursor movements
	return strings.TrimSpace(cleaned) == ""
}

// isRepetitiveUI checks if a line is part of repetitive UI updates
func isRepetitiveUI(line string) bool {
	// Remove ANSI codes for comparison
	cleaned := regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`).ReplaceAllString(line, "")
	trimmed := strings.TrimSpace(cleaned)
	
	// Skip empty lines
	if trimmed == "" {
		return true
	}
	
	// Skip lines that are just the input box borders or empty input
	if strings.HasPrefix(trimmed, "╭─") || strings.HasPrefix(trimmed, "╰─") || 
	   trimmed == "│ >                                                                            │" ||
	   strings.HasPrefix(trimmed, "│ > Try") {
		return true
	}
	
	// Skip standalone UI elements
	if trimmed == "? for shortcuts" || trimmed == "Bypassing Permissions" {
		return true
	}
	
	// Skip lines that ONLY contain these patterns (not mixed with actual content)
	spinnerPatterns := []string{
		"✻", "✶", "✳", "✢", "·", "*", "⏺",
	}
	
	for _, pattern := range spinnerPatterns {
		if strings.HasPrefix(trimmed, pattern + " ") {
			// Check if it's just a spinner with status
			if strings.Contains(trimmed, "Booping…") || 
			   strings.Contains(trimmed, "Puzzling…") || 
			   strings.Contains(trimmed, "Thinking…") ||
			   strings.Contains(trimmed, "esc to interrupt") ||
			   strings.Contains(trimmed, "tokens") {
				return true
			}
		}
	}
	
	// Skip the auto-update failed message
	if strings.Contains(trimmed, "Auto-update failed") && strings.Contains(trimmed, "claude doctor") {
		return true
	}
	
	// Keep everything else (including Claude's actual responses)
	return false
}

// broadcastScreenUpdate notifies subscribers of a screen update
func (ia *InteractiveAgent) broadcastScreenUpdate() {
	// Send a special marker to indicate screen refresh
	ia.subMutex.RLock()
	for _, ch := range ia.subscribers {
		select {
		case ch <- "[[SCREEN_UPDATE]]":
		default:
			// Skip if channel is full
		}
	}
	ia.subMutex.RUnlock()
}

// broadcastCompleteBuffer broadcasts a complete buffer update to subscribers
func (ia *InteractiveAgent) broadcastCompleteBuffer() {
	// Create a special message indicating full buffer update
	ia.outputMutex.RLock()
	bufferCopy := make([]string, len(ia.outputBuffer))
	copy(bufferCopy, ia.outputBuffer)
	ia.outputMutex.RUnlock()
	
	// Send special marker followed by all lines
	ia.subMutex.RLock()
	for _, ch := range ia.subscribers {
		// Send a marker to indicate full buffer update
		select {
		case ch <- "[[BUFFER_UPDATE]]":
		default:
		}
		
		// Send all lines
		for _, line := range bufferCopy {
			select {
			case ch <- line:
			default:
				// Skip if channel is full
			}
		}
		
		// Send end marker
		select {
		case ch <- "[[BUFFER_UPDATE_END]]":
		default:
		}
	}
	ia.subMutex.RUnlock()
}

// Subscribe creates a new subscription channel for output
func (ia *InteractiveAgent) Subscribe() (string, chan string) {
	subID := core.NewID(8)
	ch := make(chan string, 100)
	
	ia.subMutex.Lock()
	ia.subscribers[subID] = ch
	ia.subMutex.Unlock()
	
	log.Printf("[InteractiveAgent %s] New subscriber added: %s (total subscribers: %d)", ia.ID, subID, len(ia.subscribers))
	
	// Send current buffer state to new subscriber
	ia.outputMutex.RLock()
	bufferLen := len(ia.outputBuffer)
	for _, line := range ia.outputBuffer {
		select {
		case ch <- line:
		default:
			break
		}
	}
	ia.outputMutex.RUnlock()
	
	log.Printf("[InteractiveAgent %s] Sent %d buffered lines to subscriber %s", ia.ID, bufferLen, subID)
	
	return subID, ch
}

// Unsubscribe removes a subscription
func (ia *InteractiveAgent) Unsubscribe(subID string) {
	ia.subMutex.Lock()
	if ch, ok := ia.subscribers[subID]; ok {
		close(ch)
		delete(ia.subscribers, subID)
	}
	ia.subMutex.Unlock()
}

// SendInput sends input to the interactive session
func (ia *InteractiveAgent) SendInput(input string) error {
	log.Printf("[InteractiveAgent %s] Attempting to send input: %q (length: %d)", ia.ID, input, len(input))
	
	if ia.Status != "running" {
		log.Printf("[InteractiveAgent %s] Cannot send input - agent status is: %s", ia.ID, ia.Status)
		return fmt.Errorf("agent is not running")
	}
	
	if ia.ptmx == nil {
		log.Printf("[InteractiveAgent %s] Cannot send input - PTY is nil", ia.ID)
		return fmt.Errorf("PTY is not available")
	}
	
	// First, send the input text
	if input != "" {
		n, err := ia.ptmx.Write([]byte(input))
		if err != nil {
			log.Printf("[InteractiveAgent %s] Failed to write input text: %v", ia.ID, err)
			return fmt.Errorf("failed to write input: %w", err)
		}
		log.Printf("[InteractiveAgent %s] Sent input text: %d bytes", ia.ID, n)
		
		// Small delay to let the TUI process the text
		time.Sleep(10 * time.Millisecond)
	}
	
	// Then send Enter key (try carriage return which is standard for Enter in terminals)
	enterKey := []byte{'\r'}
	n2, err := ia.ptmx.Write(enterKey)
	if err != nil {
		log.Printf("[InteractiveAgent %s] Failed to write Enter key: %v", ia.ID, err)
		return fmt.Errorf("failed to write Enter: %w", err)
	}
	
	log.Printf("[InteractiveAgent %s] Sent Enter key: %d bytes", ia.ID, n2)
	ia.LastActive = time.Now()
	return nil
}

// Stop terminates the interactive session
func (ia *InteractiveAgent) Stop() error {
	if ia.cancel != nil {
		ia.cancel()
	}
	
	ia.Status = "killed"
	return nil
}

// monitorProcess monitors the claude process
func (ia *InteractiveAgent) monitorProcess() {
	if ia.cmd == nil {
		log.Printf("[InteractiveAgent %s] No command to monitor", ia.ID)
		return
	}
	
	log.Printf("[InteractiveAgent %s] Starting process monitoring", ia.ID)
	err := ia.cmd.Wait()
	
	// Update status based on exit
	if err != nil {
		if ia.Status == "killed" {
			// Already marked as killed
			log.Printf("[InteractiveAgent %s] Process was killed", ia.ID)
		} else {
			ia.Status = "failed"
			ia.Error = err.Error()
			log.Printf("[InteractiveAgent %s] Process failed with error: %v", ia.ID, err)
		}
	} else {
		ia.Status = "finished"
		log.Printf("[InteractiveAgent %s] Process finished successfully", ia.ID)
	}
	
	// Close PTY
	if ia.ptmx != nil {
		ia.ptmx.Close()
	}
	
	// Close all subscriber channels
	ia.subMutex.Lock()
	for _, ch := range ia.subscribers {
		close(ch)
	}
	ia.subscribers = make(map[string]chan string)
	ia.subMutex.Unlock()
}

// GetOutput returns the current output buffer
func (ia *InteractiveAgent) GetOutput() []string {
	ia.outputMutex.RLock()
	defer ia.outputMutex.RUnlock()
	
	// Return a copy to avoid race conditions
	output := make([]string, len(ia.outputBuffer))
	copy(output, ia.outputBuffer)
	return output
}

// InteractiveAgentManager manages multiple interactive agents
type InteractiveAgentManager struct {
	agents map[string]*InteractiveAgent
	mutex  sync.RWMutex
}

// NewInteractiveAgentManager creates a new manager
func NewInteractiveAgentManager() *InteractiveAgentManager {
	return &InteractiveAgentManager{
		agents: make(map[string]*InteractiveAgent),
	}
}

// CreateAgent creates and starts a new interactive agent
func (iam *InteractiveAgentManager) CreateAgent(ctx context.Context, folder string, mcpConfig string) (*InteractiveAgent, error) {
	log.Printf("[InteractiveAgentManager] Creating new agent for folder: %s", folder)
	
	// Check if folder exists
	if _, err := os.Stat(folder); err != nil {
		log.Printf("[InteractiveAgentManager] Folder does not exist: %s - error: %v", folder, err)
		return nil, fmt.Errorf("folder does not exist: %s", folder)
	}
	
	// Create agent
	agent := NewInteractiveAgent(folder, mcpConfig)
	log.Printf("[InteractiveAgentManager] Created agent with ID: %s", agent.ID)
	
	// Start agent
	if err := agent.Start(ctx, mcpConfig); err != nil {
		log.Printf("[InteractiveAgentManager] Failed to start agent %s: %v", agent.ID, err)
		return nil, err
	}
	
	// Store agent
	iam.mutex.Lock()
	iam.agents[agent.ID] = agent
	iam.mutex.Unlock()
	
	log.Printf("[InteractiveAgentManager] Successfully created and started agent %s", agent.ID)
	return agent, nil
}

// GetAgent retrieves an agent by ID
func (iam *InteractiveAgentManager) GetAgent(id string) *InteractiveAgent {
	iam.mutex.RLock()
	defer iam.mutex.RUnlock()
	return iam.agents[id]
}

// ListAgents returns all interactive agents
func (iam *InteractiveAgentManager) ListAgents() []*InteractiveAgent {
	iam.mutex.RLock()
	defer iam.mutex.RUnlock()
	
	agents := make([]*InteractiveAgent, 0, len(iam.agents))
	for _, agent := range iam.agents {
		agents = append(agents, agent)
	}
	return agents
}

// RemoveAgent stops and removes an agent
func (iam *InteractiveAgentManager) RemoveAgent(id string) error {
	iam.mutex.Lock()
	defer iam.mutex.Unlock()
	
	agent, ok := iam.agents[id]
	if !ok {
		return fmt.Errorf("agent not found: %s", id)
	}
	
	// Stop the agent
	if err := agent.Stop(); err != nil {
		return err
	}
	
	// Remove from map
	delete(iam.agents, id)
	return nil
}