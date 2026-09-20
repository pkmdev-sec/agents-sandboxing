package container

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	containerBin    = "container"
	defaultImage    = "ghcr.io/pkmdev-sec/agents-sandboxing-runtime:latest"
	containerPrefix = "aso-agent-"
)

// AppleRuntime implements Runtime using Apple's container CLI
type AppleRuntime struct {
	binPath string
}

// NewAppleRuntime creates a new Apple container runtime
func NewAppleRuntime() (*AppleRuntime, error) {
	// Find container binary
	binPath, err := exec.LookPath(containerBin)
	if err != nil {
		// Check common installation paths
		commonPaths := []string{
			"/usr/local/bin/container",
			"/opt/homebrew/bin/container",
		}
		for _, p := range commonPaths {
			if _, err := os.Stat(p); err == nil {
				binPath = p
				break
			}
		}
		if binPath == "" {
			return nil, fmt.Errorf("Apple container CLI not found. Install from: https://github.com/apple/container")
		}
	}

	return &AppleRuntime{binPath: binPath}, nil
}

// CreateContainer creates a new container with the given configuration
func (r *AppleRuntime) CreateContainer(ctx context.Context, config CreateConfig) (string, error) {
	args := []string{"create"}

	// Add name
	if config.Name != "" {
		args = append(args, "--name", config.Name)
	}

	// Add resources
	if config.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", config.CPUs))
	}
	if config.Memory != "" {
		args = append(args, "--memory", config.Memory)
	}

	// Add mounts
	for _, m := range config.Mounts {
		mountStr := fmt.Sprintf("%s:%s", m.Source, m.Target)
		if m.ReadOnly {
			mountStr += ":ro"
		}
		args = append(args, "-v", mountStr)
	}

	// Add environment variables
	for k, v := range config.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Add image
	image := config.Image
	if image == "" {
		image = defaultImage
	}
	args = append(args, image)

	// Add command
	args = append(args, config.Command...)

	cmd := exec.CommandContext(ctx, r.binPath, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("container create failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("container create failed: %w", err)
	}

	// Parse container ID from output
	containerID := strings.TrimSpace(string(output))
	return containerID, nil
}

// StartContainer starts a created container
func (r *AppleRuntime) StartContainer(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, r.binPath, "start", id)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("container start failed: %s", string(output))
	}
	return nil
}

// StopContainer stops a running container
func (r *AppleRuntime) StopContainer(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, r.binPath, "stop", id)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("container stop failed: %s", string(output))
	}
	return nil
}

// DeleteContainer removes a container
func (r *AppleRuntime) DeleteContainer(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, r.binPath, "delete", id)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("container delete failed: %s", string(output))
	}
	return nil
}

// WaitContainer waits for a container to exit and returns the exit code
func (r *AppleRuntime) WaitContainer(ctx context.Context, id string) (int, error) {
	// Poll container status until it exits
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-ticker.C:
			info, err := r.GetContainer(ctx, id)
			if err != nil {
				return -1, err
			}
			if info.Status == "exited" || info.Status == "stopped" {
				// Get exit code from inspect
				exitCode, _ := r.getExitCode(ctx, id)
				return exitCode, nil
			}
		}
	}
}

func (r *AppleRuntime) getExitCode(ctx context.Context, id string) (int, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "inspect", id)
	output, err := cmd.Output()
	if err != nil {
		return -1, err
	}

	var result struct {
		State struct {
			ExitCode int `json:"ExitCode"`
		} `json:"State"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return -1, err
	}

	return result.State.ExitCode, nil
}

// ListContainers returns all ASO-managed containers
func (r *AppleRuntime) ListContainers(ctx context.Context) ([]ContainerInfo, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("container list failed: %w", err)
	}

	var containers []struct {
		ID      string `json:"ID"`
		Name    string `json:"Name"`
		State   string `json:"State"`
		Created string `json:"Created"`
	}

	if err := json.Unmarshal(output, &containers); err != nil {
		// Try parsing as newline-delimited JSON
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		for _, line := range lines {
			var c struct {
				ID      string `json:"ID"`
				Name    string `json:"Name"`
				State   string `json:"State"`
				Created string `json:"Created"`
			}
			if err := json.Unmarshal([]byte(line), &c); err == nil {
				containers = append(containers, c)
			}
		}
	}

	// Filter to ASO-managed containers
	var result []ContainerInfo
	for _, c := range containers {
		if strings.HasPrefix(c.Name, containerPrefix) {
			// Extract agent type from name (aso-agent-{type}-{id})
			parts := strings.Split(c.Name, "-")
			agentType := "unknown"
			if len(parts) >= 3 {
				agentType = parts[2]
			}

			result = append(result, ContainerInfo{
				ID:      c.ID,
				Name:    c.Name,
				Type:    agentType,
				Status:  c.State,
				Started: c.Created,
			})
		}
	}

	return result, nil
}

// GetContainer returns information about a specific container
func (r *AppleRuntime) GetContainer(ctx context.Context, id string) (*ContainerInfo, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "inspect", id)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("container inspect failed: %w", err)
	}

	var result struct {
		ID    string `json:"Id"`
		Name  string `json:"Name"`
		State struct {
			Status string `json:"Status"`
		} `json:"State"`
		Created string `json:"Created"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse container info: %w", err)
	}

	return &ContainerInfo{
		ID:     result.ID,
		Name:   result.Name,
		Status: result.State.Status,
	}, nil
}

// StreamLogs streams container logs to the given writer
func (r *AppleRuntime) StreamLogs(ctx context.Context, id string, follow bool, out io.Writer) error {
	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, id)

	cmd := exec.CommandContext(ctx, r.binPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		fmt.Fprintln(out, scanner.Text())
	}

	return cmd.Wait()
}

// GetOutput returns the full output of a container
func (r *AppleRuntime) GetOutput(ctx context.Context, id string) (string, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "logs", id)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// StartSystem starts the Apple container system
func (r *AppleRuntime) StartSystem(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, r.binPath, "system", "start")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("system start failed: %s", string(output))
	}
	return nil
}

// StopSystem stops the Apple container system
func (r *AppleRuntime) StopSystem(ctx context.Context) error {
	// First stop all ASO containers
	containers, err := r.ListContainers(ctx)
	if err == nil {
		for _, c := range containers {
			r.StopContainer(ctx, c.ID)
			r.DeleteContainer(ctx, c.ID)
		}
	}

	cmd := exec.CommandContext(ctx, r.binPath, "system", "stop")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("system stop failed: %s", string(output))
	}
	return nil
}

// SystemStatus returns the status of the container system
func (r *AppleRuntime) SystemStatus(ctx context.Context) (*SystemStatus, error) {
	// Check if system is running
	cmd := exec.CommandContext(ctx, r.binPath, "--version")
	versionOutput, err := cmd.Output()
	if err != nil {
		return &SystemStatus{
			Runtime: "apple",
			Status:  "not available",
		}, nil
	}

	// Get running container count
	containers, _ := r.ListContainers(ctx)

	return &SystemStatus{
		Runtime:           "apple",
		Status:            "running",
		Version:           strings.TrimSpace(string(versionOutput)),
		RunningContainers: len(containers),
	}, nil
}

// PullImage pulls a container image
func (r *AppleRuntime) PullImage(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, r.binPath, "image", "pull", image)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("image pull failed: %s", string(output))
	}
	return nil
}

// ImageExists checks if an image exists locally
func (r *AppleRuntime) ImageExists(ctx context.Context, image string) (bool, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "image", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	return strings.Contains(string(output), image), nil
}

// Helper to generate unique container names
func GenerateContainerName(agentType string) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s%s-%d", containerPrefix, strings.ToLower(agentType), timestamp)
}

// GetLogsDir returns the logs directory for an agent
func GetLogsDir(agentID string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agent-sandboxes", "logs", agentID)
}
