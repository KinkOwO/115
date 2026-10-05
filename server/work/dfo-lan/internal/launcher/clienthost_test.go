package launcher

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Go 宿主（probe.exe 的替代路径）里那些**不需要进程、不需要权限**就能验证的口径：
// 位置参数解析、ui-mode 的显示判定、退出码常量、回退开关、日志格式。
//
// 需要真起客户端的部分（CreateProcessW + Job）在 clienthost_live_test.go 里，且默认不跑。

// `--host-client` 的位置参数与 probe.exe 的 argv 一一对应。
func TestParseHostClientArgv(t *testing.T) {
	options, err := ParseHostClientArgv([]string{`C:\client`, `C:\out\client.log`, "55", "interactive-ui"})
	if err != nil {
		t.Fatalf("ParseHostClientArgv: %v", err)
	}
	if options.ClientDir != `C:\client` || options.LogPath != `C:\out\client.log` {
		t.Errorf("客户端目录/日志 = %q / %q", options.ClientDir, options.LogPath)
	}
	if options.Seconds != 55 || options.UIMode != "interactive-ui" {
		t.Errorf("秒数/ui-mode = %d / %q", options.Seconds, options.UIMode)
	}
	if options.BreakpointsFile != "" || len(options.Args) != 0 {
		t.Errorf("第 5、6 个参数都不该有值：%+v", options)
	}

	// 完整的 probe.exe 形态：breakpoints.txt 占位 + payload。
	full, err := ParseHostClientArgv([]string{
		`C:\client`, `C:\out\client.log`, "55", "trace-root-ui", `C:\out\breakpoints.txt`,
		"3?127.0.0.1?7001?probe?0?0?0?30?0?0?0",
	})
	if err != nil {
		t.Fatalf("ParseHostClientArgv: %v", err)
	}
	if full.BreakpointsFile != `C:\out\breakpoints.txt` {
		t.Errorf("breakpoints 占位 = %q", full.BreakpointsFile)
	}
	if len(full.Args) != 1 || full.Args[0] != "3?127.0.0.1?7001?probe?0?0?0?30?0?0?0" {
		t.Errorf("payload = %q", full.Args)
	}

	// 参数不够：明确的类型错误（main 据此回退出码 2），不是空结构体。
	if _, err := ParseHostClientArgv([]string{`C:\client`, `C:\out\client.log`, "55"}); err == nil {
		t.Error("参数不够时应当报错")
	} else if _, ok := err.(*HostClientArityError); !ok {
		t.Errorf("错误类型 = %T，want *HostClientArityError", err)
	}
}

// 秒数非法时退回 55（probe.exe 的真实调用值），合法时原样保留（夹紧在宿主里做）。
func TestParseHostClientArgvSeconds(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"55", 55},
		{"1", 1},
		{"0", 0}, // 夹到 1 由 startHostedClient 负责
		{"abc", 55},
		{"", 55},
	}
	for _, testCase := range cases {
		options, err := ParseHostClientArgv([]string{"d", "l", testCase.raw, "interactive-ui"})
		if err != nil {
			t.Fatalf("%q: %v", testCase.raw, err)
		}
		if options.Seconds != testCase.want {
			t.Errorf("秒数 %q -> %d，want %d", testCase.raw, options.Seconds, testCase.want)
		}
	}
}

// 退出码常量必须与 probe.cpp 的返回码一致：3 = 看不到 DFO.exe、7 = Job、11 = CreateProcess、
// 12 = Assign。probe.json 与告警文案都建立在这些数字上。
func TestClientHostExitCodesMatchProbe(t *testing.T) {
	cases := map[string]struct{ got, want int }{
		"看不到 DFO.exe":  {exitClientHostMissingExe, 3},
		"Job 建不出来":     {exitClientHostJobError, 7},
		"CreateProcessW": {exitClientHostCreateError, 11},
		"进不了 Job":      {exitClientHostAssignError, 12},
	}
	for name, testCase := range cases {
		if testCase.got != testCase.want {
			t.Errorf("%s 的退出码 = %d，want %d", name, testCase.got, testCase.want)
		}
	}
	if clientHostMinSeconds != 1 || clientHostMaxSeconds != 55 {
		t.Errorf("秒数夹紧范围 = %d..%d，want 1..55（probe.cpp L108）",
			clientHostMinSeconds, clientHostMaxSeconds)
	}
}

// interactive-ui 是唯一"等客户端自己关"的 ui-mode（probe.cpp L137）。
func TestHostUIModeInteractive(t *testing.T) {
	if !hostUIModeIsInteractive("interactive-ui") {
		t.Error("interactive-ui 应当是不设上限的等待")
	}
	for _, mode := range []string{"trace-root-ui", "trace-owned-ui", "normal-ui", "trace-ui", ""} {
		if hostUIModeIsInteractive(mode) {
			t.Errorf("%q 不该被当成 interactive", mode)
		}
	}
}

// 回退开关：DFO_FORCE_PROBE_EXE 非空且非 0 才强制 probe.exe；
// DFO_REQUIRE_GO_ISOLATION 同理（禁止回退）。
func TestWFPFallbackSwitches(t *testing.T) {
	env := newChildEnv(nil)
	if forceProbeFallback(env) || requireGoIsolation(env) {
		t.Error("两个开关都没设时不该生效")
	}
	env.Set(probeFallbackEnvKey, "1")
	if !forceProbeFallback(env) {
		t.Error("DFO_FORCE_PROBE_EXE=1 应当强制回退")
	}
	env.Set(probeFallbackEnvKey, "0")
	if forceProbeFallback(env) {
		t.Error("DFO_FORCE_PROBE_EXE=0 不该强制回退")
	}
	env.Set(requireGoIsolationEnvKey, "1")
	if !requireGoIsolation(env) {
		t.Error("DFO_REQUIRE_GO_ISOLATION=1 应当禁止回退")
	}
	if reason := wfpProbeIsolationReason(env); !strings.Contains(reason, requireGoIsolationEnvKey) {
		t.Errorf("禁止回退的理由 = %q", reason)
	}
}

// client.log 的格式：一行 "<相对毫秒> <文字>"，UTF-8 + CRLF（probe.cpp L26 的 probe_log）。
func TestProbeStyleLogFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client.log")
	log := newProbeStyleLog(path, nil)
	log.line("WFP_READY filters=24 NON_LOOPBACK_BLOCKED IPV4_IPV6")
	log.line("ROOT_PID 1234")
	log.close()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("client.log 没写出来：%v", err)
	}
	lines := strings.Split(strings.TrimRight(string(body), "\r\n"), "\r\n")
	if len(lines) != 2 {
		t.Fatalf("client.log 行数 = %d：%q", len(lines), body)
	}
	for index, line := range lines {
		fields := strings.SplitN(line, " ", 2)
		if len(fields) != 2 {
			t.Fatalf("第 %d 行不是 <tick> <文字>：%q", index, line)
		}
		if !isDecimal(fields[0]) {
			t.Errorf("第 %d 行的时间戳不是十进制：%q", index, fields[0])
		}
	}
	if !strings.HasSuffix(lines[0], "WFP_READY filters=24 NON_LOOPBACK_BLOCKED IPV4_IPV6") {
		t.Errorf("第一行 = %q", lines[0])
	}
	if !strings.Contains(string(body), "\r\n") {
		t.Error("client.log 应当是 CRLF（probe 的文本流）")
	}

	// 再开一次必须截断（std::wofstream::open 默认 trunc），不是追加。
	second := newProbeStyleLog(path, nil)
	second.line("ONLY")
	second.close()
	body, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(body), "\r\n") != 1 || !strings.HasSuffix(string(body), " ONLY\r\n") {
		t.Errorf("第二次应当截断重写：%q", body)
	}
}

// isDecimal 报告字符串是不是纯十进制数字（日志时间戳）。
func isDecimal(text string) bool {
	if text == "" {
		return false
	}
	for _, r := range text {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// 宿主在 client_dir 里看不到 DFO.exe 时，必须在**写 client.log 之前**以返回码 3 收场
// （probe.cpp L106-L107）—— 这条与"没有隔离"是两件事，不能混。
func TestStartHostedClientMissingExeReturnsCode3(t *testing.T) {
	out := t.TempDir()
	hosted, code, err := startHostedClient(ClientHostOptions{
		ClientDir: out,
		LogPath:   filepath.Join(out, "client.log"),
		Seconds:   1,
		UIMode:    "trace-root-ui", // 不用 interactive：万一走到起进程那一步也不会挂住
	}, false)
	if err != nil {
		t.Fatalf("看不到 DFO.exe 不该返回 error：%v", err)
	}
	if code != exitClientHostMissingExe {
		t.Errorf("退出码 = %d，want 3", code)
	}
	if hosted != nil {
		t.Errorf("没有客户端进程时不该返回句柄：%+v", hosted)
	}
	if _, statErr := os.Stat(filepath.Join(out, "client.log")); !os.IsNotExist(statErr) {
		t.Error("返回码 3 时不该写 client.log（probe.cpp 在写日志之前就返回了）")
	}
}

// DFO.exe 是目录（不是普通文件）时同样按返回码 3 处理（regularFile 的口径）。
func TestStartHostedClientRefusesDirectoryExe(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("退出码语义只在 Windows 上有意义")
	}
	out := t.TempDir()
	if err := os.MkdirAll(filepath.Join(out, "DFO.exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, code, err := startHostedClient(ClientHostOptions{
		ClientDir: out,
		LogPath:   filepath.Join(out, "client.log"),
		Seconds:   1,
		UIMode:    "trace-root-ui",
	}, false)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if code != exitClientHostMissingExe {
		t.Errorf("退出码 = %d，want 3", code)
	}
}
