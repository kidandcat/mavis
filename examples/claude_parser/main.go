package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
	
	"mavis/codeagent"
)

func main() {
	// Example 1: Creating an interactive agent with Claude parser
	fmt.Println("=== Claude Code CLI Parser Example ===\n")
	
	// Create a new interactive agent (which includes the parser)
	agent := codeagent.NewInteractiveAgent("/path/to/project", "")
	
	// Start the agent
	ctx := context.Background()
	if err := agent.Start(ctx, ""); err != nil {
		log.Fatal("Failed to start agent:", err)
	}
	
	// Give it some time to process
	time.Sleep(10 * time.Second)
	
	// Example 2: Getting the full message history
	fmt.Println("\n--- Full Message History ---")
	history := agent.GetMessageHistory()
	for _, msg := range history {
		fmt.Printf("[%s] %s: %s\n", msg.Timestamp.Format("15:04:05"), msg.Type, msg.Content)
		if tokens, ok := msg.Metadata["tokens"]; ok {
			fmt.Printf("  Tokens: %s\n", tokens)
		}
	}
	
	// Example 3: Getting filtered history (only assistant messages)
	fmt.Println("\n--- Assistant Messages Only ---")
	assistantFilter := codeagent.MessageFilter{
		Type: "assistant",
	}
	assistantMessages := agent.GetFilteredHistory(assistantFilter)
	for _, msg := range assistantMessages {
		fmt.Printf("Claude: %s\n", msg.Content)
	}
	
	// Example 4: Getting tool execution history
	fmt.Println("\n--- Tool Executions ---")
	toolFilter := codeagent.MessageFilter{
		Type: "tool",
	}
	toolMessages := agent.GetFilteredHistory(toolFilter)
	for _, msg := range toolMessages {
		fmt.Printf("Tool: %s\n", msg.Content)
	}
	
	// Example 5: Getting the current token status
	fmt.Println("\n--- Current Token Status ---")
	tokenStatus := agent.GetLastTokenStatus()
	fmt.Printf("Tokens: %s\n", tokenStatus)
	
	// Example 6: Exporting history in different formats
	fmt.Println("\n--- Export Examples ---")
	
	// Export as Markdown
	markdownExport := agent.ExportHistory("markdown")
	fmt.Println("Markdown export (first 500 chars):")
	if len(markdownExport) > 500 {
		fmt.Println(markdownExport[:500] + "...")
	} else {
		fmt.Println(markdownExport)
	}
	
	// Export as JSON
	jsonExport := agent.ExportHistory("json")
	fmt.Println("\nJSON export (first 500 chars):")
	if len(jsonExport) > 500 {
		fmt.Println(jsonExport[:500] + "...")
	} else {
		fmt.Println(jsonExport)
	}
	
	// Example 7: Searching for specific content
	fmt.Println("\n--- Search for 'error' in messages ---")
	searchFilter := codeagent.MessageFilter{
		Contains: "error",
	}
	errorMessages := agent.GetFilteredHistory(searchFilter)
	for _, msg := range errorMessages {
		fmt.Printf("[%s] Found in %s message: %s\n", 
			msg.Timestamp.Format("15:04:05"), msg.Type, msg.Content)
	}
	
	// Example 8: Getting messages from the last 5 minutes
	fmt.Println("\n--- Recent Messages (last 5 minutes) ---")
	recentFilter := codeagent.MessageFilter{
		StartTime: time.Now().Add(-5 * time.Minute),
	}
	recentMessages := agent.GetFilteredHistory(recentFilter)
	for _, msg := range recentMessages {
		fmt.Printf("[%s] %s: %s\n", 
			msg.Timestamp.Format("15:04:05"), msg.Type, msg.Content)
	}
	
	// Example 9: Direct parser usage (without interactive agent)
	fmt.Println("\n--- Direct Parser Usage ---")
	parser := codeagent.NewClaudeParser()
	
	// Simulate parsing some output
	sampleOutput := []string{
		"You: Help me write a hello world program",
		"Claude: I'll help you write a hello world program. Here's a simple example in Python:",
		"```python",
		"print('Hello, World!')",
		"```",
		"⏺ Running Python script",
		"Hello, World!",
		"✓ Script executed successfully",
		"1,234 tokens (1,234 conversation, 0 cached) • esc to interrupt",
	}
	
	for _, line := range sampleOutput {
		parser.ParseLine(line)
	}
	
	// Get parsed history
	parsedHistory := parser.GetHistory()
	fmt.Println("\nParsed messages:")
	for _, msg := range parsedHistory {
		fmt.Printf("- Type: %s, Content: %s\n", msg.Type, msg.Content)
	}
	
	// Stop the agent
	agent.Stop()
}

// Advanced Usage Examples

// Example: Creating a web handler that streams conversation history
func ConversationHistoryHandler(agent *codeagent.InteractiveAgent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get query parameters for filtering
		msgType := r.URL.Query().Get("type")
		search := r.URL.Query().Get("search")
		format := r.URL.Query().Get("format")
		
		if format != "" {
			// Export in requested format
			export := agent.ExportHistory(format)
			
			// Set appropriate content type
			switch format {
			case "json":
				w.Header().Set("Content-Type", "application/json")
			case "markdown":
				w.Header().Set("Content-Type", "text/markdown")
			default:
				w.Header().Set("Content-Type", "text/plain")
			}
			
			w.Write([]byte(export))
			return
		}
		
		// Build filter
		filter := codeagent.MessageFilter{
			Type:     msgType,
			Contains: search,
		}
		
		// Get filtered history
		messages := agent.GetFilteredHistory(filter)
		
		// Return as JSON
		w.Header().Set("Content-Type", "application/json")
		// Convert messages to JSON (simplified for example)
		fmt.Fprintf(w, `{"messages": [`)
		for i, msg := range messages {
			if i > 0 {
				fmt.Fprintf(w, ",")
			}
			fmt.Fprintf(w, `{"type":"%s","content":"%s","timestamp":"%s"}`,
				msg.Type, escapeJSON(msg.Content), msg.Timestamp.Format(time.RFC3339))
		}
		fmt.Fprintf(w, `]}`)
	}
}

// Example: Monitoring token usage over time
func TokenMonitor(agent *codeagent.InteractiveAgent) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for range ticker.C {
		tokenStatus := agent.GetLastTokenStatus()
		if tokenStatus != "" {
			log.Printf("Token usage: %s", tokenStatus)
			
			// You could parse the token count and send to metrics system
			// e.g., prometheus.TokenGauge.Set(parseTokenCount(tokenStatus))
		}
	}
}

// Example: Auto-save conversation history
func AutoSaveHistory(agent *codeagent.InteractiveAgent, filepath string) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		// Export as markdown
		export := agent.ExportHistory("markdown")
		
		// Save to file
		if err := os.WriteFile(filepath, []byte(export), 0644); err != nil {
			log.Printf("Failed to save history: %v", err)
		} else {
			log.Printf("History saved to %s", filepath)
		}
	}
}

func escapeJSON(s string) string {
	// Simple JSON escaping for example purposes
	return s
}