// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package soul

import (
	"strings"
	"testing"
)

func TestParseAgentOutput(t *testing.T) {
	tests := []struct {
		name             string
		output           string
		wantFeatures     int
		wantBugs         int
		wantTests        int
		checkFeatureName string
		checkBugSeverity string
		checkTestPassed  bool
	}{
		{
			name: "structured_output",
			output: `
Implemented features:
- User authentication with JWT tokens
- Product CRUD API endpoints
- Database migrations

Known bugs:
- Login fails with special characters in password
- Product search is case-sensitive

Test results:
✅ Authentication tests: All 15 tests passed
❌ Product API tests: 2 out of 20 tests failed
`,
			wantFeatures:     3,
			wantBugs:         2,
			wantTests:        2,
			checkFeatureName: "authentication",
			checkBugSeverity: "high",
			checkTestPassed:  false, // One test failed
		},
		{
			name: "inline_mentions",
			output: `
Successfully implemented the user registration feature.
Added profile management functionality.
Created API endpoints for user data.

Bug: The password reset email doesn't send properly.
Issue: Memory leak detected in the connection pool.
Problem: Search function crashes with special characters.

All unit tests passed!
Integration tests failed.
`,
			wantFeatures:     3,
			wantBugs:         3,
			wantTests:        2,
			checkFeatureName: "registration",
			checkBugSeverity: "high", // Email not sending is high priority
			checkTestPassed:  true,   // Unit tests passed
		},
		{
			name: "mixed_format",
			output: `
## Work completed:

Implemented:
1. REST API with full CRUD operations
2. Authentication using OAuth2
3. Rate limiting middleware

Fixed: The database connection timeout issue
Fixed: Memory optimization in the cache layer

However, discovered bug: API returns 500 on malformed JSON input

Test summary:
- ✅ Unit tests: 45/45 passed
- ❌ Integration tests: 3/10 failed
- ✅ End-to-end tests: All passed
`,
			wantFeatures:     3,
			wantBugs:         1,
			wantTests:        3,
			checkFeatureName: "REST API",
			checkBugSeverity: "high",
			checkTestPassed:  true, // At least one test suite passed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features, bugs, testResults := ParseAgentOutput(tt.output, "test-agent-123")

			// Check counts
			if len(features) != tt.wantFeatures {
				t.Errorf("ParseAgentOutput() features count = %v, want %v", len(features), tt.wantFeatures)
			}
			if len(bugs) != tt.wantBugs {
				t.Errorf("ParseAgentOutput() bugs count = %v, want %v", len(bugs), tt.wantBugs)
			}
			if len(testResults) != tt.wantTests {
				t.Errorf("ParseAgentOutput() test results count = %v, want %v", len(testResults), tt.wantTests)
			}

			// Check specific feature
			if tt.checkFeatureName != "" && len(features) > 0 {
				found := false
				for _, f := range features {
					if strings.Contains(strings.ToLower(f.Description), tt.checkFeatureName) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find feature containing '%s'", tt.checkFeatureName)
				}
			}

			// Check bug severity
			if tt.checkBugSeverity != "" && len(bugs) > 0 {
				found := false
				for _, b := range bugs {
					if b.Severity == tt.checkBugSeverity {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected to find bug with severity '%s'", tt.checkBugSeverity)
				}
			}

			// Check test results
			if len(testResults) > 0 {
				hasPassedTest := false
				for _, tr := range testResults {
					if tr.Passed {
						hasPassedTest = true
						break
					}
				}
				if hasPassedTest != tt.checkTestPassed {
					t.Errorf("Expected hasPassedTest = %v", tt.checkTestPassed)
				}
			}
		})
	}
}

func TestGuessSeverity(t *testing.T) {
	tests := []struct {
		description string
		want        string
	}{
		{"Application crashes when user logs in", "critical"},
		{"Security vulnerability in authentication", "critical"},
		{"Data loss occurs during sync", "critical"},
		{"Login fails with special characters", "high"},
		{"Feature is broken and doesn't work", "high"},
		{"Error message shown incorrectly", "high"},
		{"Minor UI alignment issue", "low"},
		{"Typo in help text", "low"},
		{"Small cosmetic bug in footer", "low"},
		{"Button color is wrong", "medium"},
		{"Performance could be improved", "medium"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			if got := guessSeverity(tt.description); got != tt.want {
				t.Errorf("guessSeverity(%q) = %v, want %v", tt.description, got, tt.want)
			}
		})
	}
}

func TestExtractFeatureName(t *testing.T) {
	tests := []struct {
		description string
		wantContains string
	}{
		{
			"Implemented user authentication with JWT tokens and refresh token support",
			"authentication",
		},
		{
			"Created REST API endpoints for product management",
			"API",
		},
		{
			"Built frontend components for the dashboard",
			"frontend",
		},
		{
			"Added database migration scripts",
			"database",
		},
		{
			"Short feature name",
			"Short feature name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			got := extractFeatureName(tt.description)
			if !strings.Contains(got, tt.wantContains) {
				t.Errorf("extractFeatureName(%q) = %v, want to contain %v", tt.description, got, tt.wantContains)
			}
		})
	}
}

func TestDeduplication(t *testing.T) {
	// Test feature deduplication
	features := []Feature{
		{Description: "User authentication"},
		{Description: "user authentication"}, // Duplicate (case insensitive)
		{Description: "Product API"},
		{Description: "User Authentication"}, // Duplicate
	}
	
	deduped := deduplicateFeatures(features)
	if len(deduped) != 2 {
		t.Errorf("deduplicateFeatures() returned %d features, want 2", len(deduped))
	}

	// Test bug deduplication
	bugs := []Bug{
		{Description: "Login fails"},
		{Description: "login fails"}, // Duplicate
		{Description: "Search broken"},
	}
	
	dedupedBugs := deduplicateBugs(bugs)
	if len(dedupedBugs) != 2 {
		t.Errorf("deduplicateBugs() returned %d bugs, want 2", len(dedupedBugs))
	}

	// Test test result deduplication
	tests := []TestResult{
		{TestName: "Unit Tests"},
		{TestName: "unit tests"}, // Duplicate
		{TestName: "Integration Tests"},
	}
	
	dedupedTests := deduplicateTests(tests)
	if len(dedupedTests) != 2 {
		t.Errorf("deduplicateTests() returned %d tests, want 2", len(dedupedTests))
	}
}