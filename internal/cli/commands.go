package cli

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/agent-sandbox-orchestrator/aso/internal/agent"
	"github.com/agent-sandbox-orchestrator/aso/internal/container"
	"github.com/spf13/cobra"
)

// Version info (set at build time)
var (
	Version   = "dev"
	GitCommit = "unknown"
	BuildDate = "unknown"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List running sandboxed agents",
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		runtime, err := container.NewAppleRuntime()
		if err != nil {
			return err
		}

		agents, err := runtime.ListContainers(cmd.Context())
		if err != nil {
			return err
		}

		if len(agents) == 0 {
			fmt.Println("No running agents")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "AGENT ID\tTYPE\tSTATUS\tSTARTED\tPROJECT")
		for _, a := range agents {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
				a.ID[:12], a.Type, a.Status, a.Started, a.ProjectPath)
		}
		w.Flush()

		return nil
	},
}

var logsAgentID string
var logsFollow bool

var logsCmd = &cobra.Command{
	Use:   "logs [agent-id]",
	Short: "View logs from a sandboxed agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID := args[0]

		runtime, err := container.NewAppleRuntime()
		if err != nil {
			return err
		}

		return runtime.StreamLogs(cmd.Context(), agentID, logsFollow, os.Stdout)
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output")
}

var resultCmd = &cobra.Command{
	Use:   "result [agent-id]",
	Short: "Get the result/summary from a completed agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID := args[0]

		store := agent.NewStore()
		result, err := store.GetResult(agentID)
		if err != nil {
			return err
		}

		fmt.Println(result.Summary)
		return nil
	},
}

var killCmd = &cobra.Command{
	Use:   "kill [agent-id]",
	Short: "Terminate a running agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		agentID := args[0]

		runtime, err := container.NewAppleRuntime()
		if err != nil {
			return err
		}

		if err := runtime.StopContainer(cmd.Context(), agentID); err != nil {
			return err
		}

		fmt.Printf("Agent %s terminated\n", agentID)
		return nil
	},
}

var cleanupOlderThan string

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up old agent logs and containers",
	RunE: func(cmd *cobra.Command, args []string) error {
		store := agent.NewStore()
		count, err := store.Cleanup(cleanupOlderThan)
		if err != nil {
			return err
		}

		fmt.Printf("Cleaned up %d old agent records\n", count)
		return nil
	},
}

func init() {
	cleanupCmd.Flags().StringVar(&cleanupOlderThan, "older-than", "7d", "Remove records older than duration")
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("aso version %s\n", Version)
		fmt.Printf("  git commit: %s\n", GitCommit)
		fmt.Printf("  build date: %s\n", BuildDate)
	},
}
