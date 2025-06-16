# Mavis - AI-Powered Development Assistant for Telegram

Mavis is a powerful Telegram bot that brings Claude AI directly to your development workflow. Launch autonomous AI agents to write code, manage git operations, review pull requests, and expose local services to the internet - all through simple Telegram commands.

## 🚀 Features

### 🤖 Autonomous Code Agents
- **Launch Claude AI Agents**: Delegate complex coding tasks to AI agents that work independently
- **Task Planning**: Agents create and follow detailed plans using CURRENT_PLAN.md for organized execution
- **Real-time Status Tracking**: Monitor agent progress and receive instant notifications upon completion
- **Concurrent Execution**: Run multiple agents simultaneously for different projects
- **Smart Context Awareness**: Agents understand your project structure and coding conventions

### 🌿 Advanced Git Workflows
- **Automated Branch Management**: Create feature branches, implement changes, and push with a single command
- **Intelligent Commits**: AI generates contextual commit messages based on actual changes
- **PR Review Automation**: Get comprehensive pull request reviews sent directly to Telegram
- **PR Comments & Approval**: Automatically post review comments and approve PRs when ready
- **Pending Changes Review**: Review uncommitted changes before making commits
- **Visual Diffs**: See actual git diffs for modified files with syntax highlighting

### 🌐 Web Development & Deployment
- **Instant Public URLs**: Expose local development servers to the internet using [online](https://github.com/kidandcat/online) tunneling
- **Static File Hosting**: Serve and share static websites with public URLs instantly
- **Build Command Integration**: Run development servers with automatic public exposure
- **Port Management**: Handle multiple services on different ports simultaneously

### 📁 Remote File Operations
- **Direct File Downloads**: Retrieve project files up to 50MB directly in Telegram
- **Directory Navigation**: Browse project structures with detailed file information
- **Command Execution**: Run any command in your workspaces remotely via `/run`
- **Smart Path Resolution**: Supports ~/paths, relative paths, and absolute paths

### 🔐 Enterprise-Grade Security
- **Multi-user Authorization**: Secure access control with user whitelist management
- **Admin Controls**: Dedicated admin commands for user management
- **Security Notifications**: Real-time alerts for unauthorized access attempts
- **Isolated Execution**: Each agent runs in its own secure environment

## 📋 Prerequisites

- Go 1.20 or higher
- [Claude CLI](https://github.com/anthropics/claude-code) installed and configured
- Telegram Bot Token (from [@BotFather](https://t.me/botfather))
- GitHub CLI (`gh`) for PR review features (optional)
- [Online](https://github.com/kidandcat/online) tunneling tool for web exposure (optional)
- Python 3 for static file serving (optional)

## 🛠️ Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/mavis.git
cd mavis
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file with your configuration:
```bash
cat > .env << EOF
TELEGRAM_BOT_TOKEN=your_bot_token_here
ADMIN_USER_ID=your_telegram_user_id
# Optional: Custom online server URL
# ONLINE_SERVER_URL=https://your-server.com
EOF
```

4. Build and run:
```bash
go build -o mavis
./mavis
```

Or run directly:
```bash
go run .
```

Or use the continuous run script:
```bash
./run.sh
```

## 📱 Commands

### 🤖 Code Agent Commands
- `/code <directory> <task>` - Launch an AI agent to complete a coding task
- `/new_branch <directory> <task>` - Create a new branch, implement changes, and push
- `/edit_branch <directory> <branch> <task>` - Work on an existing branch
- `/ps` - List all active agents with their current status
- `/status <agent_id>` - Get detailed information about a specific agent
- `/stop <agent_id>` - Terminate a running agent

### 🌿 Git Workflow Commands
- `/commit <directory>` - Review changes, create commit, and push
- `/diff [path]` - Show git diffs (directory: all files, file: single diff)
- `/review <directory>` - Review pending changes in workspace
- `/review <directory> <pr_url>` - Get AI-powered PR review sent to Telegram
- `/pr <directory> <pr_url>` - Review PR, post comment, and approve if ready

### 🌐 Web Development Commands
- `/start <workdir> <port> <build_command>` - Start development server with public URL
- `/serve <directory> [port]` - Serve static files with public URL (default: 8080)
- `/stop` - Stop active tunnel and server processes

### 📁 File & System Commands
- `/download <file_path>` - Download files directly to Telegram (max 50MB)
- `/ls [directory]` - List directory contents with file sizes
- `/mkdir <directory>` - Create new directories
- `/run <workspace> <command> [args...]` - Execute commands in any workspace

### 🔐 Admin Commands
- `/adduser <username> <user_id>` - Authorize a new user
- `/removeuser <username>` - Revoke user access
- `/users` - List all authorized users
- `/restart` - Restart the bot service with zero-downtime green deployment

### ℹ️ General Commands
- `/help` - Show comprehensive command documentation

## 💡 Usage Examples

### Launch a coding agent to fix a bug:
```
/code ~/projects/myapp "Fix the memory leak in the WebSocket handler and add proper error handling"
```

### Create a new feature branch with AI implementation:
```
/new_branch ~/projects/api "Add GraphQL endpoint for user profile queries with proper authentication"
```

### Expose your local development server:
```
/start ~/projects/frontend 3000 "npm run dev"
# Returns: Public URL https://abc123.online.io accessible from anywhere
```

### Get AI review of pending changes:
```
/review ~/projects/backend
# AI analyzes all uncommitted changes and provides feedback
```

### Review and approve a GitHub pull request:
```
/pr ~/projects/backend https://github.com/owner/repo/pull/123
# Posts detailed review comment and approves if ready
```

### Run tests in a remote workspace:
```
/run ~/projects/api npm test
# Execute commands and see output directly in Telegram
```

### Serve static documentation site:
```
/serve ~/projects/docs 8080
# Instantly accessible at https://xyz789.online.io
```

## 🔧 Configuration

### Environment Variables
- `TELEGRAM_BOT_TOKEN` - Your Telegram bot token from [@BotFather](https://t.me/botfather) (required)
- `ADMIN_USER_ID` - Your Telegram user ID for admin access (required)
- `ONLINE_SERVER_URL` - Custom online tunnel server URL (optional)

### Getting Your Telegram User ID
1. Start a chat with [@userinfobot](https://t.me/userinfobot)
2. Send any message
3. The bot will reply with your user ID

### Data Storage
Mavis maintains persistent data in the `data/` directory:
- `data/authorized_users.json` - Whitelist of authorized users with their Telegram IDs

## 🔒 Security

- **Authorization System**: Whitelist-based access control for all bot commands
- **Admin Notifications**: Real-time alerts for unauthorized access attempts
- **Secure Execution**: Shell injection prevention and sandboxed command execution
- **Isolated Environments**: Each AI agent runs in its own working directory
- **No Credential Storage**: Relies on system-level authentication (SSH keys, git credentials)

## 🏗️ Architecture

Mavis is built with a modular architecture:
- **Core Bot**: Handles Telegram interactions and command routing
- **Agent Manager**: Manages lifecycle of Claude AI agents with task planning
- **Git Integration**: Provides git workflow automation
- **File System**: Handles remote file operations
- **Auth System**: Manages user authorization and security
- **Web Tunneling**: Integrates with online tool for public URL exposure

### Task Planning System
Each AI agent creates a `CURRENT_PLAN.md` file to:
- Document the task objectives
- Plan implementation steps
- Track progress in real-time
- Adapt to discovered requirements

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup
```bash
# Run tests
go test ./...

# Run with live reload (requires air)
air

# Build for production
go build -ldflags="-s -w" -o mavis
```

## 📄 License

This project is licensed under the MIT License - see [LICENSE](LICENSE) for details.

## 🙏 Acknowledgments

- Powered by [Claude AI](https://claude.ai) via [Claude CLI](https://github.com/anthropics/claude-code)
- Telegram integration via [go-telegram](https://github.com/go-telegram/bot)
- Online tunneling by [online](https://github.com/kidandcat/online)

## 🚀 Deployment

### Green Deployment Restart
Mavis supports zero-downtime restarts using a green deployment strategy:
- The `/restart` command (admin only) builds a new binary with a timestamp
- Starts the new process and waits for it to initialize
- Gracefully shuts down the old process
- Automatically cleans up old binaries

### Using systemd (Linux)
```bash
# Create a service file
sudo nano /etc/systemd/system/mavis.service
```

```ini
[Unit]
Description=Mavis Telegram Bot
After=network.target

[Service]
Type=simple
User=your-user
WorkingDirectory=/path/to/mavis
ExecStart=/path/to/mavis/mavis
Restart=always
RestartSec=10
Environment="PATH=/usr/local/bin:/usr/bin:/bin"

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start the service
sudo systemctl enable mavis
sudo systemctl start mavis
```

### Continuous Running Script
The included `run.sh` script ensures the bot stays running:
```bash
./run.sh
```

### Using Docker
```dockerfile
FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o mavis

FROM alpine:latest
RUN apk --no-cache add ca-certificates git
WORKDIR /app
COPY --from=builder /app/mavis .
COPY --from=builder /app/data ./data
CMD ["./mavis"]
```

---

Made with ❤️ by the Mavis contributors