package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	mcp_cache "github.com/ashton2914/mcp-netutil/pkg/cache"
	"github.com/ashton2914/mcp-netutil/pkg/diagnostics"
	"github.com/ashton2914/mcp-netutil/pkg/latency"
	"github.com/ashton2914/mcp-netutil/pkg/mcp"
	"github.com/ashton2914/mcp-netutil/pkg/port"
	"github.com/ashton2914/mcp-netutil/pkg/system"
	"github.com/ashton2914/mcp-netutil/pkg/systemd"
	"github.com/ashton2914/mcp-netutil/pkg/traceroute"
)

func main() {
	// 1. Platform Check
	if runtime.GOOS != "linux" {
		log.Fatal("This program only supports Linux systems.")
	}

	// 2. Parse Flags
	addr := flag.String("a", "", "listen address")
	p := flag.String("p", "", "listen port")
	cacheDir := flag.String("D", "", "Enable cache and define cache directory")
	verbose := flag.Bool("v", false, "Enable verbose logging")
	apiKey := flag.String("o", "", "Set API key for authentication")
	genKey := flag.Bool("generate_key", false, "Generate a standard API key")
	flag.Parse()

	// 2.0 Handle Key Generation
	if *genKey {
		key, err := generateAPIKey()
		if err != nil {
			log.Fatalf("Failed to generate key: %v", err)
		}
		fmt.Println(key)
		os.Exit(0)
	}

	// 3. Privilege Check (Required for actual operation)
	if os.Geteuid() != 0 {
		log.Fatal("This program must be run as root.")
	}

	// 2.0.1 Validate API Key if provided
	if *apiKey != "" {
		if !isValidAPIKey(*apiKey) {
			log.Fatal("Invalid API key format. Must start with 'sk-netutil-' followed by 32 characters.")
		}
	}

	if *verbose {
		enableDebugLog = true
	}

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

	// --- systemd_logs ---
	server.RegisterTool("systemd_logs", "View the journalctl logs for a specific unit (default last 100 lines)", json.RawMessage(`{
			"type": "object",
			"properties": {
				"unit": { "type": "string", "description": "Systemd unit name (e.g. ssh, nginx)" },
				"lines": { "type": "integer", "description": "Number of lines to retrieve (default 100)" }
			},
			"required": ["unit"]
		}`), func(args map[string]interface{}) (mcp.CallToolResult, error) {
		unit, _ := args["unit"].(string)
		lines := 0
		if l, ok := args["lines"].(float64); ok {
			lines = int(l)
		}

		logs, err := systemd.GetJournalLogs(unit, lines)
		if err != nil {
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: err.Error()}}}, nil
		}

		// Join logs for display
		logContent := ""
		for _, line := range logs {
			logContent += line + "\n"
		}

		// Record to cache (maybe truncate if too large?)
		_ = mcp_cache.SaveRecord("systemd_logs", logContent)

		return mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: logContent}}}, nil
	})

	// --- manage_service ---
	server.RegisterTool("manage_service", "Manage systemd services", json.RawMessage(`{
			"type": "object",
			"properties": {
				"unit": { "type": "string", "description": "Systemd unit name" },
				"action": { "type": "string", "description": "Action to perform: start, stop, restart, reload, enable, disable, status" }
			},
			"required": ["unit", "action"]
		}`), func(args map[string]interface{}) (mcp.CallToolResult, error) {
		unit, _ := args["unit"].(string)
		action, _ := args["action"].(string)

		output, err := systemd.ControlService(unit, action)
		if err != nil {
			// output might contain partial output even on error
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: fmt.Sprintf("Error: %v\nOutput: %s", err, output)}}}, nil
		}

		resultMsg := fmt.Sprintf("Successfully executed '%s' on service '%s'\nOutput:\n%s", action, unit, output)

		// Record to cache
		_ = mcp_cache.SaveRecord("manage_service", resultMsg)

		return mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: resultMsg}}}, nil
	})

	// --- systemd_list_units ---
	server.RegisterTool("systemd_list_units", "List all loaded systemd units (services)", json.RawMessage(`{
			"type": "object",
			"properties": {},
			"required": []
		}`), func(args map[string]interface{}) (mcp.CallToolResult, error) {
		res, err := systemd.ListUnits()
		if err != nil {
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: err.Error()}}}, nil
		}

		// Record to cache
		_ = mcp_cache.SaveRecord("systemd_list_units", res)

		return mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: res}}}, nil
	})

	// --- systemd_list_unit_files ---
	server.RegisterTool("systemd_list_unit_files", "List all installed systemd unit files", json.RawMessage(`{
			"type": "object",
			"properties": {},
			"required": []
		}`), func(args map[string]interface{}) (mcp.CallToolResult, error) {
		res, err := systemd.ListUnitFiles()
		if err != nil {
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: err.Error()}}}, nil
		}

		// Record to cache
		_ = mcp_cache.SaveRecord("systemd_list_unit_files", res)

		return mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: res}}}, nil
	})

	// --- system_diagnostics ---
	server.RegisterTool("system_diagnostics", "Get system diagnostics (logs, dmesg, login history)", json.RawMessage(`{
			"type": "object",
			"properties": {},
			"required": []
		}`), func(args map[string]interface{}) (mcp.CallToolResult, error) {
		res, err := diagnostics.RunDiagnostics()
		if err != nil {
			return mcp.CallToolResult{IsError: true, Content: []mcp.ToolContent{{Type: "text", Text: err.Error()}}}, nil
		}

		jsonBytes, _ := json.MarshalIndent(res, "", "  ")
		resultStr := string(jsonBytes)

		// Record to cache
		_ = mcp_cache.SaveRecord("system_diagnostics", resultStr)

		return mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: resultStr}}}, nil
	})

	// 5. Start Server
	if *addr != "" && *p != "" {
		startSSEServer(server, *addr, *p, *apiKey)
	} else {
		startStdioServer(server)
	}
}

func generateAPIKey() (string, error) {
	const prefix = "sk-netutil-"
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	length := 32
	b := make([]byte, length)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		b[i] = charset[num.Int64()]
	}
	return prefix + string(b), nil
}

func isValidAPIKey(key string) bool {
	if !strings.HasPrefix(key, "sk-netutil-") {
		return false
	}
	if len(key) != len("sk-netutil-")+32 {
		return false
	}
	return true
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

		if enableDebugLog {
			log.Printf("[DEBUG] Stdio Request: %+v", req)
		}

		resp := server.HandleRequest(req)
		if resp == nil {
			continue
		}

		if enableDebugLog {
			log.Printf("[DEBUG] Stdio Response: %+v", resp)
		}

		respBytes, _ := json.Marshal(resp)
		fmt.Println(string(respBytes))
	}
}

var enableDebugLog bool

func debugLog(format string, v ...interface{}) {
	if enableDebugLog {
		log.Printf("[DEBUG] "+format, v...)
	}
}

// SessionManager manages SSE client sessions
type SessionManager struct {
	clients map[chan mcp.JSONRPCResponse]bool
	lock    sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		clients: make(map[chan mcp.JSONRPCResponse]bool),
	}
}

func (sm *SessionManager) Add(ch chan mcp.JSONRPCResponse) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	sm.clients[ch] = true
	debugLog("New SSE client connected, total clients: %d", len(sm.clients))
}

func (sm *SessionManager) Remove(ch chan mcp.JSONRPCResponse) {
	sm.lock.Lock()
	defer sm.lock.Unlock()
	if _, ok := sm.clients[ch]; ok {
		delete(sm.clients, ch)
		close(ch)
		debugLog("SSE client disconnected, total clients: %d", len(sm.clients))
	}
}

func (sm *SessionManager) Broadcast(resp mcp.JSONRPCResponse) {
	sm.lock.RLock()
	defer sm.lock.RUnlock()

	debugLog("Broadcasting message to %d clients", len(sm.clients))
	for ch := range sm.clients {
		select {
		case ch <- resp:
		case <-time.After(100 * time.Millisecond):
			debugLog("Warning: Dropped message for slow client")
		}
	}
}

func startSSEServer(server *mcp.Server, addr, port, apiKey string) {
	mux := http.NewServeMux()
	sessionMgr := NewSessionManager()

	ssePath := "/sse"
	if apiKey != "" {
		ssePath = fmt.Sprintf("/sse/%s", apiKey)
	}

	mux.HandleFunc(ssePath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Buffer channel slightly to avoid dropping immediately on bursts
		msgCh := make(chan mcp.JSONRPCResponse, 5)
		sessionMgr.Add(msgCh)
		defer sessionMgr.Remove(msgCh)

		// Send endpoint event
		fmt.Fprintf(w, "event: endpoint\ndata: /message\n\n")
		w.(http.Flusher).Flush()

		// Stream responses
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case resp, ok := <-msgCh:
				if !ok {
					return
				}
				data, _ := json.Marshal(resp)
				if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
					return
				}
				w.(http.Flusher).Flush()
			case <-ticker.C:
				if _, err := fmt.Fprintf(w, ": keepalive\n\n"); err != nil {
					return
				}
				w.(http.Flusher).Flush()
			case <-r.Context().Done():
				return
			}
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

		if enableDebugLog {
			log.Printf("[DEBUG] HTTP POST /message Request: %+v", req)
		}

		// Handle asynchronously
		go func() {
			resp := server.HandleRequest(req)
			if resp != nil {
				if enableDebugLog {
					log.Printf("[DEBUG] HTTP POST /message Response: %+v", resp)
				}
				sessionMgr.Broadcast(*resp)
			}
		}()

		w.WriteHeader(http.StatusAccepted)
	})

	listenAddr := fmt.Sprintf("%s:%s", addr, port)
	log.Printf("Starting SSE server on %s...", listenAddr)
	if err := http.ListenAndServe(listenAddr, mux); err != nil {
		log.Fatal(err)
	}
}
