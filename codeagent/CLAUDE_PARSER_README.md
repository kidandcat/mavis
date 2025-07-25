# Claude Code CLI Parser

A robust parser for Claude Code CLI output that properly handles message history, token counting, and terminal clear operations without spamming.

## Features

- **Message History Tracking**: Maintains a complete conversation history with proper message type detection (user, assistant, tool, system)
- **Token Status Monitoring**: Tracks token usage without duplicating when the CLI refreshes the display
- **Smart Terminal Handling**: Processes ANSI escape sequences and terminal clear operations intelligently
- **Filtering & Search**: Find messages by type, content, or time range
- **Export Formats**: Export conversation history as Markdown, JSON, or plain text
- **Thread-Safe**: All operations are protected with proper synchronization

## Architecture

The parser consists of two main components:

### 1. ClaudeParser (`claude_parser.go`)
- Parses individual lines of Claude CLI output
- Detects message boundaries and types
- Maintains conversation history
- Handles token status updates separately from messages
- Provides filtering and export capabilities

### 2. Integration with InteractiveAgent
- The existing `InteractiveAgent` has been enhanced with a `ClaudeParser` instance
- Terminal output is processed through both the terminal buffer (for display) and the parser (for history)
- Prevents duplicate messages when the CLI clears and redraws the screen

## How It Works

### Message Detection
The parser identifies different message types by their prefixes:
- **User**: Lines starting with "You: " or "Human: "
- **Assistant**: Lines starting with "Claude: " or "Assistant: "
- **Tool**: Lines with tool execution markers (⏺, ✓, ✗, etc.)
- **System**: Lines starting with "System: " or "[System]"

### Terminal Clear Handling
When Claude CLI clears the terminal (e.g., to update the token counter), the parser:
1. Compares new terminal content with existing history
2. Only processes genuinely new content
3. Preserves the complete conversation history
4. Updates the token status without creating duplicate entries

### Token Status Management
- Token status lines are extracted and stored separately
- The latest token status is always available via `GetLastTokenStatus()`
- Token information is attached as metadata to assistant messages

## Usage

### Basic Usage

```go
// The parser is automatically created with each InteractiveAgent
agent := codeagent.NewInteractiveAgent("/path/to/project", "")

// Start the agent
ctx := context.Background()
agent.Start(ctx, "")

// Get full message history
messages := agent.GetMessageHistory()
for _, msg := range messages {
    fmt.Printf("[%s] %s: %s\n", msg.Type, msg.Timestamp, msg.Content)
}

// Get current token status
tokenStatus := agent.GetLastTokenStatus()
fmt.Printf("Current tokens: %s\n", tokenStatus)
```

### Filtering Messages

```go
// Get only assistant messages
filter := codeagent.MessageFilter{
    Type: "assistant",
}
assistantMessages := agent.GetFilteredHistory(filter)

// Search for messages containing "error"
searchFilter := codeagent.MessageFilter{
    Contains: "error",
}
errorMessages := agent.GetFilteredHistory(searchFilter)

// Get messages from the last hour
recentFilter := codeagent.MessageFilter{
    StartTime: time.Now().Add(-1 * time.Hour),
}
recentMessages := agent.GetFilteredHistory(recentFilter)
```

### Exporting History

```go
// Export as Markdown
markdownExport := agent.ExportHistory("markdown")

// Export as JSON
jsonExport := agent.ExportHistory("json")

// Export as plain text
textExport := agent.ExportHistory("text")
```

## API Reference

### InteractiveAgent Methods

- `GetMessageHistory() []Message` - Returns the complete conversation history
- `GetFilteredHistory(filter MessageFilter) []Message` - Returns filtered messages
- `GetLastTokenStatus() string` - Returns the latest token count status
- `ExportHistory(format string) string` - Exports history in specified format
- `ClearHistory()` - Clears the conversation history

### Message Structure

```go
type Message struct {
    ID        string
    Type      string              // "user", "assistant", "system", "tool"
    Content   string
    Timestamp time.Time
    Metadata  map[string]string   // Additional info like token counts
}
```

### MessageFilter Structure

```go
type MessageFilter struct {
    Type      string     // Filter by message type
    StartTime time.Time  // Messages after this time
    EndTime   time.Time  // Messages before this time
    Contains  string     // Search in message content
}
```

## Implementation Details

### Terminal Buffer Processing
The parser works alongside the existing terminal buffer system:
1. Terminal output is processed by `TerminalBuffer` for display rendering
2. The same output is fed to `ClaudeParser` for history tracking
3. Both systems work independently to avoid interference

### Deduplication Strategy
To handle terminal clears without creating duplicates:
1. The parser tracks the last processed position in the terminal buffer
2. When processing a new buffer, it finds where new content begins
3. Only content after the last processed position is parsed
4. This prevents re-parsing the same messages after a clear operation

### Thread Safety
- All parser operations use read-write mutexes
- Message history is protected from concurrent access
- Safe for use in multi-threaded environments

## Benefits

1. **Complete History**: Never lose conversation context, even with terminal clears
2. **Accurate Token Tracking**: Always know current token usage without duplicates
3. **Flexible Querying**: Find specific messages quickly with powerful filtering
4. **Export Options**: Save conversations in multiple formats for documentation
5. **Performance**: Efficient processing that doesn't impact CLI responsiveness

## Example Use Cases

1. **Audit Trail**: Keep a complete record of all Claude interactions
2. **Error Analysis**: Quickly find and review error messages
3. **Token Monitoring**: Track token usage over time for cost analysis
4. **Documentation**: Export conversations as markdown for documentation
5. **Integration**: Build tools that analyze Claude's responses programmatically