//go:build windows

package wfpisolate

import (
	"fmt"
	"net"
	"syscall"
	"time"
)

// RunNetSelfTest 复刻 probe.cpp 的 net_check()（L64-L73）：
//
//	1. 在 127.0.0.1 上 bind + listen + connect —— 隔离若把回环也拦了，这里就失败
//	   （对启动链是致命的：客户端连不上本机网关）；
//	2. 非阻塞 connect 到 192.0.2.1:9（TEST-NET-1，永远不可达），等 2 秒，期望
//	   WSAEACCES —— 那正是 probe 认的"被 WFP 拦掉"的错误码。
//
// probe 的返回值语义（L72）：WSAEACCES -> 0；其它错误 -> 该错误码；连接居然成功 -> 24。
// 这里改成结构体，但 Errno 保留同一个错误码，日志文案也照抄。
func RunNetSelfTest() NetSelfTest {
	result := NetSelfTest{}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		result.Detail = fmt.Sprintf("NETWORK_SELFTEST_FAILED loopback_listen err=%v", err)
		return result
	}
	address := listener.Addr().String()
	conn, err := net.DialTimeout("tcp4", address, 2*time.Second)
	if err != nil {
		_ = listener.Close()
		result.Detail = fmt.Sprintf("NETWORK_SELFTEST_FAILED loopback_connect err=%v", err)
		return result
	}
	_ = conn.Close()
	_ = listener.Close()
	result.Loopback = true

	// 第二步：非回环必须被明确拒绝（WSAEACCES）。
	dialer := net.Dialer{Timeout: 2 * time.Second}
	remote, err := dialer.Dial("tcp4", "192.0.2.1:9")
	if err == nil {
		_ = remote.Close()
		result.Errno = 24 // probe.cpp 的"居然连上了"
		result.Detail = "NETWORK_SELFTEST_FAILED remote_connect_succeeded"
		return result
	}
	result.Errno = winsockErrno(err)
	if result.Errno == int(syscall.EACCES) || result.Errno == 5 {
		result.RemoteDenied = true
		result.Detail = "NETWORK_SELFTEST_PASS loopback_ok remote_WSAEACCES"
		return result
	}
	// 超时（10060）不算"被拦"：probe.cpp 也只把 WSAEACCES 当成功。Windows 上这个
	// 地址通常直接 WSAEACCES；若本机网络栈让它超时，如实报告，不做粉饰。
	result.Detail = fmt.Sprintf("NETWORK_SELFTEST_FAILED remote_connect_err=%d", result.Errno)
	return result
}

// winsockErrno 从 net 的错误链里取出 Winsock 错误码（取不到时退回 Go 的 errno）。
func winsockErrno(err error) int {
	var errno syscall.Errno
	for current := err; current != nil; current = unwrap(current) {
		if value, ok := current.(syscall.Errno); ok {
			errno = value
			break
		}
	}
	if errno == 0 {
		return 0
	}
	return int(errno)
}

// unwrap 走一层 error 包装（net.OpError -> os.SyscallError -> Errno）。
func unwrap(err error) error {
	type wrapper interface{ Unwrap() error }
	if wrapped, ok := err.(wrapper); ok {
		return wrapped.Unwrap()
	}
	type multi interface{ Unwrap() []error }
	if wrapped, ok := err.(multi); ok {
		for _, inner := range wrapped.Unwrap() {
			if inner != nil {
				return inner
			}
		}
	}
	return nil
}
