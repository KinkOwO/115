package main

// 本文件钉的是"mod 自带编辑页"这条链路上的**三件事**：
//
//  1. 发现：列表能认出哪个 mod 带 settings（前沿而不是事后才发现点了没反应）；
//  2. 契约：settings 页面能拿到自己的 key（靠真实 URL，不是 srcdoc）；
//  3. 安全闸门：**只**能写 manifest 声明过的 target，路径不许逃出 scope 根。
//
// 第 3 条比前两条重要：mod 包是外来的，这一步挡不住就是任意文件写。
// （本文件不额外依赖 modkit.exe，也不监听端口 —— httptest.NewRecorder 够用了。）

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newTestServer 造一个可控的小世界：假的 mods 目录 + 假的客户端/服务端根。
func newTestServer(t *testing.T) (*Server, string, string) {
	t.Helper()
	tmp := t.TempDir()
	modsDir := filepath.Join(tmp, "root", "mods")
	client := filepath.Join(tmp, "client")
	module := filepath.Join(tmp, "root", "server", "work", "dfo-lan")
	for _, d := range []string{modsDir, client, module} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	store, err := NewStore(modsDir)
	if err != nil {
		t.Fatal(err)
	}
	// handler 内部会调 refreshEnv()（因为它每次都要重认一遍路径），
	// 所以这里的假世界必须**真的能被既有探测逻辑认出来**：
	//   · <root>/server/work/dfo-lan/go.mod 里 module dfolan → 认出服务端模块根
	//   · <root>/server/launcher.local.json 的 client_dir → 认出客户端目录
	// 少一个都会被 refreshEnv 冲掉，测试就变成"测了个空路径"。
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module dfolan\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	local := "{\n  \"client_dir\": " + mustJSON(t, filepath.ToSlash(client)) + ",\n  \"server_binary\": \"bin/x.exe\"\n}\n"
	if err := os.WriteFile(filepath.Join(tmp, "root", "server", "launcher.local.json"), []byte(local), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := &Server{Store: store, Procs: func() []int { return nil }}
	srv.env = EnvInfo{ModsDir: modsDir, ClientDir: client, ModuleDir: module}
	return srv, modsDir, module
}

func mustJSON(t *testing.T, s string) string {
	t.Helper()
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// writeMod 在 mods 目录里造一个带 settings 的 mod。
func writeMod(t *testing.T, modsDir, id, targetScope, targetPath string) {
	t.Helper()
	root := filepath.Join(modsDir, id)
	if err := os.MkdirAll(filepath.Join(root, settingsDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(root, manifestName), `{
      "schema":2,"id":"`+id+`","version":"1.0.0","name":"测试 mod",
      "layers":{"client":{"ops":[]}}}`)
	write(t, filepath.Join(root, settingsDirName, settingsManifestName), `{
      "schema":1,"title":"测试面板","entry":"index.html",
      "targets":[{"key":"cfg","label":"配置文件","scope":"`+targetScope+`","path":"`+targetPath+`","hotReload":true}]}`)
	write(t, filepath.Join(root, settingsDirName, "index.html"), `<!doctype html><title>面板</title>`)
}

// callGET / callPOST 按 handler 的真实签名调用（本包里是 (w, r) 这个顺序）。
func callGET(h func(http.ResponseWriter, *http.Request), target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func callPOST(h func(http.ResponseWriter, *http.Request), target, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodPost, target, strings.NewReader(body)))
	return rec
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestModsListMarksSettings(t *testing.T) {
	srv, modsDir, _ := newTestServer(t)
	writeMod(t, modsDir, "alpha.withset", "mods", "alpha.withset/cfg.json")
	// 另一个不带 settings 的对照
	write(t, filepath.Join(modsDir, "beta.plain", manifestName), `{"schema":2,"id":"beta.plain","version":"1.0.0","name":"没设置"}`)

	srv.mu.Lock()
	_ = srv.env
	srv.mu.Unlock()
	rec := httptest.NewRecorder()
	srv.handleMods(rec, httptest.NewRequest(http.MethodGet, "/api/mods", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("列表接口返回 %d", rec.Code)
	}
	var out struct {
		Items []ModView `json:"items"`
		Total int       `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 {
		t.Fatalf("应当有 2 项，实际 %d", out.Total)
	}
	byID := map[string]*ModSettingsHint{}
	for i := range out.Items {
		byID[out.Items[i].ID] = out.Items[i].Settings
	}
	if byID["alpha.withset"] == nil || byID["alpha.withset"].Title != "测试面板" {
		t.Fatalf("带 settings 的 mod 没有提示：%+v", byID["alpha.withset"])
	}
	if byID["beta.plain"] != nil {
		t.Fatal("没有 settings 的 mod 不该有提示")
	}

	// 筛选：只看有设置的
	rec2 := httptest.NewRecorder()
	srv.handleMods(rec2, httptest.NewRequest(http.MethodGet, "/api/mods?hasSettings=1", nil))
	var out2 struct {
		Total int `json:"total"`
	}
	_ = json.Unmarshal(rec2.Body.Bytes(), &out2)
	if out2.Total != 1 {
		t.Fatalf("hasSettings=1 应当只剩 1 项，实际 %d", out2.Total)
	}
}

func TestSettingsPageAndTargets(t *testing.T) {
	srv, modsDir, module := newTestServer(t)
	writeMod(t, modsDir, "data.mod", "module", "configs/tuning.d/data.mod.json")
	// 先放一份内容进去，验证能被读出来
	write(t, filepath.Join(module, "configs", "tuning.d", "data.mod.json"), `{"a":1}`)

	rec := httptest.NewRecorder()
	srv.handleSettings(rec, httptest.NewRequest(http.MethodGet, "/api/settings?key=data.mod", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("settings 接口返回 %d：%s", rec.Code, rec.Body.String())
	}
	var b SettingsBundle
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if b.Manifest.Title != "测试面板" || len(b.Targets) != 1 {
		t.Fatalf("bundle 不对：%+v", b)
	}
	if !b.Targets[0].Exists || strings.TrimSpace(b.Targets[0].Content) != `{"a":1}` {
		t.Fatalf("target 内容没读出来：%+v", b.Targets[0])
	}
	want := filepath.Join(module, "configs", "tuning.d", "data.mod.json")
	if b.Targets[0].Resolved != want {
		t.Fatalf("路径解析不对：%q != %q", b.Targets[0].Resolved, want)
	}

	// 页面 itself：必须原样返回 index.html（前端 iframe 用它）
	rec2 := httptest.NewRecorder()
	srv.handleSettingsPage(rec2, httptest.NewRequest(http.MethodGet, "/api/settings/page?key=data.mod", nil))
	if rec2.Code != http.StatusOK || !strings.Contains(rec2.Body.String(), "面板") {
		t.Fatalf("页面接口不对：%d %q", rec2.Code, rec2.Body.String())
	}
	if ct := rec2.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("页面必须是 text/html，实际 %q", ct)
	}
}

func TestSettingsSaveWritesOnlyDeclaredTargets(t *testing.T) {
	srv, modsDir, module := newTestServer(t)
	writeMod(t, modsDir, "save.mod", "module", "configs/tuning.d/save.mod.json")

	body := strings.NewReader(`{"hello":"world"}`)
	rec := httptest.NewRecorder()
	srv.handleSettingsSave(rec, httptest.NewRequest(http.MethodPost,
		"/api/settings?key=save.mod&target=cfg", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("保存失败：%d %s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(module, "configs", "tuning.d", "save.mod.json"))
	if err != nil {
		t.Fatalf("文件没写进去：%v", err)
	}
	if strings.TrimSpace(string(got)) != `{"hello":"world"}` {
		t.Fatalf("内容不对：%q", string(got))
	}

	// 没声明过的 target：一律不许写
	rec2 := httptest.NewRecorder()
	srv.handleSettingsSave(rec2, httptest.NewRequest(http.MethodPost,
		"/api/settings?key=save.mod&target=nope", strings.NewReader(`{}`)))
	if rec2.Code == http.StatusOK {
		t.Fatal("未声明的 target 竟然写成功了")
	}

	// 非法 JSON：不许写（写坏了服务端会起不来）
	before, _ := os.ReadFile(filepath.Join(module, "configs", "tuning.d", "save.mod.json"))
	rec3 := httptest.NewRecorder()
	srv.handleSettingsSave(rec3, httptest.NewRequest(http.MethodPost,
		"/api/settings?key=save.mod&target=cfg", strings.NewReader(`{broken`)))
	if rec3.Code == http.StatusOK {
		t.Fatal("非法 JSON 竟然写成功了")
	}
	after, _ := os.ReadFile(filepath.Join(module, "configs", "tuning.d", "save.mod.json"))
	if string(before) != string(after) {
		t.Fatal("校验失败的写入不能改动原文件")
	}
}

// 这条是整个能力的红线：mod 包是外来的，绝不能让它指向 scope 根之外。
func TestSettingsRejectsPathEscape(t *testing.T) {
	srv, modsDir, module := newTestServer(t)

	escapes := map[string]string{
		"父目录穿越": "../outside.json",
		"多级穿越":  "../../outside.json",
		"绝对路径":  "C:/Windows/win.ini",
	}
	for name, path := range escapes {
		t.Run(name, func(t *testing.T) {
			writeMod(t, modsDir, "evil.mod", "module", path)
			rec := httptest.NewRecorder()
			srv.handleSettings(rec, httptest.NewRequest(http.MethodGet, "/api/settings?key=evil.mod", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("接口应当 200 并在 target 上报错，实际 %d", rec.Code)
			}
			var b SettingsBundle
			_ = json.Unmarshal(rec.Body.Bytes(), &b)
			if len(b.Targets) != 1 || b.Targets[0].Error == "" {
				t.Fatalf("越界路径没有被拦住：%+v", b.Targets)
			}
			// 也要保证写通道拦得住（不是只在读的时候报错）
			rec2 := httptest.NewRecorder()
			srv.handleSettingsSave(rec2, httptest.NewRequest(http.MethodPost,
				"/api/settings?key=evil.mod&target=cfg", strings.NewReader(`{}`)))
			if rec2.Code == http.StatusOK {
				t.Fatal("越界 target 竟然能写")
			}
		})
	}
	// scope 不对也不能含糊过去
	writeMod(t, modsDir, "bad.scope", "somewhere-else", "a.json")
	rec := httptest.NewRecorder()
	srv.handleSettings(rec, httptest.NewRequest(http.MethodGet, "/api/settings?key=bad.scope", nil))
	var b SettingsBundle
	_ = json.Unmarshal(rec.Body.Bytes(), &b)
	if len(b.Targets) != 1 || b.Targets[0].Error == "" {
		t.Fatalf("未知 scope 没有被拦住：%+v", b.Targets)
	}
	_ = module
}
