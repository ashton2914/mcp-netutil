package mcp

import (
	"encoding/json"
	"fmt"
)

// JSON-RPC Request/Response structures
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      interface{}     `json:"id"`
}

type JSONRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Result  interface{}   `json:"result,omitempty"`
	Error   *JSONRPCError `json:"error,omitempty"`
	ID      interface{}   `json:"id"`
}

type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// MCP Specific Structures
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

type CallToolResult struct {
	Content []ToolContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Server logic
type Server struct {
	tools map[string]RegisteredTool
}

type RegisteredTool struct {
	Definition Tool
	Handler    ToolHandler
}

type ToolHandler func(arguments map[string]interface{}) (CallToolResult, error)

func NewServer() *Server {
	return &Server{
		tools: make(map[string]RegisteredTool),
	}
}

func (s *Server) RegisterTool(name string, description string, schema json.RawMessage, handler ToolHandler) {
	s.tools[name] = RegisteredTool{
		Definition: Tool{
			Name:        name,
			Description: description,
			InputSchema: schema,
		},
		Handler: handler,
	}
}

func (s *Server) HandleRequest(req JSONRPCRequest) JSONRPCResponse {
	switch req.Method {
	case "tools/list":
		return s.handleListTools(req.ID)
	case "tools/call":
		return s.handleCallTool(req.ID, req.Params)
	case "initialize":
		return s.handleInitialize(req.ID)
	case "notifications/initialized":
		// Just ignore
		return JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
	default:
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &JSONRPCError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

func (s *Server) handleInitialize(id interface{}) JSONRPCResponse {
	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"serverInfo": map[string]string{
				"name":    "mcp-netutil",
				"version": "0.1.6",
			},
		},
	}
}

func (s *Server) handleListTools(id interface{}) JSONRPCResponse {
	var toolsList []Tool
	for _, t := range s.tools {
		toolsList = append(toolsList, t.Definition)
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]interface{}{
			"tools": toolsList,
		},
	}
}

func (s *Server) handleCallTool(id interface{}, params json.RawMessage) JSONRPCResponse {
	var callParams struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments"`
	}
	if err := json.Unmarshal(params, &callParams); err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &JSONRPCError{Code: -32700, Message: "Parse error"},
		}
	}

	tool, ok := s.tools[callParams.Name]
	if !ok {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &JSONRPCError{Code: -32601, Message: fmt.Sprintf("Tool %s not found", callParams.Name)},
		}
	}

	result, err := tool.Handler(callParams.Arguments)
	if err != nil {
		return JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      id,
			Error:   &JSONRPCError{Code: -32000, Message: err.Error()},
		}
	}

	return JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}
