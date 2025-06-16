# Contributing to Mavis

Thank you for your interest in contributing to Mavis! This document provides guidelines and information for contributors.

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct. Be respectful, inclusive, and constructive in all interactions.

## How to Contribute

### Reporting Bugs

1. **Search existing issues** to avoid duplicates
2. **Use the bug report template** when creating a new issue
3. **Provide detailed information**:
   - Steps to reproduce
   - Expected behavior
   - Actual behavior
   - Environment details (Go version, OS, etc.)
   - Relevant logs or error messages

### Suggesting Enhancements

1. **Check existing feature requests** to avoid duplicates
2. **Provide a clear description** of the enhancement
3. **Explain the use case** and why it would be valuable
4. **Consider implementation details** if you have technical insights

### Contributing Code

#### Development Setup

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/yourusername/mavis.git
   cd mavis
   ```
3. **Set up environment**:
   ```bash
   cp .env.example .env
   # Edit .env with your test API keys
   ```
4. **Install dependencies**:
   ```bash
   go mod download
   ```
5. **Create a branch** for your feature:
   ```bash
   git checkout -b feature/your-feature-name
   ```

#### Coding Standards

- **Follow Go conventions**: Use `gofmt`, `golint`, and `go vet`
- **Write clear commit messages**: Use the format "type: description"
  - `feat:` for new features
  - `fix:` for bug fixes
  - `docs:` for documentation changes
  - `refactor:` for code refactoring
  - `test:` for adding tests
- **Keep commits focused**: One logical change per commit
- **Add tests** when applicable
- **Update documentation** if you change APIs or add features

#### Pull Request Process

1. **Update your branch** with the latest main:
   ```bash
   git checkout main
   git pull upstream main
   git checkout your-feature-branch
   git rebase main
   ```

2. **Test your changes**:
   ```bash
   go build .
   go test ./...
   ```

3. **Create a pull request** with:
   - Clear title and description
   - Reference to related issues
   - Screenshots for UI changes
   - Test instructions if needed

4. **Respond to feedback** and make requested changes

5. **Squash commits** if requested before merging

## Project Structure

```
mavis/
├── main.go           # Application entry point
├── handleMessage.go  # Message processing
├── openai.go        # AI integration
├── user.go          # User management
├── reminder.go      # Reminder system
├── tools.go         # Tool definitions
├── code.go          # Code execution
├── utils.go         # Utilities
└── data/            # User data storage
```

## Adding New Features

### Adding New Tools

1. **Define the tool** in `tools.go`:
   ```go
   {
       Type: openai.ToolTypeFunction,
       Function: &openai.FunctionDefinition{
           Name:        "your_tool_name",
           Description: "Description of what your tool does",
           Parameters: jsonschema.Definition{
               // Define parameters
           },
       },
   }
   ```

2. **Implement the handler** in `ToolCall()` function:
   ```go
   case "your_tool_name":
       // Handle the tool call
       return "Tool result"
   ```

3. **Add tests** for your new tool
4. **Update documentation** in README.md

### Modifying AI Behavior

- The main AI prompt is in `openai.go` in the `handleOpenaiCompletion` function
- Be careful with prompt changes as they affect all interactions
- Test thoroughly with different conversation scenarios

### Database/Storage Changes

- User data is stored as JSON files in the `data/` directory
- Each user has their own file: `data/{user_id}.json`
- Be careful with data structure changes to maintain backward compatibility
- Consider migration scripts for breaking changes

## Testing

### Manual Testing

1. **Set up test environment** with test API keys
2. **Create a test bot** with @BotFather
3. **Test all features**:
   - Basic conversation
   - Memory system
   - Reminders
   - Audio transcription
   - Code execution
   - Admin commands

### Automated Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

## Security Considerations

- **Never commit API keys** or sensitive data
- **Validate all user inputs** to prevent injection attacks
- **Be cautious with code execution** features
- **Review dependencies** for security vulnerabilities
- **Use environment variables** for all configuration

## Documentation

- **Update README.md** for user-facing changes
- **Add code comments** for complex logic
- **Update this CONTRIBUTING.md** for process changes
- **Include examples** for new features

## Release Process

1. **Version bumping** follows semantic versioning (MAJOR.MINOR.PATCH)
2. **Update CHANGELOG.md** with release notes
3. **Tag releases** in Git
4. **Create GitHub releases** with binary attachments

## Getting Help

- **Check existing issues** and documentation first
- **Ask questions** in issue discussions
- **Join discussions** about features and improvements
- **Be patient** - maintainers are volunteers

## License

By contributing to Mavis, you agree that your contributions will be licensed under the MIT License.

---

Thank you for contributing to Mavis! 🚀 