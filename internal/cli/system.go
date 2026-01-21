package cli

import (
	"fmt"

	"github.com/agent-sandbox-orchestrator/aso/internal/container"
	"github.com/spf13/cobra"
)

var systemCmd = &cobra.Command{
	Use:   "system",
	Short: "Manage the ASO system and Apple container runtime",
}

var systemStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the ASO daemon and container runtime",
	RunE: func(cmd *cobra.Command, args []string) error {
		runtime, err := container.NewAppleRuntime()
		if err != nil {
			return err
		}

		if err := runtime.StartSystem(cmd.Context()); err != nil {
			return fmt.Errorf("failed to start system: %w", err)
		}

		fmt.Println("ASO system started")
		return nil
	},
}

var systemStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the ASO daemon and all containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		runtime, err := container.NewAppleRuntime()
		if err != nil {
			return err
		}

		if err := runtime.StopSystem(cmd.Context()); err != nil {
			return fmt.Errorf("failed to stop system: %w", err)
		}

		fmt.Println("ASO system stopped")
		return nil
	},
}

var systemStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show ASO system status",
	RunE: func(cmd *cobra.Command, args []string) error {
		runtime, err := container.NewAppleRuntime()
		if err != nil {
			return err
		}

		status, err := runtime.SystemStatus(cmd.Context())
		if err != nil {
			return err
		}

		fmt.Printf("Runtime: %s\n", status.Runtime)
		fmt.Printf("Status:  %s\n", status.Status)
		fmt.Printf("Version: %s\n", status.Version)
		fmt.Printf("Running containers: %d\n", status.RunningContainers)

		return nil
	},
}

func init() {
	systemCmd.AddCommand(systemStartCmd)
	systemCmd.AddCommand(systemStopCmd)
	systemCmd.AddCommand(systemStatusCmd)
}
