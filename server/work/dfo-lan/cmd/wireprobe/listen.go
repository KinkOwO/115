package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
)

// listenTCP4 是可注入的绑定入口：生产路径就是 net.Listen("tcp4", address)，
// 测试用它模拟"某个端口落在 Windows 保留段里"（见 listen_test.go）。
var listenTCP4 = func(address string) (net.Listener, error) {
	return net.Listen("tcp4", address)
}

// openGameListeners 把**一组**游戏端口打开：主监听 + base+1 … base+extra。
//
// 现场（2026-10-05 真机，业主的启动连续被挡两次里的第一次）：编排下发
// `-game-listen 127.0.0.2:0`，系统挑了一个临时端口（那次 61857）当基准；主监听绑得上，
// 但频道端口（base+1 … base+49）里的某个端口落在 Windows 的**动态保留段**里
// （WinNAT / Hyper-V；`netsh int ipv4 show excludedportrange protocol=tcp` 是快照，
// 动态保留未必列在里面），bind 直接被拒：
//
//	listen tcp4 127.0.0.2:61857: bind: An attempt was made to access a socket
//	in a way forbidden by its access permissions            （= WSAEACCES）
//
// 旧实现只对**主监听**做换端口重试（listenGamePort），频道监听在重试之外，任意一个频道端口
// 失败就 return err —— 进程在写出 ready.json 之前退出，启动器只能报"会话编排异常退出
// （尚未进入会话）"。间隔 95 秒再点一次就成功，唯一变化是系统换了一批端口（61857 → 61953…62002）。
//
// 所以这里恢复"整组"语义：基准端口每次重新抽，主监听与 base+1 … base+extra 作为一组依次打开；
// 组内任意一个绑定失败就**关掉本次已打开的全部监听**，重新抽基准端口再来一次，直到成功或
// 达到 ephemeralListenAttempts。返回的切片下标 0 是主监听，下标 i 是 base+i。
//
// 重抽前还会把内核的临时端口游标推过刚刚失败的那一组（skipPastGroup），让相邻两次尝试的端口段
// **不重叠**：只靠"下一次 :0 抽签"的话，基准端口每次只前进 1，而 Windows 的保留段常常比一组还宽
// （2026-10-05 真机实测：基准 62764…62771 连续 8 次都撞在保留段 62787-62886 上、重试用尽后仍然退出），
// 推过一次就能整段跨过去。
//
// 端口写固定值（例如 7001）时**不换端口** —— 那是"本来就该由我们占"的端口，换掉会破坏
// 调用方（频道目录 / ready.json）对地址的预期；此时只按同一个基准重试。
func openGameListeners(addr string, extra int) ([]net.Listener, error) {
	if extra < 0 {
		extra = 0
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ephemeral := port == "" || port == "0"
	var last error
	for attempt := 0; attempt < ephemeralListenAttempts; attempt++ {
		base := addr
		if ephemeral {
			base = net.JoinHostPort(host, "0")
		}
		block, baseAddr, endPort, err := openGameListenBlock(base, host, extra)
		if err == nil {
			if attempt > 0 {
				log.Printf("game listen: block base %s bound after %d rejected attempt(s) (last error: %v)", block[0].Addr(), attempt, last)
			}
			return block, nil
		}
		last = err
		shown := base
		if baseAddr != "" {
			// 主监听已经绑上、是它后面的频道端口被拒：报出这一组真正的基准地址。
			shown = baseAddr
		}
		log.Printf("game listen: block base %s rejected: %v (attempt %d/%d)", shown, err, attempt+1, ephemeralListenAttempts)
		if ephemeral && attempt+1 < ephemeralListenAttempts {
			skipPastGroup(host, endPort)
		}
	}
	return nil, fmt.Errorf("绑定 %s 失败（已重试 %d 次，系统挑的临时端口可能一直落在 Windows 保留段里；"+
		"可用 netsh int ipv4 show excludedportrange protocol=tcp 查看）：%w",
		addr, ephemeralListenAttempts, last)
}

// openGameListenBlock 打开一组监听。失败时先把本组已打开的监听全部关掉，再原样返回 bind 错误
// （WSAEACCES 的原文是这条缺陷唯一的现场特征，不改写、不包装）；baseAddr 是主监听实际绑到的
// 地址，主监听就没绑上时为空；endPort 是这一组**本该占到的最后一个端口**（主监听都没绑上时为 0），
// 供 skipPastGroup 判断"下一组要从哪里开始才算不重叠"。
func openGameListenBlock(base, host string, extra int) ([]net.Listener, string, int, error) {
	first, err := listenTCP4(base)
	if err != nil {
		return nil, "", 0, err
	}
	baseAddr := first.Addr().String()
	_, portText, err := net.SplitHostPort(baseAddr)
	if err != nil {
		first.Close()
		return nil, baseAddr, 0, err
	}
	basePort, err := strconv.Atoi(portText)
	if err != nil {
		first.Close()
		return nil, baseAddr, 0, err
	}
	endPort := basePort + extra
	lns := make([]net.Listener, 0, extra+1)
	lns = append(lns, first)
	for i := 1; i <= extra; i++ {
		ln, err := listenTCP4(net.JoinHostPort(host, strconv.Itoa(basePort+i)))
		if err != nil {
			closeGameListeners(lns)
			return nil, baseAddr, endPort, err
		}
		lns = append(lns, ln)
	}
	return lns, baseAddr, 0, nil
}

// skipPastGroup 把内核的临时端口游标推过刚刚失败的那一组：反复 bind :0 再立刻关闭，直到下一枚
// 可用端口已经超过 endPort。每次重抽因此落在与上一次不重叠的端口段上；保留段比一组还宽时，
// 一次推过就能整段跨过去（推不动时直接返回，让外层照旧重抽，不改变原有语义）。
func skipPastGroup(host string, endPort int) {
	if endPort <= 0 {
		return
	}
	for i := 0; i < skipProbeLimit; i++ {
		l, err := listenTCP4(net.JoinHostPort(host, "0"))
		if err != nil {
			return
		}
		_, portText, splitErr := net.SplitHostPort(l.Addr().String())
		l.Close()
		if splitErr != nil {
			return
		}
		port, convErr := strconv.Atoi(portText)
		if convErr != nil {
			return
		}
		if port > endPort {
			return
		}
	}
}

// closeGameListeners 关掉一组监听（整组重抽与启动返回时都用它）。
func closeGameListeners(lns []net.Listener) {
	for _, ln := range lns {
		if ln != nil {
			ln.Close()
		}
	}
}

// listenGamePort 只打开主监听（extra = 0），保留给"不开频道"的调用点与既有用例；
// 启动路径统一走 openGameListeners（整组），两者共用同一套重试实现。
//
// 端口写 0 时每次让系统重新挑（最多 ephemeralListenAttempts 次）；端口是固定值时
// （例如 7001）不换端口 —— 重试几次即可，换端口会破坏调用方对地址的预期。
func listenGamePort(addr string) (net.Listener, error) {
	lns, err := openGameListeners(addr, 0)
	if err != nil {
		return nil, err
	}
	return lns[0], nil
}

// ephemeralListenAttempts 是"整组换端口重抽"的次数上限。
//
// 为什么是 8：保留段通常只覆盖几百个端口，而临时端口范围有 ~1.6 万个，落到保留段的概率不高；
// 连撞 8 次几乎只可能是"整个范围都被保留"这种极端情况 —— 那时应当尽快报错而不是无限重试。
// 有了 skipPastGroup，8 次尝试覆盖的是 8 段互不重叠的端口（≈ 8 × (extra+1) 个端口）。
const ephemeralListenAttempts = 8

// skipProbeLimit 是单次"推过刚失败的那一组"最多 bind 多少个临时端口。
// 正常只需要 extra+1 次（推过一组就停），上限只是防止游标推不动时死循环。
const skipProbeLimit = 4096
