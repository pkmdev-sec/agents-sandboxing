package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Store manages persistent agent data
type Store struct {
	baseDir string
}

// NewStore creates a new agent store
func NewStore() *Store {
	homeDir, _ := os.UserHomeDir()
	baseDir := filepath.Join(homeDir, ".agent-sandboxes")
	os.MkdirAll(baseDir, 0755)

	return &Store{baseDir: baseDir}
}

// SaveResult persists an agent result
func (s *Store) SaveResult(result *Result) error {
	resultsDir := filepath.Join(s.baseDir, "results")
	os.MkdirAll(resultsDir, 0755)

	filePath := filepath.Join(resultsDir, result.AgentID+".json")

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// GetResult retrieves an agent result
func (s *Store) GetResult(agentID string) (*Result, error) {
	// Handle partial IDs
	if !strings.HasPrefix(agentID, "agent-") {
		agentID = "agent-" + agentID
	}

	resultsDir := filepath.Join(s.baseDir, "results")

	// Try exact match first
	filePath := filepath.Join(resultsDir, agentID+".json")
	if _, err := os.Stat(filePath); err == nil {
		return s.loadResult(filePath)
	}

	// Try partial match
	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), agentID) {
			return s.loadResult(filepath.Join(resultsDir, entry.Name()))
		}
	}

	return nil, fmt.Errorf("agent not found: %s", agentID)
}

func (s *Store) loadResult(filePath string) (*Result, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ListResults returns all stored results
func (s *Store) ListResults() ([]*Result, error) {
	resultsDir := filepath.Join(s.baseDir, "results")

	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		return nil, err
	}

	var results []*Result
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			result, err := s.loadResult(filepath.Join(resultsDir, entry.Name()))
			if err == nil {
				results = append(results, result)
			}
		}
	}

	return results, nil
}

// MapAgentToContainer stores the mapping between agent ID and container ID
func (s *Store) MapAgentToContainer(agentID, containerID string) error {
	mappingDir := filepath.Join(s.baseDir, "mappings")
	os.MkdirAll(mappingDir, 0755)

	filePath := filepath.Join(mappingDir, agentID)
	return os.WriteFile(filePath, []byte(containerID), 0644)
}

// GetContainerID returns the container ID for an agent
func (s *Store) GetContainerID(agentID string) (string, error) {
	mappingDir := filepath.Join(s.baseDir, "mappings")
	filePath := filepath.Join(mappingDir, agentID)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("container not found for agent: %s", agentID)
	}

	return string(data), nil
}

// Cleanup removes old agent data
func (s *Store) Cleanup(olderThan string) (int, error) {
	duration, err := parseDuration(olderThan)
	if err != nil {
		return 0, err
	}

	cutoff := time.Now().Add(-duration)
	count := 0

	// Clean results
	resultsDir := filepath.Join(s.baseDir, "results")
	if entries, err := os.ReadDir(resultsDir); err == nil {
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.Remove(filepath.Join(resultsDir, entry.Name()))
				count++
			}
		}
	}

	// Clean logs
	logsDir := filepath.Join(s.baseDir, "logs")
	if entries, err := os.ReadDir(logsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.RemoveAll(filepath.Join(logsDir, entry.Name()))
				count++
			}
		}
	}

	// Clean mappings
	mappingDir := filepath.Join(s.baseDir, "mappings")
	if entries, err := os.ReadDir(mappingDir); err == nil {
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.Remove(filepath.Join(mappingDir, entry.Name()))
			}
		}
	}

	return count, nil
}

// parseDuration parses durations like "7d", "24h", "30m"
func parseDuration(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration: %s", s)
	}

	unit := s[len(s)-1]
	value := s[:len(s)-1]

	var multiplier time.Duration
	switch unit {
	case 'd':
		multiplier = 24 * time.Hour
	case 'h':
		multiplier = time.Hour
	case 'm':
		multiplier = time.Minute
	case 's':
		multiplier = time.Second
	default:
		return time.ParseDuration(s)
	}

	var num int
	_, err := fmt.Sscanf(value, "%d", &num)
	if err != nil {
		return 0, err
	}

	return time.Duration(num) * multiplier, nil
}
