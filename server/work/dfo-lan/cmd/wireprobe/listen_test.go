package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
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

// ---- 整组（主监听 + 频道端口）重抽：2026-10-05 回归 ----
//
// 现场：主监听抽到的基准端口可用，但 base+1…base+49 里的某个端口落在 Windows 动态保留段
// （WinNAT/Hyper-V）里，旧实现只重试主监听、频道端口一次失败就 return err，进程在写出
// ready.json 之前退出。下面这些用例把"整组"语义钉住。

// fakeListener 只用于断言：Addr 是注入的端口，Close 记录是否被调用（据此判断有没有泄漏 fd）。
type fakeListener struct {
	addr   net.Addr
	mu     sync.Mutex
	closed bool
}

func (f *fakeListener) Accept() (net.Conn, error) { return nil, errors.New("fake listener") }

func (f *fakeListener) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *fakeListener) Addr() net.Addr { return f.addr }

func (f *fakeListener) isClosed() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.closed
}

// 整组语义：第一次抽到的基准端口 + 1 被占住（模拟保留段），网关必须丢掉整组、重新抽基准端口，
// 最终开出一组连续端口，而不是直接退出。被丢掉的那一组必须释放端口（否则就是泄漏 fd）。
func TestOpenGameListenersRedrawsWholeBlockWhenChannelPortRejected(t *testing.T) {
	const host = "127.0.0.2"
	base, occupied := freeBaseWithOccupiedNextPort(t, host)
	defer occupied.Close()

	restore := listenTCP4
	t.Cleanup(func() { listenTCP4 = restore })
	masterCalls := 0
	listenTCP4 = func(address string) (net.Listener, error) {
		if _, port, err := net.SplitHostPort(address); err == nil && port == "0" {
			masterCalls++
			if masterCalls == 1 {
				// 第一次"系统抽签"抽到 base —— 而 base+1 正被别人占着（模拟保留段）。
				return net.Listen("tcp4", net.JoinHostPort(host, strconv.Itoa(base)))
			}
		}
		return net.Listen("tcp4", address)
	}

	lns, err := openGameListeners(net.JoinHostPort(host, "0"), 5)
	if err != nil {
		t.Fatalf("频道端口被占住时应重抽整组而不是退出，实际报错：%v", err)
	}
	if len(lns) != 6 {
		t.Fatalf("应打开 1 个主监听 + 5 个频道端口，实际 %d 个", len(lns))
	}
	if masterCalls < 2 {
		t.Fatalf("应丢弃第一组并重新抽基准端口，实际只抽签 %d 次", masterCalls)
	}
	first := lns[0].Addr().(*net.TCPAddr).Port
	for i, ln := range lns {
		if got := ln.Addr().(*net.TCPAddr).Port; got != first+i {
			t.Fatalf("第 %d 个监听应为连续的 %d，实际 %d", i, first+i, got)
		}
	}
	for _, ln := range lns {
		ln.Close()
	}
	reclaim, err := net.Listen("tcp4", net.JoinHostPort(host, strconv.Itoa(base)))
	if err != nil {
		t.Fatalf("被拒的那一组没有释放端口 %d（泄漏 fd）：%v", base, err)
	}
	reclaim.Close()
}

// 临时端口整组：即使一次成功，频道端口也要是紧邻主监听的连续端口（ready.json 与频道目录依赖这个约定）。
func TestOpenGameListenersEphemeralBlockIsContiguous(t *testing.T) {
	lns, err := openGameListeners("127.0.0.2:0", 4)
	if err != nil {
		t.Fatalf("openGameListeners(127.0.0.2:0, 4) = %v", err)
	}
	defer closeGameListeners(lns)
	if len(lns) != 5 {
		t.Fatalf("应打开 5 个监听，实际 %d 个", len(lns))
	}
	first := lns[0].Addr().(*net.TCPAddr).Port
	if first == 0 {
		t.Fatal("主监听应拿到一个非 0 端口")
	}
	for i, ln := range lns {
		if got := ln.Addr().(*net.TCPAddr).Port; got != first+i {
			t.Fatalf("第 %d 个监听应为连续的 %d，实际 %d", i, first+i, got)
		}
	}
}

// 保留段一直命中：重抽用尽后必须报错，且错误里保留**最后一次真实的 bind 错误原文**
// （WSAEACCES 是这条缺陷唯一的现场特征，不能被吞掉或改写）；每一组已打开的监听都要关干净。
func TestOpenGameListenersKeepsLastBindErrorAndClosesEveryBlock(t *testing.T) {
	const wsaeacces = "bind: An attempt was made to access a socket in a way forbidden by its access permissions."
	restore := listenTCP4
	t.Cleanup(func() { listenTCP4 = restore })
	var opened []*fakeListener
	blockAttempts := 0
	next := 61000
	listenTCP4 = func(address string) (net.Listener, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if port != "0" {
			// 频道端口永远"落在保留段里"。
			blockAttempts++
			return nil, fmt.Errorf("listen tcp4 %s: %s", address, wsaeacces)
		}
		next++
		fl := &fakeListener{addr: &net.TCPAddr{IP: net.ParseIP(host), Port: next}}
		opened = append(opened, fl)
		return fl, nil
	}

	if _, err := openGameListeners("127.0.0.2:0", 2); err == nil {
		t.Fatal("每个频道端口都被拒时应报错")
	} else if !strings.Contains(err.Error(), wsaeacces) {
		t.Fatalf("返回值必须保留最后一次真实 bind 错误，实际：%v", err)
	}
	if blockAttempts != ephemeralListenAttempts {
		t.Fatalf("临时端口应重抽 %d 次，实际 %d 次", ephemeralListenAttempts, blockAttempts)
	}
	for i, fl := range opened {
		if !fl.isClosed() {
			t.Fatalf("第 %d 个临时端口监听没有关闭（泄漏 fd）", i+1)
		}
	}
}

// 重抽必须落在与上一次**不重叠**的端口段上：只靠"下一次 :0 抽签"的话基准端口每次只前进 1，
// 比一组还宽的 Windows 保留段会连续撞满 8 次（2026-10-05 真机实测 62764…62771）。
func TestOpenGameListenersSkipsPastRejectedGroup(t *testing.T) {
	const extra = 3
	restore := listenTCP4
	t.Cleanup(func() { listenTCP4 = restore })
	var opened []*fakeListener
	var draws []int
	channelRejections := 0
	next := 62000
	listenTCP4 = func(address string) (net.Listener, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		if port != "0" {
			// 第一组的频道端口被拒（模拟保留段），之后放行。
			if channelRejections == 0 {
				channelRejections++
				return nil, fmt.Errorf("listen tcp4 %s: bind: An attempt was made to access a socket in a way forbidden by its access permissions.", address)
			}
			n, convErr := strconv.Atoi(port)
			if convErr != nil {
				return nil, convErr
			}
			fl := &fakeListener{addr: &net.TCPAddr{IP: net.ParseIP(host), Port: n}}
			opened = append(opened, fl)
			return fl, nil
		}
		next++
		draws = append(draws, next)
		fl := &fakeListener{addr: &net.TCPAddr{IP: net.ParseIP(host), Port: next}}
		opened = append(opened, fl)
		return fl, nil
	}

	lns, err := openGameListeners("127.0.0.2:0", extra)
	if err != nil {
		t.Fatalf("整组重抽后应成功，实际：%v", err)
	}
	defer closeGameListeners(lns)
	if len(lns) != extra+1 {
		t.Fatalf("应打开 1 个主监听 + %d 个频道端口，实际 %d 个", extra, len(lns))
	}
	firstBase := draws[0]
	secondBase := lns[0].Addr().(*net.TCPAddr).Port
	if secondBase <= firstBase+extra {
		t.Fatalf("重抽应与上一组不重叠：第一组 %d..%d，第二组基准 %d（抽签序列 %v）",
			firstBase, firstBase+extra, secondBase, draws)
	}
	for i, fl := range opened {
		// 失败的那一组（主监听 + 推游标用的临时监听）必须已经关掉；本次返回的一组由测试自己关。
		if !fl.isClosed() && fl.Addr().(*net.TCPAddr).Port < secondBase {
			t.Fatalf("第 %d 个监听（端口 %d）没有关闭（泄漏 fd）", i+1, fl.Addr().(*net.TCPAddr).Port)
		}
	}
}

// 端口固定时（例如 7001）不换端口：重试只用同一个基准端口，绝不重新抽签，错误照原样带出。
func TestOpenGameListenersFixedPortDoesNotRedraw(t *testing.T) {
	const addr = "127.0.0.2:7001"
	restore := listenTCP4
	t.Cleanup(func() { listenTCP4 = restore })
	var masters []*fakeListener
	var attempts []string
	listenTCP4 = func(address string) (net.Listener, error) {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		attempts = append(attempts, address)
		if address == addr {
			fl := &fakeListener{addr: &net.TCPAddr{IP: net.ParseIP(host), Port: 7001}}
			masters = append(masters, fl)
			return fl, nil
		}
		return nil, fmt.Errorf("listen tcp4 %s: bind: Only one usage of each socket address is normally permitted.", address)
	}

	if _, err := openGameListeners(addr, 2); err == nil {
		t.Fatal("固定端口的频道端口被占住时应报错，而不是换端口")
	}
	if len(masters) != ephemeralListenAttempts {
		t.Fatalf("固定端口应重试 %d 次，实际 %d 次", ephemeralListenAttempts, len(masters))
	}
	for _, a := range attempts {
		if a != addr && a != "127.0.0.2:7002" && a != "127.0.0.2:7003" {
			t.Fatalf("固定端口不应被换掉，实际尝试了 %s", a)
		}
	}
	for i, fl := range masters {
		if !fl.isClosed() {
			t.Fatalf("第 %d 次重试的主监听没有关闭（泄漏 fd）", i+1)
		}
	}
}

// 固定端口整组可用时，端口就是 base..base+extra，原样保留。
func TestOpenGameListenersFixedPortBlockKeepsConsecutivePorts(t *testing.T) {
	const host = "127.0.0.2"
	base := freePortRun(t, host, 2)
	lns, err := openGameListeners(net.JoinHostPort(host, strconv.Itoa(base)), 2)
	if err != nil {
		t.Fatalf("openGameListeners(%s:%d, 2) = %v", host, base, err)
	}
	defer closeGameListeners(lns)
	for i, ln := range lns {
		if got := ln.Addr().(*net.TCPAddr).Port; got != base+i {
			t.Fatalf("第 %d 个监听应为 %d，实际 %d", i, base+i, got)
		}
	}
}

// freePortRun 找一个 base..base+extra 全空的端口段（真实 socket 断言用）。
func freePortRun(t *testing.T, host string, extra int) int {
	t.Helper()
	for attempt := 0; attempt < 200; attempt++ {
		probe, err := net.Listen("tcp4", net.JoinHostPort(host, "0"))
		if err != nil {
			t.Fatal(err)
		}
		port := probe.Addr().(*net.TCPAddr).Port
		probe.Close()
		var held []net.Listener
		free := port > 1024
		for i := 0; free && i <= extra; i++ {
			ln, err := net.Listen("tcp4", net.JoinHostPort(host, strconv.Itoa(port+i)))
			if err != nil {
				free = false
				break
			}
			held = append(held, ln)
		}
		for _, ln := range held {
			ln.Close()
		}
		if free {
			return port
		}
	}
	t.Fatalf("找不到 %d 个连续空闲端口", extra+1)
	return 0
}

// freeBaseWithOccupiedNextPort 返回一个 base（空闲）与一个占着 base+1 的真实监听，
// 用来模拟"基准端口可用、但它后面的频道端口落在保留段里"。
func freeBaseWithOccupiedNextPort(t *testing.T, host string) (int, net.Listener) {
	t.Helper()
	for attempt := 0; attempt < 200; attempt++ {
		occupied, err := net.Listen("tcp4", net.JoinHostPort(host, "0"))
		if err != nil {
			t.Fatal(err)
		}
		port := occupied.Addr().(*net.TCPAddr).Port
		if port <= 1025 {
			occupied.Close()
			continue
		}
		base := port - 1
		check, err := net.Listen("tcp4", net.JoinHostPort(host, strconv.Itoa(base)))
		if err != nil {
			occupied.Close()
			continue
		}
		check.Close()
		return base, occupied
	}
	t.Fatal("找不到 base 空闲、base+1 被占住的端口段")
	return 0, nil
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
