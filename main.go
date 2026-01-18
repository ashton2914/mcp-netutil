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

	"github.com/ashton2914/mcp-netutil/pkg/cache"
	"github.com/ashton2914/mcp-netutil/pkg/latency"
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
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
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
	dbDir := flag.String("D", "", "Directory to store execution records (e.g. /home/user/mcp/share/mcp-netutil)")
	flag.Parse()

	// Set log output to stderr
	log.SetOutput(os.Stderr)

	if *dbDir != "" {
		if err := cache.Init(*dbDir); err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		log.Printf("Database initialized at %s", *dbDir)
	}

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
		// 300 second timeout for the request (5 minutes) to allow for long running tools like latency_check
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
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
			errResp := errorResponse(req.ID, -32000, "Request timed out after 300s")
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
		ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
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
				"protocolVersion": "2024-11-05", // Updated 2024-11-05
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{},
				},
				"serverInfo": map[string]string{
					"name":    "mcp-netutil",
					"version": "0.1.4",
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
					{
						Name:        "latency_check",
						Description: "Check latency and packet loss to a target",
						InputSchema: Schema{
							Type: "object",
							Properties: map[string]Property{
								"target": {
									Type:        "string",
									Description: "The target IP address or hostname",
								},
								"mode": {
									Type:        "string",
									Description: "Test mode: 'quick' (10 pkts) or 'standard' (100 pkts)",
								},
							},
							Required: []string{"target"},
						},
					},
					{
						Name:        "read_records",
						Description: "Read execution records from the database. Time format must be YYYYMMDDhhmmss.",
						InputSchema: Schema{
							Type: "object",
							Properties: map[string]Property{
								"tool": {
									Type:        "string",
									Description: "Tool name to query (latency, traceroute, system_stats)",
								},
								"start_time": {
									Type:        "string",
									Description: "Start time (YYYYMMDDhhmmss) for filtering",
								},
								"end_time": {
									Type:        "string",
									Description: "End time (YYYYMMDDhhmmss) for filtering",
								},
								"target": {
									Type:        "string",
									Description: "Target host to filter by (for latency and traceroute)",
								},
							},
							Required: []string{"tool"},
						},
					},
					{
						Name:        "kill_process",
						Description: "Terminate a process by PID. The AI Agent must resolve the process name to a PID (e.g., using system_stats) before calling this.",
						InputSchema: Schema{
							Type: "object",
							Properties: map[string]Property{
								"pid": {
									Type:        "integer",
									Description: "Process ID to terminate",
								},
							},
							Required: []string{"pid"},
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
				return createErrorResponse(req.ID, fmt.Sprintf("Failed to get system stats: %v", err))
			}

			// Record stats
			if err := cache.RecordSystemStats(output); err != nil {
				log.Printf("Failed to record system stats: %v", err)
			}

			return createSuccessResponse(req.ID, output)
		}

		if params.Name == "latency_check" {
			target := getStringArg(params.Arguments, "target")
			if target == "" {
				return createErrorResponse(req.ID, "Error: Missing target argument")
			}

			mode := getStringArg(params.Arguments, "mode")
			result, err := latency.Run(ctx, target, mode)
			if err != nil {
				return createErrorResponse(req.ID, fmt.Sprintf("Latency check failed: %v", err))
			}

			var output string
			if s, ok := result.(string); ok {
				output = s
			} else {
				jsonBytes, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					output = fmt.Sprintf("Error marshaling result: %v", err)
				} else {
					output = string(jsonBytes)
				}
			}

			// Record latency result
			if err := cache.RecordLatency(target, mode, output); err != nil {
				log.Printf("Failed to record latency: %v", err)
			}

			return createSuccessResponse(req.ID, output)
		}

		if params.Name == "traceroute" {
			target := getStringArg(params.Arguments, "target")
			if target == "" {
				return createErrorResponse(req.ID, "Error: Missing target argument")
			}

			output, err := traceroute.Run(ctx, target)
			if err != nil {
				return createErrorResponse(req.ID, fmt.Sprintf("Traceroute execution failed: %v\nOutput:\n%s", err, output))
			}

			// Record traceroute result
			if err := cache.RecordTraceroute(target, output); err != nil {
				log.Printf("Failed to record traceroute: %v", err)
			}

			return createSuccessResponse(req.ID, output)
		}

		if params.Name == "read_records" {
			tool := getStringArg(params.Arguments, "tool")
			if tool == "" {
				return createErrorResponse(req.ID, "Error: Missing tool argument")
			}
			startTime := getStringArg(params.Arguments, "start_time")
			endTime := getStringArg(params.Arguments, "end_time")
			target := getStringArg(params.Arguments, "target")

			records, err := cache.QueryRecords(tool, startTime, endTime, target)
			if err != nil {
				return createErrorResponse(req.ID, fmt.Sprintf("Failed to query records: %v", err))
			}

			jsonBytes, err := json.MarshalIndent(records, "", "  ")
			if err != nil {
				return createErrorResponse(req.ID, fmt.Sprintf("Error marshaling records: %v", err))
			}

			return createSuccessResponse(req.ID, string(jsonBytes))
		}

		if params.Name == "kill_process" {
			var pid int
			
			// Extract PID. JSON numbers are float64 in interface{}
			if p, ok := params.Arguments["pid"]; ok {
				switch v := p.(type) {
				case float64:
					pid = int(v)
				case string:
					fmt.Sscanf(v, "%d", &pid)
				case int: // unlikely from JSON unmarshal into interface{}, but safe to handle
					pid = v
				}
			}

			if pid == 0 {
				return createErrorResponse(req.ID, "Error: Must provide valid 'pid'")
			}

			if err := system.KillProcess(int32(pid)); err != nil {
				return createErrorResponse(req.ID, fmt.Sprintf("Failed to kill process %d: %v", pid, err))
			}
			return createSuccessResponse(req.ID, fmt.Sprintf("Successfully killed process %d", pid))
		}

		return errorResponse(req.ID, -32601, "Tool not found")

	default:
		return errorResponse(req.ID, -32601, "Method not found")
	}
}

// getStringArg safely extracts a string argument from the map
func getStringArg(args map[string]interface{}, key string) string {
	if val, ok := args[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
		// Fallback for numbers sent as strings if needed, but not common for "target" etc.
	}
	return ""
}

func createSuccessResponse(id interface{}, output string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []Content{
				{Type: "text", Text: output},
			},
		},
	}
}

func createErrorResponse(id interface{}, message string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: CallToolResult{
			Content: []Content{
				{Type: "text", Text: message},
			},
			IsError: true,
		},
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
