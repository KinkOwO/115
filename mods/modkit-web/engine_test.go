package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 假环境：假客户端目录 + 假服务端模块 + 假 modkit 引擎
//
// 为什么全用假的：装/卸的**真**路径要求"游戏没在运行"，而且真跑一次会去改客户端与
// 服务端目录——测试不能碰真实游戏目录，也不该受"业主此刻开没开游戏"影响。
// 于是这里替换掉两样东西：引擎子进程（run）与进程探测（procs）。
// 页面自己那条"游戏在跑就拒绝"的判决、以及"计划被阻断就停手"，因此都能被确定地测到。
// ---------------------------------------------------------------------------

type fakeRunner struct {
	calls [][]string
	reply func(args []string) runResult
}

func (f *fakeRunner) run(exe, dir string, args []string) runResult {
	f.calls = append(f.calls, args)
	if f.reply != nil {
		return f.reply(args)
	}
	return runResult{Cmd: displayCommand(exe, args), ExitCode: 0, Output: "# 假引擎：OK\n"}
}

func (f *fakeRunner) subcommands() []string {
	var out []string
	for _, c := range f.calls {
		if len(c) > 0 {
			out = append(out, c[0])
		}
	}
	return out
}

type testEnv struct {
	srv       *Server
	store     *Store
	modsDir   string
	base      string
	clientDir string
	rootDir   string
	moduleDir string
	engine    string
	run       *fakeRunner
	mux       *http.ServeMux
}

// newTestEnv 铺一个自洽的假环境（不碰任何真实目录）。
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "115")
	mods := filepath.Join(root, "mods")
	module := filepath.Join(root, "server", "work", "dfo-lan")
	client := filepath.Join(base, "DFO")
	for _, d := range []string{mods, module, client} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module dfolan\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"DFO.exe", "Script.pvf", "sk.dat"} {
		if err := os.WriteFile(filepath.Join(client, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	engine := filepath.Join(base, "modkit.exe")
	if err := os.WriteFile(engine, []byte("not a real exe"), 0o644); err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(mods)
	if err != nil {
		t.Fatal(err)
	}
	fr := &fakeRunner{}
	srv := &Server{
		Store: store,
		Cfg:   EngineConfig{ClientDir: client, Root: root, ModkitExe: engine},
		Run:   fr.run,
		Procs: func() []int { return nil },
	}
	srv.refreshEnv()
	return &testEnv{
		srv: srv, store: store, modsDir: mods, base: base, clientDir: client,
		rootDir: root, moduleDir: module, engine: engine, run: fr, mux: srv.Mux(),
	}
}

// addPackage 往 mods 目录里放一个 zip 成品包（清单含 server + client 两层）。
func (e *testEnv) addPackage(t *testing.T, rel string, id string) string {
	t.Helper()
	full := filepath.Join(e.modsDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	writeZip(t, full, map[string]string{
		"mod.json":      manifestJSON(id, "测试 "+id),
		"client/a.txt":  "hi",
		"server/mod.go": "package modpkg\n\nfunc Register() {}\n",
	})
	return full
}

// addInstalledDir 往 mods 目录里放一个已装目录（含 mod.json）。
func (e *testEnv) addInstalledDir(t *testing.T, dir string, id string) string {
	t.Helper()
	full := filepath.Join(e.modsDir, filepath.FromSlash(dir))
	if err := os.MkdirAll(full, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(full, "mod.json"), []byte(manifestJSON(id, "测试 "+id)), 0o644); err != nil {
		t.Fatal(err)
	}
	return full
}

func (e *testEnv) post(t *testing.T, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是 JSON：%v\n%s", err, rec.Body.String())
	}
	return rec, out
}

func stepsOf(t *testing.T, res map[string]any) []map[string]any {
	t.Helper()
	raw, _ := json.Marshal(res["steps"])
	var out []map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("steps 结构不对：%v", err)
	}
	return out
}

// ---------------------------------------------------------------------------
// 路径解析：复用启动器自己的配置，不硬编码
// ---------------------------------------------------------------------------

func TestResolveClientDirFromLauncherConfig(t *testing.T) {
	env := newTestEnv(t)
	// 启动器配置：client_dir 相对 <根>/server 解析（与启动器 config.ClientDir 同一口径）
	cfg := `{"client_dir": "../../DFO", "server_binary": "work/dfo-lan/bin/wireprobe-pvf.exe"}`
	if err := os.WriteFile(filepath.Join(env.rootDir, "server", "launcher.local.json"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	env.srv.Cfg.ClientDir = ""
	got := env.srv.refreshEnv()
	if got.ClientDir == "" {
		t.Fatalf("没能从 launcher.local.json 读出客户端目录：%+v", got)
	}
	if !strings.EqualFold(got.ClientDir, env.clientDir) {
		t.Fatalf("客户端目录解析错：%s ≠ %s", got.ClientDir, env.clientDir)
	}
	if !strings.Contains(got.ClientDirWhy, "launcher.local.json") {
		t.Fatalf("来源说明没写清是启动器配置：%q", got.ClientDirWhy)
	}
}

func TestResolveClientDirFallsBackToProbe(t *testing.T) {
	env := newTestEnv(t)
	env.srv.Cfg.ClientDir = ""
	got := env.srv.refreshEnv()
	if !strings.EqualFold(got.ClientDir, env.clientDir) {
		t.Fatalf("探测没命中客户端目录：%s", got.ClientDir)
	}
	if !strings.Contains(got.ClientDirWhy, "自动探测") {
		t.Fatalf("来源说明不对：%q", got.ClientDirWhy)
	}
}

func TestResolveLauncherRootFromNestedModsDir(t *testing.T) {
	env := newTestEnv(t)
	nested := filepath.Join(env.moduleDir, "mods") // 服务端模块自己的 mods 目录也要能定位
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(nested)
	if err != nil {
		t.Fatal(err)
	}
	got := resolveEnv(s.Dir, EngineConfig{}, func() []int { return nil })
	if !strings.EqualFold(got.LauncherRoot, env.rootDir) {
		t.Fatalf("没能从 %s 向上找到启动器根：%+v", nested, got)
	}
	if !strings.EqualFold(got.ModuleDir, env.moduleDir) {
		t.Fatalf("服务端模块根不对：%s", got.ModuleDir)
	}
}

// ---------------------------------------------------------------------------
// 前置检查：游戏在跑
// ---------------------------------------------------------------------------

func TestInstallRefusedWhileGameRunning(t *testing.T) {
	env := newTestEnv(t)
	env.addPackage(t, "pkg/demo.mod-1.0.0.zip", "demo.mod")
	env.srv.Procs = func() []int { return []int{4242} }

	rec, res := env.post(t, "/api/install", map[string]any{"key": "pkg/demo.mod-1.0.0.zip"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("游戏在跑时应回 409，实际 %d：%s", rec.Code, rec.Body.String())
	}
	if msg, _ := res["error"].(string); !strings.Contains(msg, "请先退出游戏") {
		t.Fatalf("错误里没有提示先退出游戏：%v", res["error"])
	}
	if len(env.run.calls) != 0 {
		t.Fatalf("被拒的安装不许调用引擎，实际调用了：%v", env.run.calls)
	}
	steps := stepsOf(t, res)
	if len(steps) == 0 {
		t.Fatal("被拒时也要把「检查了哪几步」带回给页面")
	}
	dump, _ := json.Marshal(steps)
	if !strings.Contains(string(dump), "DFO.exe") {
		t.Fatalf("步骤里没写清是 DFO.exe 检查：%s", dump)
	}
}

func TestUninstallRefusedWhileGameRunning(t *testing.T) {
	env := newTestEnv(t)
	env.addInstalledDir(t, "demo.srv", "demo.srv")
	env.srv.Procs = func() []int { return []int{7} }

	rec, res := env.post(t, "/api/uninstall", map[string]any{"key": "demo.srv"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("游戏在跑时应回 409，实际 %d", rec.Code)
	}
	if msg, _ := res["error"].(string); !strings.Contains(msg, "请先退出游戏") {
		t.Fatalf("错误里没有提示先退出游戏：%v", res["error"])
	}
	if len(env.run.calls) != 0 {
		t.Fatalf("被拒的卸载不许调用引擎：%v", env.run.calls)
	}
}

func TestServerOnlyInstallAllowedWhileGameRunning(t *testing.T) {
	env := newTestEnv(t)
	// 只有 server 层：不写客户端文件，引擎也不会拒绝（它的门只管客户端写）
	writeZip(t, filepath.Join(env.modsDir, "srv-only.zip"), map[string]string{
		"mod.json": `{"schema":2,"id":"srv.only","version":"1.0.0","name":"只有服务端",
      "permissions":["server.hook"],
      "layers":{"server":{"package":".","hooks":[{"name":"server.boot"}]}}}`,
		"server/mod.go": "package modpkg\n\nfunc Register() {}\n",
	})
	env.srv.Procs = func() []int { return []int{11} }

	rec, res := env.post(t, "/api/install", map[string]any{"key": "srv-only.zip"})
	if rec.Code != http.StatusOK {
		t.Fatalf("只有 server 层时不该被游戏运行挡住，实际 %d：%s", rec.Code, rec.Body.String())
	}
	if res["ok"] != true {
		t.Fatalf("应当成功：%s", rec.Body.String())
	}
	if res["needsRebuild"] != true {
		t.Fatal("含 server 层必须提示要重新编译服务端")
	}
}

// ---------------------------------------------------------------------------
// 预演 / 阻断 / 正常路径
// ---------------------------------------------------------------------------

func TestDryRunAllowedWhileGameRunning(t *testing.T) {
	env := newTestEnv(t)
	env.addPackage(t, "demo.mod.zip", "demo.mod")
	env.srv.Procs = func() []int { return []int{99} }

	rec, res := env.post(t, "/api/install", map[string]any{"key": "demo.mod.zip", "dryRun": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("预演应放行，实际 %d：%s", rec.Code, rec.Body.String())
	}
	if res["dryRun"] != true {
		t.Fatal("响应没标出这是预演")
	}
	joined := strings.Join(env.run.subcommands(), " ")
	if !strings.Contains(joined, "install") {
		t.Fatalf("预演也要走 install（带 --dry-run）：%v", env.run.calls)
	}
	last := env.run.calls[len(env.run.calls)-1]
	if !contains(last, "--dry-run") {
		t.Fatalf("预演没有加 --dry-run：%v", last)
	}
}

func TestPlanBlockedStopsBeforeInstall(t *testing.T) {
	env := newTestEnv(t)
	env.addPackage(t, "blocked.mod.zip", "blocked.mod")
	const conflict = "mod blocked.mod（x）v1\n冲突：目标已被 mod other.mod 占用\n"
	env.run.reply = func(args []string) runResult {
		if len(args) > 0 && args[0] == "plan" {
			return runResult{Cmd: "modkit plan", ExitCode: 2, Output: conflict}
		}
		return runResult{Cmd: "modkit " + args[0], ExitCode: 0, Output: "ok\n"}
	}

	rec, res := env.post(t, "/api/install", map[string]any{"key": "blocked.mod.zip"})
	if rec.Code != http.StatusOK {
		t.Fatalf("计划被阻断属于「跑过了引擎」，应回 200，实际 %d", rec.Code)
	}
	if res["blocked"] != true || res["ok"] == true {
		t.Fatalf("应当报 blocked=true / ok=false：%s", rec.Body.String())
	}
	subs := strings.Join(env.run.subcommands(), ",")
	if strings.Contains(subs, "install") {
		t.Fatalf("计划被阻断后不许再 install：%v", env.run.calls)
	}
	if !strings.Contains(rec.Body.String(), "占用") {
		t.Fatal("plan 的原文必须原样带回给页面")
	}
	steps := stepsOf(t, res)
	last := steps[len(steps)-1]
	if last["blocked"] != true {
		t.Fatalf("被阻断的那一步要标出来：%v", last)
	}
}

func TestInstallRunsVerifyPlanInstall(t *testing.T) {
	env := newTestEnv(t)
	pkg := env.addPackage(t, "demo.mod.zip", "demo.mod")

	rec, res := env.post(t, "/api/install", map[string]any{"key": "demo.mod.zip"})
	if rec.Code != http.StatusOK || res["ok"] != true {
		t.Fatalf("安装应当成功：%d %s", rec.Code, rec.Body.String())
	}
	want := []string{"verify", "plan", "install"}
	if got := env.run.subcommands(); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("步骤顺序不对：%v", got)
	}
	inst := env.run.calls[2]
	for _, pair := range [][2]string{
		{"--client", env.clientDir},
		{"--mod", pkg},
		{"--root", env.moduleDir},
	} {
		if !containsPair(inst, pair[0], pair[1]) {
			t.Fatalf("install 参数缺 %s=%s：%v", pair[0], pair[1], inst)
		}
	}
	if res["needsRebuild"] != true {
		t.Fatal("含 server 层要提示重编译服务端")
	}
	if !strings.Contains(rec.Body.String(), "重新编译") {
		t.Fatal("提示里要写清'重新编译服务端二进制'")
	}
}

func TestInstallInstalledDirPrefersSameIDPackage(t *testing.T) {
	env := newTestEnv(t)
	env.addInstalledDir(t, "qol.client-host", "qol.client-host")
	pkg := env.addPackage(t, "pkg/qol.client-host-1.0.0.zip", "qol.client-host")

	rec, res := env.post(t, "/api/install", map[string]any{"key": "qol.client-host"})
	if rec.Code != http.StatusOK || res["ok"] != true {
		t.Fatalf("安装应当成功：%d %s", rec.Code, rec.Body.String())
	}
	if res["package"] != pkg {
		t.Fatalf("应当优先用同 id 的成品包：%v", res["package"])
	}
	if res["packageForm"] != "zip" {
		t.Fatalf("形态应当是 zip：%v", res["packageForm"])
	}
	if contains(env.run.calls[0], "--allow-dir") {
		t.Fatal("用成品包时不该带 --allow-dir")
	}
}

func TestInstallInstalledDirFallsBackToAllowDir(t *testing.T) {
	env := newTestEnv(t)
	dir := env.addInstalledDir(t, "solo.mod", "solo.mod")

	rec, res := env.post(t, "/api/install", map[string]any{"key": "solo.mod"})
	if rec.Code != http.StatusOK || res["ok"] != true {
		t.Fatalf("安装应当成功：%d %s", rec.Code, rec.Body.String())
	}
	if res["package"] != dir || res["packageForm"] != "dir" {
		t.Fatalf("应当退回目录形态：%v / %v", res["package"], res["packageForm"])
	}
	for _, c := range env.run.calls {
		if !contains(c, "--allow-dir") {
			t.Fatalf("目录形态每一步都要带 --allow-dir：%v", c)
		}
	}
}

func TestUninstallPreviewsThenReallyUninstalls(t *testing.T) {
	env := newTestEnv(t)
	env.addInstalledDir(t, "demo.srv", "demo.srv")

	rec, res := env.post(t, "/api/uninstall", map[string]any{"key": "demo.srv"})
	if rec.Code != http.StatusOK || res["ok"] != true {
		t.Fatalf("卸载应当成功：%d %s", rec.Code, rec.Body.String())
	}
	if got := env.run.subcommands(); strings.Join(got, ",") != "uninstall,uninstall" {
		t.Fatalf("应当先 dry-run 预览再真卸：%v", env.run.calls)
	}
	if !contains(env.run.calls[0], "--dry-run") {
		t.Fatalf("第一步应当带 --dry-run：%v", env.run.calls[0])
	}
	if contains(env.run.calls[1], "--dry-run") {
		t.Fatalf("第二步不该带 --dry-run：%v", env.run.calls[1])
	}
	if !containsPair(env.run.calls[1], "--id", "demo.srv") {
		t.Fatalf("卸载要按 id：%v", env.run.calls[1])
	}
}

func TestOperationRejectsUnknownKey(t *testing.T) {
	env := newTestEnv(t)
	rec, _ := env.post(t, "/api/install", map[string]any{"key": "nope"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("未知 key 应回 400，实际 %d", rec.Code)
	}
	if len(env.run.calls) != 0 {
		t.Fatal("参数不对时不该调用引擎")
	}
}

// ---------------------------------------------------------------------------
// 引擎缺失 / 构建入口
// ---------------------------------------------------------------------------

func TestMissingEngineRefusesWithHint(t *testing.T) {
	env := newTestEnv(t)
	env.addPackage(t, "demo.mod.zip", "demo.mod")
	env.srv.Cfg.ModkitExe = filepath.Join(env.base, "does-not-exist.exe")

	rec, res := env.post(t, "/api/install", map[string]any{"key": "demo.mod.zip"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("缺引擎应回 409，实际 %d", rec.Code)
	}
	dump := rec.Body.String()
	if !strings.Contains(dump, "找不到 modkit 引擎") || !strings.Contains(dump, "构建引擎") {
		t.Fatalf("缺引擎的提示要给出路：%s", dump)
	}
	if res["ok"] == true {
		t.Fatal("缺引擎不该报成功")
	}
}

func TestEngineBuildWithoutRepoExplains(t *testing.T) {
	env := newTestEnv(t)
	env.srv.Cfg.LauncherRepo = ""
	// 同级目录里没有启动器仓 → 应当明确报"请用 --launcher-repo 指定"
	rec, res := env.post(t, "/api/engine/build", map[string]any{})
	if rec.Code == http.StatusOK {
		t.Skipf("这台机器的同级目录里真有启动器仓源码，跳过：%v", res)
	}
	if msg, _ := res["error"].(string); !strings.Contains(msg, "--launcher-repo") {
		t.Fatalf("缺源码根的提示不明确：%v", res["error"])
	}
}

// ---------------------------------------------------------------------------
// 工具函数
// ---------------------------------------------------------------------------

func TestToolchainEnvDefaultsUseExistingLayout(t *testing.T) {
	env := newTestEnv(t)
	// 造出整合包既有布局：<base>/tools/gopath 与 <base>/tools/gocache
	for _, d := range []string{filepath.Join(env.base, "tools", "gopath"), filepath.Join(env.base, "tools", "gocache")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := resolveEnv(env.modsDir, EngineConfig{ClientDir: env.clientDir, Root: env.rootDir, ModkitExe: env.engine}, func() []int { return nil })
	// 用"空环境"调用纯函数形式：跑测试的这台机器上可能本来就设了 GOPATH/GOCACHE。
	extra := strings.Join(toolchainEnvDefaultsFor(got, nil), " ")
	if !strings.Contains(extra, "GOPATH=") || !strings.Contains(extra, "gopath") {
		t.Fatalf("没复用整合包既有的 GOPATH 布局：%s", extra)
	}
}

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

func containsPair(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}
