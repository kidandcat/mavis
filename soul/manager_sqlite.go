package soul

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ManagerSQLite manages souls with SQLite storage in home directory
type ManagerSQLite struct {
	sqliteStore *SQLiteStore
	pauseMu     sync.RWMutex
	pauseState  bool
	pauseFile   string
}

// NewManagerSQLite creates a new manager with SQLite storage
func NewManagerSQLite(configDir string) (*ManagerSQLite, error) {
	// Create SQLite store in home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	dbPath := filepath.Join(homeDir, ".mavis", "souls.db")
	sqliteStore, err := NewSQLiteStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQLite store: %w", err)
	}

	manager := &ManagerSQLite{
		sqliteStore: sqliteStore,
		pauseFile:   filepath.Join(configDir, "souls_pause_state"),
	}

	// Load pause state from file if exists
	manager.loadPauseState()

	return manager, nil
}


// CreateSoul creates a new soul with the given name and project path
func (m *ManagerSQLite) CreateSoul(name, projectPath string) (*Soul, error) {
	soul := NewSoul(name, projectPath)
	if err := m.sqliteStore.Create(soul); err != nil {
		return nil, err
	}
	return soul, nil
}

// UpdateSoul updates an existing soul
func (m *ManagerSQLite) UpdateSoul(soul *Soul) error {
	return m.sqliteStore.Update(soul)
}

// GetSoul retrieves a soul by ID
func (m *ManagerSQLite) GetSoul(id string) (*Soul, error) {
	return m.sqliteStore.Get(id)
}

// GetSoulByProjectPath retrieves a soul by project path
func (m *ManagerSQLite) GetSoulByProjectPath(projectPath string) (*Soul, error) {
	return m.sqliteStore.GetByProjectPath(projectPath)
}

// ListSouls returns all souls
func (m *ManagerSQLite) ListSouls() ([]*Soul, error) {
	return m.sqliteStore.List()
}

// DeleteSoul deletes a soul
func (m *ManagerSQLite) DeleteSoul(id string) error {
	return m.sqliteStore.Delete(id)
}

// IsPaused returns whether soul iterations are paused
func (m *ManagerSQLite) IsPaused() bool {
	m.pauseMu.RLock()
	defer m.pauseMu.RUnlock()
	return m.pauseState
}

// SetPaused sets the pause state for soul iterations
func (m *ManagerSQLite) SetPaused(paused bool) error {
	m.pauseMu.Lock()
	defer m.pauseMu.Unlock()

	m.pauseState = paused

	// Save to file
	return m.savePauseState()
}

func (m *ManagerSQLite) loadPauseState() {
	data, err := os.ReadFile(m.pauseFile)
	if err != nil {
		// File doesn't exist, default to not paused
		m.pauseState = false
		return
	}

	m.pauseState = string(data) == "true"
}

func (m *ManagerSQLite) savePauseState() error {
	data := "false"
	if m.pauseState {
		data = "true"
	}
	return os.WriteFile(m.pauseFile, []byte(data), 0644)
}

// UpdateObjectives updates the objectives of a soul
func (m *ManagerSQLite) UpdateObjectives(soulID string, objectives []string) error {
	soul, err := m.GetSoul(soulID)
	if err != nil {
		return err
	}
	soul.Objectives = objectives
	soul.UpdatedAt = time.Now()
	return m.UpdateSoul(soul)
}

// UpdateRequirements updates the requirements of a soul
func (m *ManagerSQLite) UpdateRequirements(soulID string, requirements []string) error {
	soul, err := m.GetSoul(soulID)
	if err != nil {
		return err
	}
	soul.Requirements = requirements
	soul.UpdatedAt = time.Now()
	return m.UpdateSoul(soul)
}

// StartSoulIteration starts a new iteration for a soul
func (m *ManagerSQLite) StartSoulIteration(soulID, agentID, purpose string) error {
	soul, err := m.GetSoul(soulID)
	if err != nil {
		return err
	}
	soul.StartIteration(agentID, purpose)
	return m.UpdateSoul(soul)
}

// CompleteSoulIteration completes an iteration for a soul
func (m *ManagerSQLite) CompleteSoulIteration(soulID, agentID, result string) error {
	soul, err := m.GetSoul(soulID)
	if err != nil {
		return err
	}
	soul.CompleteIteration(agentID, result)
	return m.UpdateSoul(soul)
}

// AddFeature adds a feature to a soul
func (m *ManagerSQLite) AddFeature(soulID string, feature Feature) error {
	soul, err := m.GetSoul(soulID)
	if err != nil {
		return err
	}
	soul.AddImplementedFeature(feature)
	return m.UpdateSoul(soul)
}

// AddBug adds a bug to a soul
func (m *ManagerSQLite) AddBug(soulID string, bug Bug) error {
	soul, err := m.GetSoul(soulID)
	if err != nil {
		return err
	}
	soul.AddBug(bug)
	return m.UpdateSoul(soul)
}

// AddTestResult adds a test result to a soul
func (m *ManagerSQLite) AddTestResult(soulID string, result TestResult) error {
	soul, err := m.GetSoul(soulID)
	if err != nil {
		return err
	}
	soul.AddTestResult(result)
	return m.UpdateSoul(soul)
}

// IsScanning always returns false since SQLite doesn't need async scanning
func (m *ManagerSQLite) IsScanning() bool {
	return false
}

// SetPauseState is an alias for SetPaused
func (m *ManagerSQLite) SetPauseState(paused bool) error {
	return m.SetPaused(paused)
}

// TogglePause toggles the pause state
func (m *ManagerSQLite) TogglePause() (bool, error) {
	currentState := m.IsPaused()
	newState := !currentState
	if err := m.SetPaused(newState); err != nil {
		return currentState, err
	}
	return newState, nil
}

// GetSoulByProject is an alias for GetSoulByProjectPath
func (m *ManagerSQLite) GetSoulByProject(projectPath string) (*Soul, error) {
	return m.GetSoulByProjectPath(projectPath)
}

// Close closes the database connection
func (m *ManagerSQLite) Close() error {
	return m.sqliteStore.Close()
}