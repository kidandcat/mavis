// Mavis Web Application - Pure JavaScript implementation

// Global state
let eventSource = null;
let currentPage = 'agents';
// Removed selectedAgentId and agentStatusInterval - no longer needed
let reconnectAttempts = 0;

// Initialize application
document.addEventListener('DOMContentLoaded', function() {
    initializeEventSource();
    setupNavigation();
    setupModalHandlers();
    initializeAnimatedBackground();
    initializeNeonCursor();
    
    // Get current page from URL
    const path = window.location.pathname;
    if (path && path !== '/') {
        currentPage = path.substring(1);
    }
    updateActiveNavItem();
});

// Server-Sent Events setup
function initializeEventSource() {
    eventSource = new EventSource('/events');
    
    eventSource.addEventListener('connected', function(e) {
        console.log('Connected to Mavis SSE:', e.data);
        reconnectAttempts = 0; // Reset reconnect attempts on successful connection
    });
    
    eventSource.addEventListener('agents-update', function(e) {
        const agents = JSON.parse(e.data);
        updateAgentsUI(agents);
    });
    
    eventSource.addEventListener('error', function(e) {
        console.error('SSE Error Details:', {
            readyState: eventSource.readyState,
            readyStateText: ['CONNECTING', 'OPEN', 'CLOSED'][eventSource.readyState],
            url: eventSource.url,
            error: e
        });
        
        // Close the existing connection to ensure clean reconnect
        if (eventSource) {
            eventSource.close();
        }
        
        // Reconnect with exponential backoff
        const reconnectDelay = Math.min(30000, (reconnectAttempts + 1) * 2000);
        reconnectAttempts++;
        
        console.log(`SSE connection lost. Reconnecting in ${reconnectDelay/1000} seconds...`);
        setTimeout(() => {
            initializeEventSource();
        }, reconnectDelay);
    });
}

// Update agents UI from SSE data
function updateAgentsUI(agents) {
    if (currentPage !== 'agents') return;
    
    // Categorize agents by status
    const queuedAgents = [];
    const runningAgents = [];
    const finishedAgents = [];
    
    agents.forEach(agent => {
        switch (agent.Status) {
            case 'queued':
                queuedAgents.push(agent);
                break;
            case 'active':
            case 'running':
                runningAgents.push(agent);
                break;
            case 'finished':
            case 'failed':
            case 'killed':
            case 'stopped':
            case 'error':
                finishedAgents.push(agent);
                break;
            default:
                // Default to running for unknown statuses
                runningAgents.push(agent);
        }
    });
    
    // Update each column
    updateColumn('queue-column', queuedAgents);
    updateColumn('running-column', runningAgents);
    updateColumn('finished-column', finishedAgents);
    
    // Update counts
    updateColumnCount('Queue', queuedAgents.length);
    updateColumnCount('Running', runningAgents.length);
    updateColumnCount('Finished', finishedAgents.length);
    
    // Update progress for all agents that should show progress
    // This ensures progress is displayed after page refresh
    agents.forEach(agent => {
        if (agent.Status === 'running' || agent.Status === 'active' || agent.Status === 'queued') {
            updateAgentProgress(agent.ID, agent.Status);
        }
    });
}

function updateColumn(columnId, agents) {
    const column = document.getElementById(columnId);
    if (!column) return;
    
    // Create a set of current agent IDs in this column
    const currentAgentIds = new Set(agents.map(agent => agent.ID));
    
    // Remove agents that are no longer in this column
    const existingCards = column.querySelectorAll('.agent-card');
    existingCards.forEach(card => {
        const agentId = card.getAttribute('data-agent-id');
        if (!currentAgentIds.has(agentId)) {
            card.remove();
        }
    });
    
    // Update existing agents and add new ones
    agents.forEach(agent => {
        const card = document.getElementById(`agent-${agent.ID}`);
        if (card && card.parentElement.id === columnId) {
            // Update existing card in same column
            updateAgentCard(card, agent);
        } else {
            // Add new card or move from another column
            const newCard = createAgentCard(agent);
            if (card) {
                // Remove from old column
                card.remove();
            }
            // Add to new column
            column.insertAdjacentHTML('beforeend', newCard);
            // Update progress for newly added cards
            updateAgentProgress(agent.ID, agent.Status);
        }
    });
}

function updateColumnCount(columnName, count) {
    const headers = document.querySelectorAll('.kanban-header h3');
    headers.forEach(header => {
        if (header.textContent === columnName) {
            const countSpan = header.nextElementSibling;
            if (countSpan && countSpan.classList.contains('kanban-count')) {
                countSpan.textContent = `(${count})`;
            }
        }
    });
}

function updateAgentCard(card, agent) {
    // If status changed to finished, recreate the entire card
    const currentStatus = card.querySelector('.agent-status')?.textContent;
    if (currentStatus !== 'finished' && agent.Status === 'finished') {
        // Replace the entire card with the new finished format
        const newCard = createAgentCard(agent);
        card.outerHTML = newCard;
        return;
    }
    
    // Update existing card for non-finished agents
    const statusElem = card.querySelector('.agent-status');
    if (statusElem) statusElem.textContent = agent.Status;
    
    // Update stats
    const messagesElem = card.querySelector('.stat-messages');
    if (messagesElem) messagesElem.textContent = agent.MessagesSent;
    
    const queueElem = card.querySelector('.stat-queue');
    if (queueElem) queueElem.textContent = agent.QueueStatus;
    
    // Update status class
    card.className = `agent-card ${getStatusClass(agent)}`;
    
    // Update actions (stop button for running, delete button for completed)
    const actionsDiv = card.querySelector('.agent-actions');
    if (actionsDiv) {
        if (agent.Status === 'active' || agent.Status === 'running') {
            if (!actionsDiv.querySelector('.btn-danger')) {
                actionsDiv.innerHTML = `<button class="btn btn-sm btn-danger" onclick="event.stopPropagation(); stopAgent('${agent.ID}')">Stop</button>`;
            }
        } else if (agent.Status === 'finished' || agent.Status === 'failed' || agent.Status === 'killed' || agent.Status === 'stopped') {
            // Show delete button for completed agents
            actionsDiv.innerHTML = `<button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); deleteAgent('${agent.ID}')">Delete</button>`;
        } else {
            actionsDiv.innerHTML = '';
        }
    }
    
    // Update progress for running agents
    updateAgentProgress(agent.ID, agent.Status);
}

function getStatusClass(agent) {
    if (agent.Status === 'finished') {
        return 'finished';
    } else if (agent.Status === 'stopped' || agent.Status === 'error' || agent.Status === 'failed' || agent.Status === 'killed') {
        return 'failed';
    } else if (agent.Status === 'queued') {
        return 'queued';
    } else if (agent.Status === 'running' || agent.Status === 'active') {
        return 'running';
    } else if (agent.IsStale) {
        return 'stale';
    }
    return 'running'; // default to running
}

async function updateAgentProgress(agentId, status) {
    const progressDiv = document.getElementById(`progress-${agentId}`);
    if (!progressDiv) return;
    
    if (status === 'running' || status === 'active' || status === 'queued') {
        try {
            const response = await fetch(`/api/agent/${agentId}/status?progress-only=true`);
            if (!response.ok) throw new Error('Failed to get agent progress');
            
            const data = await response.json();
            const progressContent = progressDiv.querySelector('.progress-content');
            
            if (data.progress && data.progress.trim()) {
                progressContent.innerHTML = `<pre>${escapeHtml(data.progress)}</pre>`;
                progressDiv.style.display = 'block';
            } else {
                progressDiv.style.display = 'none';
            }
        } catch (error) {
            console.error('Error getting agent progress:', error);
            progressDiv.style.display = 'none';
        }
    } else {
        progressDiv.style.display = 'none';
    }
}

// Navigation setup
function setupNavigation() {
    const navItems = document.querySelectorAll('.navbar-item[data-page]');
    navItems.forEach(item => {
        item.addEventListener('click', function(e) {
            e.preventDefault();
            const page = this.getAttribute('data-page');
            navigateToPage(page);
        });
    });
}

function navigateToPage(page) {
    currentPage = page;
    history.pushState({page: page}, '', `/${page}`);
    loadPage(page);
    updateActiveNavItem();
    
    // No cleanup needed when navigating away
}

function updateActiveNavItem() {
    document.querySelectorAll('.navbar-item').forEach(item => {
        if (item.getAttribute('data-page') === currentPage) {
            item.classList.add('active');
        } else {
            item.classList.remove('active');
        }
    });
}

async function loadPage(page) {
    try {
        const mainContent = document.getElementById('main-content');
        
        // For Agents, we fetch JSON data and render client-side
        if (page === 'agents') {
            const response = await fetch(`/${page}`, {
                headers: {
                    'Accept': 'application/json'
                }
            });
            
            if (!response.ok) throw new Error('Failed to load page');
            
            const agents = await response.json();
            mainContent.innerHTML = renderAgentsSection(agents);
        } else {
            // For Files, Git, and System tabs, we fetch the HTML content directly
            const response = await fetch(`/${page}`);
            if (!response.ok) throw new Error('Failed to load page');
            
            const html = await response.text();
            // Extract the content from the full page HTML
            const parser = new DOMParser();
            const doc = parser.parseFromString(html, 'text/html');
            const content = doc.getElementById(`${page}-section`);
            
            if (content) {
                mainContent.innerHTML = content.outerHTML;
                
                // Re-initialize any JavaScript handlers for the loaded content
                if (page === 'git') {
                    // Don't auto-load git diff anymore - user needs to select folder first
                }
            } else {
                mainContent.innerHTML = `<div class="section"><h2>${page.charAt(0).toUpperCase() + page.slice(1)}</h2><p>Failed to load content</p></div>`;
            }
        }
    } catch (error) {
        console.error('Error loading page:', error);
        const mainContent = document.getElementById('main-content');
        mainContent.innerHTML = `<div class="section"><h2>Error</h2><p>Failed to load page: ${error.message}</p></div>`;
    }
}

// Modal handlers
function setupModalHandlers() {
    // Close modals on outside click
    document.addEventListener('click', function(event) {
        if (event.target.classList.contains('modal')) {
            event.target.style.display = 'none';
        }
    });
}

function showCreateAgentModal() {
    document.getElementById('create-agent-modal').style.display = 'flex';
}

function hideCreateAgentModal() {
    document.getElementById('create-agent-modal').style.display = 'none';
}

function closeModal() {
    const modals = document.querySelectorAll('.modal');
    modals.forEach(modal => modal.style.display = 'none');
}

// Agent operations
async function createAgent() {
    const form = document.getElementById('create-agent-form');
    const formData = new FormData(form);
    
    try {
        const response = await fetch('/api/code', {
            method: 'POST',
            body: formData
        });
        
        const data = await response.json();
        
        if (!response.ok) {
            throw new Error(data.error || 'Failed to create agent');
        }
        
        // Check if agent was queued
        if (data.ID && data.ID.startsWith('queued-')) {
            // Agent was queued, show notification
            const queuePos = data.QueuePosition || 'unknown';
            // alert(`Agent queued! Position in queue: ${queuePos}. It will start automatically when the current agent in this directory completes.`);
            
            // Add queued agent to queue column
            const queueColumn = document.getElementById('queue-column');
            if (queueColumn) {
                const newCard = createAgentCard(data);
                queueColumn.insertAdjacentHTML('beforeend', newCard);
                // Update progress for newly created agent
                updateAgentProgress(data.ID, data.Status);
            }
        } else {
            // Add new agent to running column
            const runningColumn = document.getElementById('running-column');
            if (runningColumn) {
                const newCard = createAgentCard(data);
                runningColumn.insertAdjacentHTML('beforeend', newCard);
                // Update progress for newly created agent
                updateAgentProgress(data.ID, data.Status);
            }
        }
        
        // Reset form and close modal
        form.reset();
        hideCreateAgentModal();
    } catch (error) {
        console.error('Error creating agent:', error);
        alert('Failed to create agent: ' + error.message);
    }
}

// Removed viewAgentStatus function as it's no longer needed

async function stopAgent(agentID) {
    if (!confirm('Are you sure you want to stop this agent?')) return;
    
    try {
        const response = await fetch(`/api/agent/${agentID}/stop`, {
            method: 'POST'
        });
        
        if (!response.ok) throw new Error('Failed to stop agent');
        
        // Update UI
        const card = document.getElementById(`agent-${agentID}`);
        if (card) {
            card.querySelector('.agent-status').textContent = 'stopped';
            card.className = 'agent-card status-stopped';
            
            // Remove stop button
            const stopBtn = card.querySelector('.btn-danger');
            if (stopBtn) stopBtn.remove();
        }
    } catch (error) {
        console.error('Error stopping agent:', error);
        alert('Failed to stop agent');
    }
}

async function deleteAgent(agentID) {
    try {
        const response = await fetch(`/api/agent/${agentID}/delete`, {
            method: 'DELETE'
        });
        
        if (!response.ok) throw new Error('Failed to delete agent');
        
        // Remove the card from UI
        const card = document.getElementById(`agent-${agentID}`);
        if (card) {
            card.remove();
            
            // Agent deleted
        }
    } catch (error) {
        console.error('Error deleting agent:', error);
        alert('Failed to delete agent');
    }
}

// Rendering functions
function renderAgentsSection(agents) {
    // Categorize agents by status
    const queuedAgents = [];
    const runningAgents = [];
    const finishedAgents = [];
    
    agents.forEach(agent => {
        switch (agent.Status) {
            case 'queued':
                queuedAgents.push(agent);
                break;
            case 'active':
            case 'running':
                runningAgents.push(agent);
                break;
            case 'finished':
            case 'failed':
            case 'killed':
            case 'stopped':
            case 'error':
                finishedAgents.push(agent);
                break;
            default:
                // Default to running for unknown statuses
                runningAgents.push(agent);
        }
    });
    
    return `
        <div id="agents-section" class="section">
            <div class="section-header">
                <h2>Agents</h2>
                <button class="btn btn-primary" onclick="showCreateAgentModal()">+ New Agent</button>
            </div>
            <div class="kanban-container">
                <!-- Queue Column -->
                <div class="kanban-column">
                    <div class="kanban-header">
                        <h3>Queue</h3>
                        <span class="kanban-count">(${queuedAgents.length})</span>
                    </div>
                    <div id="queue-column" class="kanban-cards">
                        ${queuedAgents.map(agent => createAgentCard(agent)).join('')}
                    </div>
                </div>
                <!-- Running Column -->
                <div class="kanban-column">
                    <div class="kanban-header">
                        <h3>Running</h3>
                        <span class="kanban-count">(${runningAgents.length})</span>
                    </div>
                    <div id="running-column" class="kanban-cards">
                        ${runningAgents.map(agent => createAgentCard(agent)).join('')}
                    </div>
                </div>
                <!-- Finished Column -->
                <div class="kanban-column">
                    <div class="kanban-header">
                        <h3>Finished</h3>
                        <span class="kanban-count">(${finishedAgents.length})</span>
                    </div>
                    <div id="finished-column" class="kanban-cards">
                        ${finishedAgents.map(agent => createAgentCard(agent)).join('')}
                    </div>
                </div>
            </div>
            ${createAgentModal()}
        </div>
    `;
}

function formatDuration(duration) {
    // Duration is in nanoseconds from Go
    const totalSeconds = Math.floor(duration / 1000000000);
    const minutes = Math.floor(totalSeconds / 60);
    const seconds = totalSeconds % 60;
    
    if (minutes > 0) {
        return `${minutes}m ${seconds}s`;
    }
    return `${seconds}s`;
}

function createAgentCard(agent) {
    const statusClass = getStatusClass(agent);
    const agentId = agent.ID || agent.id;
    
    // For finished agents, show only output and time taken
    if (agent.Status === 'finished') {
        const duration = agent.Duration ? formatDuration(agent.Duration) : 'N/A';
        const output = agent.Output || 'No output available';
        
        return `
            <div id="agent-${agentId}" class="agent-card ${statusClass}" data-agent-id="${agentId}">
                <div class="agent-header">
                    <h3>Agent ${agentId.substring(0, 8)}</h3>
                    <span class="agent-status">${agent.Status}</span>
                </div>
                <div class="agent-output">
                    <pre>${escapeHtml(output)}</pre>
                </div>
                <div class="agent-time">
                    <span class="time-label">Time taken:</span>
                    <span class="time-value">${duration}</span>
                </div>
                <div class="agent-actions">
                    <button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); deleteAgent('${agentId}')">Delete</button>
                </div>
            </div>
        `;
    }
    
    // For other statuses, keep the original display
    return `
        <div id="agent-${agentId}" class="agent-card ${statusClass}" data-agent-id="${agentId}">
            <div class="agent-header">
                <h3>Agent ${agentId.substring(0, 8)}</h3>
                <span class="agent-status">${agent.Status}</span>
            </div>
            <div class="agent-task">
                <p>${agent.Task}</p>
            </div>
            <div class="agent-stats">
                <div class="stat">
                    <span class="stat-label">Started:</span>
                    <span class="stat-value">${new Date(agent.StartTime).toLocaleTimeString()}</span>
                </div>
                <div class="stat">
                    <span class="stat-label">Messages:</span>
                    <span class="stat-value stat-messages">${agent.MessagesSent}</span>
                </div>
                <div class="stat">
                    <span class="stat-label">Queue:</span>
                    <span class="stat-value stat-queue">${agent.QueueStatus}</span>
                </div>
            </div>
            <div class="agent-progress" id="progress-${agentId}" style="display: none;">
                <div class="progress-header">Progress:</div>
                <div class="progress-content"></div>
            </div>
            <div class="agent-actions">
                ${(agent.Status === 'active' || agent.Status === 'running') ? `<button class="btn btn-sm btn-danger" onclick="event.stopPropagation(); stopAgent('${agentId}')">Stop</button>` : 
                  (agent.Status === 'finished' || agent.Status === 'failed' || agent.Status === 'killed' || agent.Status === 'stopped') ? `<button class="btn btn-sm btn-secondary" onclick="event.stopPropagation(); deleteAgent('${agentId}')">Delete</button>` : ''
                }
            </div>
        </div>
    `;
}

function createAgentModal() {
    return `
        <div id="create-agent-modal" class="modal" style="display: none;">
            <div class="modal-content">
                <div class="modal-header">
                    <h3>Create New Agent</h3>
                    <button class="close-btn" onclick="hideCreateAgentModal()">×</button>
                </div>
                <form id="create-agent-form" onsubmit="event.preventDefault(); createAgent();">
                    <div class="form-group">
                        <label for="task">Task Description</label>
                        <textarea id="task" name="task" rows="4" required placeholder="Enter the task for the agent..."></textarea>
                    </div>
                    <div class="form-group">
                        <label for="work_dir">Working Directory (optional)</label>
                        <input type="text" id="work_dir" name="work_dir" placeholder="Leave empty for current dir or use . or /absolute/path">
                    </div>
                    <div class="form-actions">
                        <button type="submit" class="btn btn-primary">Create Agent</button>
                        <button type="button" class="btn btn-secondary" onclick="hideCreateAgentModal()">Cancel</button>
                    </div>
                </form>
            </div>
        </div>
    `;
}

// Removed renderFilesSection, renderGitSection, and renderSystemSection functions
// These tabs now load HTML content directly from the server

// File browser functions
function navigateToPath(path) {
    window.location.href = `/files?path=${encodeURIComponent(path)}`;
}

// Git functions
async function refreshGitDiff() {
    const folderInput = document.getElementById('git-folder');
    const folder = folderInput ? folderInput.value : '.';
    
    try {
        const response = await fetch(`/api/git/diff?path=${encodeURIComponent(folder)}`);
        const data = await response.json();
        
        const container = document.getElementById('git-diff-container');
        if (container) {
            if (data.error) {
                container.innerHTML = `<div class="error">Error: ${escapeHtml(data.error)}</div>`;
            } else if (data.diff) {
                // Format diff with syntax highlighting
                const lines = data.diff.split('\n');
                let html = '<div class="git-diff"><pre>';
                
                lines.forEach(line => {
                    let className = '';
                    if (line.startsWith('+') && !line.startsWith('+++')) {
                        className = 'diff-add';
                    } else if (line.startsWith('-') && !line.startsWith('---')) {
                        className = 'diff-remove';
                    } else if (line.startsWith('@@')) {
                        className = 'diff-hunk';
                    } else if (line.startsWith('diff --git')) {
                        className = 'diff-header';
                    }
                    
                    if (className) {
                        html += `<span class="${className}">${escapeHtml(line)}</span>\n`;
                    } else {
                        html += escapeHtml(line) + '\n';
                    }
                });
                
                html += '</pre></div>';
                container.innerHTML = html;
            } else {
                container.innerHTML = '<div class="no-changes">No changes to commit</div>';
            }
        }
    } catch (error) {
        console.error('Error refreshing git diff:', error);
        const container = document.getElementById('git-diff-container');
        if (container) {
            container.innerHTML = '<div class="error">Failed to load git diff</div>';
        }
    }
}

async function submitGitCommit(event) {
    event.preventDefault();
    const form = event.target;
    
    // Get the folder path from the git-folder input
    const folderInput = document.getElementById('git-folder');
    const folder = folderInput ? folderInput.value : '.';
    
    const requestData = {
        folder: folder
    };
    
    try {
        const response = await fetch('/api/git/commit', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(requestData)
        });
        
        const data = await response.json();
        const resultDiv = document.getElementById('git-result');
        
        if (response.ok && data.success) {
            resultDiv.innerHTML = `<div class="notification success">${data.message || 'AI commit agent launched successfully'}</div>`;
            // Don't refresh git diff immediately since the AI is working
            setTimeout(() => {
                resultDiv.innerHTML = '';
            }, 5000);
        } else {
            resultDiv.innerHTML = `<div class="notification error">${data.error || 'Failed to launch commit agent'}</div>`;
        }
    } catch (error) {
        console.error('Error launching commit agent:', error);
        document.getElementById('git-result').innerHTML = '<div class="notification error">Failed to launch commit agent</div>';
    }
}

// System management functions
async function addUser(event) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);
    
    const userData = {
        name: formData.get('name'),
        admin: formData.get('admin') === 'true'
    };
    
    try {
        const response = await fetch('/api/users', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(userData)
        });
        
        if (response.ok) {
            // Reload the page to show new user
            window.location.reload();
        } else {
            alert('Failed to add user');
        }
    } catch (error) {
        console.error('Error adding user:', error);
        alert('Failed to add user');
    }
}

async function deleteUser(userId) {
    if (!confirm('Are you sure you want to delete this user?')) return;
    
    try {
        const response = await fetch(`/api/users/${userId}`, {
            method: 'DELETE'
        });
        
        if (response.ok) {
            // Remove user row from table
            const userRow = document.getElementById(`user-${userId}`);
            if (userRow) userRow.remove();
        } else {
            alert('Failed to delete user');
        }
    } catch (error) {
        console.error('Error deleting user:', error);
        alert('Failed to delete user');
    }
}

async function runCommand(event) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);
    
    try {
        const response = await fetch('/api/command/run', {
            method: 'POST',
            body: formData
        });
        
        const data = await response.json();
        const outputDiv = document.getElementById('command-output');
        
        if (response.ok) {
            outputDiv.innerHTML = `<pre class="success">${data.output}</pre>`;
        } else {
            outputDiv.innerHTML = `<pre class="error">${data.error || 'Command failed'}</pre>`;
        }
    } catch (error) {
        console.error('Error running command:', error);
        document.getElementById('command-output').innerHTML = '<pre class="error">Failed to run command</pre>';
    }
}

async function uploadImage(event) {
    event.preventDefault();
    const form = event.target;
    const formData = new FormData(form);
    
    try {
        const response = await fetch('/api/images', {
            method: 'POST',
            body: formData
        });
        
        const data = await response.json();
        const resultDiv = document.getElementById('upload-result');
        
        if (response.ok) {
            resultDiv.innerHTML = `
                <div class="upload-result success">
                    <p>Image uploaded successfully</p>
                    <div>
                        <img src="${data.url}" alt="Uploaded image" style="max-width: 300px;">
                        <p><a href="${data.url}" target="_blank">View full size</a></p>
                    </div>
                </div>
            `;
            form.reset();
        } else {
            resultDiv.innerHTML = `<div class="upload-result error">Upload failed: ${data.error || 'Unknown error'}</div>`;
        }
    } catch (error) {
        console.error('Error uploading image:', error);
        document.getElementById('upload-result').innerHTML = '<div class="upload-result error">Failed to upload image</div>';
    }
}

async function restartMavis() {
    const messageDiv = document.getElementById('restart-message');
    
    if (!confirm('Are you sure you want to restart Mavis? This will terminate all running agents.')) {
        return;
    }
    
    try {
        messageDiv.innerHTML = '<p class="info">Sending restart command...</p>';
        
        const response = await fetch('/api/system/restart', {
            method: 'POST'
        });
        
        const data = await response.json();
        
        if (response.ok) {
            messageDiv.innerHTML = '<p class="success">Mavis is restarting. The page will become unresponsive momentarily...</p>';
            
            // After restart message is shown, the server will restart
            // Show a message about reconnecting
            setTimeout(() => {
                messageDiv.innerHTML = '<p class="warning">Connection lost. Attempting to reconnect...</p>';
                attemptReconnect();
            }, 2000);
        } else {
            messageDiv.innerHTML = `<p class="error">Restart failed: ${data.error || 'Unknown error'}</p>`;
        }
    } catch (error) {
        console.error('Error restarting:', error);
        messageDiv.innerHTML = '<p class="error">Failed to send restart command</p>';
    }
}

function attemptReconnect() {
    let reconnectAttempts = 0;
    const maxAttempts = 30; // Try for about 30 seconds
    const messageDiv = document.getElementById('restart-message');
    
    const reconnectInterval = setInterval(async () => {
        reconnectAttempts++;
        
        try {
            // Try to ping the server
            const response = await fetch('/api/agents', {
                headers: {
                    'Accept': 'application/json'
                }
            });
            
            if (response.ok) {
                clearInterval(reconnectInterval);
                messageDiv.innerHTML = '<p class="success">Reconnected! Refreshing page...</p>';
                setTimeout(() => {
                    window.location.reload();
                }, 1000);
            }
        } catch (error) {
            // Server not ready yet
            if (reconnectAttempts >= maxAttempts) {
                clearInterval(reconnectInterval);
                messageDiv.innerHTML = '<p class="error">Failed to reconnect. Please refresh the page manually.</p>';
            } else {
                messageDiv.innerHTML = `<p class="warning">Reconnecting... (${reconnectAttempts}/${maxAttempts})</p>`;
            }
        }
    }, 1000);
}

// Removed agent selection functionality - progress is now shown inline

// Removed refreshAgentStatus - progress is now updated inline

// Utility function to escape HTML
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// Initialize git diff on load if on git page
document.addEventListener('DOMContentLoaded', function() {
    const gitContainer = document.getElementById('git-diff-container');
    if (gitContainer && gitContainer.getAttribute('data-load-on-init') === 'true') {
        refreshGitDiff();
    }
});

// Initialize animated background elements
function initializeAnimatedBackground() {
    // Create neon orbs
    const orbContainer = document.createElement('div');
    orbContainer.className = 'neon-orbs';
    for (let i = 0; i < 3; i++) {
        const orb = document.createElement('div');
        orb.className = 'neon-orb';
        orbContainer.appendChild(orb);
    }
    document.body.appendChild(orbContainer);
    
    // Create matrix rain effect
    const matrixContainer = document.createElement('div');
    matrixContainer.className = 'matrix-rain';
    const columns = Math.floor(window.innerWidth / 20);
    
    for (let i = 0; i < columns; i++) {
        const column = document.createElement('div');
        column.className = 'matrix-column';
        column.style.left = `${i * 20}px`;
        column.style.animationDuration = `${Math.random() * 10 + 5}s`;
        column.style.animationDelay = `${Math.random() * 5}s`;
        
        // Generate random characters
        const chars = '01アイウエオカキクケコサシスセソタチツテトナニヌネノハヒフヘホマミムメモヤユヨラリルレロワヲン';
        let content = '';
        for (let j = 0; j < 50; j++) {
            content += chars[Math.floor(Math.random() * chars.length)] + '\n';
        }
        column.textContent = content;
        matrixContainer.appendChild(column);
    }
    document.body.appendChild(matrixContainer);
}

// Initialize neon cursor trail effect
function initializeNeonCursor() {
    let mouseTimer;
    let isMoving = false;
    
    document.addEventListener('mousemove', (e) => {
        if (!isMoving) {
            isMoving = true;
            createCursorTrail(e.clientX, e.clientY);
        }
        
        clearTimeout(mouseTimer);
        mouseTimer = setTimeout(() => {
            isMoving = false;
        }, 100);
    });
}

function createCursorTrail(x, y) {
    const trail = document.createElement('div');
    trail.className = 'neon-cursor-trail';
    trail.style.left = x + 'px';
    trail.style.top = y + 'px';
    document.body.appendChild(trail);
    
    setTimeout(() => {
        trail.remove();
    }, 1000);
}

// Add glitch effect to headers on hover
document.addEventListener('DOMContentLoaded', function() {
    const headers = document.querySelectorAll('h1, h2, h3');
    headers.forEach(header => {
        header.addEventListener('mouseenter', function() {
            this.classList.add('glitch');
            this.setAttribute('data-text', this.textContent);
        });
        
        header.addEventListener('mouseleave', function() {
            this.classList.remove('glitch');
        });
    });
});