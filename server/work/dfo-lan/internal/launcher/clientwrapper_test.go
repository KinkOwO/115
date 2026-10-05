package launcher

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// 汉化启动宿主通道（见 clientwrapper.go）：这一组用例钉住"包装变量怎么被读、argv 长什么样、
// pid 从哪来、超时有没有界、做不到时怎么报错"。真起进程的那一条在 clienthost_live_test.go
// 里（DFO_CLIENT_HOST_LIVE_TEST=1）。

// 拼接后的**命令行**才是 CreateProcessW 真正用的东西，而 argv 契约要求它是
// `"<host>" "<DFO.exe>" "<dll>" payload`：宿主路径只能出现一次。
//
// 这条用例是踩坑补的：Argv 已经含 argv[0]，若调用方再把它当"目标"交给
// buildClientCommandLine（那里会自己拼一次 argv[0]），宿主收到的 argv 会变成
// `[host host DFO.exe dll payload]` —— 它会把自己又起一遍，于是"客户端"成了第二个宿主，
// 退出码 3、没有任何输出（2026-10-05 实测现场）。
func TestWrapperCommandLineShape(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("命令行转义是 Windows 的行为")
	}
	wrapper := clientWrapper{Host: `C:\mods\localization-host.exe`, DLL: `C:\mods\localization.dll`}
	argv := wrapper.Argv(`C:\client\DFO.exe`, []string{"13?payload"})
	line := buildClientCommandLine(argv[0], argv[1:])
	want := `C:\mods\localization-host.exe C:\client\DFO.exe C:\mods\localization.dll 13?payload`
	if line != want {
		t.Fatalf("命令行 = %q，want %q", line, want)
	}
	if got := strings.Count(line, "localization-host.exe"); got != 1 {
		t.Fatalf("宿主路径出现了 %d 次（应当只有一次 argv[0]）：%q", got, line)
	}

	// 含空格的路径要被转义（syscall.EscapeArg 的口径），顺序不变。
	spaced := wrapper.Argv(`C:\my client\DFO.exe`, []string{"13?payload"})
	line = buildClientCommandLine(spaced[0], spaced[1:])
	if !strings.HasPrefix(line, `"C:\mods\localization-host.exe" `) &&
		!strings.HasPrefix(line, `C:\mods\localization-host.exe `) {
		t.Fatalf("命令行开头不对：%q", line)
	}
	if !strings.Contains(line, `"C:\my client\DFO.exe"`) {
		t.Fatalf("含空格的客户端路径必须被引号包住：%q", line)
	}
}

// 环境变量：两个都给 = 走汉化宿主；都不给 = 未启用汉化（与既有行为逐字一致）；
// 只给一个 = 半套配置，必须明确报错（静默按"没启用"处理会起一个没汉化的客户端）。
func TestClientWrapperFromEnv(t *testing.T) {
	cases := []struct {
		name     string
		env      map[string]string
		wantHost string
		wantDLL  string
		wantErr  bool
	}{
		{name: "都没给（未启用汉化）"},
		{
			name:     "两个都给",
			env:      map[string]string{clientWrapperEnvKey: `C:\mods\localization-host.exe`, clientWrapperDLLEnvKey: `C:\mods\localization.dll`},
			wantHost: `C:\mods\localization-host.exe`,
			wantDLL:  `C:\mods\localization.dll`,
		},
		{
			name:     "值两边的空白会被去掉（环境里带空格是常见手滑）",
			env:      map[string]string{clientWrapperEnvKey: " C:\\mods\\host.exe ", clientWrapperDLLEnvKey: " C:\\mods\\l.dll "},
			wantHost: `C:\mods\host.exe`,
			wantDLL:  `C:\mods\l.dll`,
		},
		{
			name:    "只有宿主",
			env:     map[string]string{clientWrapperEnvKey: `C:\mods\localization-host.exe`},
			wantErr: true,
		},
		{
			name:    "只有 dll",
			env:     map[string]string{clientWrapperDLLEnvKey: `C:\mods\localization.dll`},
			wantErr: true,
		},
		{
			name:    "空串等于没给（两个都是空）",
			env:     map[string]string{clientWrapperEnvKey: "  ", clientWrapperDLLEnvKey: ""},
			wantErr: false,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := newChildEnv(nil)
			for key, value := range testCase.env {
				env.Set(key, value)
			}
			wrapper, err := clientWrapperFromEnv(env)
			if testCase.wantErr {
				if err == nil {
					t.Fatal("半套配置必须报错")
				}
				if !strings.Contains(err.Error(), "MOD 工具") {
					t.Fatalf("错误里应给出去哪儿修，实际：%v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("不该报错：%v", err)
			}
			if wrapper.Host != testCase.wantHost || wrapper.DLL != testCase.wantDLL {
				t.Fatalf("解析结果 = %+v，want %q / %q", wrapper, testCase.wantHost, testCase.wantDLL)
			}
		})
	}
}

// 汉化宿主的 argv 形状：`<host> <DFO.exe 绝对路径> <localization.dll> <payload…>`。
// 这个顺序是启动器 modtool.StartClient 与宿主之间的既有协议，写错就等于宿主找不到客户端或 dll。
func TestClientWrapperArgv(t *testing.T) {
	wrapper := clientWrapper{Host: `C:\mods\localization-host.exe`, DLL: `C:\mods\localization.dll`}
	got := wrapper.Argv(`C:\client\DFO.exe`, []string{"3?127.0.0.1?7001?probe?0?0?0?30?0?0?0"})
	want := []string{
		`C:\mods\localization-host.exe`,
		`C:\client\DFO.exe`,
		`C:\mods\localization.dll`,
		"3?127.0.0.1?7001?probe?0?0?0?30?0?0?0",
	}
	if len(got) != len(want) {
		t.Fatalf("argv = %q，want %q", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("argv[%d] = %q，want %q（整个 argv：%q）", index, got[index], want[index], got)
		}
	}
	// 多个 payload 参数要原样接在后面（probe 的 argv 契约里只有一个，但宿主不该自己吞参数）。
	multi := wrapper.Argv(`C:\client\DFO.exe`, []string{"a", "b"})
	if len(multi) != 5 || multi[3] != "a" || multi[4] != "b" {
		t.Fatalf("payload 原样转发这条不对：%q", multi)
	}
}

// 分流表：有包装变量时**必须**走 Go 宿主（UseProbe 恒为假）；被 DFO_FORCE_PROBE_EXE 要求
// 回退时明确报错 —— probe.exe 是原生程序，没有"把被拉起的程序换成汉化宿主"的能力。
func TestPlanClientLaunch(t *testing.T) {
	cases := []struct {
		name         string
		env          map[string]string
		wantUseProbe bool
		wantWrapper  bool
		wantErr      bool
	}{
		{name: "未启用汉化：首选 Go 宿主"},
		{name: "未启用汉化 + 强制 probe.exe", env: map[string]string{probeFallbackEnvKey: "1"}, wantUseProbe: true},
		{name: "未启用汉化 + 禁止回退", env: map[string]string{requireGoIsolationEnvKey: "1"}},
		{
			name: "启用汉化：必须走 Go 宿主（不带 probe 开关）",
			env: map[string]string{
				clientWrapperEnvKey:    `C:\mods\localization-host.exe`,
				clientWrapperDLLEnvKey: `C:\mods\localization.dll`,
			},
			wantWrapper: true,
		},
		{
			name: "启用汉化 + 强制 probe.exe：明确报错，绝不静默回退",
			env: map[string]string{
				clientWrapperEnvKey:    `C:\mods\localization-host.exe`,
				clientWrapperDLLEnvKey: `C:\mods\localization.dll`,
				probeFallbackEnvKey:    "1",
			},
			wantErr: true,
		},
		{
			name: "启用汉化 + 禁止回退：照旧走 Go 宿主（不冲突）",
			env: map[string]string{
				clientWrapperEnvKey:      `C:\mods\localization-host.exe`,
				clientWrapperDLLEnvKey:   `C:\mods\localization.dll`,
				requireGoIsolationEnvKey: "1",
			},
			wantWrapper: true,
		},
		{
			name:    "半套配置：明确报错",
			env:     map[string]string{clientWrapperEnvKey: `C:\mods\localization-host.exe`},
			wantErr: true,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			env := newChildEnv(nil)
			for key, value := range testCase.env {
				env.Set(key, value)
			}
			route, err := planClientLaunch(env)
			if testCase.wantErr {
				if err == nil {
					t.Fatal("必须报错")
				}
				if !strings.Contains(err.Error(), clientWrapperEnvKey) {
					t.Fatalf("错误里应说清是汉化与 probe 回退的冲突，实际：%v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("不该报错：%v", err)
			}
			if route.UseProbe != testCase.wantUseProbe {
				t.Fatalf("UseProbe = %v，want %v", route.UseProbe, testCase.wantUseProbe)
			}
			if route.Wrapper.Enabled() != testCase.wantWrapper {
				t.Fatalf("Wrapper = %+v，wantEnabled %v", route.Wrapper, testCase.wantWrapper)
			}
			// 包装变量存在时永远不走 probe.exe（这条是本通道的硬约束）。
			if route.Wrapper.Enabled() && route.UseProbe {
				t.Fatal("启用汉化时绝不能走 probe.exe")
			}
		})
	}
}

// 宿主报回的 pid/握手行：只有 `READY <正整数>` 才认（前缀与启动器同一份协议）。
func TestParseWrapperReady(t *testing.T) {
	cases := []struct {
		line    string
		want    int
		wantErr bool
	}{
		{"READY 4321\n", 4321, false},
		{"READY 1", 1, false},
		{"  READY 77  \r\n", 77, false},
		{"READY 0\n", 0, true},
		{"READY -3\n", 0, true},
		{"READY abc\n", 0, true},
		{"READY\n", 0, true},
		{"ready 4321\n", 0, true}, // 前缀大小写敏感（既有协议）
		{"", 0, true},
		{"WFP_READY filters=24\n", 0, true},
	}
	for _, testCase := range cases {
		pid, err := parseWrapperReady(testCase.line)
		if testCase.wantErr {
			if err == nil {
				t.Errorf("%q：应当报错", testCase.line)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q：%v", testCase.line, err)
			continue
		}
		if pid != testCase.want {
			t.Errorf("%q：pid = %d，want %d", testCase.line, pid, testCase.want)
		}
	}
}

// 握手：读到的 pid 就是宿主报的那个（**客户端** pid），宿主的其余 stdout 照既有规则转给 sink。
func TestAwaitWrapperReadyReadsPIDAndForwardsRest(t *testing.T) {
	var sink bytes.Buffer
	pid, done, err := awaitWrapperReady(
		strings.NewReader("READY 4321\nfake-host: dll injected\n"),
		&sink, time.Second)
	if err != nil {
		t.Fatalf("握手失败：%v", err)
	}
	if pid != 4321 {
		t.Fatalf("pid = %d，want 4321", pid)
	}
	// 等读者收尾（它要把剩余 stdout 抄完），否则断言会与它赛跑。
	awaitReader(done, 2*time.Second)
	if got := sink.String(); got != "fake-host: dll injected\n" {
		t.Fatalf("sink = %q，want %q", got, "fake-host: dll injected\n")
	}
}

// 超时有界：宿主不报 READY 时必须在上限内返回错误（不是挂住），上限本身是 30 秒常量。
func TestAwaitWrapperReadyTimeoutIsBounded(t *testing.T) {
	if clientWrapperReadyTimeout != 30*time.Second {
		t.Fatalf("clientWrapperReadyTimeout = %s，want 30s（启动器侧同一个值）", clientWrapperReadyTimeout)
	}
	reader, writer := io.Pipe()
	defer writer.Close()
	start := time.Now()
	_, done, err := awaitWrapperReady(reader, nil, 80*time.Millisecond)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("宿主不报 READY 时必须超时报错")
	}
	if !strings.Contains(err.Error(), "没有报出 READY") {
		t.Fatalf("错误应说清是没等到握手，实际：%v", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("超时判定用了 %s，疑似没有界", elapsed)
	}
	// 让读者 goroutine 收工（调用方在真实路径上会先杀宿主，管道随之断开）。
	writer.Close()
	awaitReader(done, 2*time.Second)
}

// 宿主/dll 的落地校验：空 = 没启用；缺文件 = 明确报错（带路径）。
func TestResolveWrapperTargets(t *testing.T) {
	if wrapper, err := resolveWrapperTargets("", ""); err != nil || wrapper.Enabled() {
		t.Fatalf("两个都没给时应当算「没启用」：%+v / %v", wrapper, err)
	}
	if _, err := resolveWrapperTargets(`C:\mods\host.exe`, ""); err == nil {
		t.Fatal("半套配置必须报错")
	}

	dir := t.TempDir()
	host := filepath.Join(dir, "localization-host.exe")
	dll := filepath.Join(dir, "localization.dll")
	if _, err := resolveWrapperTargets(host, dll); err == nil ||
		!strings.Contains(err.Error(), host) {
		t.Fatalf("文件不存在时必须报错并带上路径，实际：%v", err)
	}
	for _, path := range []string{host, dll} {
		if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wrapper, err := resolveWrapperTargets(host, dll)
	if err != nil || wrapper.Host != host || wrapper.DLL != dll {
		t.Fatalf("文件都在时应当通过：%+v / %v", wrapper, err)
	}
}

// "改用汉化宿主"这一步失败时必须有明确错误：原因 + 宿主路径 + 客户端目录 + 修复入口，
// 且要说清**不**回退 probe.exe。这条文案是玩家侧唯一的线索。
func TestWrapperHostFailureMessage(t *testing.T) {
	wrapper := clientWrapper{Host: `C:\mods\localization-host.exe`, DLL: `C:\mods\localization.dll`}
	err := wrapperHostFailure(wrapper, `C:\client`, io.ErrUnexpectedEOF)
	text := err.Error()
	for _, want := range []string{
		"已启用汉化", "不回落 probe.exe", `C:\mods\localization-host.exe`, `C:\client`,
		"MOD 工具", "unexpected EOF",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("错误里应包含 %q，实际：%s", want, text)
		}
	}
	// 拿不到底层原因时也要有话说（不能只有一句"失败"）。
	bare := wrapperHostFailure(wrapper, `C:\client`, nil)
	if !strings.Contains(bare.Error(), "Go 客户端宿主不可用") {
		t.Fatalf("没有底层原因时应给出兜底说法，实际：%v", bare)
	}
}

// 宿主文件不在盘上时，Go 宿主在**写 client.log 之前**就明确失败：不留下半份现场，
// 也不会走到"起一个没汉化的客户端"。
func TestStartHostedClientRejectsMissingWrapperFile(t *testing.T) {
	out := t.TempDir()
	// DFO.exe 必须先是一个普通文件，否则会先命中 probe.cpp 的返回码 3（那是另一条语义）。
	if err := os.WriteFile(filepath.Join(out, "DFO.exe"), []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(out, "client.log")
	hosted, code, err := startHostedClient(ClientHostOptions{
		ClientDir:  out,
		LogPath:    logPath,
		Seconds:    1,
		UIMode:     "trace-root-ui",
		Wrapper:    filepath.Join(out, "no-such-host.exe"),
		WrapperDLL: filepath.Join(out, "no-such.dll"),
	}, false)
	if err == nil {
		t.Fatal("宿主文件不存在时必须报错")
	}
	if code != exitClientHostWrapperError {
		t.Fatalf("返回码 = %d，want %d", code, exitClientHostWrapperError)
	}
	if hosted != nil {
		t.Fatalf("失败时不该返回客户端句柄：%+v", hosted)
	}
	if !strings.Contains(err.Error(), "localization-host") && !strings.Contains(err.Error(), "no-such-host.exe") {
		t.Fatalf("错误里应带上缺失的宿主路径，实际：%v", err)
	}
	if _, statErr := os.Stat(logPath); !os.IsNotExist(statErr) {
		t.Error("宿主缺失时不该写 client.log（probe.cpp 的返回码 3 也是这个口径：先判再写）")
	}
}

// 宿主文件在、但根本不是可执行文件：CreateProcessW 直接失败（返回码 11）。
// 这条同时覆盖"包装路径的失败不会被悄悄吞掉"与两根管道的清理。
func TestStartHostedClientFailsOnInvalidWrapperExe(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("CreateProcessW 的行为只在 Windows 上有意义")
	}
	out := t.TempDir()
	if err := os.WriteFile(filepath.Join(out, "DFO.exe"), []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
	host := filepath.Join(out, "localization-host.exe")
	dll := filepath.Join(out, "localization.dll")
	for _, path := range []string{host, dll} {
		if err := os.WriteFile(path, []byte("not a pe"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hosted, code, err := startHostedClient(ClientHostOptions{
		ClientDir:  out,
		LogPath:    filepath.Join(out, "client.log"),
		Seconds:    1,
		UIMode:     "trace-root-ui",
		Wrapper:    host,
		WrapperDLL: dll,
	}, false)
	if err == nil {
		t.Fatal("不是有效 PE 时必须报错（而不是回退 probe.exe）")
	}
	if code != exitClientHostCreateError {
		t.Fatalf("返回码 = %d，want %d", code, exitClientHostCreateError)
	}
	if hosted != nil {
		t.Fatalf("失败时不该返回客户端句柄：%+v", hosted)
	}
}
