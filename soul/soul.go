package soul

import (
	"crypto/rand"
	"encoding/hex"
	"path/filepath"
	"time"
)

type Soul struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	ProjectPath  string              `json:"project_path"`
	Objectives   []string            `json:"objectives"`
	Requirements []string            `json:"requirements"`
	Status       SoulStatus          `json:"status"`
	Feedback     SoulFeedback        `json:"feedback"`
	Iterations   []SoulIteration     `json:"iterations"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

type SoulStatus string

const (
	SoulStatusStandby SoulStatus = "standby"
	SoulStatusWorking SoulStatus = "working"
)

type SoulFeedback struct {
	ImplementedFeatures []Feature `json:"implemented_features"`
	KnownBugs          []Bug     `json:"known_bugs"`
	TestResults        []TestResult `json:"test_results"`
	LastUpdated        time.Time `json:"last_updated"`
}

type Feature struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ImplementedAt time.Time `json:"implemented_at"`
	AgentID     string    `json:"agent_id"`
}

type Bug struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Status      string    `json:"status"`
	FoundAt     time.Time `json:"found_at"`
	FixedAt     *time.Time `json:"fixed_at,omitempty"`
	AgentID     string    `json:"agent_id"`
}

type TestResult struct {
	TestName    string    `json:"test_name"`
	Passed      bool      `json:"passed"`
	Message     string    `json:"message"`
	ExecutedAt  time.Time `json:"executed_at"`
	AgentID     string    `json:"agent_id"`
}

type SoulIteration struct {
	Number      int       `json:"number"`
	AgentID     string    `json:"agent_id"`
	Purpose     string    `json:"purpose"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Result      string    `json:"result"`
}

func NewSoul(name, projectPath string) *Soul {
	// If name is empty, use the folder name from the path
	if name == "" {
		name = filepath.Base(projectPath)
	}
	
	return &Soul{
		ID:           generateID(),
		Name:         name,
		ProjectPath:  projectPath,
		Status:       SoulStatusStandby,
		Objectives:   []string{},
		Requirements: []string{},
		Feedback: SoulFeedback{
			ImplementedFeatures: []Feature{},
			KnownBugs:          []Bug{},
			TestResults:        []TestResult{},
			LastUpdated:        time.Now(),
		},
		Iterations: []SoulIteration{},
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

func (s *Soul) AddObjective(objective string) {
	s.Objectives = append(s.Objectives, objective)
	s.UpdatedAt = time.Now()
}

func (s *Soul) AddRequirement(requirement string) {
	s.Requirements = append(s.Requirements, requirement)
	s.UpdatedAt = time.Now()
}

func (s *Soul) StartIteration(agentID, purpose string) *SoulIteration {
	iteration := SoulIteration{
		Number:    len(s.Iterations) + 1,
		AgentID:   agentID,
		Purpose:   purpose,
		StartedAt: time.Now(),
	}
	s.Iterations = append(s.Iterations, iteration)
	s.UpdatedAt = time.Now()
	return &s.Iterations[len(s.Iterations)-1]
}

func (s *Soul) CompleteIteration(agentID, result string) {
	for i := range s.Iterations {
		if s.Iterations[i].AgentID == agentID && s.Iterations[i].CompletedAt == nil {
			now := time.Now()
			s.Iterations[i].CompletedAt = &now
			s.Iterations[i].Result = result
			s.UpdatedAt = time.Now()
			break
		}
	}
}

func (s *Soul) AddImplementedFeature(feature Feature) {
	s.Feedback.ImplementedFeatures = append(s.Feedback.ImplementedFeatures, feature)
	s.Feedback.LastUpdated = time.Now()
	s.UpdatedAt = time.Now()
}

func (s *Soul) AddBug(bug Bug) {
	s.Feedback.KnownBugs = append(s.Feedback.KnownBugs, bug)
	s.Feedback.LastUpdated = time.Now()
	s.UpdatedAt = time.Now()
}

func (s *Soul) AddTestResult(result TestResult) {
	s.Feedback.TestResults = append(s.Feedback.TestResults, result)
	s.Feedback.LastUpdated = time.Now()
	s.UpdatedAt = time.Now()
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

func randomString(length int) string {
	bytes := make([]byte, length/2+1)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}