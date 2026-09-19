package main

import (
	"fmt"
	"net"
)

// advertisedGameAddress decides which address the client is told to dial.
//
// Binding a wildcard makes the port reachable from other machines, but a
// wildcard is not itself a dialable destination, so the channel directory
// must carry a real host: an explicit -advertise-host, the bound address when
// it is already a concrete interface, or the machine's outgoing LAN IPv4 when
// the bind was a wildcard.
func advertisedGameAddress(explicit, listenHost string, bound net.Addr) (string, error) {
	_, port, e := net.SplitHostPort(bound.String())
	if e != nil {
		return "", e
	}
	if explicit != "" {
		ip := net.ParseIP(explicit)
		if ip == nil || ip.IsUnspecified() {
			return "", fmt.Errorf("advertise-host must be a dialable IP address")
		}
		return net.JoinHostPort(explicit, port), nil
	}
	if listenHost != "" {
		if ip := net.ParseIP(listenHost); ip != nil && !ip.IsUnspecified() {
			return net.JoinHostPort(listenHost, port), nil
		}
	}
	ip, e := outgoingIPv4()
	if e != nil {
		return "", fmt.Errorf("cannot auto-detect the LAN address for game-listen %q; pass -advertise-host", listenHost)
	}
	return net.JoinHostPort(ip, port), nil
}

// outgoingIPv4 returns the address this machine would use to reach another
// host. The UDP dial sends no packet; it only asks the routing table which
// local interface is selected, which is the interface a LAN peer reaches.
func outgoingIPv4() (string, error) {
	c, e := net.Dial("udp4", "192.0.2.1:9")
	if e != nil {
		return "", e
	}
	defer c.Close()
	a, ok := c.LocalAddr().(*net.UDPAddr)
	if !ok || a.IP == nil || a.IP.IsLoopback() || a.IP.IsUnspecified() {
		return "", fmt.Errorf("no LAN IPv4 interface")
	}
	return a.IP.String(), nil
}
