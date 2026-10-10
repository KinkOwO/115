package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// 本文件是页面的**引擎层**：把「安装 / 卸载」接到启动器仓 `internal/modkit` 的既有入口上。
//
// 为什么走 CLI 子进程（`modkit verify/plan/install/uninstall`）而不是在页面里另写一套：
//   - 落位/备份/hash/还原/exe.patch 那套逻辑只有一份真源（`internal/modkit`），
//     页面再实现一遍必然与它漂移；
//   - 那道"DFO.exe 在跑就拒绝写盘"的硬门、以及"计划被阻断就整体拒绝"的判决，
//     也都在引擎里；页面照搬它的原文展示给玩家，不做二次解释。
//
// 本包刻意**不 import 启动器代码**（页面工具要能独立编译，见 README §五）；
// 两个仓库之间只有"命令行契约"这一层耦合。

// ---------------------------------------------------------------------------
// 路径与"这几个值从哪来"
// ---------------------------------------------------------------------------

// EnvInfo 是装/卸要用的路径来源；页面顶部把它显示出来，出问题时一眼能看到用的是哪个目录。
type EnvInfo struct {
	ModsDir         string   `json:"modsDir"`
	LauncherRoot    string   `json:"launcherRoot,omitempty"`
	LauncherRootWhy string   `json:"launcherRootSource,omitempty"`
	ModuleDir       string   `json:"moduleDir,omitempty"`
	ClientDir       string   `json:"clientDir,omitempty"`
	ClientDirWhy    string   `json:"clientDirSource,omitempty"`
	ModkitExe       string   `json:"modkitExe,omitempty"`
	ModkitWhy       string   `json:"modkitExeSource,omitempty"`
	LauncherRepo    string   `json:"launcherRepo,omitempty"`
	LauncherRepoWhy string   `json:"launcherRepoSource,omitempty"`
	GoExe           string   `json:"goExe,omitempty"`
	ClientRunning   bool     `json:"clientRunning"`
	ClientProcs     []int    `json:"clientProcs,omitempty"`
	Problems        []string `json:"problems,omitempty"`
}

// EngineConfig 是启动参数里与装/卸有关的那几项（都为空时全部自动探测）。
type EngineConfig struct {
	ClientDir    string // --client：客户端目录
	Root         string // --root：启动器根 / 服务端模块根
	ModkitExe    string // --modkit：modkit 引擎可执行文件
	LauncherRepo string // --launcher-repo：启动器仓源码根（缺引擎时可就地构建）
	GoExe        string // --go：Go 工具链
}

// processEntry 是进程快照里的一项（按平台实现取，见 platform_*.go）。
type processEntry struct {
	PID  int
	Name string
}

// resolveEnv 解析装/卸要用的全部路径。它只读盘、不写盘。
func resolveEnv(modsDir string, cfg EngineConfig, procs func() []int) EnvInfo {
	env := EnvInfo{ModsDir: modsDir}

	// —— 启动器根 / 服务端模块根 ——
	root, why := resolveLauncherRoot(modsDir, cfg.Root)
	env.LauncherRoot, env.LauncherRootWhy = root, why
	if root == "" {
		env.Problems = append(env.Problems,
			"找不到启动器根（含 server/work/dfo-lan/go.mod 的目录）：装/卸的 server 层落位需要它，请用 --root 指定")
	} else if md := filepath.Join(root, "server", "work", "dfo-lan"); isServerModule(md) {
		env.ModuleDir = md
	}

	// —— 客户端目录：优先复用启动器自己的配置 server/launcher.local.json ——
	// （不硬编码路径：换盘、换机器、整合包搬家后这份配置才是真值，与启动器同一口径）
	client, cwhy := resolveClientDir(modsDir, root, cfg.ClientDir)
	env.ClientDir, env.ClientDirWhy = client, cwhy
	if client == "" {
		env.Problems = append(env.Problems,
			"找不到 DFO 客户端目录（要有 DFO.exe / Script.pvf / sk.dat）："+
				"请看 server/launcher.local.json 的 client_dir，或用 --client 指定")
	}

	// —— 引擎可执行文件 ——
	exe, ewhy := locateModkitExe(modsDir, root, cfg.ModkitExe)
	env.ModkitExe, env.ModkitWhy = exe, ewhy
	if exe == "" {
		env.Problems = append(env.Problems,
			"找不到 modkit 引擎（modkit.exe）：点页面上的「构建引擎」，或用 --modkit 指定")
	}

	// —— 构建用的源码与工具链（只在缺引擎时才用得上）——
	repo, rwhy := locateLauncherRepo(modsDir, root, cfg.LauncherRepo)
	env.LauncherRepo, env.LauncherRepoWhy = repo, rwhy
	env.GoExe = locateGo(root, cfg.GoExe)

	// —— 游戏是否在跑（客户端侧操作的硬门）——
	env.ClientProcs = procs()
	env.ClientRunning = len(env.ClientProcs) > 0
	return env
}

// resolveLauncherRoot 找出启动器根：显式值优先，其次从 mods 目录**向上**找
// 「含 server/work/dfo-lan/go.mod 且 module 是 dfolan」的那一级。
//
// 为什么向上找而不是拼 ".."：这个页面默认管的可能是 `<根>/mods`（作者工作区），
// 也可能是 `<根>/server/work/dfo-lan/mods`（服务端模块自己的 mods），两种都要能定位。
func resolveLauncherRoot(modsDir, explicit string) (string, string) {
	if strings.TrimSpace(explicit) != "" {
		if root, ok := launcherRootOf(explicit); ok {
			return root, "--root 指定：" + root
		}
		return "", "--root 指定的目录既不是启动器根，也不是服务端模块根：" + explicit
	}
	dir, err := filepath.Abs(modsDir)
	if err != nil {
		return "", ""
	}
	for i := 0; i < 6; i++ {
		if root, ok := launcherRootOf(dir); ok {
			return root, fmt.Sprintf("从 mods 目录向上找到（%d 层）：%s", i, root)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", ""
}

// launcherRootOf 判断一个目录是「启动器根」还是「服务端模块根」，返回启动器根。
func launcherRootOf(dir string) (string, bool) {
	if isServerModule(dir) {
		// <根>/server/work/dfo-lan → 回退三级得到启动器根
		return filepath.Dir(filepath.Dir(filepath.Dir(dir))), true
	}
	if isServerModule(filepath.Join(dir, "server", "work", "dfo-lan")) {
		return filepath.Clean(dir), true
	}
	return "", false
}

// isServerModule 判定目录是不是服务端 Go 模块根（go.mod 里 module dfolan）。
func isServerModule(dir string) bool {
	b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")) == "dfolan"
		}
	}
	return false
}

// resolveClientDir 解析客户端目录：
//  1. --client（显式）
//  2. 启动器配置 <根>/server/launcher.local.json 的 client_dir（相对路径以 <根>/server 为基准，
//     与启动器 config.ClientDir 同一口径）
//  3. 若干候选位置探测
func resolveClientDir(modsDir, root, explicit string) (string, string) {
	if strings.TrimSpace(explicit) != "" {
		if hasClientFiles(explicit) {
			return filepath.Clean(explicit), "--client 指定：" + explicit
		}
		return "", "--client 指定的目录里没有 DFO.exe / Script.pvf / sk.dat：" + explicit
	}
	if root != "" {
		if dir, ok := clientDirFromLauncherConfig(root); ok {
			return dir, "server/launcher.local.json 的 client_dir（启动器自己的配置）"
		}
	}
	var cands []string
	if root != "" {
		cands = append(cands,
			filepath.Join(root, "client"),
			filepath.Join(root, "..", "client"),
			filepath.Join(root, "..", "DFO"),
			filepath.Join(root, "DFO"),
		)
	}
	cands = append(cands,
		filepath.Join(modsDir, "..", "..", "DFO"),
		filepath.Join(modsDir, "..", "DFO"),
	)
	for _, c := range cands {
		if hasClientFiles(c) {
			abs, err := filepath.Abs(c)
			if err != nil {
				abs = c
			}
			return filepath.Clean(abs), "自动探测命中：" + c
		}
	}
	return "", ""
}

// clientDirFromLauncherConfig 读 <根>/server/launcher.local.json 的 client_dir。
func clientDirFromLauncherConfig(root string) (string, bool) {
	path := filepath.Join(root, "server", "launcher.local.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var doc map[string]any
	if err := json.Unmarshal(trimBOM(raw), &doc); err != nil {
		return "", false
	}
	value, _ := doc["client_dir"].(string)
	if strings.TrimSpace(value) == "" {
		return "", false
	}
	p := filepath.FromSlash(value)
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, "server", p)
	}
	if !hasClientFiles(p) {
		return "", false
	}
	return filepath.Clean(p), true
}

// hasClientFiles 报告目录是否含客户端三件套。
func hasClientFiles(dir string) bool {
	if dir == "" {
		return false
	}
	for _, name := range []string{"DFO.exe", "Script.pvf", "sk.dat"} {
		st, err := os.Stat(filepath.Join(dir, name))
		if err != nil || !st.Mode().IsRegular() {
			return false
		}
	}
	return true
}

// toolDir 返回"页面自己的目录"：run.cmd / 源码 / 已编好的 exe 所在的那一层。
//
// 为什么要挑：`go run .` 时 os.Executable() 指向临时构建目录，把引擎编到那里就白编了；
// 而 run.cmd 与双击 exe 两种启动方式下，当前目录都是页面目录。
func toolDir() string {
	var cands []string
	if wd, err := os.Getwd(); err == nil {
		cands = append(cands, wd)
	}
	if exe, err := os.Executable(); err == nil {
		cands = append(cands, filepath.Dir(exe))
	}
	for _, c := range cands {
		for _, marker := range []string{"run.cmd", "main.go", "modkit-web.exe"} {
			if _, err := os.Stat(filepath.Join(c, marker)); err == nil {
				return filepath.Clean(c)
			}
		}
	}
	if len(cands) > 0 {
		return filepath.Clean(cands[0])
	}
	return "."
}

// locateModkitExe 找 modkit 引擎可执行文件（只做定位，不做构建）。
func locateModkitExe(modsDir, root, explicit string) (string, string) {
	if strings.TrimSpace(explicit) != "" {
		if isRegular(explicit) {
			return filepath.Clean(explicit), "--modkit 指定"
		}
		return "", "--modkit 指定的文件不存在：" + explicit
	}
	if v := strings.TrimSpace(os.Getenv("MODKIT_EXE")); v != "" {
		if isRegular(v) {
			return filepath.Clean(v), "环境变量 MODKIT_EXE"
		}
		return "", "环境变量 MODKIT_EXE 指向的文件不存在：" + v
	}
	var cands []string
	cands = append(cands, filepath.Join(toolDir(), "modkit.exe"))
	if abs, err := filepath.Abs(modsDir); err == nil {
		cands = append(cands, filepath.Join(abs, "modkit.exe"))
	}
	if root != "" {
		cands = append(cands,
			filepath.Join(root, "bin", "modkit.exe"),
			filepath.Join(root, "mods", "modkit.exe"),
			filepath.Join(root, "mods", "modkit-web", "modkit.exe"),
		)
	}
	for _, c := range cands {
		if isRegular(c) {
			return filepath.Clean(c), "随包查找命中：" + c
		}
	}
	if p, err := exec.LookPath("modkit.exe"); err == nil {
		return p, "PATH 上的 modkit.exe"
	}
	return "", ""
}

// locateLauncherRepo 找启动器仓源码根（编 modkit.exe 用）。
//
// 判据是**内容**不是目录名：同时有 go.mod、cmd/modkit/main.go，且 go.mod 的 module 是 dfolauncher。
func locateLauncherRepo(modsDir, root, explicit string) (string, string) {
	isRepo := func(dir string) bool {
		if !isRegular(filepath.Join(dir, "cmd", "modkit", "main.go")) {
			return false
		}
		b, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err != nil {
			return false
		}
		return strings.Contains(string(b), "module dfolauncher")
	}
	if strings.TrimSpace(explicit) != "" {
		if isRepo(explicit) {
			return filepath.Clean(explicit), "--launcher-repo 指定"
		}
		return "", "--launcher-repo 指定的目录不像启动器仓（缺 cmd/modkit/main.go 或 go.mod 不是 dfolauncher）：" + explicit
	}
	if v := strings.TrimSpace(os.Getenv("MODKIT_LAUNCHER_REPO")); v != "" {
		if isRepo(v) {
			return filepath.Clean(v), "环境变量 MODKIT_LAUNCHER_REPO"
		}
	}
	var cands []string
	if abs, err := filepath.Abs(modsDir); err == nil {
		cands = append(cands,
			filepath.Join(abs, "..", "115us-dfolauncher"),
			filepath.Join(abs, "..", "..", "115us-dfolauncher"),
		)
	}
	if root != "" {
		cands = append(cands,
			filepath.Join(root, "..", "115us-dfolauncher"),
			filepath.Join(root, "..", "115us-launcher"),
		)
	}
	// 同级目录里"长得像启动器仓"的那个（目录名任取）
	for _, base := range []string{root, filepath.Join(modsDir, ".."), filepath.Join(modsDir, "..", "..")} {
		if base == "" {
			continue
		}
		ents, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range ents {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			cands = append(cands, filepath.Join(base, e.Name()))
		}
	}
	for _, c := range cands {
		if isRepo(c) {
			return filepath.Clean(c), "同级目录探测命中：" + c
		}
	}
	return "", ""
}

// locateGo 找 Go 工具链（编引擎用）：--go / 环境变量 GO / PATH / 整合包里的 tools/go。
func locateGo(root, explicit string) string {
	if strings.TrimSpace(explicit) != "" && isRegular(explicit) {
		return filepath.Clean(explicit)
	}
	if v := strings.TrimSpace(os.Getenv("GO")); v != "" && isRegular(v) {
		return filepath.Clean(v)
	}
	for _, name := range []string{"go.exe", "go"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	var cands []string
	if root != "" {
		cands = append(cands,
			filepath.Join(root, "..", "tools", "go", "bin", "go.exe"),
			filepath.Join(root, "tools", "go", "bin", "go.exe"),
		)
	}
	for _, c := range cands {
		if isRegular(c) {
			return filepath.Clean(c)
		}
	}
	return ""
}

func isRegular(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

func trimBOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

// ---------------------------------------------------------------------------
// 跑引擎
// ---------------------------------------------------------------------------

// runResult 是一条子命令的结果：命令行（给人看）、合并后的输出原文、退出码。
type runResult struct {
	Cmd      string `json:"cmd"`
	Output   string `json:"output"`
	ExitCode int    `json:"exitCode"`
	// SpawnErr 非空表示**根本没跑起来**（可执行文件不在、权限被拒等）；
	// 它能跑起来但返回非零时是 ExitCode。
	SpawnErr string `json:"spawnErr,omitempty"`
}

type runFunc func(exe, dir string, args []string) runResult

// engineRunner 是"跑一条 modkit 子命令"的实现（测试里可替换）。
var engineRunner runFunc = runEngineReal

// clientProcessProbe 是"查 DFO.exe 进程号"的实现（测试里可替换：见 engine_test.go
// 里那两条"游戏在跑就拒绝"的用例——那条路径不能只靠人工开游戏去撞）。
var clientProcessProbe = clientProcesses

func runEngineReal(exe, dir string, args []string) runResult {
	res := runResult{Cmd: displayCommand(exe, args)}
	cmd := exec.Command(exe, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	res.Output = string(trimBOM(out))
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
			res.SpawnErr = err.Error()
		}
	}
	return res
}

// displayCommand 把 argv 拼成"能照着敲"的一行（带空格的参数加引号）。
func displayCommand(exe string, args []string) string {
	parts := append([]string{exe}, args...)
	for i, p := range parts {
		if strings.ContainsAny(p, " \t") {
			parts[i] = `"` + p + `"`
		}
	}
	return strings.Join(parts, " ")
}

// ---------------------------------------------------------------------------
// 安装 / 卸载
// ---------------------------------------------------------------------------

// StepResult 是装/卸里的一步。页面按顺序把它渲染出来，**原文照抄**引擎的输出。
type StepResult struct {
	Name    string     `json:"name"`
	Detail  string     `json:"detail,omitempty"`
	Run     *runResult `json:"run,omitempty"`
	OK      bool       `json:"ok"`
	Blocked bool       `json:"blocked,omitempty"`
	Note    string     `json:"note,omitempty"`
}

// OpResult 是一次装/卸的完整交代。
type OpResult struct {
	Action       string       `json:"action"` // install | uninstall
	DryRun       bool         `json:"dryRun"`
	OK           bool         `json:"ok"`
	Blocked      bool         `json:"blocked"`
	Key          string       `json:"key"`
	ID           string       `json:"id"`
	Pkg          string       `json:"package,omitempty"`
	PkgForm      string       `json:"packageForm,omitempty"` // zip | dir
	Steps        []StepResult `json:"steps"`
	Notes        []string     `json:"notes,omitempty"`
	NeedsRebuild bool         `json:"needsRebuild"`
	Error        string       `json:"error,omitempty"`
}

// opRequest 是 /api/install 与 /api/uninstall 的请求体（键 = 列表里的相对路径）。
type opRequest struct {
	Key    string `json:"key"`
	DryRun bool   `json:"dryRun"`
}

// installSource 是"这次装的是哪个包"。
type installSource struct {
	Path     string // 绝对路径（zip 或目录）
	AllowDir bool   // 目录形态要带 --allow-dir
	Form     string // zip | dir
	Note     string
}

// pickInstallSource 决定用哪个包安装：
//   - [包] 行 → 就是那个 zip（引擎只认 zip，这是正式分发形态）；
//   - [已装] 行 → 先在同 id 的 zip 成品包里找（升级时就该装成品包）；
//     找不到才退回目录形态（--allow-dir，开发自测形态，输出里会明说）。
func (s *Server) pickInstallSource(mod Mod, list []Mod) installSource {
	full := filepath.Join(s.Store.Dir, filepath.FromSlash(mod.DirName))
	if mod.Kind == KindPackage {
		return installSource{Path: full, Form: "zip", Note: "以 zip 成品包安装：" + mod.DirName}
	}
	for _, m := range list {
		if m.Kind == KindPackage && m.ID != "" && strings.EqualFold(m.ID, mod.ID) {
			return installSource{
				Path: filepath.Join(s.Store.Dir, filepath.FromSlash(m.DirName)), Form: "zip",
				Note: "已装目录找到同 id 的成品包，按它安装：" + m.DirName,
			}
		}
	}
	return installSource{
		Path: full, AllowDir: true, Form: "dir",
		Note: "没找到同 id 的 zip 成品包，按目录形态安装（--allow-dir，开发自测形态）：" + mod.DirName,
	}
}

// layerNeedsClientWrite 报告"这一项是否要在客户端目录里动文件/跑脚本"。
// 与引擎的 needsClientWrite 同判据：client / pvf / resource 三层任一存在即算。
func layerNeedsClientWrite(layers []string) bool {
	for _, l := range layers {
		switch l {
		case "client", "pvf", "resource":
			return true
		}
	}
	return false
}

func layerHas(layers []string, want string) bool {
	for _, l := range layers {
		if l == want {
			return true
		}
	}
	return false
}

// runInstall 走 verify → plan → install 三步；plan 退出码 2 = 计划被阻断，到此为止。
func (s *Server) runInstall(res *OpResult, mod Mod, env EnvInfo, list []Mod) {
	src := s.pickInstallSource(mod, list)
	res.Pkg, res.PkgForm = src.Path, src.Form
	res.Notes = append(res.Notes, src.Note)
	res.NeedsRebuild = layerHas(mod.Layers, "server")

	base := []string{}
	if src.AllowDir {
		base = append(base, "--allow-dir")
	}

	// 1) verify：清单/包内文件/权限位一次核对（引擎的权威校验器）。
	verifyArgs := append([]string{"verify", "--mod", src.Path}, base...)
	if st, ok := s.step(res, env, "modkit verify（核对清单与包内文件）", verifyArgs); !ok {
		st.Note = "清单没通过：先修包，再装"
		return
	}
	// 2) plan：求解施工计划；退出码 2 = 会阻断，绝不硬来。
	planArgs := append([]string{"plan", "--client", env.ClientDir, "--mod", src.Path, "--root", env.RootFor()}, base...)
	st, ok := s.step(res, env, "modkit plan（求解计划，只读）", planArgs)
	if st.Run != nil && st.Run.ExitCode == 2 {
		idx := len(res.Steps) - 1
		res.Blocked = true
		res.Steps[idx].Blocked = true
		res.Steps[idx].Note = "计划被阻断：引擎判定这次安装会破坏现场（详见上面 plan 原文的「冲突：」行）；没有改动任何文件"
		res.Error = "计划被阻断（退出码 2）：引擎判定这次安装会破坏现场，已停手"
		return
	}
	if !ok {
		st.Note = "计划求解失败"
		return
	}
	// 3) install：真正落盘（dry-run 时不写）。
	installArgs := append([]string{"install", "--client", env.ClientDir, "--mod", src.Path, "--root", env.RootFor()}, base...)
	if res.DryRun {
		installArgs = append(installArgs, "--dry-run")
	}
	st, ok = s.step(res, env, "modkit install（落位）", installArgs)
	if !ok {
		return
	}
	if hasEngineWarning(st.Run.Output, "重新编译服务端") {
		res.NeedsRebuild = true
	}
	res.OK = true
	if res.NeedsRebuild {
		res.Notes = append(res.Notes,
			"含 server 层：源码已落位，但**必须重新编译服务端二进制**才会生效"+
				"（启动器点「开始游戏」时会就地编译；也可用 server/Build-Server.ps1）")
	}
	if res.DryRun {
		res.Notes = append(res.Notes, "这是预演（dry-run）：没有改动任何文件")
	}
}

// runUninstall 先 dry-run 预览，再真卸。
func (s *Server) runUninstall(res *OpResult, mod Mod, env EnvInfo) {
	base := []string{"uninstall", "--client", env.ClientDir, "--id", mod.ID, "--root", env.RootFor()}
	if !res.DryRun {
		preview := append(append([]string{}, base...), "--dry-run")
		if _, ok := s.step(res, env, "modkit uninstall --dry-run（先看会还原什么）", preview); !ok {
			res.Notes = append(res.Notes, "预览失败：可能是这个 mod 不是用 modkit 装的（引擎只认自己的注册表）")
			return
		}
	}
	final := append([]string{}, base...)
	if res.DryRun {
		final = append(final, "--dry-run")
	}
	st, ok := s.step(res, env, "modkit uninstall（逐字节还原）", final)
	if !ok {
		return
	}
	res.OK = true
	res.NeedsRebuild = layerHas(mod.Layers, "server")
	if res.NeedsRebuild {
		res.Notes = append(res.Notes, "含 server 层：源码已撤走，重新编译服务端二进制后才会生效")
	}
	if hasEngineWarning(st.Run.Output, "重新编译服务端") {
		res.NeedsRebuild = true
	}
	if res.DryRun {
		res.Notes = append(res.Notes, "这是预演（dry-run）：没有改动任何文件")
	}
}

// step 追加一步并跑它；ok=false 表示这一步失败（调用方据此停下并把原文留给页面展示）。
func (s *Server) step(res *OpResult, env EnvInfo, name string, args []string) (StepResult, bool) {
	run := s.Run(env.ModkitExe, "", args)
	st := StepResult{Name: name, Run: &run}
	st.OK = run.ExitCode == 0 && run.SpawnErr == ""
	res.Steps = append(res.Steps, st)
	last := len(res.Steps) - 1
	if st.OK {
		return res.Steps[last], true
	}
	if run.SpawnErr != "" {
		res.Steps[last].Note = "引擎没能启动：" + run.SpawnErr
		res.Error = "引擎没能启动（" + env.ModkitExe + "）：" + run.SpawnErr
	} else {
		res.Steps[last].Note = fmt.Sprintf("退出码 %d", run.ExitCode)
		res.Error = fmt.Sprintf("命令失败（退出码 %d）：%s", run.ExitCode, strings.TrimSpace(firstLine(run.Output)))
	}
	return res.Steps[last], false
}

// hasEngineWarning 在引擎输出里找一句提示（页面据此点亮"要重编译"的提醒）。
func hasEngineWarning(output, needle string) bool {
	return strings.Contains(output, needle)
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}

// RootFor 返回传给引擎 --root 的值：优先服务端模块根（最精确），否则启动器根。
func (e EnvInfo) RootFor() string {
	if e.ModuleDir != "" {
		return e.ModuleDir
	}
	return e.LauncherRoot
}

// ---------------------------------------------------------------------------
// 就地构建引擎（缺 modkit.exe 时的一键入口）
// ---------------------------------------------------------------------------

// buildEngineResult 是一次"构建引擎"的结果。
type buildEngineResult struct {
	OK       bool       `json:"ok"`
	Exe      string     `json:"exe,omitempty"`
	Repo     string     `json:"repo,omitempty"`
	GoExe    string     `json:"goExe,omitempty"`
	Run      *runResult `json:"run,omitempty"`
	Output   string     `json:"output,omitempty"`
	Error    string     `json:"error,omitempty"`
	EnvAdded []string   `json:"envAdded,omitempty"`
}

// buildEngine 用包内/系统 Go 在启动器仓里编出 cmd/modkit，输出到页面目录下的 modkit.exe。
func buildEngine(env EnvInfo) buildEngineResult {
	out := buildEngineResult{Repo: env.LauncherRepo, GoExe: env.GoExe}
	if env.LauncherRepo == "" {
		out.Error = "找不到启动器仓源码根（含 cmd/modkit/main.go 与 module dfolauncher 的目录）：" +
			"请用 --launcher-repo 指定"
		return out
	}
	if env.GoExe == "" {
		out.Error = "找不到 Go 工具链（需要 " + goVersionHint() + "）：请用 --go 指定，" +
			"或把 go.exe 放进 PATH / <整合包>/tools/go/bin/"
		return out
	}
	target := filepath.Join(toolDir(), "modkit.exe")
	args := []string{"build", "-o", target, "./cmd/modkit"}
	extra := toolchainEnvDefaults(env)
	run := runEngineWithEnv(env.GoExe, env.LauncherRepo, args, extra)
	out.Run, out.Output, out.EnvAdded = &run, run.Output, extra
	if run.ExitCode != 0 || run.SpawnErr != "" {
		out.Error = fmt.Sprintf("构建失败（退出码 %d）：%s", run.ExitCode, strings.TrimSpace(firstLine(run.Output)))
		if run.SpawnErr != "" {
			out.Error = "Go 没能启动：" + run.SpawnErr
		}
		return out
	}
	if !isRegular(target) {
		out.Error = "构建命令返回成功，但产物不存在：" + target
		return out
	}
	out.OK, out.Exe = true, target
	return out
}

// toolchainEnvDefaults 补齐 Go 构建需要的环境变量（**只在用户没设时补**，
// 一律用整合包既有的目录约定，不硬编码新位置）。
func toolchainEnvDefaults(env EnvInfo) []string {
	return toolchainEnvDefaultsFor(env, os.Environ())
}

// toolchainEnvDefaultsFor 是 toolchainEnvDefaults 的纯函数形式（ambient = 现有环境变量），
// 好让用例不依赖"跑测试的这台机器上 GOPATH 有没有被设过"。
func toolchainEnvDefaultsFor(env EnvInfo, ambient []string) []string {
	have := map[string]bool{}
	for _, e := range ambient {
		if i := strings.IndexByte(e, '='); i > 0 {
			have[strings.ToUpper(e[:i])] = true
		}
	}
	var out []string
	if !have["GOPROXY"] {
		out = append(out, "GOPROXY=https://goproxy.cn,direct")
	}
	if !have["GOTOOLCHAIN"] {
		out = append(out, "GOTOOLCHAIN=local")
	}
	base := env.LauncherRoot
	if base == "" {
		base = filepath.Dir(env.ModsDir)
	}
	cands := [][2]string{
		{"GOPATH", "gopath"},
		{"GOCACHE", "gocache"},
	}
	for _, c := range cands {
		if have[c[0]] {
			continue
		}
		for _, root := range []string{filepath.Join(base, "..", "tools"), filepath.Join(base, "tools")} {
			dir := filepath.Join(root, c[1])
			if st, err := os.Stat(dir); err == nil && st.IsDir() {
				out = append(out, c[0]+"="+filepath.Clean(dir))
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// runEngineWithEnv 跑一条命令并附加环境变量（构建用；测试里不替换 engineRunner，
// 因为它与"引擎子命令"不是一回事）。
func runEngineWithEnv(exe, dir string, args, extraEnv []string) runResult {
	res := runResult{Cmd: displayCommand(exe, args)}
	cmd := exec.Command(exe, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), extraEnv...)
	hideWindow(cmd)
	out, err := cmd.CombinedOutput()
	res.Output = string(trimBOM(out))
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.ExitCode = ee.ExitCode()
		} else {
			res.ExitCode = -1
			res.SpawnErr = err.Error()
		}
	}
	return res
}

// goVersionHint 给报错文案用（说明这套代码期望的 Go）。
func goVersionHint() string {
	return "Go 1.21+（" + runtime.GOOS + "/" + runtime.GOARCH + "）"
}
