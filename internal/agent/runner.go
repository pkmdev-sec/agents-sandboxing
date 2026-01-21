package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/agent-sandbox-orchestrator/aso/internal/container"
)

// Config holds the configuration for running an agent
type Config struct {
	Type        string
	Prompt      string
	ProjectPath string
	Profile     Profile
	Timeout     string
}

// Result holds the result of an agent execution
type Result struct {
	AgentID   string
	Type      string
	Status    string // completed, failed, timeout
	Summary   string
	Output    string
	StartTime time.Time
	EndTime   time.Time
	ExitCode  int
}

// Runner manages agent execution in containers
type Runner struct {
	runtime container.Runtime
	store   *Store
}

// NewRunner creates a new agent runner
func NewRunner(runtime container.Runtime) *Runner {
	return &Runner{
		runtime: runtime,
		store:   NewStore(),
	}
}

// Run executes an agent in a sandboxed container
func (r *Runner) Run(ctx context.Context, config Config) (*Result, error) {
	// Generate unique agent ID
	agentID := generateAgentID()

	// Create logs directory
	logsDir := container.GetLogsDir(agentID)
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Prepare container configuration
	containerName := container.GenerateContainerName(config.Type)

	mounts := []container.Mount{
		{
			Source:   config.ProjectPath,
			Target:   "/workspace",
			ReadOnly: false,
		},
		{
			Source:   logsDir,
			Target:   "/var/log/agent",
			ReadOnly: false,
		},
	}

	// Note: We intentionally do NOT mount the host's ~/.claude directory
	// as it contains MCP servers, hooks, and other config that can interfere
	// with sandboxed execution. The API key is passed via environment variable.

	// Environment variables
	env := map[string]string{
		"AGENT_ID":     agentID,
		"AGENT_TYPE":   config.Type,
		"AGENT_PROMPT": config.Prompt,
	}

	// Pass through API key if set
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		env["ANTHROPIC_API_KEY"] = apiKey
	}

	// Parse timeout for container
	timeout, err := time.ParseDuration(config.Timeout)
	if err != nil {
		timeout = 30 * time.Minute
	}
	env["AGENT_TIMEOUT"] = fmt.Sprintf("%d", int(timeout.Seconds()))

	// Create container config
	containerConfig := container.CreateConfig{
		Name:        containerName,
		Image:       "ghcr.io/agent-sandbox-orchestrator/agent-runtime:latest",
		CPUs:        config.Profile.CPUs,
		Memory:      config.Profile.Memory,
		Mounts:      mounts,
		Environment: env,
		Command:     []string{}, // Use entrypoint default
	}

	// Record start
	result := &Result{
		AgentID:   agentID,
		Type:      config.Type,
		Status:    "running",
		StartTime: time.Now(),
	}

	// Save initial state
	r.store.SaveResult(result)

	// For Docker runtime, use RunAndWait which is more reliable
	if dockerRuntime, ok := r.runtime.(*container.DockerRuntime); ok {
		timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		output, exitCode, err := dockerRuntime.RunAndWait(timeoutCtx, containerConfig)
		result.EndTime = time.Now()

		if err != nil {
			if timeoutCtx.Err() == context.DeadlineExceeded {
				result.Status = "timeout"
				result.Summary = fmt.Sprintf("Agent timed out after %s", config.Timeout)
			} else {
				result.Status = "failed"
				result.Summary = fmt.Sprintf("Agent execution error: %v", err)
			}
		} else {
			result.ExitCode = exitCode
			result.Output = output
			if exitCode == 0 {
				result.Status = "completed"
				result.Summary = extractSummary(output)
			} else {
				result.Status = "failed"
				result.Summary = extractSummary(output)
			}
		}

		r.store.SaveResult(result)
		return result, nil
	}

	// For other runtimes (Apple), use create/start/wait pattern
	containerID, err := r.runtime.CreateContainer(ctx, containerConfig)
	if err != nil {
		result.Status = "failed"
		result.Summary = fmt.Sprintf("Failed to create container: %v", err)
		r.store.SaveResult(result)
		return result, err
	}

	// Store container mapping
	r.store.MapAgentToContainer(agentID, containerID)

	if err := r.runtime.StartContainer(ctx, containerID); err != nil {
		result.Status = "failed"
		result.Summary = fmt.Sprintf("Failed to start container: %v", err)
		r.store.SaveResult(result)
		r.runtime.DeleteContainer(ctx, containerID)
		return result, err
	}

	// Wait for completion with timeout
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	exitCode, err := r.runtime.WaitContainer(waitCtx, containerID)
	result.EndTime = time.Now()

	if err != nil {
		if waitCtx.Err() == context.DeadlineExceeded {
			result.Status = "timeout"
			result.Summary = fmt.Sprintf("Agent timed out after %s", config.Timeout)
			r.runtime.StopContainer(ctx, containerID)
		} else {
			result.Status = "failed"
			result.Summary = fmt.Sprintf("Agent execution error: %v", err)
		}
	} else {
		result.ExitCode = exitCode
		if exitCode == 0 {
			result.Status = "completed"
		} else {
			result.Status = "failed"
		}
	}

	// Get output
	output, _ := r.runtime.GetOutput(ctx, containerID)
	result.Output = output

	// Read summary from logs directory
	summaryPath := filepath.Join(logsDir, "summary.txt")
	if summaryBytes, err := os.ReadFile(summaryPath); err == nil {
		result.Summary = string(summaryBytes)
	} else if result.Summary == "" {
		// Use last lines of output as summary
		result.Summary = extractSummary(output)
	}

	// Save final result
	r.store.SaveResult(result)

	// Cleanup container
	r.runtime.DeleteContainer(ctx, containerID)

	return result, nil
}

// generateAgentID creates a unique agent identifier
func generateAgentID() string {
	return fmt.Sprintf("agent-%d", time.Now().UnixNano())
}

// extractSummary extracts a summary from the output
func extractSummary(output string) string {
	lines := splitLines(output)
	if len(lines) == 0 {
		return "No output"
	}

	// Return last few non-empty lines
	var summary []string
	for i := len(lines) - 1; i >= 0 && len(summary) < 5; i-- {
		line := lines[i]
		if line != "" {
			summary = append([]string{line}, summary...)
		}
	}

	if len(summary) == 0 {
		return "No output"
	}

	result := ""
	for _, line := range summary {
		result += line + "\n"
	}
	return result
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
