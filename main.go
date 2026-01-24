package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"

	mcp_cache "github.com/ashton2914/mcp-netutil/pkg/cache"
	"github.com/ashton2914/mcp-netutil/pkg/latency"
	"github.com/ashton2914/mcp-netutil/pkg/mcp"
	"github.com/ashton2914/mcp-netutil/pkg/port"
	"github.com/ashton2914/mcp-netutil/pkg/system"
	"github.com/ashton2914/mcp-netutil/pkg/traceroute"
)

func main() {
	// 1. Platform & Privilege Check
	if runtime.GOOS != "linux" {
		log.Fatal("This program only supports Linux systems.")
	}
	if os.Geteuid() != 0 {
		log.Fatal("This program must be run as root.")
	}

	// 2. Parse Flags
	addr := flag.String("a", "", "listen address")
	p := flag.String("p", "", "listen port")
	cacheDir := flag.String("D", "", "Enable cache and define cache directory")
	flag.Parse()

	// 2.1 Initialize Cache if requested
	if *cacheDir != "" {
		if err := mcp_cache.Init(*cacheDir); err != nil {
			log.Fatalf("Failed to initialize cache at %s: %v", *cacheDir, err)
		}
		log.Printf("Cache initialized at %s", *cacheDir)
	}

	// 3. Initialize Server
	server := mcp.NewServer()

	// 4. Register Tools

	// --- latency (ping) ---
	server.RegisterTool("latency", "Check network latency to a target", json.RawMessage(`{
		"type": "object",
		"properties": {
			"target": { "type": "string", "description": "Target IP or hostname" },
			"mode": { "type": "string", "description": "quick (10 pkts) or standard (100 pkts)" }
		},
		"required": ["target", "mode"]
	}`), func(args map[string]interface{}) (mcp.CallToolResult, error) {
		target, _ := args["target"].(string)
		mode, _ := args["mode"].(string)

		res, err := latency.Run(context.Background(), target, mode)
		if err != nil {
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: err.Error()}}}, nil
		}

		jsonBytes, _ := json.Marshal(res)
		resultStr := string(jsonBytes)

		// Record to cache
		_ = mcp_cache.SaveRecord("latency", resultStr)

		return mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: resultStr}}}, nil
	})

	// --- traceroute ---
	server.RegisterTool("traceroute", "Trace path to a network target", json.RawMessage(`{
		"type": "object",
		"properties": {
			"target": { "type": "string", "description": "Target IP or hostname" }
		},
		"required": ["target"]
	}`), func(args map[string]interface{}) (mcp.CallToolResult, error) {
		target, _ := args["target"].(string)

		res, err := traceroute.Run(context.Background(), target)
		if err != nil {
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: err.Error()}}}, nil
		}
		// Record to cache
		_ = mcp_cache.SaveRecord("traceroute", res)

		return mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: res}}}, nil
	})

	// --- system_stats ---
	server.RegisterTool("system_stats", "Get system statistics", json.RawMessage(`{
		"type": "object",
		"properties": {},
		"required": []
	}`), func(args map[string]interface{}) (mcp.CallToolResult, error) {
		res, err := system.GetStats(context.Background())
		if err != nil {
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: err.Error()}}}, nil
		}
		// Record to cache
		_ = mcp_cache.SaveRecord("system_stats", res)

		return mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: res}}}, nil
	})

	// --- pkill ---
	server.RegisterTool("pkill", "Kill a process by PID", json.RawMessage(`{
		"type": "object",
		"properties": {
			"pid": { "type": "integer", "description": "Process ID to kill" }
		},
		"required": ["pid"]
	}`), func(args map[string]interface{}) (mcp.CallToolResult, error) {
		pidFloat, ok := args["pid"].(float64) // JSON numbers are floats
		if !ok {
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: "invalid pid"}}}, nil
		}
		pid := int32(pidFloat)

		err := system.KillProcess(pid)
		if err != nil {
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: err.Error()}}}, nil
		}
		return mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: fmt.Sprintf("Process %d killed", pid)}}}, nil
	})

	// --- port_status ---
	server.RegisterTool("port_status", "Check status of ports", json.RawMessage(`{
		"type": "object",
		"properties": {
			"port": { "type": "integer", "description": "Specific port to check (optional, 0 for all)" }
		}
	}`), func(args map[string]interface{}) (mcp.CallToolResult, error) {
		portNum := 0
		if p, ok := args["port"].(float64); ok {
			portNum = int(p)
		}

		res, err := port.GetPortStatus(context.Background(), portNum)
		if err != nil {
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: err.Error()}}}, nil
		}

		jsonBytes, _ := json.MarshalIndent(res, "", "  ")
		return mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(jsonBytes)}}}, nil
	})

	// --- read_records ---
	server.RegisterTool("read_records", "Read execution records from the database", json.RawMessage(`{
		"type": "object",
		"properties": {
			"tool_name": { "type": "string", "description": "Tool name to query (latency, traceroute, system_stats)" },
			"start_time": { "type": "string", "description": "Start time (YYYYMMDDhhmmss) for filtering" },
			"end_time": { "type": "string", "description": "End time (YYYYMMDDhhmmss) for filtering" }
		},
		"required": ["start_time"]
	}`), func(args map[string]interface{}) (mcp.CallToolResult, error) {
		toolName, _ := args["tool_name"].(string)
		startTime, _ := args["start_time"].(string)
		endTime, _ := args["end_time"].(string)

		records, err := mcp_cache.QueryRecords(toolName, startTime, endTime)
		if err != nil {
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: err.Error()}}}, nil
		}

		jsonBytes, _ := json.MarshalIndent(records, "", "  ")
		return mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(jsonBytes)}}}, nil
	})

	// 5. Start Server
	if *addr != "" && *p != "" {
		startSSEServer(server, *addr, *p)
	} else {
		startStdioServer(server)
	}
}

func startStdioServer(server *mcp.Server) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		var req mcp.JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			// Log error but continue
			log.Printf("Invalid JSON: %v", err)
			continue
		}

		resp := server.HandleRequest(req)
		respBytes, _ := json.Marshal(resp)
		fmt.Println(string(respBytes))
	}
}

func startSSEServer(server *mcp.Server, addr, port string) {
	// Simple SSE/HTTP implementation for MCP
	// /sse -> Establishing connection
	// /message -> Receiving messages (POST)

	// Since MCP over SSE typically requires tracking sessions and providing an endpoint for POST,
	// implementing a full compliant one in one file without a library is tricky.
	// However, standard says:
	// GET /sse -> return text/event-stream
	// The client receives an endpoint event to know where to POST messages.

	// For simplicity, we'll implement a basic loop.
	// NOTE: This implementation might need refinement for multiple concurrent clients.

	mux := http.NewServeMux()

	// Simple message channel map for sessions could be implemented here,
	// but for this task, let's assume one active session or basic broadcast?
	// Actually better to just support basic POST for RPC if using simple HTTP,
	// but MCP requires SSE for server->client notifications usually.
	// Since we don't have server->client notifications (sampling etc) yet,
	// maybe we can just use HTTP POST for request/response?
	// README mentions: "use the following configuration to connect via SSE: ... serverUrl: https://yourserver/sse"
	// So it expects SSE.

	// Let's implement a very basic SSE endpoint.

	msgCh := make(chan mcp.JSONRPCResponse)

	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Send endpoint event
		fmt.Fprintf(w, "event: endpoint\ndata: /message\n\n")
		w.(http.Flusher).Flush()

		// Stream responses
		for resp := range msgCh {
			data, _ := json.Marshal(resp)
			fmt.Fprintf(w, "data: %s\n\n", data)
			w.(http.Flusher).Flush()
		}
	})

	mux.HandleFunc("/message", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req mcp.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Handle asynchronously
		go func() {
			resp := server.HandleRequest(req)
			msgCh <- resp
		}()

		w.WriteHeader(http.StatusAccepted)
	})

	listenAddr := fmt.Sprintf("%s:%s", addr, port)
	log.Printf("Starting SSE server on %s...", listenAddr)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatal(err)
	}
}
