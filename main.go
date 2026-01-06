package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/ashton2914/mcp-netutil/pkg/system"
	"github.com/ashton2914/mcp-netutil/pkg/traceroute"
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
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP Specific structures
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema Schema `json:"inputSchema"`
}

type Schema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string            `json:"name"`
	Arguments map[string]string `json:"arguments"`
}

type CallToolResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Session Management
type Session struct {
	ID       string
	MsgChan  chan *JSONRPCResponse
	LastUsed time.Time
}

var (
	sessions = make(map[string]*Session)
	mu       sync.RWMutex
)

func main() {
	addr := flag.String("a", "0.0.0.0", "Address to bind to (e.g., 0.0.0.0)")
	port := flag.Int("p", 0, "Port to listen on (0 for Stdio mode)")
	flag.Parse()

	// Set log output to stderr
	log.SetOutput(os.Stderr)

	if *port > 0 {
		startSSEServer(*addr, *port)
	} else {
		serveStdio()
	}
}

func startSSEServer(addr string, port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", handleSSE)
	mux.HandleFunc("/messages", handleMessages)

	bindAddr := fmt.Sprintf("%s:%d", addr, port)
	log.Printf("MCP Server (SSE) listening on %s", bindAddr)
	log.Printf("SSE Endpoint: http://%s/sse", bindAddr)

	if err := http.ListenAndServe(bindAddr, mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create session
	sessionID := generateID()
	session := &Session{
		ID:       sessionID,
		MsgChan:  make(chan *JSONRPCResponse, 10),
		LastUsed: time.Now(),
	}

	mu.Lock()
	sessions[sessionID] = session
	mu.Unlock()

	// Cleanup on disconnect
	defer func() {
		mu.Lock()
		delete(sessions, sessionID)
		mu.Unlock()
		close(session.MsgChan)
		log.Printf("Session %s disconnected", sessionID)
	}()

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	log.Printf("New session connected: %s", sessionID)

	// Send initial endpoint event
	// The client uses this to know where to post messages
	// We'll infer protocol from request (http vs https) - assuming http for this simple tool
	endpoint := fmt.Sprintf("/messages?session_id=%s", sessionID)
	fmt.Fprintf(w, "event: endpoint\ndata: %s\n\n", endpoint)
	flusher.Flush()

	// Loop and send messages
	for {
		select {
		case resp := <-session.MsgChan:
			data, err := json.Marshal(resp)
			if err != nil {
				log.Printf("Error marshaling response: %v", err)
				continue
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func handleMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Error(w, "Missing session_id", http.StatusBadRequest)
		return
	}

	mu.RLock()
	session, exists := sessions[sessionID]
	mu.RUnlock()

	if !exists {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	var req JSONRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Handle request
	// Note: In a real agentic scenario, we might want to handle this asynchronously
	// if the tool calls take a long time, but for simplicity we block here.
	// Actually for traceroute, it takes time. If we block handleMessages, the client POST request hangs.
	// We should probably respond to the POST immediately (Accepted) and send the result via SSE.
	// However, standard JSON-RPC over HTTP usually expects a response?
	// But in MCP SSE, the flow is: POST -> "Accepted" -> SSE event "message" with JSON-RPC response.

	w.WriteHeader(http.StatusAccepted)

	// Process in background to not block the POST request
	go func() {
		// 10 second timeout for the request
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		resultChan := make(chan *JSONRPCResponse, 1)

		go func() {
			resultChan <- handleRequest(ctx, &req)
		}()

		select {
		case resp := <-resultChan:
			if resp != nil {
				session.MsgChan <- resp
			}
		case <-ctx.Done():
			errResp := errorResponse(req.ID, -32000, "Request timed out after 10s")
			session.MsgChan <- errResp
		}
	}()
}

func serveStdio() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		var req JSONRPCRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				break
			}
			log.Printf("Error decoding request: %v", err)
			continue
		}

		// Stdio mode also gets simple timeout handling, though stdio is typically synchronous
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		resp := handleRequest(ctx, &req)
		cancel()
		if resp != nil {
			if err := encoder.Encode(resp); err != nil {
				log.Printf("Error encoding response: %v", err)
			}
		}
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func handleRequest(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	switch req.Method {
	case "initialize":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2024-11-05",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]string{
					"name":    "mcp-netutil",
					"version": "0.1.2",
				},
			},
		}
	case "notifications/initialized":
		return nil
	case "tools/list":
		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: ListToolsResult{
				Tools: []Tool{
					{
						Name:        "traceroute",
						Description: "Execute a traceroute to a target IP or domain",
						InputSchema: Schema{
							Type: "object",
							Properties: map[string]Property{
								"target": {
									Type:        "string",
									Description: "The target IP address or hostname to traceroute",
								},
							},
							Required: []string{"target"},
						},
					},
					{
						Name:        "system_stats",
						Description: "Get system statistics (CPU, Memory, Disk)",
						InputSchema: Schema{
							Type:       "object",
							Properties: map[string]Property{},
						},
					},
				},
			},
		}
	case "tools/call":
		var params CallToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return errorResponse(req.ID, -32700, "Parse error")
		}

		if params.Name == "system_stats" {
			output, err := system.GetStats(ctx)
			if err != nil {
				return &JSONRPCResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result: CallToolResult{
						Content: []Content{
							{Type: "text", Text: fmt.Sprintf("Failed to get system stats: %v", err)},
						},
						IsError: true,
					},
				}
			}

			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: CallToolResult{
					Content: []Content{
						{Type: "text", Text: output},
					},
				},
			}
		}

		if params.Name != "traceroute" {
			return errorResponse(req.ID, -32601, "Tool not found")
		}

		target, ok := params.Arguments["target"]
		if !ok || target == "" {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: CallToolResult{
					Content: []Content{
						{Type: "text", Text: "Error: Missing target argument"},
					},
					IsError: true,
				},
			}
		}

		output, err := traceroute.Run(ctx, target)
		if err != nil {
			return &JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: CallToolResult{
					Content: []Content{
						{Type: "text", Text: fmt.Sprintf("Traceroute execution failed: %v\nOutput:\n%s", err, output)},
					},
					IsError: true,
				},
			}
		}

		return &JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: CallToolResult{
				Content: []Content{
					{Type: "text", Text: output},
				},
			},
		}
	default:
		return errorResponse(req.ID, -32601, "Method not found")
	}
}

func errorResponse(id interface{}, code int, message string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
}
