package container

import (
	"context"
	"io"
)

// ContainerInfo represents information about a running container
type ContainerInfo struct {
	ID          string
	Name        string
	Type        string // Agent type (Explore, Plan, Bash, etc.)
	Status      string // running, stopped, exited
	Started     string
	ProjectPath string
	CPUs        int
	Memory      string
}

// SystemStatus represents the status of the container runtime
type SystemStatus struct {
	Runtime           string
	Status            string
	Version           string
	RunningContainers int
}

// CreateConfig holds configuration for creating a container
type CreateConfig struct {
	Name        string
	Image       string
	CPUs        int
	Memory      string
	Mounts      []Mount
	Environment map[string]string
	Command     []string
}

// Mount represents a volume mount
type Mount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// Runtime defines the interface for container runtimes
type Runtime interface {
	// Container lifecycle
	CreateContainer(ctx context.Context, config CreateConfig) (string, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	DeleteContainer(ctx context.Context, id string) error
	WaitContainer(ctx context.Context, id string) (int, error)

	// Container inspection
	ListContainers(ctx context.Context) ([]ContainerInfo, error)
	GetContainer(ctx context.Context, id string) (*ContainerInfo, error)

	// Logs and output
	StreamLogs(ctx context.Context, id string, follow bool, out io.Writer) error
	GetOutput(ctx context.Context, id string) (string, error)

	// System management
	StartSystem(ctx context.Context) error
	StopSystem(ctx context.Context) error
	SystemStatus(ctx context.Context) (*SystemStatus, error)

	// Image management
	PullImage(ctx context.Context, image string) error
	ImageExists(ctx context.Context, image string) (bool, error)
}
