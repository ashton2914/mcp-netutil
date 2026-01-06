package latency

import (
	"testing"
)

func TestParsePingOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		mode     string
		expected LatencyResult
		wantErr  bool
	}{
		{
			name: "Linux Standard Output",
			output: `PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.
64 bytes from 8.8.8.8: icmp_seq=1 ttl=115 time=14.1 ms

--- 8.8.8.8 ping statistics ---
10 packets transmitted, 10 received, 0% packet loss, time 9014ms
rtt min/avg/max/mdev = 14.123/14.567/15.890/0.987 ms`,
			mode: "standard",
			expected: LatencyResult{
				AvgLatency: "14.567 ms",
				Jitter:     "0.987 ms",
				PacketLoss: "0%",
			},
			wantErr: false,
		},
		{
			name: "macOS Output (stddev)",
			output: `PING 8.8.8.8 (8.8.8.8): 56 data bytes
64 bytes from 8.8.8.8: icmp_seq=0 ttl=58 time=14.123 ms

--- 8.8.8.8 ping statistics ---
10 packets transmitted, 10 packets received, 0.0% packet loss
round-trip min/avg/max/stddev = 14.123/14.567/15.890/0.987 ms`,
			mode: "standard",
			expected: LatencyResult{
				AvgLatency: "14.567 ms",
				Jitter:     "0.987 ms",
				PacketLoss: "0.0%",
			},
			wantErr: false,
		},
		{
			name: "Windows Output",
			output: `
Pinging 8.8.8.8 with 32 bytes of data:
Reply from 8.8.8.8: bytes=32 time=14ms TTL=115
Reply from 8.8.8.8: bytes=32 time=15ms TTL=115

Ping statistics for 8.8.8.8:
    Packets: Sent = 4, Received = 4, Lost = 0 (0% loss),
Approximate round trip times in milli-seconds:
    Minimum = 14ms, Maximum = 16ms, Average = 15ms`,
			mode: "standard",
			expected: LatencyResult{
				AvgLatency: "15 ms",
				Jitter:     "N/A",
				PacketLoss: "0%",
			},
			wantErr: false,
		},
		{
			name: "Quick Mode (Linux)",
			output: `PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.
--- 8.8.8.8 ping statistics ---
10 packets transmitted, 10 received, 0% packet loss, time 9014ms
rtt min/avg/max/mdev = 14.123/14.567/15.890/0.987 ms`,
			mode: "quick",
			expected: LatencyResult{
				AvgLatency: "14.567 ms",
			},
			wantErr: false,
		},
		{
			name: "Quick Mode (Windows)",
			output: `
Ping statistics for 8.8.8.8:
    Packets: Sent = 4, Received = 4, Lost = 0 (0% loss),
Approximate round trip times in milli-seconds:
    Minimum = 14ms, Maximum = 16ms, Average = 15ms`,
			mode: "quick",
			expected: LatencyResult{
				AvgLatency: "15 ms",
			},
			wantErr: false,
		},
		{
			name: "Packet Loss (Linux)",
			output: `PING 8.8.8.8 (8.8.8.8) 56(84) bytes of data.
--- 8.8.8.8 ping statistics ---
10 packets transmitted, 5 received, 50% packet loss, time 9014ms
rtt min/avg/max/mdev = 14.123/14.567/15.890/0.987 ms`,
			mode: "standard",
			expected: LatencyResult{
				AvgLatency: "14.567 ms",
				Jitter:     "0.987 ms",
				PacketLoss: "50%",
			},
			wantErr: false,
		},
		{
			name:   "100% Packet Loss",
			output: `10 packets transmitted, 0 received, 100% packet loss, time 9014ms`,
			mode:   "standard",
			expected: LatencyResult{
				PacketLoss: "100%",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePingOutput(tt.output, tt.mode)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePingOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("parsePingOutput() = %v, want %v", got, tt.expected)
			}
		})
	}
}
