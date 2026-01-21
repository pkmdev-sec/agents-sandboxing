# ASO - Agent Sandbox Orchestrator

Automatic agent sandboxing for Claude Code and Conductor. Each agent runs in an isolated Docker container with its own context window and token budget.

## Why ASO?

When Claude Code spawns subagents via the Task tool, they share the parent's context window and token budget. This causes:

- **Token conflicts**: Parallel agents compete for the same token allocation
- **Context pollution**: Agent outputs intermingle in the parent's context
- **No true parallelism**: Agents effectively serialize due to shared resources

ASO solves this by running each agent in an isolated container with its own Claude session.

## Quick Start

```bash
# One-command setup
make setup

# Or step by step:
make build
make install
make runtime-image
make mcp-config  # Configures Conductor integration
```

## Requirements

- macOS or Linux
- Docker Desktop
- Go 1.21+ (for building)
- `ANTHROPIC_API_KEY` environment variable set

## Usage

### CLI Commands

```bash
# Spawn an agent directly
aso spawn --type Explore --prompt "Find authentication code" --project .

# List running/completed agents
aso list

# Get agent result
aso result <agent-id>

# View agent logs
aso logs <agent-id>

# Kill a running agent
aso kill <agent-id>

# Clean up old agent data (default: 7 days)
aso cleanup --older-than 7d

# Check system status
aso system status
```

### MCP Server (for Conductor)

Start the MCP server:

```bash
aso mcp serve
```

Or configure Conductor to start it automatically by running `make mcp-config`.

### MCP Tools

When connected to Conductor, these tools are available:

| Tool | Description |
|------|-------------|
| `spawn_sandboxed_agent` | Spawn a new agent in isolated container |
| `get_agent_status` | Get agent status and result |
| `list_sandboxed_agents` | List all agents |
| `kill_sandboxed_agent` | Terminate a running agent |

### Agent Types

| Type | CPUs | Memory | Use Case |
|------|------|--------|----------|
| Explore | 2 | 1GB | Codebase exploration, file search |
| Plan | 2 | 1GB | Implementation planning |
| Bash | 4 | 2GB | Running tests, builds, scripts |
| code-reviewer | 2 | 1GB | Code review tasks |
| general-purpose | 2 | 1GB | General agent tasks |

### Examples

**Spawn an exploration agent:**
```bash
aso spawn -t Explore -p "Find all files that handle user authentication" --project /path/to/repo
```

**Spawn a bash agent to run tests:**
```bash
aso spawn -t Bash -p "Run the test suite and summarize failures" --project . --timeout 1h
```

**Custom resources:**
```bash
aso spawn -t Bash -p "Build the project" --cpus 4 --memory 4g --project .
```

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Main Session (Claude Code / Conductor)                 │
│                                                         │
│  "Spawn an agent to explore auth code"                  │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼ MCP: spawn_sandboxed_agent
┌─────────────────────────────────────────────────────────┐
│  ASO (Agent Sandbox Orchestrator)                       │
│  - Creates Docker container                             │
│  - Mounts project workspace                             │
│  - Runs Claude Code in container                        │
│  - Returns summary when complete                        │
└─────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────┐
│  Docker Container                                       │
│  ┌─────────────────────────────────────────────────┐   │
│  │  /workspace (mounted from host)                  │   │
│  │  Claude Code CLI                                 │   │
│  │  Own API key, own context, own token budget     │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## Configuration

### Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `ANTHROPIC_API_KEY` | API key for Claude | Yes |

### Conductor MCP Config

Add to `~/.claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "aso": {
      "command": "/usr/local/bin/aso",
      "args": ["mcp", "serve"]
    }
  }
}
```

Or run `make mcp-config` to set this up automatically.

## Development

```bash
# Build
make build

# Run tests
make test

# Format code
make fmt

# Development mode (auto-rebuild on changes)
make dev
```

## How It Works

1. **Agent Request**: Claude Code or Conductor calls `spawn_sandboxed_agent` via MCP
2. **Container Creation**: ASO creates a Docker container with:
   - Project directory mounted at `/workspace`
   - ANTHROPIC_API_KEY passed through
   - Resource limits based on agent type
3. **Execution**: Claude Code CLI runs inside the container with the prompt
4. **Result**: Output is captured and returned via `get_agent_status`
5. **Cleanup**: Container is removed after completion

## Troubleshooting

### "Docker daemon not running"
Start Docker Desktop and try again.

### "ANTHROPIC_API_KEY not set"
Export your API key: `export ANTHROPIC_API_KEY=sk-ant-...`

### Agent times out
Increase timeout: `aso spawn --timeout 1h ...`

### Container networking issues
If behind a corporate VPN/proxy, Docker usually handles this automatically. If issues persist, check Docker Desktop's proxy settings.

## License

MIT
