#!/bin/bash
# Claude Code Task Hook
# Intercepts Task tool calls and routes them through ASO
#
# Installation:
#   1. Copy this script to ~/.claude/hooks/
#   2. Make it executable: chmod +x ~/.claude/hooks/claude-task-hook.sh
#   3. Configure Claude Code to use this hook (see settings below)
#
# Claude Code settings.json configuration:
# {
#   "hooks": {
#     "Task": {
#       "command": "~/.claude/hooks/claude-task-hook.sh",
#       "mode": "replace"
#     }
#   }
# }

set -e

# Configuration
ASO_BIN="${ASO_BIN:-aso}"
LOG_FILE="${HOME}/.agent-sandboxes/hook.log"

# Ensure log directory exists
mkdir -p "$(dirname "${LOG_FILE}")"

# Log function
log() {
    echo "[$(date -Iseconds)] $*" >> "${LOG_FILE}"
}

# Parse input from Claude Code
# Input comes as JSON on stdin
INPUT=$(cat)

log "Received Task input: ${INPUT:0:200}..."

# Extract fields from JSON input
SUBAGENT_TYPE=$(echo "${INPUT}" | jq -r '.subagent_type // "Explore"')
PROMPT=$(echo "${INPUT}" | jq -r '.prompt // ""')
PROJECT_PATH=$(echo "${INPUT}" | jq -r '.project_path // "."')

log "Subagent type: ${SUBAGENT_TYPE}"
log "Prompt: ${PROMPT:0:100}..."

# Check if ASO is available
if ! command -v "${ASO_BIN}" &> /dev/null; then
    log "ASO not found, falling back to native execution"
    # Output indicates hook should not intercept
    echo '{"intercept": false}'
    exit 0
fi

# Check if ASO system is running
if ! "${ASO_BIN}" system status &> /dev/null; then
    log "ASO system not running, starting..."
    "${ASO_BIN}" system start &>> "${LOG_FILE}" || {
        log "Failed to start ASO system, falling back to native"
        echo '{"intercept": false}'
        exit 0
    }
fi

# Spawn sandboxed agent
log "Spawning sandboxed agent..."
AGENT_ID=$("${ASO_BIN}" spawn \
    --type "${SUBAGENT_TYPE}" \
    --prompt "${PROMPT}" \
    --project "${PROJECT_PATH}" \
    2>> "${LOG_FILE}")

if [ -z "${AGENT_ID}" ]; then
    log "Failed to spawn agent"
    echo '{"intercept": false}'
    exit 0
fi

log "Agent spawned: ${AGENT_ID}"

# Wait for agent to complete and get result
log "Waiting for agent completion..."
RESULT=$("${ASO_BIN}" result "${AGENT_ID}" 2>> "${LOG_FILE}")

log "Agent completed, result length: ${#RESULT}"

# Return result to Claude Code
# Format: JSON with the agent's output
jq -n \
    --arg agent_id "${AGENT_ID}" \
    --arg result "${RESULT}" \
    '{
        "intercept": true,
        "agent_id": $agent_id,
        "result": $result
    }'

log "Hook completed successfully"
