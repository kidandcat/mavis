// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package telegram

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"mavis/core"

	"github.com/go-telegram/bot/models"
)

// generateUniquePlanFilename creates a unique plan filename for different command types
func generateUniquePlanFilename(commandType string) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s_PLAN_%d.md", strings.ToUpper(commandType), timestamp)
}

func handleGitCodeCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 3 {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Please provide directory and task.\nUsage: /new_branch <directory> <task>\n\nExample: /new_branch ~/myproject implement new feature")
		return
	}

	directory := strings.TrimSpace(parts[1])
	task := strings.TrimSpace(strings.Join(parts[2:], " "))

	if directory == "" || task == "" {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Both directory and task are required.\n\nExample: /new_branch ~/myproject implement new feature")
		return
	}

	launchGitCodeAgent(ctx, directory, task)
}

func launchGitCodeAgent(ctx context.Context, directory, task string) {
	chatID := AdminUserID
	// Resolve the directory path relative to home directory
	absDir, err := core.ResolvePath(directory)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🔍 Checking git repository status in %s...", absDir))

	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "git-agent-*")
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to create temp directory: %v", err))
		return
	}

	// Copy the repository to temp directory
	core.SendMessage(ctx, b, chatID, "📋 Copying repository to temporary workspace...")

	// Use rsync to copy the directory, excluding .git if needed
	cmd := exec.Command("rsync", "-av", "--exclude=node_modules", "--exclude=.DS_Store", absDir+"/", tempDir+"/")
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tempDir)
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to copy repository: %v\nOutput: %s", err, string(output)))
		return
	}

	// Prepare the git-specific prompt
	gitPrompt := fmt.Sprintf(`IMPORTANT GIT WORKFLOW INSTRUCTIONS:
You are working on a git repository. You MUST follow these steps:

1. First, create a new branch for your changes using: git checkout -b feature/<descriptive-name>
2. Make all the necessary changes to complete the task: %s
3. Stage and commit your changes with a descriptive commit message
   IMPORTANT: When staging files, NEVER include *_PLAN_*.md files. Use commands like:
   - git add . && git reset *_PLAN_*.md  (to add all except plan files)
   - Or stage files individually, explicitly excluding *_PLAN_*.md files
4. Try to push the branch to the remote repository using: git push -u origin <branch-name>
5. If the push fails due to authentication or permissions, that's okay - just report the status

Remember:
- Always work on a new branch, never directly on main/master
- Make atomic, well-described commits
- Include a clear commit message explaining what was changed and why
- NEVER commit *_PLAN_*.md files - they're for your planning only

Task: %s`, task, task)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching git-aware code agent in temporary workspace...\n📁 Original: %s\n📁 Workspace: %s", absDir, tempDir))

	// Launch the agent with the git-specific prompt
	agentID, err := agentManager.LaunchAgent(ctx, tempDir, gitPrompt)
	if err != nil {
		os.RemoveAll(tempDir)
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("✅ Git-aware code agent launched!\n🆔 ID: `%s`\n📝 Task: %s\n📁 Original Directory: %s\n📁 Working Directory: %s\n🌿 The agent will create a new branch and attempt to push changes\n\nUse `/status %s` to check status.",
		agentID, task, directory, tempDir, agentID))
}

func handleGitBranchCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 4 {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Please provide directory, branch name, and task.\nUsage: /edit_branch <directory> <branch> <task>\n\nExample: /edit_branch ~/myproject feature/add-auth implement authentication system")
		return
	}

	directory := strings.TrimSpace(parts[1])
	branch := strings.TrimSpace(parts[2])
	task := strings.TrimSpace(strings.Join(parts[3:], " "))

	if directory == "" || branch == "" || task == "" {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Directory, branch name, and task are all required.\n\nExample: /edit_branch ~/myproject feature/add-auth implement authentication system")
		return
	}

	launchGitBranchAgent(ctx, directory, branch, task)
}

func launchGitBranchAgent(ctx context.Context, directory, branch, task string) {
	chatID := AdminUserID
	// Resolve the directory path relative to home directory
	absDir, err := core.ResolvePath(directory)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🔍 Checking git repository and branch status in %s...", absDir))

	// Check if the branch exists
	checkBranchCmd := exec.Command("git", "branch", "--list", branch)
	checkBranchCmd.Dir = absDir
	branchOutput, _ := checkBranchCmd.CombinedOutput()

	// Check remote branches too
	checkRemoteBranchCmd := exec.Command("git", "branch", "-r", "--list", fmt.Sprintf("origin/%s", branch))
	checkRemoteBranchCmd.Dir = absDir
	remoteBranchOutput, _ := checkRemoteBranchCmd.CombinedOutput()

	branchExists := len(strings.TrimSpace(string(branchOutput))) > 0 || len(strings.TrimSpace(string(remoteBranchOutput))) > 0

	if !branchExists {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Branch '%s' does not exist locally or remotely.\n\n💡 Use `/new_branch` to create a new branch, or check the branch name and try again.", branch))
		return
	}

	// Create a temp directory
	tempDir, err := os.MkdirTemp("", "git-branch-agent-*")
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to create temp directory: %v", err))
		return
	}

	// Copy the repository to temp directory
	core.SendMessage(ctx, b, chatID, "📋 Copying repository to temporary workspace...")

	// Use rsync to copy the directory, excluding .git if needed
	cmd := exec.Command("rsync", "-av", "--exclude=node_modules", "--exclude=.DS_Store", absDir+"/", tempDir+"/")
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(tempDir)
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to copy repository: %v\nOutput: %s", err, string(output)))
		return
	}

	// Prepare the git-specific prompt for existing branch
	gitBranchPrompt := fmt.Sprintf(`IMPORTANT GIT WORKFLOW INSTRUCTIONS:
You are working on a git repository with an existing branch. You MUST follow these steps:

1. First, fetch the latest changes: git fetch origin
2. Checkout the existing branch '%s' using: git checkout %s
3. If the branch exists only on remote, use: git checkout -b %s origin/%s
4. Pull the latest changes from the remote branch: git pull origin %s
5. Make all the necessary changes to complete the task: %s
6. Stage and commit your changes with a descriptive commit message
   IMPORTANT: When staging files, NEVER include *_PLAN_*.md files. Use commands like:
   - git add . && git reset *_PLAN_*.md  (to add all except plan files)
   - Or stage files individually, explicitly excluding *_PLAN_*.md files
7. Try to push the changes to the remote repository using: git push origin %s
8. If the push fails due to authentication or permissions, that's okay - just report the status

Remember:
- You are working on the existing branch '%s'
- Make atomic, well-described commits
- Include a clear commit message explaining what was changed and why
- Ensure you're up to date with the remote branch before making changes
- NEVER commit *_PLAN_*.md files - they're for your planning only

Task: %s`, branch, branch, branch, branch, branch, task, branch, branch, task)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching git-aware code agent for existing branch...\n📁 Original: %s\n📁 Workspace: %s\n🌿 Branch: %s", absDir, tempDir, branch))

	// Launch the agent with the git branch-specific prompt
	agentID, err := agentManager.LaunchAgent(ctx, tempDir, gitBranchPrompt)
	if err != nil {
		os.RemoveAll(tempDir)
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("✅ Git-aware code agent launched for existing branch!\n🆔 ID: `%s`\n📝 Task: %s\n🌿 Branch: %s\n📁 Original Directory: %s\n📁 Working Directory: %s\n\nThe agent will work on the existing branch and attempt to push changes.\n\nUse `/status %s` to check status.",
		agentID, task, branch, directory, tempDir, agentID))
}

func handleCommitCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Please provide a directory.\nUsage: /commit <directory>\n\nExample: /commit ~/myproject")
		return
	}

	directory := strings.TrimSpace(strings.Join(parts[1:], " "))

	if directory == "" {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Directory is required.\n\nExample: /commit ~/myproject")
		return
	}

	launchCommitAgent(ctx, directory)
}

func launchCommitAgent(ctx context.Context, directory string) {
	chatID := AdminUserID
	// Resolve the directory path relative to home directory
	absDir, err := core.ResolvePath(directory)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🔍 Checking git repository status in %s...", absDir))

	// Prepare the commit-specific prompt
	commitPrompt := `IMPORTANT COMMIT INSTRUCTIONS:
You are tasked with committing the current changes in a git repository and pushing to remote. Follow these steps:

1. First, check the current git status using: git status
2. Review all uncommitted changes using: git diff
3. Stage all relevant changes (be careful not to stage files that shouldn't be committed)
   IMPORTANT: NEVER stage *_PLAN_*.md files. Use commands like:
   - git add . && git reset *_PLAN_*.md  (to add all except plan files)
   - Or stage files individually, explicitly excluding *_PLAN_*.md files
4. Create a meaningful commit with a descriptive message based on the changes
5. Push the commit to the remote repository using: git push
6. If the push fails due to authentication, that's okay - just report the status

Remember:
- Write clear, descriptive commit messages
- Don't commit files that shouldn't be in version control (like .env, node_modules, *_PLAN_*.md files, etc.)
- Make sure the commit message accurately describes what was changed
- If there are no changes to commit, report that clearly

Your task: Review the changes, commit them with an appropriate message, and push to remote.`

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching Claude Code to commit changes...\n📁 Directory: %s", absDir))

	// Launch the agent with the commit-specific prompt
	agentID, err := agentManager.LaunchAgent(ctx, absDir, commitPrompt)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("✅ Commit agent launched!\n🆔 ID: `%s`\n📁 Directory: %s\n\nThe agent will:\n• Review uncommitted changes\n• Create a meaningful commit\n• Push to the remote repository\n\nUse `/status %s` to check status.",
		agentID, directory, agentID))
}

func handleDiffCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	path := "." // Default to current directory

	if len(parts) >= 2 {
		// Join all parts after the command in case the path has spaces
		path = strings.Join(parts[1:], " ")
	}

	// Resolve the path
	absPath, err := core.ResolvePath(path)
	if err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Error resolving path: %v", err))
		return
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Path not found: %s", absPath))
		return
	}

	// Determine if it's a file or directory
	if !info.IsDir() {
		// Handle single file diff
		handleFileDiff(ctx, absPath, path)
		return
	}

	// It's a directory - check if it's a git repository
	gitDir := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Directory is not a git repository: %s", absPath))
		return
	}

	core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("📊 Checking git status in %s...", path))

	// Run git status to get modified files
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = absPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("❌ Failed to get git status: %v\nOutput: %s", err, string(output)))
		return
	}

	if len(output) == 0 {
		core.SendMessage(ctx, b, message.Chat.ID, "✅ Working directory is clean. No modified files.")
		return
	}

	// Parse the git status output
	lines := strings.Split(string(output), "\n")
	var staged []string
	var modified []string
	var untracked []string
	var deleted []string

	for _, line := range lines {
		if line == "" {
			continue
		}

		// Git status format: XY filename
		// X = staged status, Y = unstaged status
		if len(line) < 3 {
			continue
		}

		status := line[:2]
		filename := strings.TrimSpace(line[2:])

		switch {
		case status[0] == 'A' || status[0] == 'M':
			staged = append(staged, filename)
		case status[1] == 'M':
			modified = append(modified, filename)
		case status[1] == 'D':
			deleted = append(deleted, filename)
		case status == "??":
			untracked = append(untracked, filename)
		}
	}

	// Build the summary message
	var responseMsg strings.Builder
	responseMsg.WriteString(fmt.Sprintf("📊 *Git Status - %s*\n\n", path))

	totalFiles := 0
	if len(staged) > 0 {
		responseMsg.WriteString(fmt.Sprintf("✅ *Staged files:* %d\n", len(staged)))
		totalFiles += len(staged)
	}

	if len(modified) > 0 {
		responseMsg.WriteString(fmt.Sprintf("📝 *Modified files:* %d\n", len(modified)))
		totalFiles += len(modified)
	}

	if len(deleted) > 0 {
		responseMsg.WriteString(fmt.Sprintf("🗑️ *Deleted files:* %d\n", len(deleted)))
		totalFiles += len(deleted)
	}

	if len(untracked) > 0 {
		responseMsg.WriteString(fmt.Sprintf("❓ *Untracked files:* %d\n", len(untracked)))
		totalFiles += len(untracked)
	}

	responseMsg.WriteString(fmt.Sprintf("\n📈 *Total:* %d file(s) with changes", totalFiles))

	// Send the summary message
	core.SendMessage(ctx, b, message.Chat.ID, responseMsg.String())

	// Send diffs for each modified or staged file
	processedFiles := make(map[string]bool)

	// Process staged files
	for _, file := range staged {
		if !processedFiles[file] {
			processedFiles[file] = true
			sendGitDiff(ctx, absPath, file, true)
			time.Sleep(100 * time.Millisecond) // Small delay to avoid rate limiting
		}
	}

	// Process modified files
	for _, file := range modified {
		if !processedFiles[file] {
			processedFiles[file] = true
			sendGitDiff(ctx, absPath, file, false)
			time.Sleep(100 * time.Millisecond) // Small delay to avoid rate limiting
		}
	}

	// For deleted files, just send a notification
	for _, file := range deleted {
		if !processedFiles[file] {
			processedFiles[file] = true
			core.SendMessage(ctx, b, message.Chat.ID, fmt.Sprintf("🗑️ *Deleted:* `%s`", file))
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// handleFileDiff handles diff for a single file
func handleFileDiff(ctx context.Context, absPath, displayPath string) {
	chatID := AdminUserID
	// Get the directory and filename
	dir := filepath.Dir(absPath)
	filename := filepath.Base(absPath)

	// Check if the parent directory is a git repository
	gitDir := filepath.Join(dir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ File is not in a git repository: %s", displayPath))
		return
	}

	// Check git status for this specific file
	cmd := exec.Command("git", "status", "--porcelain", filename)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to get git status for file: %v", err))
		return
	}

	if len(output) == 0 {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("✅ File has no changes: `%s`", displayPath))
		return
	}

	// Parse the status
	line := strings.TrimSpace(string(output))
	if len(line) < 3 {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Invalid git status output for file: %s", displayPath))
		return
	}

	status := line[:2]
	staged := status[0] == 'A' || status[0] == 'M'

	// Send the diff
	sendGitDiff(ctx, dir, filename, staged)
}

// sendGitDiff sends the git diff for a specific file
func sendGitDiff(ctx context.Context, repoDir, filename string, staged bool) {
	chatID := AdminUserID
	var cmd *exec.Cmd
	if staged {
		// For staged files, use --cached
		cmd = exec.Command("git", "diff", "--cached", "--", filename)
	} else {
		// For unstaged files
		cmd = exec.Command("git", "diff", "--", filename)
	}
	cmd.Dir = repoDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to get diff for %s: %v", filename, err))
		return
	}

	// If no diff output (e.g., for new untracked files), try to show the file content
	if len(output) == 0 {
		// Check if it's a new file
		statusCmd := exec.Command("git", "status", "--porcelain", filename)
		statusCmd.Dir = repoDir
		statusOutput, _ := statusCmd.CombinedOutput()

		if strings.HasPrefix(string(statusOutput), "??") {
			// It's an untracked file, show its content
			content, err := os.ReadFile(filepath.Join(repoDir, filename))
			if err != nil {
				core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to read file %s: %v", filename, err))
				return
			}

			// Prepare the message
			var msg strings.Builder
			msg.WriteString(fmt.Sprintf("📄 *New file:* `%s`\n\n", filename))
			msg.WriteString("```\n")

			// Truncate content if too long
			contentStr := string(content)
			if len(contentStr) > 3000 {
				contentStr = contentStr[:2997] + "..."
			}
			msg.WriteString(contentStr)
			msg.WriteString("\n```")

			core.SendMessage(ctx, b, chatID, msg.String())
			return
		}

		// No changes to show
		return
	}

	// Prepare the diff message
	var msg strings.Builder
	statusIcon := "📝"
	if staged {
		statusIcon = "✅"
	}
	msg.WriteString(fmt.Sprintf("%s *File:* `%s`\n\n", statusIcon, filename))
	msg.WriteString("```diff\n")

	// Truncate diff if too long
	diffStr := string(output)
	if len(diffStr) > 3500 {
		diffStr = diffStr[:3497] + "..."
	}
	msg.WriteString(diffStr)
	msg.WriteString("\n```")

	// Send the diff as a separate message
	core.SendLongMessage(ctx, b, chatID, msg.String())
}

func handleReviewCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 2 {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Please provide workspace directory.\nUsage:\n• `/review <directory>` - Review pending changes\n• `/review <directory> <pr_url>` - Review PR\n\nExamples:\n• `/review ~/myproject`\n• `/review ~/myproject https://github.com/owner/repo/pull/123`")
		return
	}

	directory := strings.TrimSpace(parts[1])

	// If only directory is provided, review pending changes
	if len(parts) == 2 {
		launchPendingChangesReviewAgent(ctx, directory)
		return
	}

	// If PR URL is provided, review the PR
	prURL := strings.TrimSpace(parts[2])
	if directory == "" || prURL == "" {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Both directory and PR URL are required for PR review.\n\nExample: /review ~/myproject https://github.com/owner/repo/pull/123")
		return
	}

	launchPRReviewAgent(ctx, directory, prURL)
}

func launchPRReviewAgent(ctx context.Context, directory, prURL string) {
	chatID := AdminUserID
	// Resolve the directory path relative to home directory
	absDir, err := core.ResolvePath(directory)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🔍 Preparing PR review for %s...", prURL))

	// Prepare the PR review prompt
	prReviewPrompt := fmt.Sprintf(`IMPORTANT PR REVIEW INSTRUCTIONS:
You are tasked with reviewing a pull request. Follow these steps carefully:

1. First, use the gh CLI to get information about the PR: %s
   - Get PR details: gh pr view %s
   - Get PR diff: gh pr diff %s
   - Get PR checks status: gh pr checks %s

2. Analyze the code changes:
   - Look for potential bugs, security issues, or performance problems
   - Check if the implementation aligns with the requirements
   - Verify that the code follows project conventions and best practices
   - Check for missing tests or documentation

4. Write a concise PR review and send it to Telegram:
   - DO NOT use gh pr comment or post anything to the PR
   - Structure your review as follows:
     * List any bugs or issues found (if any)
     * Code improvement suggestions (following the project conventions and best practices) (if any)
     * Final verdict: Approve, Request Changes, or Needs Discussion

Remember:
- Be concise
- Point out specific line numbers or files when mentioning issues
- If everything looks good, don't say anything
- Send your review message directly to the output (it will be sent to Telegram)

PR URL: %s`, prURL, prURL, prURL, prURL, prURL)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching PR review agent...\n📁 Repository: %s\n🔗 PR: %s", absDir, prURL))

	// Launch the agent with the PR review prompt and unique plan file
	planFilename := generateUniquePlanFilename("PR_REVIEW")
	agentID, err := agentManager.LaunchAgentWithPlanFile(ctx, absDir, prReviewPrompt, planFilename)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("✅ PR review agent launched!\n🆔 ID: `%s`\n🔗 PR: %s\n📁 Repository: %s\n\nThe agent will:\n• Analyze the PR changes\n• Review code quality and bugs\n• Send the review to this Telegram chat\n\nUse `/status %s` to check status.",
		agentID, prURL, directory, agentID))
}

func launchPendingChangesReviewAgent(ctx context.Context, directory string) {
	chatID := AdminUserID
	// Resolve the directory path relative to home directory
	absDir, err := core.ResolvePath(directory)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	core.SendMessage(ctx, b, chatID, "🔍 Preparing to review pending changes...")

	// Prepare the pending changes review prompt
	pendingChangesPrompt := `IMPORTANT PENDING CHANGES REVIEW INSTRUCTIONS:
You are tasked with reviewing the pending changes in this git repository. Follow these steps carefully:

1. First, check the current git status:
   - Run: git status
   - Run: git diff --staged
   - Run: git diff

2. Analyze the pending changes:
   - Look for potential bugs, security issues, or performance problems
   - Check if the implementation aligns with the requirements
   - Verify that the code follows project conventions and best practices
   - Check for missing tests or documentation
   - Review both staged and unstaged changes

4. Write a concise review of the pending changes:
   - DO NOT make any commits or push changes
   - Structure your review as follows:
     * Summary of changes (files modified, added, deleted)
     * List any bugs or issues found (if any)
     * Code improvement suggestions (following the project conventions and best practices) (if any)
     * Recommendations for next steps (e.g., ready to commit, needs more work, etc.)

Remember:
- Be concise and focused on the actual changes
- Point out specific files and line numbers when mentioning issues
- If everything looks good, say so briefly
- Send your review message directly to the output (it will be sent to Telegram)`

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching pending changes review agent...\n📁 Repository: %s", absDir))

	// Launch the agent with the pending changes review prompt and unique plan file
	planFilename := generateUniquePlanFilename("REVIEW")
	agentID, err := agentManager.LaunchAgentWithPlanFile(ctx, absDir, pendingChangesPrompt, planFilename)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("✅ Pending changes review agent launched!\n🆔 ID: `%s`\n📁 Repository: %s\n\nThe agent will:\n• Check git status and diffs\n• Review code quality and bugs\n• Send the review to this Telegram chat\n\nUse `/status %s` to check status.",
		agentID, directory, agentID))
}

func handlePRCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 3 {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Please provide workspace directory and PR URL.\nUsage: `/pr <directory> <pr_url>`\n\nExample: `/pr ~/myproject https://github.com/owner/repo/pull/123`")
		return
	}

	directory := strings.TrimSpace(parts[1])
	prURL := strings.TrimSpace(parts[2])

	if directory == "" || prURL == "" {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Both directory and PR URL are required.\n\nExample: /pr ~/myproject https://github.com/owner/repo/pull/123")
		return
	}

	launchPRCommentAgent(ctx, message.Chat.ID, directory, prURL)
}

func launchPRCommentAgent(ctx context.Context, chatID int64, directory, prURL string) {
	// Resolve the directory path relative to home directory
	absDir, err := core.ResolvePath(directory)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🔍 Preparing PR review for %s...", prURL))

	// Prepare the PR review and comment prompt
	prCommentPrompt := fmt.Sprintf(`IMPORTANT PR REVIEW AND COMMENT INSTRUCTIONS:
You must review this pull request and post your review using the gh CLI.

1. Get PR information:
   - gh pr view %s
   - gh pr diff %s
   - gh pr checks %s

2. Analyze the code for issues ONLY:
   - Bugs, security vulnerabilities, logic errors
   - Breaking changes or regressions
   - Missing critical tests for new functionality
   - Verify that the code follows project conventions and best practices
   - Check for missing tests
   
   DO NOT comment on:
   - Code style preferences
   - Minor improvements that work correctly
   - Things that are already good

3. Post your review using EXACTLY ONE command:
   
   If NO ISSUES found:
   gh pr review %s --approve --body "LGTM"
   
   If ISSUES found:
   gh pr review %s --request-changes --body "- Issue 1: [specific problem at file:line]
- Issue 2: [specific problem at file:line]"

CRITICAL RULES:
- ONLY list actual problems that need fixing
- NO summaries, NO strengths, NO general observations
- If code works correctly, just approve with "LGTM"
- Execute ONLY ONE gh pr review command
- DO NOT run the command twice

PR URL: %s`, prURL, prURL, prURL, prURL, prURL, prURL)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching PR review agent...\n📁 Repository: %s\n🔗 PR: %s", absDir, prURL))

	// Launch the agent with the PR comment prompt and unique plan file
	planFilename := generateUniquePlanFilename("PR_COMMENT")
	agentID, err := agentManager.LaunchAgentWithPlanFile(ctx, absDir, prCommentPrompt, planFilename)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("✅ PR review agent launched!\n🆔 ID: `%s`\n🔗 PR: %s\n📁 Repository: %s\n\nThe agent will:\n• Analyze the PR changes\n• Post a review comment on the PR\n• Approve the PR if it's ready to merge\n\nUse `/status %s` to check status.",
		agentID, prURL, directory, agentID))
}

func handleApproveCommand(ctx context.Context, message *models.Message) {
	parts := strings.Fields(message.Text)
	if len(parts) < 3 {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Please provide workspace directory and PR URL.\nUsage: `/approve <directory> <pr_url>`\n\nExample: `/approve ~/myproject https://github.com/owner/repo/pull/123`")
		return
	}

	directory := strings.TrimSpace(parts[1])
	prURL := strings.TrimSpace(parts[2])

	if directory == "" || prURL == "" {
		core.SendMessage(ctx, b, message.Chat.ID, "❌ Both directory and PR URL are required.\n\nExample: /approve ~/myproject https://github.com/owner/repo/pull/123")
		return
	}

	launchPRApprovalAgent(ctx, message.Chat.ID, directory, prURL)
}

func launchPRApprovalAgent(ctx context.Context, chatID int64, directory, prURL string) {
	// Resolve the directory path relative to home directory
	absDir, err := core.ResolvePath(directory)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Error resolving directory path: %v", err))
		return
	}

	// Check if directory exists
	info, err := os.Stat(absDir)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory not found: %s", absDir))
		return
	}
	if !info.IsDir() {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Path is not a directory: %s", absDir))
		return
	}

	// Check if it's a git repository
	gitDir := filepath.Join(absDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Directory is not a git repository: %s", absDir))
		return
	}

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🔍 Preparing PR approval for %s...", prURL))

	// Prepare the PR approval prompt - perform checks but always approve
	prApprovalPrompt := fmt.Sprintf(`IMPORTANT PR REVIEW AND APPROVAL INSTRUCTIONS:
You must review this pull request and then approve it using the gh CLI.

1. Get PR information:
   - gh pr view %s
   - gh pr diff %s
   - gh pr checks %s

2. Analyze the code for issues:
   - Bugs, security vulnerabilities, logic errors
   - Breaking changes or regressions
   - Missing critical tests for new functionality
   - Verify that the code follows project conventions and best practices
   - Check for missing tests
   
   DO NOT comment on:
   - Code style preferences
   - Minor improvements that work correctly
   - Things that are already good

3. Post your review using EXACTLY ONE command:
   
   If NO ISSUES found:
   gh pr review %s --approve --body "LGTM ✅"
   
   If ISSUES found (but still approve):
   gh pr review %s --approve --body "LGTM ✅

However, I found the following issues that should be addressed in a follow-up:
- Issue 1: [specific problem at file:line]
- Issue 2: [specific problem at file:line]"

CRITICAL RULES:
- You MUST always use the --approve flag regardless of issues found
- ONLY list actual problems that need fixing
- Execute ONLY ONE gh pr review command
- DO NOT run the command twice
- Always approve even if there are issues

PR URL: %s`, prURL, prURL, prURL, prURL, prURL, prURL)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("🚀 Launching PR approval agent...\n📁 Repository: %s\n🔗 PR: %s", absDir, prURL))

	// Launch the agent with the PR approval prompt and unique plan file
	planFilename := generateUniquePlanFilename("PR_APPROVE")
	agentID, err := agentManager.LaunchAgentWithPlanFile(ctx, absDir, prApprovalPrompt, planFilename)
	if err != nil {
		core.SendMessage(ctx, b, chatID, fmt.Sprintf("❌ Failed to launch agent: %v", err))
		return
	}

	// Register the agent for this user to receive notifications
	RegisterAgentForUser(agentID, AdminUserID)

	core.SendMessage(ctx, b, chatID, fmt.Sprintf("✅ PR approval agent launched!\n🆔 ID: `%s`\n🔗 PR: %s\n📁 Repository: %s\n\nThe agent will:\n• Review the PR for issues\n• Post findings as comments\n• Always approve the PR (even with issues)\n\nUse `/status %s` to check status.",
		agentID, prURL, directory, agentID))
}
