//go:build !windows

package featurebridge

import "net"

// Windows client extensions do not introduce an unauthenticated network fallback.
func Start(net.Conn, <-chan struct{}) (<-chan Call, func(), error) { return nil, func() {}, nil }
