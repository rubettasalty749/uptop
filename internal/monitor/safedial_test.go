package monitor

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip      string
		private bool
	}{
		{"127.0.0.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"192.168.1.1", true},
		{"169.254.169.254", true},
		{"::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"93.184.216.34", false},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		got := isPrivateIP(ip)
		if got != tt.private {
			t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
		}
	}
}

func TestSafeDialContext_BlocksPrivate(t *testing.T) {
	dial := SafeDialContext(false)
	_, err := dial(t.Context(), "tcp", "127.0.0.1:80")
	if err == nil {
		t.Error("expected error dialing loopback with private blocking enabled")
	}
}

func TestSafeDialContext_AllowsPrivate(t *testing.T) {
	dial := SafeDialContext(true)
	_, err := dial(t.Context(), "tcp", "127.0.0.1:80")
	// Will fail to connect (nothing listening) but should NOT be blocked
	if err != nil && err.Error() == "blocked: 127.0.0.1 resolves to private address 127.0.0.1" {
		t.Error("should not block private IPs when allowPrivate is true")
	}
}
