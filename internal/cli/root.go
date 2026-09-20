package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aso",
	Short: "Run Claude Code jobs in isolated containers",
	Long: `ASO (Agent Sandbox Orchestrator) automatically sandboxes AI agents
in Docker or Apple containers for process and filesystem isolation.

Each agent runs in a separate Claude Code process with bounded CPU and memory.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(spawnCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(resultCmd)
	rootCmd.AddCommand(killCmd)
	rootCmd.AddCommand(systemCmd)
	rootCmd.AddCommand(cleanupCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(mcpCmd)
}
