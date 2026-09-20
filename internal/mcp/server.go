package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/pkmdev-sec/agents-sandboxing/internal/agent"
	"github.com/pkmdev-sec/agents-sandboxing/internal/container"
)

// Server implements an MCP server for agent sandboxing
type Server struct {
	runtime container.Runtime
	runner  *agent.Runner
	store   *agent.Store

	// Running agents
	mu     sync.Mutex
	agents map[string]*runningAgent
}

type runningAgent struct {
	ID       string
	Type     string
	Status   string
	Progress []string
	Result   *agent.Result
}

// NewServer creates a new MCP server
func NewServer() (*Server, error) {
	runtime, err := container.AutoDetectRuntime()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize runtime: %w", err)
	}

	return &Server{
		runtime: runtime,
		runner:  agent.NewRunner(runtime),
		store:   agent.NewStore(),
		agents:  make(map[string]*runningAgent),
	}, nil
}

// Serve starts the MCP server, reading from stdin and writing to stdout
func (s *Server) Serve(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	// Send server info
	s.sendServerInfo(writer)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Read JSON-RPC request
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read error: %w", err)
		}

		// Parse and handle request
		response := s.handleRequest(ctx, line)
		if response != nil {
			responseBytes, _ := json.Marshal(response)
			fmt.Fprintln(writer, string(responseBytes))
		}
	}
}

// JSON-RPC types
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (s *Server) sendServerInfo(w io.Writer) {
	info := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "aso",
				"version": "0.1.0",
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{
					"listChanged": false,
				},
			},
		},
	}
	bytes, _ := json.Marshal(info)
	fmt.Fprintln(w, string(bytes))
}

func (s *Server) handleRequest(ctx context.Context, data []byte) *jsonRPCResponse {
	var req jsonRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      nil,
			Error:   &rpcError{Code: -32700, Message: "Parse error"},
		}
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	case "ping":
		return &jsonRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: map[string]string{}}
	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "Method not found"},
		}
	}
}

func (s *Server) handleInitialize(req jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "aso",
				"version": "0.1.0",
			},
			"capabilities": map[string]interface{}{
				"tools": map[string]bool{
					"listChanged": false,
				},
			},
		},
	}
}

func (s *Server) handleToolsList(req jsonRPCRequest) *jsonRPCResponse {
	tools := []map[string]interface{}{
		{
			"name":        "spawn_sandboxed_agent",
			"description": "Spawn a separate Claude Code process in an isolated container",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Agent type (Explore, Plan, Bash, code-reviewer)",
						"enum":        []string{"Explore", "Plan", "Bash", "code-reviewer", "general-purpose"},
					},
					"prompt": map[string]interface{}{
						"type":        "string",
						"description": "The task prompt for the agent",
					},
					"project_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the project directory to mount (default: current directory)",
					},
					"cpus": map[string]interface{}{
						"type":        "integer",
						"description": "Number of CPU cores (default: 2)",
					},
					"memory": map[string]interface{}{
						"type":        "string",
						"description": "Memory limit (e.g., '1g', '512m')",
					},
					"timeout": map[string]interface{}{
						"type":        "string",
						"description": "Execution timeout (e.g., '30m', '1h')",
					},
				},
				"required": []string{"prompt"},
			},
		},
		{
			"name":        "get_agent_status",
			"description": "Get the status and result of a sandboxed agent",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "The agent ID returned from spawn_sandboxed_agent",
					},
				},
				"required": []string{"agent_id"},
			},
		},
		{
			"name":        "list_sandboxed_agents",
			"description": "List all sandboxed agents and their status",
			"inputSchema": map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			"name":        "kill_sandboxed_agent",
			"description": "Terminate a running sandboxed agent",
			"inputSchema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"agent_id": map[string]interface{}{
						"type":        "string",
						"description": "The agent ID to terminate",
					},
				},
				"required": []string{"agent_id"},
			},
		},
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]interface{}{"tools": tools},
	}
}

func (s *Server) handleToolsCall(ctx context.Context, req jsonRPCRequest) *jsonRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "Invalid params"},
		}
	}

	var result interface{}
	var err error

	switch params.Name {
	case "spawn_sandboxed_agent":
		result, err = s.toolSpawnAgent(ctx, params.Arguments)
	case "get_agent_status":
		result, err = s.toolGetAgentStatus(params.Arguments)
	case "list_sandboxed_agents":
		result, err = s.toolListAgents()
	case "kill_sandboxed_agent":
		result, err = s.toolKillAgent(ctx, params.Arguments)
	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "Unknown tool: " + params.Name},
		}
	}

	if err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"content": []map[string]interface{}{
					{"type": "text", "text": fmt.Sprintf("Error: %v", err)},
				},
				"isError": true,
			},
		}
	}

	// Format result as MCP tool result
	resultText, _ := json.MarshalIndent(result, "", "  ")
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": string(resultText)},
			},
		},
	}
}

// Tool implementations

func (s *Server) toolSpawnAgent(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Type        string `json:"type"`
		Prompt      string `json:"prompt"`
		ProjectPath string `json:"project_path"`
		CPUs        int    `json:"cpus"`
		Memory      string `json:"memory"`
		Timeout     string `json:"timeout"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	// Defaults
	if params.Type == "" {
		params.Type = "Explore"
	}
	if params.ProjectPath == "" {
		params.ProjectPath = "."
	}
	if params.Timeout == "" {
		params.Timeout = "30m"
	}

	// Get resource profile
	profile := agent.GetProfile(params.Type)
	if params.CPUs > 0 {
		profile.CPUs = params.CPUs
	}
	if params.Memory != "" {
		profile.Memory = params.Memory
	}

	// Create agent config
	config := agent.Config{
		Type:        params.Type,
		Prompt:      params.Prompt,
		ProjectPath: params.ProjectPath,
		Profile:     profile,
		Timeout:     params.Timeout,
	}

	// Track the agent
	agentID := fmt.Sprintf("agent-%d", time.Now().UnixNano())
	s.mu.Lock()
	s.agents[agentID] = &runningAgent{
		ID:       agentID,
		Type:     params.Type,
		Status:   "starting",
		Progress: []string{"Agent spawning..."},
	}
	s.mu.Unlock()

	// Run agent in background
	go func() {
		s.mu.Lock()
		if ra, ok := s.agents[agentID]; ok {
			ra.Status = "running"
			ra.Progress = append(ra.Progress, "Container started")
		}
		s.mu.Unlock()

		result, err := s.runner.Run(ctx, config)

		s.mu.Lock()
		defer s.mu.Unlock()
		if ra, ok := s.agents[agentID]; ok {
			if err != nil {
				ra.Status = "failed"
				ra.Progress = append(ra.Progress, fmt.Sprintf("Error: %v", err))
			} else {
				ra.Status = result.Status
				ra.Result = result
				ra.Progress = append(ra.Progress, "Agent completed")
			}
		}
	}()

	return map[string]interface{}{
		"agent_id": agentID,
		"type":     params.Type,
		"status":   "starting",
		"message":  fmt.Sprintf("Agent %s spawned with type %s", agentID, params.Type),
	}, nil
}

func (s *Server) toolGetAgentStatus(args json.RawMessage) (interface{}, error) {
	var params struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ra, ok := s.agents[params.AgentID]
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", params.AgentID)
	}

	response := map[string]interface{}{
		"agent_id": ra.ID,
		"type":     ra.Type,
		"status":   ra.Status,
		"progress": ra.Progress,
	}

	if ra.Result != nil {
		response["result"] = map[string]interface{}{
			"summary":   ra.Result.Summary,
			"output":    ra.Result.Output,
			"exit_code": ra.Result.ExitCode,
			"duration":  ra.Result.EndTime.Sub(ra.Result.StartTime).String(),
		}
	}

	return response, nil
}

func (s *Server) toolListAgents() (interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	agents := make([]map[string]interface{}, 0, len(s.agents))
	for _, ra := range s.agents {
		agent := map[string]interface{}{
			"agent_id": ra.ID,
			"type":     ra.Type,
			"status":   ra.Status,
		}
		if ra.Result != nil {
			agent["summary"] = ra.Result.Summary
		}
		agents = append(agents, agent)
	}

	return map[string]interface{}{
		"agents": agents,
		"count":  len(agents),
	}, nil
}

func (s *Server) toolKillAgent(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.AgentID == "" {
		return nil, fmt.Errorf("agent_id is required")
	}

	s.mu.Lock()
	ra, ok := s.agents[params.AgentID]
	if !ok {
		s.mu.Unlock()
		return nil, fmt.Errorf("agent not found: %s", params.AgentID)
	}

	// Mark as killed
	ra.Status = "killed"
	ra.Progress = append(ra.Progress, "Agent terminated by user")
	s.mu.Unlock()

	// Try to stop the container if we have a mapping
	containerID, err := s.store.GetContainerID(params.AgentID)
	if err == nil && containerID != "" {
		s.runtime.StopContainer(ctx, containerID)
		s.runtime.DeleteContainer(ctx, containerID)
	}

	return map[string]interface{}{
		"agent_id": params.AgentID,
		"status":   "killed",
		"message":  "Agent terminated",
	}, nil
}
