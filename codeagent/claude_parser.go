package codeagent

import (
	"regexp"
	"strings"
	"sync"
	"time"
	
	"mavis/core"
)

// Message represents a single message in the conversation
type Message struct {
	ID        string
	Type      string // "user", "assistant", "system", "tool"
	Content   string
	Timestamp time.Time
	Metadata  map[string]string
}

// ConversationHistory maintains the full history of messages
type ConversationHistory struct {
	Messages []Message
	mu       sync.RWMutex
}

// ClaudeParser handles parsing of Claude Code CLI output
type ClaudeParser struct {
	history         *ConversationHistory
	currentMessage  *strings.Builder
	currentType     string
	lastTokenStatus string
	tokenPattern    *regexp.Regexp
	toolPattern     *regexp.Regexp
	ansiPattern     *regexp.Regexp
	uiPatterns      []*regexp.Regexp
	inToolOutput    bool
	toolDepth       int
}

// NewClaudeParser creates a new parser instance
func NewClaudeParser() *ClaudeParser {
	return &ClaudeParser{
		history: &ConversationHistory{
			Messages: make([]Message, 0),
		},
		currentMessage: &strings.Builder{},
		currentType:    "", // Start with no type
		tokenPattern:   regexp.MustCompile(`(\d+(?:,\d+)?)\s+tokens.*esc to interrupt`),
		toolPattern:    regexp.MustCompile(`^[⏺✓✗✢+*]\s+(.+?)(?:\s+\(.+?\))?$`),
		ansiPattern:    regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]|\x1b\][^\x07]*\x07|\x1b\[[0-9;]*m`),
		uiPatterns: []*regexp.Regexp{
			regexp.MustCompile(`^[╭│╰─╮╯]+`),
			regexp.MustCompile(`^\s*>\s*$`),
			regexp.MustCompile(`\? for shortcuts`),
			regexp.MustCompile(`Bypassing Permissions`),
			regexp.MustCompile(`Auto-update failed`),
		},
	}
}

// ParseLine processes a single line of output
func (p *ClaudeParser) ParseLine(line string) {
	// Remove ANSI escape sequences for analysis
	cleanLine := p.stripANSI(line)
	trimmed := strings.TrimSpace(cleanLine)
	
	// Skip empty lines if we're not in a message
	if trimmed == "" {
		// But if we have a message in progress, empty lines might be part of it
		if p.currentMessage.Len() > 0 && p.currentType != "" {
			p.currentMessage.WriteString("\n")
		}
		return
	}
	
	// Check for token status update FIRST
	if p.tokenPattern.MatchString(trimmed) {
		p.lastTokenStatus = trimmed
		// If we have a pending assistant message, flush it before updating token status
		if p.currentType == "assistant" && p.currentMessage.Len() > 0 {
			p.flushCurrentMessage()
		}
		return // Don't add to message history
	}
	
	// Skip lines that are clearly just UI elements
	if p.isDefinitelyUI(trimmed) {
		return
	}
	
	// Skip obvious UI elements
	if p.isUIElement(trimmed) && p.currentMessage.Len() == 0 {
		return
	}
	
	// Detect message boundaries and types
	if p.detectMessageBoundary(trimmed) {
		p.flushCurrentMessage()
		p.startNewMessage(trimmed)
		return
	}
	
	// If we have no current type but this looks like content, assume it's assistant
	if p.currentType == "" && !p.isUIElement(trimmed) && len(trimmed) > 0 {
		p.currentType = "assistant"
		p.currentMessage.WriteString(cleanLine)
		return
	}
	
	// Add line to current message if we have a type
	if p.currentType != "" {
		if p.currentMessage.Len() > 0 {
			p.currentMessage.WriteString("\n")
		}
		p.currentMessage.WriteString(cleanLine)
	}
}

// detectMessageBoundary checks if a line indicates a new message
func (p *ClaudeParser) detectMessageBoundary(line string) bool {
	// Check for ⏺ prefix without parentheses FIRST (Claude's response indicator)
	if strings.HasPrefix(line, "⏺ ") && !strings.Contains(line, "(") {
		// This is likely Claude's response, not a tool execution
		p.inToolOutput = false
		return true
	}
	
	// Tool execution markers (only if not Claude's response)
	if p.toolPattern.MatchString(line) {
		if !p.inToolOutput {
			p.inToolOutput = true
			p.toolDepth++
			return true
		}
		return false
	}
	
	// User input (after prompt) - check for various formats
	if strings.HasPrefix(line, "You: ") || strings.HasPrefix(line, "Human: ") ||
	   strings.HasPrefix(line, "> ") && len(line) > 2 && !strings.HasPrefix(line, "> Try") {
		// The "> " prefix is often used for user input in the Claude CLI
		p.inToolOutput = false
		return true
	}
	
	// Assistant response start patterns
	if strings.HasPrefix(line, "Claude: ") || strings.HasPrefix(line, "Assistant: ") {
		p.inToolOutput = false
		return true
	}
	
	// Check if this looks like the start of Claude's response without a prefix
	// Claude often starts responses without any prefix, sometimes with ⏺
	if !p.inToolOutput && p.currentType == "user" && len(line) > 0 && 
	   !strings.HasPrefix(line, "│") && !strings.HasPrefix(line, "╭") && 
	   !strings.HasPrefix(line, "╰") && !strings.Contains(line, "tokens") &&
	   !p.isUIElement(line) {
		// This might be the start of Claude's response
		return true
	}
	
	// System messages
	if strings.HasPrefix(line, "System: ") || strings.HasPrefix(line, "[System]") {
		return true
	}
	
	return false
}

// startNewMessage begins tracking a new message
func (p *ClaudeParser) startNewMessage(line string) {
	// Check for Claude's response with ⏺ prefix FIRST
	if strings.HasPrefix(line, "⏺ ") && !strings.Contains(line, "(") {
		// Claude's response with ⏺ prefix
		p.currentType = "assistant"
		// Remove the ⏺ prefix
		content := strings.TrimPrefix(line, "⏺ ")
		p.currentMessage.WriteString(content)
	} else if p.toolPattern.MatchString(line) {
		p.currentType = "tool"
		p.currentMessage.WriteString(line)
	} else if strings.HasPrefix(line, "You: ") || strings.HasPrefix(line, "Human: ") {
		p.currentType = "user"
		// Remove the prefix
		content := strings.TrimPrefix(line, "You: ")
		content = strings.TrimPrefix(content, "Human: ")
		p.currentMessage.WriteString(content)
	} else if strings.HasPrefix(line, "> ") && len(line) > 2 && !strings.HasPrefix(line, "> Try") {
		p.currentType = "user"
		// Remove the "> " prefix
		content := strings.TrimPrefix(line, "> ")
		p.currentMessage.WriteString(content)
	} else if strings.HasPrefix(line, "Claude: ") || strings.HasPrefix(line, "Assistant: ") {
		p.currentType = "assistant"
		// Remove the prefix
		content := strings.TrimPrefix(line, "Claude: ")
		content = strings.TrimPrefix(content, "Assistant: ")
		p.currentMessage.WriteString(content)
	} else if strings.HasPrefix(line, "System: ") || strings.HasPrefix(line, "[System]") {
		p.currentType = "system"
		content := strings.TrimPrefix(line, "System: ")
		content = strings.TrimPrefix(content, "[System]")
		p.currentMessage.WriteString(content)
	} else if !p.inToolOutput && p.currentType == "user" {
		// This is likely the start of Claude's response without a prefix
		p.currentType = "assistant"
		p.currentMessage.WriteString(line)
	}
}

// flushCurrentMessage saves the current message to history
func (p *ClaudeParser) flushCurrentMessage() {
	if p.currentMessage.Len() == 0 {
		return
	}
	
	message := Message{
		ID:        generateMessageID(),
		Type:      p.currentType,
		Content:   strings.TrimSpace(p.currentMessage.String()),
		Timestamp: time.Now(),
		Metadata:  make(map[string]string),
	}
	
	// Debug logging - commented out
	// if message.Type == "assistant" {
	// 	contentPreview := message.Content
	// 	if len(contentPreview) > 100 {
	// 		contentPreview = contentPreview[:100] + "..."
	// 	}
	// 	log.Printf("[ClaudeParser] Captured assistant message: %q", contentPreview)
	// }
	
	// Add token status if this is an assistant message
	if p.currentType == "assistant" && p.lastTokenStatus != "" {
		message.Metadata["tokens"] = p.lastTokenStatus
	}
	
	p.history.mu.Lock()
	p.history.Messages = append(p.history.Messages, message)
	p.history.mu.Unlock()
	
	// Reset for next message
	p.currentMessage.Reset()
	p.currentType = ""
}

// isUIElement checks if a line is a UI element to be filtered
func (p *ClaudeParser) isUIElement(line string) bool {
	for _, pattern := range p.uiPatterns {
		if pattern.MatchString(line) {
			return true
		}
	}
	return false
}

// isDefinitelyUI checks if a line is definitely just UI chrome
func (p *ClaudeParser) isDefinitelyUI(line string) bool {
	// Skip lines that are only box drawing characters
	if p.isOnlyBoxDrawing(line) {
		return true
	}
	
	// Skip known UI messages
	uiMessages := []string{
		"? for shortcuts",
		"Bypassing Permissions",
		"Auto-update failed",
		"Try claude doctor",
		"npm i -g @anthropic-ai/claude-code",
		"@anthropic-ai/claude-code",
		"✗ Auto-update failed",
		"esc to interrupt",
		"tokens",
	}
	
	for _, msg := range uiMessages {
		if strings.Contains(line, msg) {
			return true
		}
	}
	
	return false
}

// isOnlyBoxDrawing checks if a string contains only box drawing characters
func (p *ClaudeParser) isOnlyBoxDrawing(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '─' && r != '│' && r != '╭' && r != '╮' && 
		   r != '╯' && r != '╰' && r != '┴' && r != '┬' && r != '├' && 
		   r != '┤' && r != '┼' && r != '═' && r != '║' && r != '╔' && 
		   r != '╗' && r != '╝' && r != '╚' && r != '>' {
			return false
		}
	}
	return len(strings.TrimSpace(s)) > 0 // Not empty after trimming
}

// mightBeRealContent checks if a line that looks like UI might actually be content
func (p *ClaudeParser) mightBeRealContent(line string) bool {
	// If we're currently building a message, UI-looking lines might be part of the content
	if p.currentMessage.Len() > 0 {
		// Check if this line contains meaningful content despite matching UI patterns
		if len(line) > 10 && !strings.HasPrefix(line, "╭") && !strings.HasPrefix(line, "╰") &&
		   !strings.HasPrefix(line, "│") {
			return true
		}
	}
	return false
}

// stripANSI removes ANSI escape sequences from a string
func (p *ClaudeParser) stripANSI(text string) string {
	return p.ansiPattern.ReplaceAllString(text, "")
}

// GetHistory returns the conversation history
func (p *ClaudeParser) GetHistory() []Message {
	p.history.mu.RLock()
	defer p.history.mu.RUnlock()
	
	// Make a copy to avoid race conditions
	messages := make([]Message, len(p.history.Messages))
	copy(messages, p.history.Messages)
	return messages
}

// GetLastTokenStatus returns the most recent token status
func (p *ClaudeParser) GetLastTokenStatus() string {
	return p.lastTokenStatus
}

// ProcessTerminalBuffer processes a full terminal buffer update
func (p *ClaudeParser) ProcessTerminalBuffer(lines []string) {
	// This method handles bulk updates from terminal clear operations
	// It intelligently merges with existing history instead of duplicating
	
	// Find the last processed position
	lastProcessedIndex := p.findLastProcessedPosition(lines)
	
	// Process only new lines
	for i := lastProcessedIndex + 1; i < len(lines); i++ {
		p.ParseLine(lines[i])
	}
	
	// Ensure any pending message is flushed
	p.flushCurrentMessage()
}

// findLastProcessedPosition finds where new content starts
func (p *ClaudeParser) findLastProcessedPosition(lines []string) int {
	p.history.mu.RLock()
	defer p.history.mu.RUnlock()
	
	if len(p.history.Messages) == 0 {
		return -1
	}
	
	// Get the last few messages for comparison
	lastMessage := p.history.Messages[len(p.history.Messages)-1]
	lastContent := p.stripANSI(lastMessage.Content)
	
	// Search backwards through lines for matching content
	for i := len(lines) - 1; i >= 0; i-- {
		cleanLine := p.stripANSI(lines[i])
		if strings.Contains(lastContent, cleanLine) || strings.Contains(cleanLine, lastContent) {
			return i
		}
	}
	
	return -1
}

// ClearHistory clears the conversation history
func (p *ClaudeParser) ClearHistory() {
	p.history.mu.Lock()
	defer p.history.mu.Unlock()
	p.history.Messages = make([]Message, 0)
	p.currentMessage.Reset()
	p.currentType = ""
	p.lastTokenStatus = ""
}

// FlushPending flushes any pending message
func (p *ClaudeParser) FlushPending() {
	p.flushCurrentMessage()
}

// generateMessageID creates a unique message ID
func generateMessageID() string {
	return core.NewID(8)
}

// MessageFilter allows filtering messages by criteria
type MessageFilter struct {
	Type      string
	StartTime time.Time
	EndTime   time.Time
	Contains  string
}

// GetFilteredHistory returns messages matching the filter criteria
func (p *ClaudeParser) GetFilteredHistory(filter MessageFilter) []Message {
	p.history.mu.RLock()
	defer p.history.mu.RUnlock()
	
	var filtered []Message
	for _, msg := range p.history.Messages {
		// Check type filter
		if filter.Type != "" && msg.Type != filter.Type {
			continue
		}
		
		// Check time filters
		if !filter.StartTime.IsZero() && msg.Timestamp.Before(filter.StartTime) {
			continue
		}
		if !filter.EndTime.IsZero() && msg.Timestamp.After(filter.EndTime) {
			continue
		}
		
		// Check content filter
		if filter.Contains != "" && !strings.Contains(strings.ToLower(msg.Content), strings.ToLower(filter.Contains)) {
			continue
		}
		
		filtered = append(filtered, msg)
	}
	
	return filtered
}

// ExportHistory exports the conversation history in various formats
func (p *ClaudeParser) ExportHistory(format string) string {
	messages := p.GetHistory()
	
	switch format {
	case "markdown":
		return p.exportMarkdown(messages)
	case "json":
		return p.exportJSON(messages)
	default:
		return p.exportPlainText(messages)
	}
}

func (p *ClaudeParser) exportMarkdown(messages []Message) string {
	var sb strings.Builder
	sb.WriteString("# Claude Code Conversation\n\n")
	
	for _, msg := range messages {
		switch msg.Type {
		case "user":
			sb.WriteString("## User\n")
		case "assistant":
			sb.WriteString("## Claude\n")
		case "tool":
			sb.WriteString("### Tool Execution\n")
		case "system":
			sb.WriteString("### System\n")
		}
		
		sb.WriteString(msg.Content)
		sb.WriteString("\n\n")
		
		if tokens, ok := msg.Metadata["tokens"]; ok {
			sb.WriteString("*" + tokens + "*\n\n")
		}
	}
	
	return sb.String()
}

func (p *ClaudeParser) exportJSON(messages []Message) string {
	// Simple JSON export (you might want to use encoding/json for production)
	var sb strings.Builder
	sb.WriteString("[\n")
	
	for i, msg := range messages {
		sb.WriteString("  {\n")
		sb.WriteString(`    "id": "` + msg.ID + `",` + "\n")
		sb.WriteString(`    "type": "` + msg.Type + `",` + "\n")
		sb.WriteString(`    "content": "` + escapeJSON(msg.Content) + `",` + "\n")
		sb.WriteString(`    "timestamp": "` + msg.Timestamp.Format(time.RFC3339) + `"` + "\n")
		sb.WriteString("  }")
		
		if i < len(messages)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}
	
	sb.WriteString("]\n")
	return sb.String()
}

func (p *ClaudeParser) exportPlainText(messages []Message) string {
	var sb strings.Builder
	
	for _, msg := range messages {
		prefix := ""
		switch msg.Type {
		case "user":
			prefix = "You: "
		case "assistant":
			prefix = "Claude: "
		case "tool":
			prefix = "Tool: "
		case "system":
			prefix = "System: "
		}
		
		sb.WriteString(prefix + msg.Content + "\n\n")
	}
	
	return sb.String()
}

// escapeJSON escapes special characters for JSON
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}