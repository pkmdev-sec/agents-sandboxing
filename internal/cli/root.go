package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aso",
	Short: "Agent Sandbox Orchestrator - Run agents in isolated Apple containers",
	Long: `ASO (Agent Sandbox Orchestrator) automatically sandboxes AI agents
in Apple containers for context/token isolation and parallel execution.

Each agent runs in its own lightweight VM with dedicated resources,
preventing token budget conflicts and enabling true parallel execution.`,
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
