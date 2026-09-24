package main

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// 每连接一个 goroutine，原先全服务端 0 个 recover：任意一次 panic 会带走整个网关
// 进程、所有人一起掉线（实机 2026-09-23 的 CMD72 ACK 越界就是这么炸的）。
func TestRecoverConnectionContainsPanic(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	var events []map[string]any
	event := func(m map[string]any) { events = append(events, m) }

	func() {
		defer recoverConnection("10.0.0.9:5000", 7, server, event)
		panic("index out of range [2] with length 2")
	}()

	if len(events) != 1 {
		t.Fatalf("want exactly one recovery event, got %d: %+v", len(events), events)
	}
	got := events[0]
	if got["kind"] != "connection_panic_recovered" || got["peer"] != "10.0.0.9:5000" || got["channel"] != uint32(7) {
		t.Fatalf("recovery event missing context: %+v", got)
	}
	if !strings.Contains(got["error"].(string), "index out of range") {
		t.Fatalf("panic value lost: %+v", got)
	}
	stack, _ := got["stack"].(string)
	if !strings.Contains(stack, "TestRecoverConnectionContainsPanic") {
		t.Fatalf("stack does not name the panic site: %q", stack)
	}
	// 连接必须被关掉，否则半死的连接会一直挂着。
	client.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, e := client.Read(make([]byte, 1)); e == nil {
		t.Fatal("connection left open after a recovered panic")
	} else if e != io.EOF {
		t.Fatalf("connection closed with %v", e)
	}
}

// 没有 panic 时不能打事件（否则日志会被"恢复"噪声灌满）。
func TestRecoverConnectionSilentWithoutPanic(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()
	calls := 0
	event := func(map[string]any) { calls++ }
	func() {
		defer recoverConnection("10.0.0.9:5000", 7, server, event)
	}()
	if calls != 0 {
		t.Fatalf("recovery event emitted without a panic: %d", calls)
	}
}
