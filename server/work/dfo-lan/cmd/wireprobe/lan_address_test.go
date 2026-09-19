package main

import (
	"net"
	"strings"
	"testing"
)

func TestAdvertisedGameAddress(t *testing.T) {
	bound := &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 41234}
	cases := []struct {
		name       string
		explicit   string
		listenHost string
		want       string
		wantError  bool
	}{
		{"explicit host overrides wildcard bind", "192.168.1.20", "0.0.0.0", "192.168.1.20:41234", false},
		{"concrete bind is its own advertisement", "", "10.0.0.7", "10.0.0.7:41234", false},
		{"loopback bind stays loopback", "", "127.0.0.1", "127.0.0.1:41234", false},
		{"wildcard without explicit host is refused as an advertisement", "0.0.0.0", "0.0.0.0", "", true},
		{"non-address host is refused", "example.invalid", "0.0.0.0", "", true},
	}
	for _, c := range cases {
		got, err := advertisedGameAddress(c.explicit, c.listenHost, bound)
		if c.wantError {
			if err == nil {
				t.Errorf("%s: expected an error, got %q", c.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

// A wildcard bind still has to hand the client something dialable, so the
// fallback resolves the machine's outgoing interface instead of the wildcard.
func TestAdvertisedGameAddressWildcardFallback(t *testing.T) {
	bound := &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 7002}
	got, err := advertisedGameAddress("", "0.0.0.0", bound)
	if err != nil {
		t.Skipf("no LAN interface in this environment: %v", err)
	}
	host, port, err := net.SplitHostPort(got)
	if err != nil {
		t.Fatalf("not host:port: %v", err)
	}
	if port != "7002" {
		t.Errorf("port not preserved: %q", got)
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.To4() == nil {
		t.Errorf("fallback is not a dialable IPv4 address: %q", got)
	}
	if strings.HasSuffix(host, ":0") {
		t.Errorf("fallback kept the wildcard: %q", got)
	}
}
