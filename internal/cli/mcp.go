package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/pkmdev-sec/agents-sandboxing/internal/mcp"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP server commands",
	Long:  `Commands for running ASO as an MCP server.`,
}

var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the MCP server",
	Long: `Start ASO as an MCP server that communicates via stdio.

This allows Conductor and other MCP clients to spawn sandboxed agents.

The server implements the MCP protocol and provides these tools:
  - spawn_sandboxed_agent: Spawn a new agent in an isolated container
  - get_agent_status: Get the status and result of an agent
  - list_sandboxed_agents: List all agents
  - kill_sandboxed_agent: Terminate a running agent`,
	RunE: runMCPServe,
}

func init() {
	mcpCmd.AddCommand(mcpServeCmd)
}

func runMCPServe(cmd *cobra.Command, args []string) error {
	// Create MCP server
	server, err := mcp.NewServer()
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "Shutting down MCP server...")
		cancel()
	}()

	// Run the server
	fmt.Fprintln(os.Stderr, "ASO MCP server started")
	return server.Serve(ctx)
}
