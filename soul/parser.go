// Copyright (c) 2024 Mavis Contributors
// SPDX-License-Identifier: MIT

package soul

import (
	"regexp"
	"strings"
	"time"
)

// ParseAgentOutput parses agent output and extracts features, bugs, and test results
func ParseAgentOutput(output string, agentID string) (features []Feature, bugs []Bug, testResults []TestResult) {
	lines := strings.Split(output, "\n")
	
	// Regular expressions for pattern matching
	featureRegex := regexp.MustCompile(`(?i)(?:implemented|added|created|built|developed)\s*[:：]?\s*(.+)`)
	bugRegex := regexp.MustCompile(`(?i)(?:bug|issue|problem|error|defect|broken)\s*[:：]?\s*(.+)`)
	fixedBugRegex := regexp.MustCompile(`(?i)(?:fixed|resolved|solved|repaired)\s*[:：]?\s*(.+)`)
	testPassRegex := regexp.MustCompile(`(?i)(?:✅|passed?|success(?:ful)?)\s*[:：]?\s*(.+?)(?:\s*test)?`)
	testFailRegex := regexp.MustCompile(`(?i)(?:❌|failed?|error)\s*[:：]?\s*(.+?)(?:\s*test)?`)
	
	// Extract features
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Check for implemented features
		if matches := featureRegex.FindStringSubmatch(line); len(matches) > 1 {
			featureName := strings.TrimSpace(matches[1])
			// Clean up the feature name
			featureName = cleanupText(featureName)
			if featureName != "" {
				features = append(features, Feature{
					Name:          extractFeatureName(featureName),
					Description:   featureName,
					ImplementedAt: time.Now(),
					AgentID:       agentID,
				})
			}
		}
		
		// Check for bugs (excluding fixed ones)
		if matches := bugRegex.FindStringSubmatch(line); len(matches) > 1 {
			// Check if this is about a fixed bug
			if fixedBugRegex.MatchString(line) {
				continue // Skip fixed bugs
			}
			
			bugDescription := strings.TrimSpace(matches[1])
			bugDescription = cleanupText(bugDescription)
			if bugDescription != "" {
				bugs = append(bugs, Bug{
					Description: bugDescription,
					Severity:    guessSeverity(bugDescription),
					Status:      "open",
					FoundAt:     time.Now(),
					AgentID:     agentID,
				})
			}
		}
		
		// Check for test results
		if matches := testPassRegex.FindStringSubmatch(line); len(matches) > 1 {
			testName := strings.TrimSpace(matches[1])
			testName = cleanupText(testName)
			if testName != "" {
				testResults = append(testResults, TestResult{
					TestName:   testName,
					Passed:     true,
					Message:    "Test passed",
					ExecutedAt: time.Now(),
					AgentID:    agentID,
				})
			}
		} else if matches := testFailRegex.FindStringSubmatch(line); len(matches) > 1 {
			testName := strings.TrimSpace(matches[1])
			testName = cleanupText(testName)
			if testName != "" {
				testResults = append(testResults, TestResult{
					TestName:   testName,
					Passed:     false,
					Message:    "Test failed",
					ExecutedAt: time.Now(),
					AgentID:    agentID,
				})
			}
		}
	}
	
	// Also look for structured sections in the output
	features = append(features, parseStructuredFeatures(output, agentID)...)
	bugs = append(bugs, parseStructuredBugs(output, agentID)...)
	testResults = append(testResults, parseStructuredTests(output, agentID)...)
	
	// Remove duplicates
	features = deduplicateFeatures(features)
	bugs = deduplicateBugs(bugs)
	testResults = deduplicateTests(testResults)
	
	return features, bugs, testResults
}

// parseStructuredFeatures looks for structured feature lists
func parseStructuredFeatures(output string, agentID string) []Feature {
	var features []Feature
	
	// Look for sections like "Features:", "Implemented:", etc.
	sections := []string{
		"features implemented",
		"implemented features",
		"new features",
		"features added",
		"completed features",
	}
	
	for _, section := range sections {
		if idx := strings.Index(strings.ToLower(output), section); idx != -1 {
			// Extract the section
			sectionText := output[idx:]
			lines := strings.Split(sectionText, "\n")
			
			// Parse bullet points or numbered lists
			for i := 1; i < len(lines) && i < 20; i++ { // Limit to 20 lines
				line := strings.TrimSpace(lines[i])
				
				// Stop at next section
				if strings.Contains(strings.ToLower(line), "bug") ||
					strings.Contains(strings.ToLower(line), "test") ||
					strings.Contains(strings.ToLower(line), "issue") {
					break
				}
				
				// Extract from bullet points or numbered lists
				if match := regexp.MustCompile(`^[-*•]\s*(.+)`).FindStringSubmatch(line); len(match) > 1 {
					featureText := cleanupText(match[1])
					if featureText != "" {
						features = append(features, Feature{
							Name:          extractFeatureName(featureText),
							Description:   featureText,
							ImplementedAt: time.Now(),
							AgentID:       agentID,
						})
					}
				} else if match := regexp.MustCompile(`^\d+\.\s*(.+)`).FindStringSubmatch(line); len(match) > 1 {
					featureText := cleanupText(match[1])
					if featureText != "" {
						features = append(features, Feature{
							Name:          extractFeatureName(featureText),
							Description:   featureText,
							ImplementedAt: time.Now(),
							AgentID:       agentID,
						})
					}
				}
			}
		}
	}
	
	return features
}

// parseStructuredBugs looks for structured bug lists
func parseStructuredBugs(output string, agentID string) []Bug {
	var bugs []Bug
	
	// Look for sections like "Bugs:", "Issues:", etc.
	sections := []string{
		"known bugs",
		"bugs found",
		"issues found",
		"problems found",
		"current issues",
	}
	
	for _, section := range sections {
		if idx := strings.Index(strings.ToLower(output), section); idx != -1 {
			// Extract the section
			sectionText := output[idx:]
			lines := strings.Split(sectionText, "\n")
			
			// Parse bullet points or numbered lists
			for i := 1; i < len(lines) && i < 20; i++ { // Limit to 20 lines
				line := strings.TrimSpace(lines[i])
				
				// Stop at next section
				if strings.Contains(strings.ToLower(line), "feature") ||
					strings.Contains(strings.ToLower(line), "test") ||
					strings.Contains(strings.ToLower(line), "implement") {
					break
				}
				
				// Extract from bullet points or numbered lists
				if match := regexp.MustCompile(`^[-*•]\s*(.+)`).FindStringSubmatch(line); len(match) > 1 {
					bugText := cleanupText(match[1])
					if bugText != "" {
						bugs = append(bugs, Bug{
							Description: bugText,
							Severity:    guessSeverity(bugText),
							Status:      "open",
							FoundAt:     time.Now(),
							AgentID:     agentID,
						})
					}
				} else if match := regexp.MustCompile(`^\d+\.\s*(.+)`).FindStringSubmatch(line); len(match) > 1 {
					bugText := cleanupText(match[1])
					if bugText != "" {
						bugs = append(bugs, Bug{
							Description: bugText,
							Severity:    guessSeverity(bugText),
							Status:      "open",
							FoundAt:     time.Now(),
							AgentID:     agentID,
						})
					}
				}
			}
		}
	}
	
	return bugs
}

// parseStructuredTests looks for structured test results
func parseStructuredTests(output string, agentID string) []TestResult {
	var testResults []TestResult
	
	// Look for test output patterns
	testPatterns := []string{
		`(\d+) passed.* (\d+) failed`,
		`Tests?: (\d+) passed`,
		`All tests passed`,
		`(\d+) test\(s\) failed`,
	}
	
	for _, pattern := range testPatterns {
		regex := regexp.MustCompile(pattern)
		if matches := regex.FindStringSubmatch(output); len(matches) > 0 {
			if strings.Contains(matches[0], "All tests passed") {
				testResults = append(testResults, TestResult{
					TestName:   "All Tests",
					Passed:     true,
					Message:    "All tests passed",
					ExecutedAt: time.Now(),
					AgentID:    agentID,
				})
			} else if len(matches) > 2 {
				// Extract passed/failed counts
				failedCount := matches[2]
				testResults = append(testResults, TestResult{
					TestName:   "Test Suite",
					Passed:     failedCount == "0",
					Message:    matches[0],
					ExecutedAt: time.Now(),
					AgentID:    agentID,
				})
			}
		}
	}
	
	return testResults
}

// Helper functions

func cleanupText(text string) string {
	// Remove common prefixes and clean up
	text = strings.TrimSpace(text)
	text = strings.TrimSuffix(text, ".")
	text = strings.TrimSuffix(text, ",")
	text = strings.TrimPrefix(text, "- ")
	text = strings.TrimPrefix(text, "* ")
	text = strings.TrimPrefix(text, "• ")
	
	// Remove quotes
	text = strings.Trim(text, `"'`)
	
	return text
}

func extractFeatureName(description string) string {
	// Try to extract a concise name from the description
	if len(description) <= 50 {
		return description
	}
	
	// Look for key action words
	actionWords := []string{"API", "authentication", "login", "database", "UI", "frontend", "backend", "endpoint", "route", "component", "function", "method", "class", "module"}
	
	lower := strings.ToLower(description)
	for _, word := range actionWords {
		if strings.Contains(lower, strings.ToLower(word)) {
			// Extract phrase around the action word
			idx := strings.Index(lower, strings.ToLower(word))
			start := idx - 20
			if start < 0 {
				start = 0
			}
			end := idx + len(word) + 20
			if end > len(description) {
				end = len(description)
			}
			return strings.TrimSpace(description[start:end]) + "..."
		}
	}
	
	// Default: first 50 characters
	return description[:50] + "..."
}

func guessSeverity(description string) string {
	lower := strings.ToLower(description)
	
	// Critical severity indicators
	if strings.Contains(lower, "crash") ||
		strings.Contains(lower, "security") ||
		strings.Contains(lower, "data loss") ||
		strings.Contains(lower, "critical") ||
		strings.Contains(lower, "urgent") {
		return "critical"
	}
	
	// High severity indicators
	if strings.Contains(lower, "broken") ||
		strings.Contains(lower, "fail") ||
		strings.Contains(lower, "error") ||
		strings.Contains(lower, "cannot") ||
		strings.Contains(lower, "doesn't work") {
		return "high"
	}
	
	// Low severity indicators
	if strings.Contains(lower, "minor") ||
		strings.Contains(lower, "cosmetic") ||
		strings.Contains(lower, "typo") ||
		strings.Contains(lower, "improvement") {
		return "low"
	}
	
	// Default to medium
	return "medium"
}

func deduplicateFeatures(features []Feature) []Feature {
	seen := make(map[string]bool)
	result := []Feature{}
	
	for _, f := range features {
		key := strings.ToLower(f.Description)
		if !seen[key] {
			seen[key] = true
			result = append(result, f)
		}
	}
	
	return result
}

func deduplicateBugs(bugs []Bug) []Bug {
	seen := make(map[string]bool)
	result := []Bug{}
	
	for _, b := range bugs {
		key := strings.ToLower(b.Description)
		if !seen[key] {
			seen[key] = true
			result = append(result, b)
		}
	}
	
	return result
}

func deduplicateTests(tests []TestResult) []TestResult {
	seen := make(map[string]bool)
	result := []TestResult{}
	
	for _, t := range tests {
		key := strings.ToLower(t.TestName)
		if !seen[key] {
			seen[key] = true
			result = append(result, t)
		}
	}
	
	return result
}