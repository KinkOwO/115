package main

import (
	"fmt"
	"net"
	"strconv"
)

// openGameListeners reserves the whole contiguous channel port block before
// advertising any channel. A rejected neighbour releases the partial block;
// only an ephemeral base address may choose a different block on retry.
func openGameListeners(addr string, extra int) ([]net.Listener, error) {
	return openGameListenersWith(addr, extra, net.Listen)
}

func openGameListenersWith(addr string, extra int, listen func(string, string) (net.Listener, error)) ([]net.Listener, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	requested, err := strconv.Atoi(port)
	if err != nil || requested < 0 || requested > 65535 || extra < 0 || extra > 65534 {
		return nil, fmt.Errorf("invalid game listener block %q (+%d)", addr, extra)
	}
	ephemeral := requested == 0
	attempts := 1
	if ephemeral {
		attempts = ephemeralListenAttempts
	}
	var last error
	for attempt := 0; attempt < attempts; attempt++ {
		first, err := listen("tcp4", addr)
		if err != nil {
			last = err
			continue
		}
		block := []net.Listener{first}
		_, baseText, err := net.SplitHostPort(first.Addr().String())
		base, convErr := strconv.Atoi(baseText)
		if err != nil || convErr != nil || base < 1 || base+extra > 65535 {
			closeGameListeners(block)
			last = fmt.Errorf("game listener block exceeds valid TCP port range")
			continue
		}
		for i := 1; i <= extra; i++ {
			next, err := listen("tcp4", net.JoinHostPort(host, strconv.Itoa(base+i)))
			if err != nil {
				last = err
				break
			}
			block = append(block, next)
		}
		if len(block) == extra+1 {
			return block, nil
		}
		closeGameListeners(block)
	}
	return nil, fmt.Errorf("reserve game listener block %s (+%d): %w", addr, extra, last)
}

func closeGameListeners(block []net.Listener) {
	for _, listener := range block {
		if listener != nil {
			_ = listener.Close()
		}
	}
}
