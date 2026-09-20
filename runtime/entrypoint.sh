#!/bin/bash
# Agent Sandbox Entrypoint
# Runs Claude Code with the provided prompt and captures output

# Configuration
LOG_DIR="${LOG_DIR:-/var/log/agent}"
OUTPUT_LOG="${LOG_DIR}/output.log"
TIMEOUT="${AGENT_TIMEOUT:-1800}"  # Default 30 min timeout

# Ensure log directory exists
mkdir -p "${LOG_DIR}" 2>/dev/null || true

# Log start time
{
    echo "Agent started at $(date -Iseconds)"
    echo "Agent ID: ${AGENT_ID:-unknown}"
    echo "Agent Type: ${AGENT_TYPE:-unknown}"
    echo "---"
} > "${OUTPUT_LOG}" 2>/dev/null || true

# Check if prompt is provided
if [ -z "${AGENT_PROMPT}" ]; then
    echo "Error: AGENT_PROMPT environment variable not set"
    exit 1
fi

# Check for API key
if [ -z "${ANTHROPIC_API_KEY}" ]; then
    echo "Warning: ANTHROPIC_API_KEY not set"
fi

echo "Executing prompt: ${AGENT_PROMPT:0:100}..."

# Run Claude Code in non-interactive print mode with timeout
# --print runs the prompt and exits
# Using timeout to prevent hanging
timeout "${TIMEOUT}" claude --print --dangerously-skip-permissions "${AGENT_PROMPT}" 2>&1
CLAUDE_EXIT_CODE=$?

if [ ${CLAUDE_EXIT_CODE} -eq 124 ]; then
    echo "Agent timed out after ${TIMEOUT} seconds"
    exit 124
fi

echo ""
echo "=== AGENT COMPLETED (exit code: ${CLAUDE_EXIT_CODE}) ==="

exit ${CLAUDE_EXIT_CODE}
