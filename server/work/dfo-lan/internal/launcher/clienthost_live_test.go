package launcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// 这个文件里的两条测试**默认不跑**：它们会真的拉起进程。门禁是环境变量
// DFO_CLIENT_HOST_LIVE_TEST=1，理由与 Stage 3 的其它实测一致 —— 本机不一定有
// 管理员权限、不一定没有会话在跑，也不该让日常 `go test` 起一堆进程。
//
//	① TestLiveGoClientHostRunsAndReaps：不需要网关，只验证 Go 宿主的进程语义
//	   （CreateProcessW + Job kill-on-close + 退出码 + client.log）。
//	② TestLiveLaunchClientBothPaths：对着真实仓库跑完整的 LaunchClient，
//	   分别走 Go 宿主与 probe.exe 回退，比对 run.json / probe.json 与清理。

func liveClientHostEnabled(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("客户端宿主只跑 Windows")
	}
	if os.Getenv("DFO_CLIENT_HOST_LIVE_TEST") != "1" {
		t.Skip("需要 DFO_CLIENT_HOST_LIVE_TEST=1（会真的拉起子进程）")
	}
}

// hostTestClientExe 把系统里的 cmd.exe 复制成 DFO.exe 当"客户端"：
// 它能接受任意参数、立刻退出、退出码可控，而且不是游戏本体。
func hostTestClientExe(t *testing.T, clientDir string) string {
	t.Helper()
	system32 := filepath.Join(os.Getenv("SystemRoot"), "System32")
	source := filepath.Join(system32, "cmd.exe")
	if !regularFile(source) {
		t.Skipf("找不到 %s", source)
	}
	target := filepath.Join(clientDir, "DFO.exe")
	body, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("读 %s: %v", source, err)
	}
	if err := os.WriteFile(target, body, 0o755); err != nil {
		t.Fatalf("写 %s: %v", target, err)
	}
	return target
}

// ① Go 宿主的进程语义：命令行、工作目录、Job、退出码、client.log、pid。
func TestLiveGoClientHostRunsAndReaps(t *testing.T) {
	liveClientHostEnabled(t)

	out := t.TempDir()
	clientDir := filepath.Join(out, "client")
	if err := os.MkdirAll(clientDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hostTestClientExe(t, clientDir)
	logPath := filepath.Join(out, "client.log")

	hosted, code, err := startHostedClient(ClientHostOptions{
		ClientDir: clientDir,
		LogPath:   logPath,
		Seconds:   55,
		// interactive-ui：走"等客户端自己关"那一条（真实启动链用的就是它）。
		UIMode: "interactive-ui",
		Args:   []string{"/c", "exit", "0"},
	}, false)
	if err != nil {
		t.Fatalf("startHostedClient: %v", err)
	}
	if code != 0 {
		t.Errorf("宿主退出码 = %d，want 0（probe 只在起不来时才非 0）", code)
	}
	if hosted == nil {
		t.Fatal("没有返回宿主句柄")
	}
	if hosted.Pid <= 0 {
		t.Errorf("客户端 pid = %d", hosted.Pid)
	}
	if hosted.ExitCode != 0 {
		t.Errorf("客户端退出码 = %d，want 0", hosted.ExitCode)
	}
	if !hosted.JobClosed {
		t.Error("Job 应当已经关掉（JOB_CLOSED）")
	}

	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("client.log: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"WFP_NOT_AVAILABLE_RUNNING_WITHOUT_ISOLATION", // isolate=false：如实写明没有隔离
		"ROOT_PID ",
		"NORMAL_RUN no_debugger no_breakpoints",
		"INTERACTIVE_RUN until_client_closes",
		"SUMMARY exit=0x0 normal_run=1",
		"JOB_CLOSED",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("client.log 缺少 %q：\n%s", want, text)
		}
	}
	// 失败退出码必须原样传给 probe.json 的口径（宿主自己仍然回 0）。
	secondLog := filepath.Join(out, "client2.log")
	second, code, err := startHostedClient(ClientHostOptions{
		ClientDir: clientDir,
		LogPath:   secondLog,
		Seconds:   55,
		UIMode:    "trace-root-ui",
		Args:      []string{"/c", "exit", "7"},
	}, false)
	if err != nil {
		t.Fatalf("startHostedClient: %v", err)
	}
	if code != 0 {
		t.Errorf("宿主退出码 = %d，want 0", code)
	}
	if second == nil || second.ExitCode != 7 {
		t.Errorf("客户端退出码 = %+v，want 7", second)
	}
	body, err = os.ReadFile(secondLog)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "SUMMARY exit=0x7 normal_run=1") {
		t.Errorf("client.log 的 SUMMARY 不对：\n%s", body)
	}
}

// liveRepoRoot 找到仓库根（含 server/work/dfo-lan 的那一层）；找不到就 skip。
func liveRepoRoot(t *testing.T) string {
	t.Helper()
	current, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for dir := current; ; {
		if regularFile(filepath.Join(dir, "server", "work", "dfo-lan", "bin", "wireprobe-pvf.exe")) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("找不到带 bin/wireprobe-pvf.exe 的仓库根")
		}
		dir = parent
	}
}

// runLiveSession 跑一次完整的 LaunchClient，返回会话目录与 stdout。
func runLiveSession(t *testing.T, root, tag string, env map[string]string) (string, string, error) {
	t.Helper()
	clientDir := filepath.Join(t.TempDir(), "client")
	if err := os.MkdirAll(clientDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hostTestClientExe(t, clientDir)

	home := t.TempDir() // 隔离 trace：不让它读到/写到真实的 LocalLow\DNF
	for key, value := range env {
		t.Setenv(key, value)
	}
	t.Setenv("DFO_CLIENT_DIR", clientDir)
	t.Setenv("USERPROFILE", home)

	var console bytes.Buffer
	err := LaunchClient(t.Context(), root, LaunchOptions{Tag: tag}, &console)
	return filepath.Join(root, "server", "work", "dfo-lan", "runtime", tag), console.String(), err
}

// TestLiveLaunchClientBothPaths 是对着**真实仓库**跑的端到端验收，分别走 Go 宿主与
// probe.exe 回退。
//
// ⚠️ 它需要一个 SQLite 档的 `runtime/storage/local.json`：真实仓库当前是 PostgreSQL 档，
// 而 PostgreSQL 常常已经有一个实例占用 25438，pg_ctl 起不来。所以运行前把 storage 配置
// 换成临时 SQLite 文件，跑完还原（见 swapStorageConfig）。这一步**只在 DFO_CLIENT_HOST_LIVE_TEST=1
// 时发生**，日常测试不碰仓库配置。
//
//	D=1
//	$env:DFO_CLIENT_HOST_LIVE_TEST='1'
//	go test ./internal/launcher/... -run LiveLaunchClientBothPaths -v -timeout 900s
func TestLiveLaunchClientBothPaths(t *testing.T) {
	liveClientHostEnabled(t)
	if PortListening(GatewayPort, 300*1000*1000) {
		t.Skipf("7001 已被占用（有别的会话在跑），跳过：%d", GatewayPort)
	}
	root := liveRepoRoot(t)
	storagePath := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage", "local.json")
	originalStorage, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("读 storage 配置：%v", err)
	}
	state := newLiveTempState(t)
	defer state.restore(t, root, originalStorage)

	type outcome struct {
		name        string
		tag         string
		env         map[string]string
		wantGoIsol  bool
		wantProbeEx bool
	}
	cases := []outcome{
		{
			// tag 必须满足 channel_probe.py 的档位判据（_next37 会一路降级到 _next34，
			// 而降级后的 _next26.._next34 分支要求 tag 里带 _dungeon_）—— 与真实会话同形：
			// roles_persist_select_actor_town_world_live_detail_dungeon_manual_<stamp>_next37
			name:       "Go 宿主（首选）",
			tag:        launcherTagPrefix + "20261005_120000_000001_clienthostgo_next37",
			wantGoIsol: true,
		},
		{
			name:        "probe.exe 回退（DFO_FORCE_PROBE_EXE=1）",
			tag:         launcherTagPrefix + "20261005_120000_000002_clienthostprobe_next37",
			env:         map[string]string{probeFallbackEnvKey: "1"},
			wantProbeEx: true,
		},
	}

	for _, testCase := range cases {
		state.next(t, root)
		out, console, err := runLiveSession(t, root, testCase.tag, testCase.env)
		if err != nil {
			// 排查现场：会话目录里 gateway.err / gateway.out / helper.out 是唯一线索，
			// 所以出错时保留目录并把路径打出来（成功时删掉，不给仓库留垃圾）。
			t.Errorf("%s: LaunchClient: %v\n控制台：\n%s\n会话目录保留在：%s", testCase.name, err, console, out)
			continue
		}
		defer os.RemoveAll(out)

		// run.json：三个键、json.dumps 的空格风格、单行无换行。
		runBody, err := os.ReadFile(filepath.Join(out, "run.json"))
		if err != nil {
			t.Fatalf("%s: run.json: %v", testCase.name, err)
		}
		if strings.Contains(string(runBody), "\n") {
			t.Errorf("%s: run.json 不该有换行：%q", testCase.name, runBody)
		}
		var run map[string]int
		if err := json.Unmarshal(runBody, &run); err != nil {
			t.Fatalf("%s: run.json 不是 JSON：%v", testCase.name, err)
		}
		for _, key := range []string{"server_pid", "probe_pid", "port"} {
			if _, ok := run[key]; !ok {
				t.Errorf("%s: run.json 缺 %q：%s", testCase.name, key, runBody)
			}
		}
		// probe.json：四个字段，键序与 Python 版一致。
		probeBody, err := os.ReadFile(filepath.Join(out, "probe.json"))
		if err != nil {
			t.Fatalf("%s: probe.json: %v", testCase.name, err)
		}
		var probe map[string]any
		if err := json.Unmarshal(probeBody, &probe); err != nil {
			t.Fatalf("%s: probe.json 不是 JSON：%v", testCase.name, err)
		}
		for _, key := range []string{"probe_pid", "probe_returncode", "client_dir", "payload"} {
			if _, ok := probe[key]; !ok {
				t.Errorf("%s: probe.json 缺 %q：%s", testCase.name, key, probeBody)
			}
		}
		if probe["probe_returncode"].(float64) != 0 {
			t.Errorf("%s: probe_returncode = %v，want 0（客户端正常退出）", testCase.name, probe["probe_returncode"])
		}
		if _, err := os.Stat(filepath.Join(out, "client.log")); err != nil {
			t.Errorf("%s: client.log: %v", testCase.name, err)
		}

		// 走的哪条路：Go 路径必须留下"隔离没装上"的如实记录（本机非管理员），
		// 回退路径必须留下 DFO_FORCE_PROBE_EXE 的理由。
		helper, err := os.ReadFile(filepath.Join(out, "helper.out"))
		if err != nil {
			t.Fatalf("%s: helper.out: %v", testCase.name, err)
		}
		helperText := string(helper)
		if testCase.wantProbeEx {
			if !strings.Contains(helperText, "回退 probe.exe") {
				t.Errorf("%s: helper.out 没有写明回退：\n%s", testCase.name, helperText)
			}
			clientLog, err := os.ReadFile(filepath.Join(out, "client.log"))
			if err != nil {
				t.Fatal(err)
			}
			// probe.exe 的日志里一定有它自己的会话/自检记录。
			if !strings.Contains(string(clientLog), "WFP_") && !strings.Contains(string(clientLog), "SYNTHETIC_TEST_ARGUMENTS") {
				t.Errorf("%s: client.log 不像 probe.exe 写的：\n%s", testCase.name, clientLog)
			}
		} else {
			if !strings.Contains(helperText, "WFP 隔离：Go 版") &&
				!strings.Contains(helperText, "WFP 隔离：未安装") &&
				!strings.Contains(helperText, "回退 probe.exe") {
				t.Errorf("%s: helper.out 没有写明隔离结论：\n%s", testCase.name, helperText)
			}
		}
		if !strings.Contains(console, "Client launch requested.") || !strings.Contains(console, "Logs: "+out) {
			t.Errorf("%s: 控制台缺少收尾两行：\n%s", testCase.name, console)
		}
	}

	// 清理确认：没有残留的 probe/DFO 进程，也没有占用 7001 的网关。
	assertNoLiveProcesses(t)
	if PortListening(GatewayPort, 300*1000*1000) {
		t.Errorf("7001 仍然被占用：网关没有收干净")
	}
}

// liveTempState 是一次实测用的临时存储档：每次会话用**新的** SQLite 文件。
//
// 为什么要每次换新：网关启动时会写 SQLite 管理租约 `<db>.admin-guard`（GM 互斥），
// 而上一段实测里被收掉的网关可能把租约留在盘上；60 秒 TTL 没过之前，下一次启动会被
// 拒（"已有 GM 写入正在进行…"）。用新文件 + 收尾删租约，两边都不会互相干扰。
type liveTempState struct {
	dir     string
	counter int
}

func newLiveTempState(t *testing.T) *liveTempState {
	return &liveTempState{dir: t.TempDir()}
}

// next 给第 n 次会话一份新的 SQLite 路径（同时清掉可能残留的租约）。
func (s *liveTempState) next(t *testing.T, root string) {
	t.Helper()
	s.counter++
	db := filepath.Join(s.dir, fmt.Sprintf("dfolan-live-%02d.sqlite3", s.counter))
	path := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage", "local.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 storage 配置：%v", err)
	}
	if err := os.WriteFile(path, []byte(`{"driver":"sqlite","sqlite_path":`+pythonJSONString(db)+`}`), 0o644); err != nil {
		t.Fatalf("临时换 storage 配置：%v", err)
	}
	_ = os.Remove(db + ".admin-guard")
	_ = original
}

// restore 还原仓库的 storage 配置。
func (s *liveTempState) restore(t *testing.T, root string, original []byte) {
	t.Helper()
	path := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage", "local.json")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Errorf("还原 storage 配置失败，请手工恢复 %s：%v", path, err)
	}
}

// assertNoLiveProcesses 检查这次实测可能留下的进程名。
func assertNoLiveProcesses(t *testing.T) {
	t.Helper()
	for _, image := range []string{"probe.exe", "DFO.exe", "wireprobe-pvf.exe"} {
		output, err := exec.Command("tasklist", "/FI", "IMAGENAME eq "+image, "/NH").Output()
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(output)), strings.ToLower(image)) {
			t.Errorf("仍然有 %s 在跑：\n%s", image, output)
		}
	}
}

// ③ 汉化启动宿主路径（真起进程，同样需 DFO_CLIENT_HOST_LIVE_TEST=1）：证明"启用汉化时客户端
// 由宿主拉起，宿主报回的 pid 就是客户端 pid"。
//
// 用的是**假宿主**（不是真汉化 dll）：它按真实宿主的 argv 语义收到
// `<DFO.exe> <localization.dll> <payload…>`，自己起客户端、把 `READY <pid>` 写到 stdout，
// 然后**立刻以 42 退出** —— 42 与客户端的退出码（`cmd.exe /c ping`，0）分开，于是
// "我们等的/读的是客户端还是宿主"在这条用例里是被直接证明的（不是推断）：
//
//   - hosted.Pid 必须等于假宿主写进 pid 文件的**子进程** pid；
//   - hosted.ExitCode 必须是 0（客户端的），不是 42（宿主的）；
//   - 耗时必须覆盖客户端的存活时间（宿主早就退出了，等它就会立刻返回）。
//
// 假宿主就地编（用本机 Go）；也可以先用 DFO_LIVE_WRAPPER_HOST 指定一个现成的：
//
//	$env:DFO_CLIENT_HOST_LIVE_TEST='1'
//	go test ./internal/launcher/ -run LiveWrappedClientHostReportsClientPID -v -timeout 300s
func TestLiveWrappedClientHostReportsClientPID(t *testing.T) {
	liveClientHostEnabled(t)

	out := t.TempDir()
	clientDir := filepath.Join(out, "client")
	if err := os.MkdirAll(clientDir, 0o755); err != nil {
		t.Fatal(err)
	}
	hostTestClientExe(t, clientDir) // DFO.exe = 系统 cmd.exe 的副本

	fakeHost := os.Getenv("DFO_LIVE_WRAPPER_HOST")
	if fakeHost == "" {
		fakeHost = buildFakeWrapperHost(t, out)
	} else if !regularFile(fakeHost) {
		t.Skipf("DFO_LIVE_WRAPPER_HOST 不是文件：%s", fakeHost)
	}
	dll := filepath.Join(out, "localization.dll")
	if err := os.WriteFile(dll, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 走生产那条路：环境变量 → planClientLaunch → ClientHostOptions。
	t.Setenv(clientWrapperEnvKey, fakeHost)
	t.Setenv(clientWrapperDLLEnvKey, dll)
	route, err := planClientLaunch(newChildEnv(CurrentEnv()))
	if err != nil {
		t.Fatalf("planClientLaunch: %v", err)
	}
	if route.UseProbe || route.Wrapper.Host != fakeHost || route.Wrapper.DLL != dll {
		t.Fatalf("环境变量没有被认成汉化包装：%+v", route)
	}

	// 第一档：宿主报完 READY 立刻以 42 退出；客户端是 cmd.exe 副本跑 ping（约 3 秒）。
	pidFile := filepath.Join(out, "fake-host-child.pid")
	logPath := filepath.Join(out, "client.log")
	console, stderrLog := &lockedBuffer{}, &lockedBuffer{}
	started := time.Now()
	hosted, code, err := startHostedClient(ClientHostOptions{
		ClientDir:  clientDir,
		LogPath:    logPath,
		Seconds:    55,
		UIMode:     "trace-root-ui", // 有界：万一客户端没起来也不会永远等
		Args:       []string{"/c", "ping", "-n", "4", "127.0.0.1"},
		Console:    console,
		ErrorLog:   stderrLog,
		Wrapper:    route.Wrapper.Host,
		WrapperDLL: route.Wrapper.DLL,
		// 隔离关掉：这条用例验的是宿主包装与 pid 传递，装 WFP 需要管理员权限。
		Env: append(os.Environ(), "DFO_FAKE_HOST_PIDFILE="+pidFile),
	}, false)
	elapsed := time.Since(started)
	if err != nil {
		t.Fatalf("经汉化宿主拉起客户端失败：%v（宿主 stderr：%s）", err, stderrLog.String())
	}
	if code != 0 {
		t.Errorf("宿主返回码 = %d，want 0（probe 只在起不来时才非 0）", code)
	}
	if hosted == nil {
		t.Fatal("没有返回客户端句柄")
	}
	if !hosted.Wrapped {
		t.Error("这次应当被标成经汉化宿主拉起（Wrapped）")
	}
	if hosted.WrapperPID <= 0 || hosted.WrapperPID == hosted.Pid {
		t.Errorf("宿主 pid = %d、客户端 pid = %d：两者都要有且不同", hosted.WrapperPID, hosted.Pid)
	}

	reported, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		t.Fatalf("假宿主没有写出客户端 pid（%s）：%v", pidFile, readErr)
	}
	if want := strings.TrimSpace(string(reported)); want != fmt.Sprint(hosted.Pid) {
		t.Errorf("记录的客户端 pid = %d，假宿主报的是 %s（应取宿主报回的那个）", hosted.Pid, want)
	}
	// 退出码：客户端是 0，假宿主自己是 42 —— 读到 42 就说明等错了进程。
	if hosted.ExitCode != 0 {
		t.Errorf("客户端退出码 = %d，want 0（42 说明读的是汉化宿主的退出码）", hosted.ExitCode)
	}
	// 宿主报完 READY 就退出了，所以能等到"客户端跑完"才说明等的是客户端。
	if elapsed < 1500*time.Millisecond {
		t.Errorf("整次调用只用了 %s：宿主早已退出，说明没有等客户端", elapsed)
	}
	if !hosted.JobClosed {
		t.Error("Job 应当已经关掉（JOB_CLOSED）")
	}

	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("client.log: %v", err)
	}
	text := string(body)
	// 既有行一个不少（probe.cpp 的契约），多出来的只有 WRAPPER_*。
	for _, want := range []string{
		"WFP_NOT_AVAILABLE_RUNNING_WITHOUT_ISOLATION", // isolate=false：如实写明没有隔离
		"ROOT_PID ",
		"NORMAL_RUN no_debugger no_breakpoints",
		"SUMMARY exit=0x0 normal_run=1",
		"JOB_CLOSED",
		"WRAPPER_HOST " + fakeHost,
		fmt.Sprintf("WRAPPER_READY client_pid=%d", hosted.Pid),
	} {
		if !strings.Contains(text, want) {
			t.Errorf("client.log 缺少 %q：\n%s", want, text)
		}
	}
	if strings.Contains(text, "WRAPPER_ATTACH_ERROR") {
		t.Errorf("应当能拿到客户端句柄（否则等的是宿主，pid 传递就没被证明）：\n%s", text)
	}
	t.Logf("汉化宿主 pid=%d，客户端 pid=%d（假宿主写出的 pid 文件也是 %s），客户端退出码=%d，整次调用 %s",
		hosted.WrapperPID, hosted.Pid, strings.TrimSpace(string(reported)), hosted.ExitCode, elapsed.Round(time.Millisecond))
	t.Logf("client.log:\n%s", text)
	// 宿主的其余 stdout 按既有规则转给了 helper.out（Console），stderr 去了 helper.err。
	if !strings.Contains(console.String(), "fake-host: dll injected") {
		t.Errorf("宿主 stdout 的其余内容没有转发到 console：%q", console.String())
	}
	if !strings.Contains(console.String(), fmt.Sprintf("WRAPPER_READY client_pid=%d", hosted.Pid)) {
		t.Errorf("client.log 的行没有回显到 console：%q", console.String())
	}

	// 第二档：客户端换成一个**已知的非零退出码**（`/c exit 7`），宿主仍然立刻以 42 退出。
	// 上报 7 而不是 0（默认值）也不是 42（宿主）——这是"退出码从客户端来"的第二份证据。
	second, secondCode, err := startHostedClient(ClientHostOptions{
		ClientDir:  clientDir,
		LogPath:    filepath.Join(out, "client2.log"),
		Seconds:    55,
		UIMode:     "trace-root-ui",
		Args:       []string{"/c", "exit", "7"},
		Wrapper:    route.Wrapper.Host,
		WrapperDLL: route.Wrapper.DLL,
		Env:        append(os.Environ(), "DFO_FAKE_HOST_PIDFILE="+filepath.Join(out, "fake-host-child2.pid")),
	}, false)
	if err != nil {
		t.Fatalf("第二档失败：%v", err)
	}
	if secondCode != 0 || second == nil {
		t.Fatalf("第二档宿主返回码 = %d，hosted = %+v", secondCode, second)
	}
	if second.ExitCode != 7 {
		t.Errorf("客户端退出码 = %d，want 7（0 是默认值、42 是宿主的）", second.ExitCode)
	}
}

// lockedBuffer 是并发安全的 bytes.Buffer。
//
// 为什么需要：汉化宿主的 stdout 由**转发 goroutine** 抄进 Console，而 client.log 的行由主
// goroutine 写同一个 Console（生产上那是一个 helper.out 文件句柄，测试里换成 buffer 就必须
// 自己加锁，否则会丢行 —— 2026-10-05 实测踩到过）。
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// buildFakeWrapperHost 用本机 Go 就地编一个假的汉化启动宿主（源码见 fakeWrapperHostSource），
// 返回它的路径。编不出来就 skip：这条实测是"能测到多少算多少"，不该把整个用例判失败。
func buildFakeWrapperHost(t *testing.T, dir string) string {
	t.Helper()
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("没找到 go，无法就地编出假宿主（可改用 DFO_LIVE_WRAPPER_HOST 指定现成的）：%v", err)
	}
	source := filepath.Join(dir, "fake-localization-host.go")
	if err := os.WriteFile(source, []byte(fakeWrapperHostSource), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "fake-localization-host.exe")
	build := exec.Command(goTool, "build", "-o", exe, source)
	if output, err := build.CombinedOutput(); err != nil {
		t.Skipf("编假宿主失败（%v）：\n%s", err, output)
	}
	return exe
}

// fakeWrapperHostSource 是假汉化启动宿主的源码：argv = `<DFO.exe> <dll> <payload…>`
// （与真宿主同一份协议），自己起客户端、把客户端 pid 写进 DFO_FAKE_HOST_PIDFILE、
// 往 stdout 写 `READY <pid>`、再补一行输出（用来证明宿主的其余 stdout 也会被转发），
// 最后**以 42 退出** —— 42 与客户端退出码无关，用来分辨"编排程序等/读的是谁"。
const fakeWrapperHostSource = `package main

import (
	"fmt"
	"os"
	"os/exec"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: fake-host <client.exe> <dll> [args...]")
		os.Exit(2)
	}
	client := os.Args[1]
	payload := os.Args[3:]
	child := exec.Command(client, payload...)
	if err := child.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "fake-host: start failed:", err)
		os.Exit(3)
	}
	if path := os.Getenv("DFO_FAKE_HOST_PIDFILE"); path != "" {
		_ = os.WriteFile(path, []byte(fmt.Sprint(child.Process.Pid)), 0o644)
	}
	fmt.Printf("READY %d\n", child.Process.Pid)
	fmt.Println("fake-host: dll injected (stub)")
	// 宿主报完就走：真实宿主也在初始化完成后不再需要它，客户端归编排程序看护。
	os.Exit(42)
}
`
