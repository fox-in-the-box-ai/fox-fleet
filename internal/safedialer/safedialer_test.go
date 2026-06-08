package safedialer

import (
	"net"
	"testing"
)

func TestIsPublic(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"127.0.0.1", false},
		{"10.0.0.1", false},
		{"172.16.0.1", false},
		{"192.168.1.1", false},
		{"169.254.1.1", false},
		{"100.64.0.1", false},
		{"100.127.255.255", false},
		{"100.63.255.255", true},
		{"100.128.0.1", true},
		{"0.0.0.0", false},
		{"::1", false},
		{"fd00::1", false},
		{"fe80::1", false},
		{"2001:4860:4860::8888", true},
	}
	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("bad test IP %q", tt.ip)
		}
		if got := isPublic(ip); got != tt.want {
			t.Errorf("isPublic(%s) = %v, want %v", tt.ip, got, tt.want)
		}
	}
}

func TestCheckAddrRejectsPrivate(t *testing.T) {
	if err := checkAddr("tcp4", "127.0.0.1:80", nil); err == nil {
		t.Error("checkAddr should reject loopback")
	}
	if err := checkAddr("tcp4", "10.0.0.1:443", nil); err == nil {
		t.Error("checkAddr should reject RFC1918")
	}
}

func TestCheckAddrAllowsPublic(t *testing.T) {
	if err := checkAddr("tcp4", "8.8.8.8:443", nil); err != nil {
		t.Errorf("checkAddr should allow public: %v", err)
	}
}
