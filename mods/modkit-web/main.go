// modkit-web —— 一个简易的 modkit 页面（本地小服务 + 单页 UI）。
//
// 它**不接启动器、不依赖启动器代码**：只把一个 mods 目录当成 mod 列表来读写，
// 提供「分页列表 / 批量启用停用 / 批量删除 / 导入 zip / 导出 zip」。
// 以后接启动器时，按 README 的 API 契约对接即可（数据层可整体替换成引擎实现）。
//
// 用法：
//
//	modkit-web --mods-dir C:\Game\dof\115us\115\mods --addr 127.0.0.1:8931 --open
package main

import (
	_ "embed"
	"bytes"
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
	"time"
)

//go:embed ui.html
var uiHTML []byte

var store *Store

func main() {
	var (
		modsDir = flag.String("mods-dir", defaultModsDir(), "要管理的 mods 目录")
		addr    = flag.String("addr", "127.0.0.1:8931", "监听地址（默认只绑本机）")
		openIt  = flag.Bool("open", false, "启动后自动打开浏览器")
		scan    = flag.Int("scan-depth", 1, "列表认几层：1=直接子目录（已装 mod）；2=再往下一层（mod 作者工作区如 mods/examples/<mod>）")
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
	store = s

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/mods", handleMods)
	mux.HandleFunc("/api/inspect", handleInspect)
	mux.HandleFunc("/api/import", handleImport)
	mux.HandleFunc("/api/import-local", handleImportLocal)
	mux.HandleFunc("/api/export", handleExport)
	mux.HandleFunc("/api/delete", handleDelete)
	mux.HandleFunc("/api/enable", handleEnable)
	mux.HandleFunc("/api/disable", handleDisable)

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("监听 %s 失败：%v", *addr, err)
	}
	url := "http://" + ln.Addr().String() + "/"
	fmt.Printf("modkit-web 已启动\n  mods 目录：%s\n  地址：%s\n", store.Dir, url)
	if *openIt {
		openBrowser(url)
	}
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	log.Fatal(srv.Serve(ln))
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

func handleIndex(w http.ResponseWriter, r *http.Request) {
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

func handleMods(w http.ResponseWriter, r *http.Request) {
	items, note, err := store.List()
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
	writeJSON(w, 200, map[string]interface{}{
		"modsDir": store.Dir,
		"note":    note,
		"total":   total,
		"page":    page,
		"size":    size,
		"pages":   pages,
		"items":   items[from:to],
	})
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
//   `校验失败：open C:\Users\...\Temp\modkit-web-772037438.zip: Access is denied`
// —— 工作区内的 exe 写系统临时目录在某些环境（沙箱 / ACL / 以别的身份运行）会被拒。
// mods 目录是本工具**本来就要写**的地方，暂存在那里一定可写，也更符合"就近处理"。
func saveUpload(r *http.Request) (string, func(), error) {
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		return "", nil, fmt.Errorf("解析上传失败（zip 上限 512MB）：%w", err)
	}
	f, hdr, err := r.FormFile("file")
	if err != nil {
		return "", nil, fmt.Errorf("没有收到文件字段 file：%w", err)
	}
	defer f.Close()
	if hdr.Size > maxZipBytes {
		return "", nil, fmt.Errorf("zip 太大（%d > %d 字节）", hdr.Size, int64(maxZipBytes))
	}
	if !strings.HasSuffix(strings.ToLower(hdr.Filename), ".zip") {
		return "", nil, fmt.Errorf("只支持 .zip（收到 %s）", hdr.Filename)
	}

	stageDir := filepath.Join(store.Dir, stagingDirName)
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
	if _, err := io.Copy(tmp, io.LimitReader(f, maxZipBytes)); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("接收上传内容失败：%w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return tmp.Name(), cleanup, nil
}

func handleInspect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "只支持 POST")
		return
	}
	path, cleanup, err := saveUpload(r)
	if err != nil {
		fail(w, 400, "%v", err)
		return
	}
	defer cleanup()
	cands, err := store.Inspect(path)
	if err != nil {
		fail(w, 400, "%v", err)
		return
	}
	writeJSON(w, 200, map[string]interface{}{"candidates": cands})
}

func handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "只支持 POST")
		return
	}
	path, cleanup, err := saveUpload(r)
	if err != nil {
		fail(w, 400, "%v", err)
		return
	}
	defer cleanup()
	overwrite := r.FormValue("overwrite") == "1"
	dryRun := r.FormValue("dryRun") == "1"
	rep, err := store.Import(path, overwrite, dryRun)
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
func handleImportLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "只支持 POST")
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		fail(w, 400, "%v", err)
		return
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	var body struct {
		Keys      []string `json:"keys"`
		Overwrite bool     `json:"overwrite"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		fail(w, 400, "请求体要是 {\"keys\":[…]}:%v", err)
		return
	}
	keys := []string{}
	for _, k := range body.Keys {
		if k = strings.TrimSpace(k); k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		fail(w, 400, "没有选择 mod")
		return
	}
	rep, err := store.ImportLocal(keys, body.Overwrite)
	if err != nil {
		writeJSON(w, 400, map[string]interface{}{
			"error": err.Error(), "imported": rep.Imported, "skipped": rep.Skipped,
			"candidates": rep.Candidates,
		})
		return
	}
	writeJSON(w, 200, rep)
}

func handleExport(w http.ResponseWriter, r *http.Request) {
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
	if _, err := store.Export(keys, w); err != nil {
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

// readKeysBody 读 {"keys":[…]}；键是**唯一键 = 相对路径**（目录或 .zip），不是 id
// —— 同一个 mods 树里"已装目录"和"它的 zip 包"可能同 id，用路径才无歧义。
func readKeysBody(r *http.Request) ([]string, error) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	// 容忍带 UTF-8 BOM 的请求体（有些 Windows 脚本写出来的 JSON 会带 BOM）
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	var body struct {
		Keys []string `json:"keys"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("请求体要是 {\"keys\":[…]}：%w", err)
	}
	keys := []string{}
	for _, k := range body.Keys {
		k = strings.TrimSpace(k)
		if k != "" {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("没有选择 mod")
	}
	return keys, nil
}

func handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "只支持 POST")
		return
	}
	keys, err := readKeysBody(r)
	if err != nil {
		fail(w, 400, "%v", err)
		return
	}
	done, err := store.Delete(keys)
	if err != nil {
		writeJSON(w, 400, map[string]interface{}{"error": err.Error(), "done": done})
		return
	}
	writeJSON(w, 200, map[string]interface{}{"done": done})
}

func setEnabledHandler(on bool) http.HandlerFunc {
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
		res, err := store.SetEnabled(keys, on, "modkit-web")
		if err != nil {
			writeJSON(w, 400, map[string]interface{}{
				"error": err.Error(), "done": res.Done, "notes": res.Notes,
			})
			return
		}
		writeJSON(w, 200, res)
	}
}

func handleEnable(w http.ResponseWriter, r *http.Request)  { setEnabledHandler(true)(w, r) }
func handleDisable(w http.ResponseWriter, r *http.Request) { setEnabledHandler(false)(w, r) }
