//go:build !windows

package wfpisolate

// 非 Windows 上没有任何 WFP 可言：Install 一律返回 unavailable（调用方据此回退
// probe.exe，或者在 DFO_REQUIRE_GO_ISOLATION=1 时直接失败）。
//
// 这个文件存在的意义是让 internal/wfpisolate 在别的平台上也能编译、也能跑
// 「规格构造」那部分单元测试；隔离本身在非 Windows 上永远不生效 —— 这一点必须是
// 显式的错误，而不是一个看起来成功的空 Handle。

// Install 在非 Windows 上永远不装隔离。
func Install(spec Spec) (Installation, error) {
	reason := "非 Windows：没有 WFP，无法用 Go 装网络隔离"
	return Installation{Reason: reason}, UnavailableError(reason)
}

// RunNetSelfTest 在非 Windows 上不做任何事：没有隔离，自检无从谈起。
func RunNetSelfTest() NetSelfTest {
	return NetSelfTest{Detail: "non-windows: self test not applicable"}
}
