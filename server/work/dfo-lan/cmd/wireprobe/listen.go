package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
)

// listenGamePort 绑定游戏监听地址，并在"系统挑的临时端口被拒"时换端口重试。
//
// 现场（2026-10-05 真机，业主的启动被挡）：编排下发 `-game-listen 127.0.0.2:0`，让系统挑一个
// 临时端口（那次是 61222）；而 Windows 的 WinNAT / Hyper-V 会**动态保留**一些 TCP 端口段
// （`netsh int ipv4 show excludedportrange protocol=tcp`，会随服务启停变化），落在保留段里的
// 端口 bind 直接被拒：
//
//	listen tcp4 127.0.0.2:61222: bind: An attempt was made to access a socket in a way
//	forbidden by its access permissions            （= WSAEACCES）
//
// 网关于是没写出 ready.json 就退出，玩家看到的是「启动失败：会话编排异常退出（尚未进入会话）」——
// 现象与存储档、Python、客户端都无关，纯粹是端口抽签抽到了保留段。
//
// 系统挑哪个端口无法预知，唯一稳的办法是**换一个再试**：
//   - 端口写 0 时，每次让系统重新挑（最多 ephemeralListenAttempts 次）；
//   - 端口是固定值时（例如 7001）不换端口 —— 那是"本来就该由我们占"的端口，重试几次即可，
//     换端口会破坏调用方对地址的预期。
func listenGamePort(addr string) (net.Listener, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ephemeral := port == "" || port == "0"
	var last error
	for i := 0; i < ephemeralListenAttempts; i++ {
		try := addr
		if ephemeral {
			try = net.JoinHostPort(host, "0")
		}
		l, e := net.Listen("tcp4", try)
		if e == nil {
			if i > 0 {
				log.Printf("game listen: bound %s after %d rejected attempt(s) (last error: %v)", l.Addr(), i, last)
			}
			return l, nil
		}
		last = e
		log.Printf("game listen: %s rejected: %v (attempt %d/%d)", try, e, i+1, ephemeralListenAttempts)
	}
	return nil, fmt.Errorf("绑定 %s 失败（已重试 %d 次，系统挑的临时端口可能一直落在 Windows 保留段里；"+
		"可用 netsh int ipv4 show excludedportrange protocol=tcp 查看）：%w",
		addr, ephemeralListenAttempts, last)
}

// ephemeralListenAttempts 是"换端口重试"的次数上限。
//
// 为什么是 8：保留段通常只覆盖几百个端口，而临时端口范围有 ~1.6 万个，落到保留段的概率不高；
// 连撞 8 次几乎只可能是"整个范围都被保留"这种极端情况 —— 那时应当尽快报错而不是无限重试。
const ephemeralListenAttempts = 8

// openGameListeners binds the primary endpoint and its consecutive channel
// ports as one group. A failed group releases every socket before retrying.
func openGameListeners(addr string, extraPorts int) ([]net.Listener, error) {
	if extraPorts < 0 || extraPorts > 65534 {
		return nil, fmt.Errorf("invalid extra game ports: %d", extraPorts)
	}
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	port := 0
	if portText != "" {
		port, err = strconv.Atoi(portText)
		if err != nil || port < 0 || port > 65535 {
			return nil, fmt.Errorf("invalid game port %q", portText)
		}
	}
	if port > 0 && port+extraPorts > 65535 {
		return nil, fmt.Errorf("game port group %d..%d exceeds 65535", port, port+extraPorts)
	}
	var last error
	for attempt := 0; attempt < ephemeralListenAttempts; attempt++ {
		primary, bindErr := net.Listen("tcp4", net.JoinHostPort(host, strconv.Itoa(port)))
		if bindErr != nil {
			last = bindErr
			continue
		}
		block := []net.Listener{primary}
		base := primary.Addr().(*net.TCPAddr).Port
		if base+extraPorts > 65535 {
			last = fmt.Errorf("game port group %d..%d exceeds 65535", base, base+extraPorts)
			closeGameListeners(block)
			continue
		}
		for offset := 1; offset <= extraPorts; offset++ {
			listener, channelErr := net.Listen("tcp4", net.JoinHostPort(host, strconv.Itoa(base+offset)))
			if channelErr != nil {
				last = channelErr
				break
			}
			block = append(block, listener)
		}
		if len(block) == extraPorts+1 {
			return block, nil
		}
		closeGameListeners(block)
		log.Printf("game listen group: base=%d rejected: %v (attempt %d/%d)", base, last, attempt+1, ephemeralListenAttempts)
	}
	return nil, fmt.Errorf("绑定游戏监听组 %s（额外频道 %d，已重试 %d 次）失败: %w", addr, extraPorts, ephemeralListenAttempts, last)
}

func closeGameListeners(block []net.Listener) {
	for _, listener := range block {
		if listener != nil {
			_ = listener.Close()
		}
	}
}
