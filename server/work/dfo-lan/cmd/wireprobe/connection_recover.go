package main

import (
	"fmt"
	"net"
	"runtime/debug"
)

// recoverConnection keeps a panic inside one connection's goroutine from taking
// the whole gateway process down with it.
//
// Before this the server had no recover() at all, so any panic on any
// connection dropped every connected player at once. That is exactly what the
// CMD 72 acknowledgement index did on 2026-09-23:
//
//	panic: runtime error: index out of range [2] with length 2
//	main.main.func2(...)  cmd/wireprobe/main.go:2053
//
// The stack is logged in full on purpose: a panic swallowed into an empty error
// is worse than the crash it replaces.
func recoverConnection(peer string, channel uint32, c net.Conn, event func(map[string]any)) {
	rec := recover()
	if rec == nil {
		return
	}
	event(map[string]any{
		"kind":    "connection_panic_recovered",
		"peer":    peer,
		"channel": channel,
		"error":   fmt.Sprint(rec),
		"stack":   string(debug.Stack()),
	})
	if c != nil {
		_ = c.Close()
	}
}

// monsterDeathEntity reads the monster identity out of a NOTI 38 body: a bare
// little-endian u16. The acknowledgement is built by this server, so it is two
// bytes today, but the read used to be written as bare indexing on an outbound
// payload, which is the same "width guaranteed elsewhere" failure mode that
// already took the gateway down once. A short body is reported instead of
// panicking.
func monsterDeathEntity(payload []byte) (uint16, bool) {
	if len(payload) < 2 {
		return 0, false
	}
	return uint16(payload[0]) | uint16(payload[1])<<8, true
}
