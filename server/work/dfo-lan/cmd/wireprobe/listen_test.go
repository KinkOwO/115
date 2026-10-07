package main

import (
	"net"
	"testing"
)

// 端口写 0：应当绑上一个系统挑的临时端口。
func TestListenGamePortEphemeral(t *testing.T) {
	l, err := listenGamePort("127.0.0.2:0")
	if err != nil {
		t.Fatalf("listenGamePort(127.0.0.2:0) = %v", err)
	}
	defer l.Close()
	if l.Addr().(*net.TCPAddr).Port == 0 {
		t.Fatal("应拿到一个非 0 端口")
	}
}

// 固定端口被占用：重试若干次后报错，且错误里带上原地址（玩家能看到是哪个端口）。
func TestListenGamePortFixedPortInUse(t *testing.T) {
	held, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()
	addr := held.Addr().String()
	if _, err := listenGamePort(addr); err == nil {
		t.Fatal("端口被占用时应报错")
	} else if !containsAll(err.Error(), addr, "绑定") {
		t.Fatalf("错误应带上地址与说明，实际：%v", err)
	}
}

// 固定端口可用：正常返回同一个端口（不换端口）。
func TestListenGamePortFixedPortKeepsThePort(t *testing.T) {
	probe, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := probe.Addr().String()
	probe.Close() // 释放后再绑，端口应仍可用
	l, err := listenGamePort(addr)
	if err != nil {
		t.Fatalf("listenGamePort(%s) = %v", addr, err)
	}
	defer l.Close()
	if l.Addr().String() != addr {
		t.Fatalf("固定端口不应被换掉：拿到 %s，期望 %s", l.Addr(), addr)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestChannelDynamicPortsDoNotUseAdjacentNumbers(t *testing.T) {
	// The failed session attempted 50556 for a later channel even though it
	// requested :0. That address is occupied/excluded independently of the
	// first port, so it must never be inferred from the first listener.
	first := &net.TCPAddr{IP: net.ParseIP("127.0.0.2"), Port: 50555}
	for i := 1; i < 120; i++ {
		address, err := channelListenAddress("127.0.0.2:0", first, i)
		if err != nil || address != "127.0.0.2:0" {
			t.Fatalf("channel %d address=%q err=%v", i, address, err)
		}
	}
	fixed, err := channelListenAddress("127.0.0.2:50555", first, 1)
	if err != nil || fixed != "127.0.0.2:50556" {
		t.Fatalf("explicit base changed: %q %v", fixed, err)
	}
}

func TestDynamicChannelPortsAdvertiseActualUniqueListeners(t *testing.T) {
	first, err := listenGamePort("127.0.0.2:0")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	ports := map[uint16]bool{}
	port, err := channelBoundPort(first.Addr())
	if err != nil {
		t.Fatal(err)
	}
	ports[port] = true
	for i := 1; i < 12; i++ {
		address, err := channelListenAddress("127.0.0.2:0", first.Addr(), i)
		if err != nil {
			t.Fatal(err)
		}
		listener, err := listenGamePort(address)
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()
		port, err := channelBoundPort(listener.Addr())
		if err != nil || ports[port] {
			t.Fatalf("duplicate/invalid advertised port %d: %v", port, err)
		}
		ports[port] = true
	}
}
