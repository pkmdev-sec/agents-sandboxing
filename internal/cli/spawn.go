package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkmdev-sec/agents-sandboxing/internal/agent"
	"github.com/pkmdev-sec/agents-sandboxing/internal/container"
	"github.com/spf13/cobra"
)

var (
	spawnType    string
	spawnPrompt  string
	spawnProject string
	spawnCPUs    int
	spawnMemory  string
	spawnTimeout string
	spawnRuntime string
	spawnVerbose bool
)

var spawnCmd = &cobra.Command{
	Use:   "spawn",
	Short: "Spawn a new sandboxed agent",
	Long: `Spawn a new agent in an isolated container.

The agent runs as a separate Claude Code process inside a container.

Supports both Apple container and Docker runtimes.`,
	Example: `  # Spawn an Explore agent (auto-detect runtime)
  aso spawn --type Explore --prompt "Find authentication code" --project .

  # Spawn with specific runtime
  aso spawn --runtime docker --type Bash --prompt "Run tests" --project .

  # Spawn with custom resources
  aso spawn --type Bash --prompt "Run tests" --project . --cpus 4 --memory 2g`,
	RunE: runSpawn,
}

func init() {
	spawnCmd.Flags().StringVarP(&spawnType, "type", "t", "Explore", "Agent type (Explore, Plan, Bash, code-reviewer)")
	spawnCmd.Flags().StringVarP(&spawnPrompt, "prompt", "p", "", "Prompt for the agent (required)")
	spawnCmd.Flags().StringVar(&spawnProject, "project", ".", "Project directory to mount")
	spawnCmd.Flags().IntVarP(&spawnCPUs, "cpus", "c", 0, "CPU cores (0 = use profile default)")
	spawnCmd.Flags().StringVarP(&spawnMemory, "memory", "m", "", "Memory limit (e.g., 1g, 512m)")
	spawnCmd.Flags().StringVar(&spawnTimeout, "timeout", "30m", "Execution timeout")
	spawnCmd.Flags().StringVarP(&spawnRuntime, "runtime", "r", "auto", "Container runtime (auto, docker, apple)")
	spawnCmd.Flags().BoolVarP(&spawnVerbose, "verbose", "v", false, "Show verbose output")

	spawnCmd.MarkFlagRequired("prompt")
}

func runSpawn(cmd *cobra.Command, args []string) error {
	// Resolve project path
	projectPath, err := filepath.Abs(spawnProject)
	if err != nil {
		return fmt.Errorf("invalid project path: %w", err)
	}

	// Verify project exists
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return fmt.Errorf("project directory does not exist: %s", projectPath)
	}

	// Get resource profile for agent type
	profile := agent.GetProfile(spawnType)
	if spawnCPUs > 0 {
		profile.CPUs = spawnCPUs
	}
	if spawnMemory != "" {
		profile.Memory = spawnMemory
	}

	// Create container runtime
	var runtime container.Runtime
	var runtimeName string

	switch spawnRuntime {
	case "auto":
		runtime, err = container.AutoDetectRuntime()
		runtimeName = container.GetRuntimeName(runtime)
	case "docker":
		runtime, err = container.NewDockerRuntime()
		runtimeName = "docker"
	case "apple":
		runtime, err = container.NewAppleRuntime()
		runtimeName = "apple"
	default:
		return fmt.Errorf("unknown runtime: %s (use auto, docker, or apple)", spawnRuntime)
	}

	if err != nil {
		return fmt.Errorf("failed to initialize container runtime: %w", err)
	}

	if spawnVerbose {
		fmt.Fprintf(os.Stderr, "Using %s runtime\n", runtimeName)
		fmt.Fprintf(os.Stderr, "Agent type: %s\n", spawnType)
		fmt.Fprintf(os.Stderr, "Project: %s\n", projectPath)
		fmt.Fprintf(os.Stderr, "Resources: %d CPUs, %s memory\n", profile.CPUs, profile.Memory)
	}

	// Create and run agent
	runner := agent.NewRunner(runtime)

	config := agent.Config{
		Type:        spawnType,
		Prompt:      spawnPrompt,
		ProjectPath: projectPath,
		Profile:     profile,
		Timeout:     spawnTimeout,
	}

	if spawnVerbose {
		fmt.Fprintf(os.Stderr, "Spawning agent...\n")
	}

	result, err := runner.Run(cmd.Context(), config)
	if err != nil {
		return fmt.Errorf("agent execution failed: %w", err)
	}

	if spawnVerbose {
		fmt.Fprintf(os.Stderr, "Agent completed with status: %s\n", result.Status)
		fmt.Fprintf(os.Stderr, "Duration: %s\n", result.EndTime.Sub(result.StartTime))
		if result.Summary != "" {
			fmt.Fprintf(os.Stderr, "\n--- Summary ---\n%s\n", result.Summary)
		}
	}

	// Output agent ID for scripting
	fmt.Println(result.AgentID)

	return nil
}
