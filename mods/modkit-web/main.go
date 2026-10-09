// modkit-web —— modkit 的本地页面（本地小服务 + 单页 UI）。
//
// 它把一个 mods 目录当成 mod 列表来读写：分页列表 / 批量启用停用 / 批量删除 /
// 导入 zip / 导出 zip，并且**接入 modkit 引擎**做安装与卸载
// （客户端 DLL mod、带补丁的包、含 server 层的 mod 都走同一条既有入口）。
//
// 数据层（列表 / 启停 / 导入导出）是本文件与 store.go 自己实现的；
// 装/卸**不另写逻辑**，而是调用启动器仓 `internal/modkit` 的 CLI 入口
// （`modkit verify/plan/install/uninstall`），见 engine.go 的头注释。
//
// 用法：
//
//	modkit-web --mods-dir C:\Game\dof\115us\115\mods --addr 127.0.0.1:8931 --open
//
// 装/卸相关参数（都可以省略，省略时自动探测）：
//
//	--client <客户端目录>        默认读 <启动器根>/server/launcher.local.json 的 client_dir
//	--root <启动器根/模块根>     默认从 mods 目录向上找 server/work/dfo-lan
//	--modkit <modkit.exe>       默认找页面同目录 / mods 目录 / <根>/bin / PATH
//	--launcher-repo <源码根>     缺引擎时用它就地构建（默认在同级目录里探测）
//	--go <go.exe>               构建用的 Go 工具链（默认 PATH / 整合包 tools/go）
package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed ui.html
var uiHTML []byte

// Server 是把 HTTP 层与它的依赖装在一起的一个壳：路径解析结果、引擎调用、进程探测。
//
// 为什么不做成全局变量：这些依赖必须能在测试里整体替换（假目录 + 假引擎 + 假进程表），
// 否则"游戏在跑就拒绝""计划被阻断"这两条路径就只能靠人工开游戏去撞。
type Server struct {
	Store *Store
	Cfg   EngineConfig
	Run   runFunc
	Procs func() []int

	mu  sync.Mutex
	env EnvInfo
}

// Env 返回当前解析好的路径信息（每次操作前会重新查一次"游戏在不在跑"）。
func (s *Server) Env() EnvInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.env
}

// refreshEnv 重新解析路径与进程状态。
func (s *Server) refreshEnv() EnvInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.env = resolveEnv(s.Store.Dir, s.Cfg, s.Procs)
	return s.env
}

func main() {
	var (
		modsDir = flag.String("mods-dir", defaultModsDir(), "要管理的 mods 目录")
		addr    = flag.String("addr", "127.0.0.1:8931", "监听地址（默认只绑本机）")
		openIt  = flag.Bool("open", false, "启动后自动打开浏览器")
		scan    = flag.Int("scan-depth", 1, "列表认几层：1=直接子目录（已装 mod）；2=再往下一层（mod 作者工作区如 mods/examples/<mod>）")
		// 装/卸相关（都为空 = 自动探测）
		client   = flag.String("client", "", "DFO 客户端根目录（默认读 <启动器根>/server/launcher.local.json 的 client_dir）")
		root     = flag.String("root", "", "启动器根或服务端模块根（默认从 mods 目录向上找 server/work/dfo-lan）")
		modkit   = flag.String("modkit", "", "modkit 引擎可执行文件（默认找页面同目录 / mods 目录 / <根>/bin / PATH）")
		repo     = flag.String("launcher-repo", "", "启动器仓源码根（缺引擎时就地构建 ./cmd/modkit 用）")
		goExe    = flag.String("go", "", "Go 工具链（构建引擎用；默认 PATH / 整合包 tools/go）")
		noEngine = flag.Bool("no-engine", false, "禁用安装/卸载（只保留列表/导入导出/启停）")
	)
	flag.Parse()

	s, err := NewStore(*modsDir)
	if err != nil {
		log.Fatalf("打不开 mods 目录 %s：%v", *modsDir, err)
	}
	if *scan < 1 {
		*scan = 1
	}
	if *scan > 4 {
		*scan = 4
	}
	s.ScanDepth = *scan

	srv := &Server{Store: s, Run: engineRunner, Procs: clientProcessProbe}
	if !*noEngine {
		srv.Cfg = EngineConfig{
			ClientDir: *client, Root: *root, ModkitExe: *modkit,
			LauncherRepo: *repo, GoExe: *goExe,
		}
	}
	env := srv.refreshEnv()

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("监听 %s 失败：%v", *addr, err)
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Printf("modkit-web 已启动\n  mods 目录：%s\n  地址：%s\n", s.Dir, url)
	printEngineEnv(env)
	if *openIt {
		openBrowser(url)
	}
	httpSrv := &http.Server{Handler: srv.Mux(), ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(httpSrv.Serve(ln))
}

// printEngineEnv 把"装/卸要用的那几个路径从哪来"打到控制台（页面顶部也显示同一份）。
func printEngineEnv(env EnvInfo) {
	fmt.Printf("  客户端目录：%s（%s）\n", orDash(env.ClientDir), orDash(env.ClientDirWhy))
	fmt.Printf("  服务端模块：%s\n", orDash(env.ModuleDir))
	fmt.Printf("  modkit 引擎：%s（%s）\n", orDash(env.ModkitExe), orDash(env.ModkitWhy))
	if env.LauncherRepo != "" {
		fmt.Printf("  启动器源码：%s\n", env.LauncherRepo)
	}
	if env.ClientRunning {
		fmt.Println("  注意：DFO.exe 正在运行 —— 客户端侧的安装/卸载会被拒绝（请先退出游戏）")
	}
	for _, p := range env.Problems {
		fmt.Println("  提示：" + p)
	}
}

func orDash(s string) string {
	if s == "" {
		return "（未找到）"
	}
	return s
}

// defaultModsDir 优先用当前目录下的 mods/，否则用可执行文件旁边的 mods/。
func defaultModsDir() string {
	if wd, err := os.Getwd(); err == nil {
		p := filepath.Join(wd, "mods")
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			return p
		}
		return wd
	}
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "mods")
	}
	return "mods"
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// Mux 注册全部路由（main 与测试共用同一份，路由改一处两边都跟上）。
func (s *Server) Mux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/mods", s.handleMods)
	mux.HandleFunc("/api/inspect", s.handleInspect)
	mux.HandleFunc("/api/import", s.handleImport)
	mux.HandleFunc("/api/import-local", s.handleImportLocal)
	mux.HandleFunc("/api/export", s.handleExport)
	mux.HandleFunc("/api/delete", s.handleDelete)
	mux.HandleFunc("/api/enable", s.handleEnable)
	mux.HandleFunc("/api/disable", s.handleDisable)
	// 装/卸（走 modkit 引擎）
	mux.HandleFunc("/api/install", func(w http.ResponseWriter, r *http.Request) { s.handleOp(w, r, "install") })
	mux.HandleFunc("/api/uninstall", func(w http.ResponseWriter, r *http.Request) { s.handleOp(w, r, "uninstall") })
	mux.HandleFunc("/api/engine/build", s.handleEngineBuild)
	return mux
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(uiHTML)
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func fail(w http.ResponseWriter, code int, format string, args ...interface{}) {
	writeJSON(w, code, map[string]interface{}{"error": fmt.Sprintf(format, args...)})
}

func (s *Server) handleMods(w http.ResponseWriter, r *http.Request) {
	items, note, err := s.Store.List()
	if err != nil {
		fail(w, 500, "读 mods 目录失败：%v", err)
		return
	}
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q != "" {
		kept := items[:0]
		for _, m := range items {
			hay := strings.ToLower(m.ID + " " + m.Name + " " + m.Author + " " + m.Description)
			if strings.Contains(hay, q) {
				kept = append(kept, m)
			}
		}
		items = kept
	}
	page := atoiDefault(r.URL.Query().Get("page"), 1)
	size := atoiDefault(r.URL.Query().Get("size"), 20)
	if size <= 0 || size > 200 {
		size = 20
	}
	total := len(items)
	pages := (total + size - 1) / size
	if pages == 0 {
		pages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > pages {
		page = pages
	}
	from := (page - 1) * size
	to := from + size
	if from > total {
		from = total
	}
	if to > total {
		to = total
	}
	env := s.refreshEnv() // 每次刷新列表都重查一次"游戏在不在跑"，界面上的按钮才对得上现场
	writeJSON(w, 200, map[string]interface{}{
		"modsDir": s.Store.Dir,
		"note":    note,
		"total":   total,
		"page":    page,
		"size":    size,
		"pages":   pages,
		"items":   items[from:to],
		"env":     env,
	})
}

// handleOp 是安装/卸载的统一入口：先做前置检查（路径、引擎、DFO.exe），
// 再按 action 走 verify → plan → install（或 uninstall 预览 → uninstall）。
//
// 返回码：200 = 引擎跑过了（成功 / 被阻断 / 失败都在 steps 与 error 里）；
// 400 = 请求本身不对（key 不存在、id 缺失等）；409 = 前置检查拒绝（游戏在跑、找不到目录/引擎）。
func (s *Server) handleOp(w http.ResponseWriter, r *http.Request, action string) {
	if r.Method != http.MethodPost {
		fail(w, 405, "只支持 POST")
		return
	}
	var req opRequest
	if err := readJSONBody(r, &req); err != nil {
		fail(w, 400, "%v", err)
		return
	}
	key := strings.TrimSpace(req.Key)
	if key == "" {
		fail(w, 400, "没有指定要处理的项（key = 列表里的相对路径）")
		return
	}
	list, _, err := s.Store.List()
	if err != nil {
		fail(w, 500, "读 mods 目录失败：%v", err)
		return
	}
	var mod *Mod
	for i := range list {
		if list[i].DirName == key {
			mod = &list[i]
			break
		}
	}
	if mod == nil {
		fail(w, 400, "列表里没有这一项：%s", key)
		return
	}

	env := s.refreshEnv()
	res := OpResult{
		Action: action, DryRun: req.DryRun, Key: key, ID: mod.ID,
		NeedsRebuild: layerHas(mod.Layers, "server"),
	}
	if mod.ID == "" || mod.ID == "(清单缺 id)" {
		fail(w, 400, "这一项的清单里没有合法 id，装/卸都要靠 id：%s", key)
		return
	}
	if len(mod.Problems) > 0 {
		res.Notes = append(res.Notes, "这一项的清单有问题："+strings.Join(mod.Problems, "；"))
	}

	// —— 前置：目录与引擎 ——
	envStep := StepResult{Name: "环境：客户端目录 / 服务端模块根 / 引擎"}
	envStep.Detail = fmt.Sprintf("客户端 %s；服务端模块 %s；引擎 %s",
		orDash(env.ClientDir), orDash(env.ModuleDir), orDash(env.ModkitExe))
	envStep.OK = env.ClientDir != "" && env.ModkitExe != "" && env.RootFor() != ""
	res.Steps = append(res.Steps, envStep)
	if !envStep.OK {
		res.Error = strings.Join(env.Problems, "；")
		if res.Error == "" {
			res.Error = "客户端目录 / 服务端模块根 / modkit 引擎 有缺项，无法装/卸"
		}
		writeJSON(w, 409, res)
		return
	}

	// —— 前置：DFO.exe 在跑就禁止客户端侧操作（引擎里也有同一道门，这里提前挡住并给提示）——
	runStep := StepResult{Name: "检查 DFO.exe 是否在运行"}
	if env.ClientRunning {
		runStep.Detail = fmt.Sprintf("正在运行（PID %v）", env.ClientProcs)
	} else {
		runStep.Detail = "未运行"
	}
	runStep.OK = !env.ClientRunning || req.DryRun
	res.Steps = append(res.Steps, runStep)
	// 注意：StepResult 是按值进切片的，改备注要改**切片里的那一份**，否则页面上看不到。
	setRunNote := func(note string) { res.Steps[len(res.Steps)-1].Note = note }

	needClient := action == "uninstall" || layerNeedsClientWrite(mod.Layers)
	switch {
	case env.ClientRunning && !req.DryRun && needClient:
		var note string
		if action == "install" {
			note = "拒绝：该 mod 含客户端层（client/pvf/resource），请先退出游戏再安装"
		} else {
			note = "拒绝：卸载要还原客户端文件，请先退出游戏再卸载"
		}
		setRunNote(note)
		res.Error = note
		writeJSON(w, 409, res)
		return
	case env.ClientRunning && !req.DryRun:
		setRunNote("该 mod 只有 server 层：不写客户端文件，这次放行（但仍**需要重新编译服务端**）")
	case env.ClientRunning && req.DryRun:
		setRunNote("游戏在运行：这次是预演（dry-run），不写盘，放行")
	}

	// —— 干活 ——
	if action == "install" {
		s.runInstall(&res, *mod, env, list)
	} else {
		s.runUninstall(&res, *mod, env)
	}
	if res.OK {
		res.Error = ""
	}
	writeJSON(w, 200, res)
}

// handleEngineBuild 就地编出 modkit 引擎（缺 modkit.exe 时的一键入口）。
func (s *Server) handleEngineBuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "只支持 POST")
		return
	}
	env := s.refreshEnv()
	out := buildEngine(env)
	if out.OK {
		s.mu.Lock()
		s.env.ModkitExe = out.Exe
		s.env.ModkitWhy = "刚构建：" + out.Exe
		s.mu.Unlock()
		// 构建完再解析一次：客户端/服务端目录的缺项可能一并有了结果。
		s.refreshEnv()
	}
	code := 200
	if !out.OK {
		code = 500
	}
	writeJSON(w, code, out)
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// saveUpload 把上传的 zip 落到「mods 目录下的暂存目录」，返回路径与清理函数。
//
// 为什么不用 %TEMP%：业主 2026-10-06 实机反馈
//
//	`校验失败：open C:\Users\...\Temp\modkit-web-772037438.zip: Access is denied`
//
// —— 工作区内的 exe 写系统临时目录在某些环境（沙箱 / ACL / 以别的身份运行）会被拒。
// mods 目录是本工具**本来就要写**的地方，暂存在那里一定可写，也更符合"就近处理"。
func (s *Server) saveUpload(r *http.Request) (string, func(), error) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		return "", nil, fmt.Errorf("解析上传失败：%w", err)
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return "", nil, fmt.Errorf("没有收到文件字段 file：%w", err)
	}
	defer f.Close()
	if !strings.HasSuffix(strings.ToLower(hdr.Filename), ".zip") {
		return "", nil, fmt.Errorf("只支持 .zip（收到 %s）", hdr.Filename)
	}

	stageDir := filepath.Join(s.Store.Dir, stagingDirName)
	if err := os.MkdirAll(stageDir, 0o755); err != nil {
		return "", nil, fmt.Errorf("建暂存目录失败（%s）：%w", stageDir, err)
	}
	tmp, err := os.CreateTemp(stageDir, "upload-*.zip")
	if err != nil {
		return "", nil, fmt.Errorf("在 %s 下建暂存文件失败：%w", stageDir, err)
	}
	cleanup := func() {
		name := tmp.Name()
		_ = tmp.Close()
		_ = os.Remove(name)
		_ = os.Remove(stageDir) // 空了就顺手删掉；非空会失败，忽略
	}
	// 上限取消后不再截断：截断会静默产出半个 zip，后续解压必然失败且看不出原因。
	if _, err := io.Copy(tmp, f); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("接收上传内容失败：%w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return tmp.Name(), cleanup, nil
}

func (s *Server) handleInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "只支持 POST")
		return
	}
	path, cleanup, err := s.saveUpload(r)
	if err != nil {
		fail(w, 400, "%v", err)
		return
	}
	defer cleanup()
	cands, err := s.Store.Inspect(path)
	if err != nil {
		fail(w, 400, "%v", err)
		return
	}
	writeJSON(w, 200, map[string]interface{}{"candidates": cands})
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "只支持 POST")
		return
	}
	path, cleanup, err := s.saveUpload(r)
	if err != nil {
		fail(w, 400, "%v", err)
		return
	}
	defer cleanup()
	overwrite := r.FormValue("overwrite") == "1"
	dryRun := r.FormValue("dryRun") == "1"
	rep, err := s.Store.Import(path, overwrite, dryRun)
	if err != nil {
		// 校验失败也要把逐条问题带回给界面
		writeJSON(w, 400, map[string]interface{}{
			"error":      err.Error(),
			"candidates": rep.Candidates,
			"dryRun":     dryRun,
		})
		return
	}
	writeJSON(w, 200, rep)
}

// handleImportLocal 把 mods 目录里已有的 zip 包就地导入成目录。
// 请求体：{"keys":[…], "overwrite": true|false}
func (s *Server) handleImportLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "只支持 POST")
		return
	}
	var body struct {
		Keys      []string `json:"keys"`
		Overwrite bool     `json:"overwrite"`
	}
	if err := readJSONBody(r, &body); err != nil {
		fail(w, 400, "%v", err)
		return
	}
	keys := cleanKeys(body.Keys)
	if len(keys) == 0 {
		fail(w, 400, "没有选择 mod")
		return
	}
	rep, err := s.Store.ImportLocal(keys, body.Overwrite)
	if err != nil {
		writeJSON(w, 400, map[string]interface{}{
			"error": err.Error(), "imported": rep.Imported, "skipped": rep.Skipped,
			"candidates": rep.Candidates,
		})
		return
	}
	writeJSON(w, 200, rep)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	keys := splitKeys(r.URL.Query().Get("keys"))
	if len(keys) == 0 {
		fail(w, 400, "没有选择要导出的 mod")
		return
	}
	name := "mods-export"
	if len(keys) == 1 {
		name = filepath.Base(keys[0])
		name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=\"%s.zip\"", strings.ReplaceAll(name, "\"", "")))
	w.Header().Set("Cache-Control", "no-store")
	if _, err := s.Store.Export(keys, w); err != nil {
		// 头部已发出，只能记日志
		log.Printf("导出失败：%v", err)
	}
}

// splitKeys 解析逗号分隔的唯一键（相对路径）。
func splitKeys(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// readJSONBody 读一个 JSON 请求体；容忍 UTF-8 BOM（有些 Windows 脚本写出来的 JSON 带 BOM）。
func readJSONBody(r *http.Request, v interface{}) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if len(bytes.TrimSpace(raw)) == 0 {
		return fmt.Errorf("请求体是空的")
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return fmt.Errorf("请求体不是合法 JSON：%w", err)
	}
	return nil
}

// cleanKeys 去掉空白项（键是**唯一键 = 相对路径**，不是 id
// —— 同一个 mods 树里"已装目录"和"它的 zip 包"可能同 id，用路径才无歧义）。
func cleanKeys(in []string) []string {
	out := []string{}
	for _, k := range in {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// readKeysBody 读 {"keys":[…]}。
func readKeysBody(r *http.Request) ([]string, error) {
	var body struct {
		Keys []string `json:"keys"`
	}
	if err := readJSONBody(r, &body); err != nil {
		return nil, fmt.Errorf("请求体要是 {\"keys\":[…]}：%w", err)
	}
	keys := cleanKeys(body.Keys)
	if len(keys) == 0 {
		return nil, fmt.Errorf("没有选择 mod")
	}
	return keys, nil
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "只支持 POST")
		return
	}
	keys, err := readKeysBody(r)
	if err != nil {
		fail(w, 400, "%v", err)
		return
	}
	done, err := s.Store.Delete(keys)
	if err != nil {
		writeJSON(w, 400, map[string]interface{}{"error": err.Error(), "done": done})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"done": done})
}

func (s *Server) setEnabledHandler(on bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fail(w, 405, "只支持 POST")
			return
		}
		keys, err := readKeysBody(r)
		if err != nil {
			fail(w, 400, "%v", err)
			return
		}
		res, err := s.Store.SetEnabled(keys, on, "modkit-web")
		if err != nil {
			writeJSON(w, 400, map[string]interface{}{
				"error": err.Error(), "done": res.Done, "notes": res.Notes,
			})
			return
		}
		writeJSON(w, 200, res)
	}
}

func (s *Server) handleEnable(w http.ResponseWriter, r *http.Request) {
	s.setEnabledHandler(true)(w, r)
}
func (s *Server) handleDisable(w http.ResponseWriter, r *http.Request) {
	s.setEnabledHandler(false)(w, r)
}
