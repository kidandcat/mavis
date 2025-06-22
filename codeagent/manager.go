// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package codeagent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// QueuedTask represents a task waiting to be executed
type QueuedTask struct {
	Folder  string
	Prompt  string
	Ctx     context.Context
	QueueID string // Unique ID for this queued task
}

// AgentStartCallback is called when a queued agent starts
type AgentStartCallback func(agentID string, folder string, prompt string, queueID string)

// Manager manages multiple code agents
type Manager struct {
	agents           map[string]*Agent
	mu               sync.RWMutex
	nextID           int
	availableIDs     []int                   // Pool of reusable IDs from cleaned up agents
	folderQueues     map[string][]QueuedTask // Queue of tasks per folder
	runningPerFolder map[string]string       // Maps folder to currently running agent ID
	queueMu          sync.Mutex              // Separate mutex for queue operations
	startCallback    AgentStartCallback      // Callback when queued agent starts
}

// NewManager creates a new agent manager
func NewManager() *Manager {
	return &Manager{
		agents:           make(map[string]*Agent),
		nextID:           1,
		availableIDs:     make([]int, 0),
		folderQueues:     make(map[string][]QueuedTask),
		runningPerFolder: make(map[string]string),
	}
}

// SetAgentStartCallback sets the callback for when queued agents start
func (m *Manager) SetAgentStartCallback(callback AgentStartCallback) {
	m.startCallback = callback
}

// LaunchAgent creates and starts a new agent or queues it if one is already running in the folder
func (m *Manager) LaunchAgent(ctx context.Context, folder, prompt string) (string, error) {
	// Check if an agent is already running in this folder
	m.queueMu.Lock()
	if runningID, exists := m.runningPerFolder[folder]; exists {
		// Agent is already running in this folder, add to queue
		queueID := fmt.Sprintf("queue-%d-%s", time.Now().Unix(), folder)
		task := QueuedTask{
			Folder:  folder,
			Prompt:  prompt,
			Ctx:     ctx,
			QueueID: queueID,
		}

		m.folderQueues[folder] = append(m.folderQueues[folder], task)
		queuePos := len(m.folderQueues[folder])
		m.queueMu.Unlock()

		// Return a placeholder ID indicating the task is queued
		return fmt.Sprintf("queued-%s-pos-%d-qid-%s", runningID, queuePos, queueID), nil
	}

	// No agent running in this folder, start immediately
	id := m.createAndStartAgent(ctx, folder, prompt)
	m.runningPerFolder[folder] = id
	m.queueMu.Unlock()

	return id, nil
}

// createAndStartAgent is a helper that creates and starts an agent
func (m *Manager) createAndStartAgent(ctx context.Context, folder, prompt string) string {
	m.mu.Lock()
	var agentNum int
	if len(m.availableIDs) > 0 {
		// Reuse an available ID
		agentNum = m.availableIDs[0]
		m.availableIDs = m.availableIDs[1:]
	} else {
		// Use next sequential ID
		agentNum = m.nextID
		m.nextID++
	}
	id := fmt.Sprintf("%d", agentNum)
	m.mu.Unlock()

	agent := NewAgent(id, folder, prompt)

	m.mu.Lock()
	m.agents[id] = agent
	m.mu.Unlock()

	// Start agent with completion callback
	go func() {
		agent.Start(ctx)
		// Don't process queue immediately - let the monitor handle notification first
		// The monitor will call RemoveAgent which will trigger processQueueForFolder
	}()

	return id
}

// processQueueForFolder checks if there are queued tasks for a folder and starts the next one
func (m *Manager) processQueueForFolder(folder string) {
	log.Printf("[QueueProcessor] Processing queue for folder: %s", folder)
	m.queueMu.Lock()

	// Remove the current running agent for this folder
	delete(m.runningPerFolder, folder)
	log.Printf("[QueueProcessor] Removed running agent for folder %s", folder)

	// Check if there are queued tasks
	var taskToProcess *QueuedTask
	if queue, exists := m.folderQueues[folder]; exists && len(queue) > 0 {
		log.Printf("[QueueProcessor] Found %d queued tasks for folder %s", len(queue), folder)
		// Get the next task
		taskToProcess = &queue[0]
		m.folderQueues[folder] = queue[1:]

		// If queue is now empty, remove it
		if len(m.folderQueues[folder]) == 0 {
			delete(m.folderQueues, folder)
		}
	} else {
		log.Printf("[QueueProcessor] No queued tasks for folder %s", folder)
	}

	m.queueMu.Unlock()

	// Process the task outside of the queue lock to avoid deadlock
	if taskToProcess != nil {
		log.Printf("[QueueProcessor] Starting queued task for folder %s, QueueID: %s", folder, taskToProcess.QueueID)
		// Start the queued task
		id := m.createAndStartAgent(taskToProcess.Ctx, taskToProcess.Folder, taskToProcess.Prompt)

		// Update the running agent for this folder
		m.queueMu.Lock()
		m.runningPerFolder[folder] = id
		m.queueMu.Unlock()
		log.Printf("[QueueProcessor] Started agent %s for queued task in folder %s", id, folder)

		// Call the callback if set
		if m.startCallback != nil {
			log.Printf("[QueueProcessor] Calling start callback for agent %s", id)
			m.startCallback(id, taskToProcess.Folder, taskToProcess.Prompt, taskToProcess.QueueID)
		}
	}
}

// LaunchAgentWithID creates and starts a new agent with a custom ID
func (m *Manager) LaunchAgentWithID(ctx context.Context, id, folder, prompt string) error {
	m.mu.Lock()
	if _, exists := m.agents[id]; exists {
		m.mu.Unlock()
		return fmt.Errorf("agent with ID %s already exists", id)
	}
	m.mu.Unlock()

	agent := NewAgent(id, folder, prompt)

	m.mu.Lock()
	m.agents[id] = agent
	m.mu.Unlock()

	// Track this agent as running in its folder
	m.queueMu.Lock()
	m.runningPerFolder[folder] = id
	m.queueMu.Unlock()

	// Start agent with completion callback for queue processing
	go func() {
		agent.Start(ctx)
		// Don't process queue immediately - let the monitor handle notification first
		// The monitor will call RemoveAgent which will trigger processQueueForFolder
	}()

	return nil
}

// LaunchAgentWithPlanFile creates and starts a new agent with a custom plan filename
func (m *Manager) LaunchAgentWithPlanFile(ctx context.Context, folder, prompt, planFilename string) (string, error) {
	// Check if an agent is already running in this folder
	m.queueMu.Lock()
	if runningID, exists := m.runningPerFolder[folder]; exists {
		// Agent is already running in this folder, add to queue
		queueID := fmt.Sprintf("queue-%d-%s", time.Now().Unix(), folder)
		task := QueuedTask{
			Folder:  folder,
			Prompt:  prompt,
			Ctx:     ctx,
			QueueID: queueID,
		}

		m.folderQueues[folder] = append(m.folderQueues[folder], task)
		queuePos := len(m.folderQueues[folder])
		m.queueMu.Unlock()

		// Return a placeholder ID indicating the task is queued
		return fmt.Sprintf("queued-%s-pos-%d-qid-%s", runningID, queuePos, queueID), nil
	}
	m.queueMu.Unlock()

	m.mu.Lock()
	var agentNum int
	if len(m.availableIDs) > 0 {
		// Reuse an available ID
		agentNum = m.availableIDs[0]
		m.availableIDs = m.availableIDs[1:]
	} else {
		// Use next sequential ID
		agentNum = m.nextID
		m.nextID++
	}
	id := fmt.Sprintf("%d", agentNum)
	m.mu.Unlock()

	agent := NewAgentWithPlanFile(id, folder, prompt, planFilename)

	m.mu.Lock()
	m.agents[id] = agent
	m.mu.Unlock()

	// Track this agent as running in its folder
	m.queueMu.Lock()
	m.runningPerFolder[folder] = id
	m.queueMu.Unlock()

	// Start agent with completion callback for queue processing
	go func() {
		agent.Start(ctx)
		// Don't process queue immediately - let the monitor handle notification first
		// The monitor will call RemoveAgent which will trigger processQueueForFolder
	}()

	return id, nil
}

// GetAgent returns an agent by ID
func (m *Manager) GetAgent(id string) (*Agent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	agent, exists := m.agents[id]
	if !exists {
		return nil, fmt.Errorf("agent %s not found", id)
	}

	return agent, nil
}

// GetAgentInfo returns information about an agent
func (m *Manager) GetAgentInfo(id string) (AgentInfo, error) {
	agent, err := m.GetAgent(id)
	if err != nil {
		return AgentInfo{}, err
	}

	return agent.ToInfo(), nil
}

// ListAgents returns information about all agents
func (m *Manager) ListAgents() []AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]AgentInfo, 0, len(m.agents))
	for _, agent := range m.agents {
		infos = append(infos, agent.ToInfo())
	}

	return infos
}

// ListAgentsByStatus returns agents with a specific status
func (m *Manager) ListAgentsByStatus(status AgentStatus) []AgentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var infos []AgentInfo
	for _, agent := range m.agents {
		if agent.GetStatus() == status {
			infos = append(infos, agent.ToInfo())
		}
	}

	return infos
}

// KillAgent terminates a running agent
func (m *Manager) KillAgent(id string) error {
	agent, err := m.GetAgent(id)
	if err != nil {
		return err
	}

	return agent.Kill()
}

// RemoveAgent removes an agent from the manager
func (m *Manager) RemoveAgent(id string) error {
	log.Printf("[RemoveAgent] Attempting to remove agent %s", id)
	m.mu.Lock()
	agent, exists := m.agents[id]
	if !exists {
		m.mu.Unlock()
		log.Printf("[RemoveAgent] Agent %s not found", id)
		return fmt.Errorf("agent %s not found", id)
	}

	// Get agent status and folder before releasing the lock
	status := agent.GetStatus()
	agentFolder := agent.Folder // Get the folder directly from the agent

	// Check if this agent was running for a folder
	var folderToProcess string
	m.queueMu.Lock()
	for folder, runningID := range m.runningPerFolder {
		if runningID == id {
			delete(m.runningPerFolder, folder)
			folderToProcess = folder
			break
		}
	}
	m.queueMu.Unlock()

	// If we didn't find the folder in runningPerFolder, use the agent's folder
	if folderToProcess == "" && agentFolder != "" {
		log.Printf("[RemoveAgent] Using agent's folder %s for queue processing", agentFolder)
		folderToProcess = agentFolder
	}

	// Kill if running
	if status == StatusRunning {
		if err := agent.Kill(); err != nil {
			m.mu.Unlock()
			return fmt.Errorf("failed to kill agent: %w", err)
		}
	}

	delete(m.agents, id)

	// Add the ID back to available pool for reuse
	var agentNum int
	if n, _ := fmt.Sscanf(id, "%d", &agentNum); n == 1 {
		m.availableIDs = append(m.availableIDs, agentNum)
	}

	m.mu.Unlock()

	// Process queue for the folder if needed (after releasing main mutex)
	if folderToProcess != "" {
		log.Printf("[RemoveAgent] Processing queue for folder: %s", folderToProcess)
		m.processQueueForFolder(folderToProcess)
	} else {
		log.Printf("[RemoveAgent] No folder to process queue for agent %s", id)
	}

	log.Printf("[RemoveAgent] Successfully removed agent %s", id)
	return nil
}

// WaitForAgent waits for an agent to finish
func (m *Manager) WaitForAgent(ctx context.Context, id string) (AgentInfo, error) {
	agent, err := m.GetAgent(id)
	if err != nil {
		return AgentInfo{}, err
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return AgentInfo{}, ctx.Err()
		case <-ticker.C:
			status := agent.GetStatus()
			if status == StatusFinished || status == StatusFailed || status == StatusKilled {
				// ToInfo() is safe to call here as it no longer has nested lock issues
				return agent.ToInfo(), nil
			}
		}
	}
}

// WaitForMultipleAgents waits for multiple agents to finish
func (m *Manager) WaitForMultipleAgents(ctx context.Context, ids []string) ([]AgentInfo, error) {
	var wg sync.WaitGroup
	results := make([]AgentInfo, len(ids))
	errors := make([]error, len(ids))

	for i, id := range ids {
		wg.Add(1)
		go func(index int, agentID string) {
			defer wg.Done()
			info, err := m.WaitForAgent(ctx, agentID)
			results[index] = info
			errors[index] = err
		}(i, id)
	}

	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

// CleanupFinishedAgents removes all finished, failed, or killed agents
func (m *Manager) CleanupFinishedAgents() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	agentsToRemove := make([]string, 0)

	// First pass: identify agents to remove
	for id, agent := range m.agents {
		status := agent.GetStatus()
		if status == StatusFinished || status == StatusFailed || status == StatusKilled {
			agentsToRemove = append(agentsToRemove, id)
		}
	}

	// Second pass: remove the identified agents
	for _, id := range agentsToRemove {
		delete(m.agents, id)
		count++

		// Add the ID back to available pool for reuse
		var agentNum int
		if n, _ := fmt.Sscanf(id, "%d", &agentNum); n == 1 {
			m.availableIDs = append(m.availableIDs, agentNum)
		}
	}

	return count
}

// GetRunningCount returns the number of currently running agents
func (m *Manager) GetRunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, agent := range m.agents {
		status := agent.GetStatus()
		if status == StatusRunning {
			count++
		}
	}

	return count
}

// GetTotalCount returns the total number of agents
func (m *Manager) GetTotalCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.agents)
}

// GetQueueStatus returns information about queued tasks for each folder
func (m *Manager) GetQueueStatus() map[string]int {
	m.queueMu.Lock()
	defer m.queueMu.Unlock()

	status := make(map[string]int)
	for folder, queue := range m.folderQueues {
		status[folder] = len(queue)
	}

	return status
}

// GetQueuedTasksForFolder returns the number of queued tasks for a specific folder
func (m *Manager) GetQueuedTasksForFolder(folder string) int {
	m.queueMu.Lock()
	defer m.queueMu.Unlock()

	if queue, exists := m.folderQueues[folder]; exists {
		return len(queue)
	}

	return 0
}

// IsAgentRunningInFolder checks if an agent is currently running in the specified folder
func (m *Manager) IsAgentRunningInFolder(folder string) (bool, string) {
	m.queueMu.Lock()
	defer m.queueMu.Unlock()

	if agentID, exists := m.runningPerFolder[folder]; exists {
		return true, agentID
	}

	return false, ""
}
