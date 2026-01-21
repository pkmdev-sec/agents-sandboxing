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

const dockerBin = "docker"

// DockerRuntime implements Runtime using Docker
type DockerRuntime struct {
	binPath string
}

// NewDockerRuntime creates a new Docker container runtime
func NewDockerRuntime() (*DockerRuntime, error) {
	binPath, err := exec.LookPath(dockerBin)
	if err != nil {
		return nil, fmt.Errorf("Docker not found: %w", err)
	}

	// Verify Docker is running
	cmd := exec.Command(binPath, "info")
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("Docker daemon not running: %w", err)
	}

	return &DockerRuntime{binPath: binPath}, nil
}

// CreateContainer creates a new Docker container
func (r *DockerRuntime) CreateContainer(ctx context.Context, config CreateConfig) (string, error) {
	args := []string{"create"}

	if config.Name != "" {
		args = append(args, "--name", config.Name)
	}

	if config.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", config.CPUs))
	}
	if config.Memory != "" {
		args = append(args, "--memory", config.Memory)
	}

	for _, m := range config.Mounts {
		mountStr := fmt.Sprintf("%s:%s", m.Source, m.Target)
		if m.ReadOnly {
			mountStr += ":ro"
		}
		args = append(args, "-v", mountStr)
	}

	for k, v := range config.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	image := config.Image
	if image == "" {
		image = defaultImage
	}
	args = append(args, image)
	args = append(args, config.Command...)

	cmd := exec.CommandContext(ctx, r.binPath, args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("docker create failed: %s", string(exitErr.Stderr))
		}
		return "", fmt.Errorf("docker create failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// StartContainer starts a created container
func (r *DockerRuntime) StartContainer(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, r.binPath, "start", id)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker start failed: %s", string(output))
	}
	return nil
}

// StopContainer stops a running container
func (r *DockerRuntime) StopContainer(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, r.binPath, "stop", "-t", "10", id)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker stop failed: %s", string(output))
	}
	return nil
}

// DeleteContainer removes a container
func (r *DockerRuntime) DeleteContainer(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, r.binPath, "rm", "-f", id)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker rm failed: %s", string(output))
	}
	return nil
}

// WaitContainer waits for a container to exit
func (r *DockerRuntime) WaitContainer(ctx context.Context, id string) (int, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "wait", id)
	output, err := cmd.Output()
	if err != nil {
		return -1, fmt.Errorf("docker wait failed: %w", err)
	}

	var exitCode int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &exitCode)
	if err != nil {
		return -1, fmt.Errorf("failed to parse exit code: %w", err)
	}

	return exitCode, nil
}

// ListContainers returns all ASO-managed containers
func (r *DockerRuntime) ListContainers(ctx context.Context) ([]ContainerInfo, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "ps", "-a",
		"--filter", "name="+containerPrefix,
		"--format", "{{json .}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	var result []ContainerInfo
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		var c struct {
			ID      string `json:"ID"`
			Names   string `json:"Names"`
			State   string `json:"State"`
			Created string `json:"CreatedAt"`
		}
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			continue
		}

		// Extract agent type from name
		parts := strings.Split(c.Names, "-")
		agentType := "unknown"
		if len(parts) >= 3 {
			agentType = parts[2]
		}

		result = append(result, ContainerInfo{
			ID:      c.ID,
			Name:    c.Names,
			Type:    agentType,
			Status:  c.State,
			Started: c.Created,
		})
	}

	return result, nil
}

// GetContainer returns information about a specific container
func (r *DockerRuntime) GetContainer(ctx context.Context, id string) (*ContainerInfo, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "inspect", id)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker inspect failed: %w", err)
	}

	var containers []struct {
		ID    string `json:"Id"`
		Name  string `json:"Name"`
		State struct {
			Status string `json:"Status"`
		} `json:"State"`
	}

	if err := json.Unmarshal(output, &containers); err != nil {
		return nil, fmt.Errorf("failed to parse container info: %w", err)
	}

	if len(containers) == 0 {
		return nil, fmt.Errorf("container not found: %s", id)
	}

	c := containers[0]
	return &ContainerInfo{
		ID:     c.ID,
		Name:   strings.TrimPrefix(c.Name, "/"),
		Status: c.State.Status,
	}, nil
}

// StreamLogs streams container logs
func (r *DockerRuntime) StreamLogs(ctx context.Context, id string, follow bool, out io.Writer) error {
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
func (r *DockerRuntime) GetOutput(ctx context.Context, id string) (string, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "logs", id)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// StartSystem is a no-op for Docker (already running)
func (r *DockerRuntime) StartSystem(ctx context.Context) error {
	return nil
}

// StopSystem stops all ASO containers but not Docker itself
func (r *DockerRuntime) StopSystem(ctx context.Context) error {
	containers, err := r.ListContainers(ctx)
	if err != nil {
		return err
	}

	for _, c := range containers {
		r.StopContainer(ctx, c.ID)
		r.DeleteContainer(ctx, c.ID)
	}

	return nil
}

// SystemStatus returns Docker status
func (r *DockerRuntime) SystemStatus(ctx context.Context) (*SystemStatus, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "version", "--format", "{{.Server.Version}}")
	versionOutput, err := cmd.Output()
	if err != nil {
		return &SystemStatus{
			Runtime: "docker",
			Status:  "not running",
		}, nil
	}

	containers, _ := r.ListContainers(ctx)

	return &SystemStatus{
		Runtime:           "docker",
		Status:            "running",
		Version:           strings.TrimSpace(string(versionOutput)),
		RunningContainers: len(containers),
	}, nil
}

// PullImage pulls a container image
func (r *DockerRuntime) PullImage(ctx context.Context, image string) error {
	cmd := exec.CommandContext(ctx, r.binPath, "pull", image)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker pull failed: %s", string(output))
	}
	return nil
}

// ImageExists checks if an image exists locally
func (r *DockerRuntime) ImageExists(ctx context.Context, image string) (bool, error) {
	cmd := exec.CommandContext(ctx, r.binPath, "images", "-q", image)
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(output)) != "", nil
}

// RunAndWait is a convenience method for Docker that runs a container and waits for completion
func (r *DockerRuntime) RunAndWait(ctx context.Context, config CreateConfig) (string, int, error) {
	args := []string{"run", "--rm"}

	if config.Name != "" {
		args = append(args, "--name", config.Name)
	}

	if config.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", config.CPUs))
	}
	if config.Memory != "" {
		args = append(args, "--memory", config.Memory)
	}

	for _, m := range config.Mounts {
		mountStr := fmt.Sprintf("%s:%s", m.Source, m.Target)
		if m.ReadOnly {
			mountStr += ":ro"
		}
		args = append(args, "-v", mountStr)
	}

	for k, v := range config.Environment {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	image := config.Image
	if image == "" {
		image = defaultImage
	}
	args = append(args, image)
	args = append(args, config.Command...)

	// Debug: uncomment to see command
	// fmt.Fprintf(os.Stderr, "DEBUG: docker %s\n", strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, r.binPath, args...)

	// Capture output in a way that survives context cancellation
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Combine output
	output := stdout.String() + stderr.String()

	exitCode := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return output, 124, fmt.Errorf("timeout: %w", ctx.Err())
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return output, -1, err
		}
	}

	return output, exitCode, nil
}

// GetLogsDir returns the logs directory for an agent (same as Apple runtime)
func GetDockerLogsDir(agentID string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agent-sandboxes", "logs", agentID)
}

// GenerateDockerContainerName generates a unique container name
func GenerateDockerContainerName(agentType string) string {
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("%s%s-%d", containerPrefix, strings.ToLower(agentType), timestamp)
}
