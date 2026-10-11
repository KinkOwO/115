//go:build windows

package wfpisolate

import (
	"os"
	"testing"
)

// TestLiveWFPIsolation 是唯一真的装过滤器的测试：
//
//	$env:DFO_WFP_LIVE_TEST='1'
//	go test ./internal/wfpisolate/... -run LiveWFP -v        （需要管理员权限）
//
// 它验证的是隔离**真的生效**：装完之后回环仍然连通（否则客户端连不上本机网关），
// 而非回环被明确拒绝（WSAEACCES）。装不上时 Skip 而不是 Fail —— 没有管理员权限是
// 这台机器的状态，不是代码的错；但一旦装上了而自检失败，就是**真的失败**
// （安全底线不允许把"隔离没生效"写成成功）。
func TestLiveWFPIsolation(t *testing.T) {
	if os.Getenv("DFO_WFP_LIVE_TEST") != "1" {
		t.Skip("需要 DFO_WFP_LIVE_TEST=1（真的会装 WFP 过滤器）")
	}
	if testing.Short() {
		t.Skip("testing.Short()：跳过需要管理员权限的集成测试")
	}
	if !isElevated() {
		t.Skip("当前进程不是管理员：WFP 过滤器装不上（probe.exe 在这里同样是优雅降级）")
	}

	result, err := Install(Spec{Root: `C:\Windows\System32`, WalkRoot: false})
	if err != nil {
		t.Fatalf("管理员权限下 Install 不该失败：%v", err)
	}
	if !result.Installed || result.Handle == nil {
		t.Fatalf("Install 没有装上：%+v", result)
	}
	closed := false
	defer func() {
		if !closed {
			_ = result.Handle.Close()
		}
	}()

	if result.Filters == 0 {
		t.Error("装上了却一条过滤器都没有")
	}
	// 关掉引擎之前做自检：这就是"隔离真的生效"的证据。
	selfTest := RunNetSelfTest()
	if !selfTest.Loopback {
		t.Errorf("隔离把回环也拦了：%+v（客户端将连不上本机网关）", selfTest)
	}
	if !selfTest.RemoteDenied {
		t.Errorf("非回环没有被拒：%+v（隔离没有生效，必须回退 probe.exe）", selfTest)
	}
	if err := result.Handle.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	closed = true
	// 幂等：动态会话已经关了，再关一次不该报错。
	if err := result.Handle.Close(); err != nil {
		t.Errorf("Close 不幂等：%v", err)
	}
}
