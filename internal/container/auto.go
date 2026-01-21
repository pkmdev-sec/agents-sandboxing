package container

import (
	"fmt"
	"os/exec"
)

// RuntimeType represents the container runtime type
type RuntimeType string

const (
	RuntimeApple  RuntimeType = "apple"
	RuntimeDocker RuntimeType = "docker"
	RuntimeAuto   RuntimeType = "auto"
)

// NewRuntime creates a runtime based on the specified type
func NewRuntime(runtimeType RuntimeType) (Runtime, error) {
	switch runtimeType {
	case RuntimeApple:
		return NewAppleRuntime()
	case RuntimeDocker:
		return NewDockerRuntime()
	case RuntimeAuto:
		return AutoDetectRuntime()
	default:
		return nil, fmt.Errorf("unknown runtime type: %s", runtimeType)
	}
}

// AutoDetectRuntime automatically selects the best available runtime
// Currently defaults to Docker as it has better enterprise proxy/VPN compatibility
func AutoDetectRuntime() (Runtime, error) {
	// Use Docker as the primary runtime (works with Zscaler and other enterprise proxies)
	if dockerRuntime, err := NewDockerRuntime(); err == nil {
		return dockerRuntime, nil
	}

	// Fall back to Apple container if Docker isn't available
	if appleRuntime, err := NewAppleRuntime(); err == nil {
		return appleRuntime, nil
	}

	return nil, fmt.Errorf("no container runtime available. Install Docker Desktop")
}

// PreferredRuntime returns the preferred runtime (Docker first)
func PreferredRuntime() (Runtime, string, error) {
	// Check for Docker first (better enterprise compatibility)
	if _, err := exec.LookPath("docker"); err == nil {
		runtime, err := NewDockerRuntime()
		if err == nil {
			return runtime, "docker", nil
		}
	}

	// Fall back to Apple container
	if _, err := exec.LookPath("container"); err == nil {
		runtime, err := NewAppleRuntime()
		if err == nil {
			return runtime, "apple", nil
		}
	}

	return nil, "", fmt.Errorf("no container runtime available")
}

// GetRuntimeName returns the name of the runtime type
func GetRuntimeName(runtime Runtime) string {
	switch runtime.(type) {
	case *AppleRuntime:
		return "apple"
	case *DockerRuntime:
		return "docker"
	default:
		return "unknown"
	}
}
