//go:build !windows

package launcher

import (
	"errors"
	"time"
)

// 非 Windows 上 Go 宿主不提供任何能力：隔离要靠 WFP（Windows 独有），客户端本身也只
// 跑在 Windows。所有入口都返回明确错误，调用方据此回退 probe.exe（probe.exe 在别的
// 平台上同样跑不了，所以实际是"两条路都不可用"，必须如实报错，而不是静默放行）。

// errClientHostUnsupported 是"这个平台没有 Go 宿主"的明确错误。
var errClientHostUnsupported = errors.New("非 Windows：Go 版客户端宿主不可用（隔离依赖 WFP）")

// hostStartTime 是这个包被加载的时刻，仅用于非 Windows 的日志时间戳。
var hostStartTime = time.Now()

// hostTickCount 在别的平台上退回进程内单调时钟（只用于日志时间戳）。
func hostTickCount() uint64 { return uint64(time.Since(hostStartTime).Milliseconds()) }

// wfpRootForHost 在别的平台上原样返回（没有隔离器要跟它对齐）。
func wfpRootForHost(path string) string { return path }

// installHostIsolation 在别的平台上永远报"没装上"。
func installHostIsolation(clientDir string, want bool) (wfpInstallation, error) {
	return wfpInstallation{Reason: errClientHostUnsupported.Error()}, errClientHostUnsupported
}

// filterSummaryForApps 在别的平台上没有意义。
func filterSummaryForApps(apps int) string { return "filters=0" }

// runHostNetSelfTest 在别的平台上不做任何事（没有隔离，无从自检）。
func runHostNetSelfTest() hostNetSelfTestResult {
	return hostNetSelfTestResult{Detail: "NETWORK_SELFTEST_SKIPPED non-windows"}
}

// startHostProcess 在别的平台上直接报错（客户端只跑 Windows）。
func startHostProcess(spec hostProcessSpec) (*hostProcess, int, error) {
	return nil, exitClientHostCreateError, errClientHostUnsupported
}
