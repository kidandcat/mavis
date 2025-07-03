package soul

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore manages souls in a SQLite database
type SQLiteStore struct {
	db *sql.DB
	mu sync.RWMutex
}

// NewSQLiteStore creates a new SQLite-based soul store
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open the database
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := &SQLiteStore{db: db}

	// Create tables
	if err := store.createTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return store, nil
}

func (s *SQLiteStore) createTables() error {
	schema := `
	CREATE TABLE IF NOT EXISTS souls (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		project_path TEXT NOT NULL UNIQUE,
		objectives TEXT NOT NULL,
		requirements TEXT NOT NULL,
		status TEXT NOT NULL,
		feedback TEXT NOT NULL,
		iterations TEXT NOT NULL,
		created_at TIMESTAMP NOT NULL,
		updated_at TIMESTAMP NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_souls_project_path ON souls(project_path);
	CREATE INDEX IF NOT EXISTS idx_souls_status ON souls(status);
	`

	_, err := s.db.Exec(schema)
	return err
}

func (s *SQLiteStore) Create(soul *Soul) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	objectives, _ := json.Marshal(soul.Objectives)
	requirements, _ := json.Marshal(soul.Requirements)
	feedback, _ := json.Marshal(soul.Feedback)
	iterations, _ := json.Marshal(soul.Iterations)

	query := `
	INSERT INTO souls (id, name, project_path, objectives, requirements, status, feedback, iterations, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.Exec(query,
		soul.ID,
		soul.Name,
		soul.ProjectPath,
		string(objectives),
		string(requirements),
		string(soul.Status),
		string(feedback),
		string(iterations),
		soul.CreatedAt,
		soul.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create soul: %w", err)
	}

	return nil
}

func (s *SQLiteStore) Update(soul *Soul) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	objectives, _ := json.Marshal(soul.Objectives)
	requirements, _ := json.Marshal(soul.Requirements)
	feedback, _ := json.Marshal(soul.Feedback)
	iterations, _ := json.Marshal(soul.Iterations)

	query := `
	UPDATE souls 
	SET name = ?, project_path = ?, objectives = ?, requirements = ?, status = ?, 
	    feedback = ?, iterations = ?, updated_at = ?
	WHERE id = ?
	`

	result, err := s.db.Exec(query,
		soul.Name,
		soul.ProjectPath,
		string(objectives),
		string(requirements),
		string(soul.Status),
		string(feedback),
		string(iterations),
		soul.UpdatedAt,
		soul.ID,
	)

	if err != nil {
		return fmt.Errorf("failed to update soul: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check update result: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("soul with ID %s not found", soul.ID)
	}

	return nil
}

func (s *SQLiteStore) Get(id string) (*Soul, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, name, project_path, objectives, requirements, status, feedback, iterations, created_at, updated_at
	FROM souls
	WHERE id = ?
	`

	soul := &Soul{}
	var objectives, requirements, feedback, iterations string

	err := s.db.QueryRow(query, id).Scan(
		&soul.ID,
		&soul.Name,
		&soul.ProjectPath,
		&objectives,
		&requirements,
		&soul.Status,
		&feedback,
		&iterations,
		&soul.CreatedAt,
		&soul.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("soul with ID %s not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get soul: %w", err)
	}

	// Unmarshal JSON fields
	json.Unmarshal([]byte(objectives), &soul.Objectives)
	json.Unmarshal([]byte(requirements), &soul.Requirements)
	json.Unmarshal([]byte(feedback), &soul.Feedback)
	json.Unmarshal([]byte(iterations), &soul.Iterations)

	return soul, nil
}

func (s *SQLiteStore) GetByProjectPath(projectPath string) (*Soul, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, name, project_path, objectives, requirements, status, feedback, iterations, created_at, updated_at
	FROM souls
	WHERE project_path = ?
	`

	soul := &Soul{}
	var objectives, requirements, feedback, iterations string

	err := s.db.QueryRow(query, projectPath).Scan(
		&soul.ID,
		&soul.Name,
		&soul.ProjectPath,
		&objectives,
		&requirements,
		&soul.Status,
		&feedback,
		&iterations,
		&soul.CreatedAt,
		&soul.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("soul for project path %s not found", projectPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get soul: %w", err)
	}

	// Unmarshal JSON fields
	json.Unmarshal([]byte(objectives), &soul.Objectives)
	json.Unmarshal([]byte(requirements), &soul.Requirements)
	json.Unmarshal([]byte(feedback), &soul.Feedback)
	json.Unmarshal([]byte(iterations), &soul.Iterations)

	return soul, nil
}

func (s *SQLiteStore) List() ([]*Soul, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `
	SELECT id, name, project_path, objectives, requirements, status, feedback, iterations, created_at, updated_at
	FROM souls
	ORDER BY updated_at DESC
	`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list souls: %w", err)
	}
	defer rows.Close()

	var souls []*Soul
	for rows.Next() {
		soul := &Soul{}
		var objectives, requirements, feedback, iterations string

		err := rows.Scan(
			&soul.ID,
			&soul.Name,
			&soul.ProjectPath,
			&objectives,
			&requirements,
			&soul.Status,
			&feedback,
			&iterations,
			&soul.CreatedAt,
			&soul.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan soul: %w", err)
		}

		// Unmarshal JSON fields
		json.Unmarshal([]byte(objectives), &soul.Objectives)
		json.Unmarshal([]byte(requirements), &soul.Requirements)
		json.Unmarshal([]byte(feedback), &soul.Feedback)
		json.Unmarshal([]byte(iterations), &soul.Iterations)

		souls = append(souls, soul)
	}

	return souls, nil
}

func (s *SQLiteStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `DELETE FROM souls WHERE id = ?`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete soul: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check delete result: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("soul with ID %s not found", id)
	}

	return nil
}

// Close closes the database connection
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}