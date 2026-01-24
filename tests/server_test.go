package tests

import (
	"testing"

	"github.com/ashton2914/mcp-netutil/pkg/mcp"
)

func TestHandleRequest(t *testing.T) {
	server := mcp.NewServer()

	tests := []struct {
		name       string
		req        mcp.JSONRPCRequest
		wantNil    bool
		wantResult bool
		wantError  bool
	}{
		{
			name: "initialize",
			req: mcp.JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "initialize",
				ID:      1,
			},
			wantNil:    false,
			wantResult: true,
		},
		{
			name: "notifications/initialized",
			req: mcp.JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "notifications/initialized",
				ID:      nil, // Notifications usually have no ID or null
			},
			wantNil: true,
		},
		{
			name: "unknown method",
			req: mcp.JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "unknown",
				ID:      2,
			},
			wantNil:   false,
			wantError: true,
		},
		{
			name: "unknown notification",
			req: mcp.JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "unknown_notif",
				ID:      nil,
			},
			wantNil: true,
		},
		{
			name: "tools/list as notification (no id)",
			req: mcp.JSONRPCRequest{
				JSONRPC: "2.0",
				Method:  "tools/list",
				ID:      nil,
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := server.HandleRequest(tt.req)
			if tt.wantNil {
				if got != nil {
					t.Errorf("HandleRequest() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatalf("HandleRequest() = nil, want non-nil")
			}

			if tt.wantResult && got.Result == nil {
				t.Errorf("HandleRequest() result is nil, want Result")
			}
			if tt.wantError && got.Error == nil {
				t.Errorf("HandleRequest() error is nil, want Error")
			}
		})
	}
}
