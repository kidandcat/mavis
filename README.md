# Mavis - Command a Fleet of Claude Code Agents via Telegram & Web

Mavis is a powerful bot that brings Claude AI directly to your development workflow. Access it through Telegram commands or a modern web interface. Launch autonomous AI agents to write code, manage git operations, review pull requests, and serve local projects on your LAN network.

<img width="1918" alt="Screenshot 2025-06-26 at 21 21 17" src="https://github.com/user-attachments/assets/9b932b75-3e3b-4a12-ada6-c381b767a0c3" />


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

### 🌐 Web Development & LAN Serving
- **LAN Access**: Serve local development servers on your network with .local domain support
- **Static File Hosting**: Host static websites accessible from any device on your LAN
- **Build Command Integration**: Run development servers accessible via multiple URLs
- **mDNS Support**: Access services using mavis.local domain (requires Bonjour/mDNS)

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

### 🌐 Web Interface
- **Modern Dashboard**: Real-time agent monitoring with live status updates
- **Server-Sent Events (SSE)**: Instant notifications for agent events
- **Full Feature Parity**: All Telegram commands available through web UI
- **Responsive Design**: Works on desktop and mobile devices
- **Concurrent Access**: Use both Telegram and web interface simultaneously

## 📋 Prerequisites

- Go 1.20 or higher
- [Claude CLI](https://github.com/anthropics/claude-code) installed and configured
- Telegram Bot Token (from [@BotFather](https://t.me/botfather))
- GitHub CLI (`gh`) for PR review features (optional)
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
WEB_PORT=8080  # Optional: Enable web interface
WEB_PASSWORD=your_secure_password  # Optional: Web interface password
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

### 🌐 LAN Server Commands
- `/start <workdir> <port> <build_command>` - Start development server on LAN
- `/serve <directory> [port]` - Serve static files on LAN (default: 8080)
- `/stop` - Stop active LAN server

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

## 🌐 Web Interface

If you've enabled the web interface by setting `WEB_PORT`, you can access Mavis through your browser:

1. Open `http://localhost:8080` (or your configured port)
2. Login with the password set in `WEB_PASSWORD`
3. Access all Mavis features through the modern web UI:
   - **Agent Dashboard**: Monitor all running and queued agents
   - **Real-time Updates**: Get instant notifications via Server-Sent Events
   - **File Browser**: Navigate and download project files
   - **Git Operations**: View diffs and commit changes
   - **System Management**: Manage users and bot settings

The web interface provides the same functionality as Telegram commands with a more visual experience.

## 💡 Usage Examples

### Launch a coding agent to fix a bug:
```
/code ~/projects/myapp "Fix the memory leak in the WebSocket handler and add proper error handling"
```

### Create a new feature branch with AI implementation:
```
/new_branch ~/projects/api "Add GraphQL endpoint for user profile queries with proper authentication"
```

### Start your local development server on LAN:
```
/start ~/projects/frontend 3000 "npm run dev"
# Accessible at:
# - http://localhost:3000
# - http://192.168.1.100:3000 (your LAN IP)
# - http://mavis.local:3000 (mDNS)
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

### Serve static documentation site on LAN:
```
/serve ~/projects/docs 8080
# Accessible from any device on your network at:
# - http://192.168.1.100:8080
# - http://mavis.local:8080
```

## 🔧 Configuration

### Environment Variables
- `TELEGRAM_BOT_TOKEN` - Your Telegram bot token from [@BotFather](https://t.me/botfather) (required)
- `ADMIN_USER_ID` - Your Telegram user ID for admin access (required)

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
- **LAN Server**: Provides LAN-accessible servers with mDNS support

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
