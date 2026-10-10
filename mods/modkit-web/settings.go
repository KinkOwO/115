package main

// 本文件实现「**有设置的 mod 可以点进它自己的编辑页**」。
//
// # 为什么要这么做（而不是在页面上硬编码三个 mod 的表单）
//
// 一开始的自然做法是给「血量 / 掉率 / 强化」各写一组控件。那样第一步很快，
// 但从此每加一个可调参数都得改这个页面 —— 页面变成所有 mod 的共同耦合点，
// mod 作者想加一个输入框还得等页面发版。
//
// 所以改成：**谁的数值谁负责**。mod 在包里带一个 `settings/` 目录，
// 里面一份 `manifest.json`（声明它要读写哪些文件）加一张 `index.html`（它自己的编辑页），
// 本页面只做三件事：发现它、把它渲染出来、替它读写那几个文件。
//
// # 发现顺序
//
//   1. mods 目录里扫到的那个 mod 目录 / zip 包（最常见：`<mods-dir>/<mod>/settings/`）
//   2. modkit 注册表里记的安装源 `<client>/.launcher-mods/modkit/registry.json` 的 `sourceRoot`
//      （有些 mod 是从 dist 里的 zip 装的，源在别处）
//   3. 服务端已落的 Go 包旁 `<服务端模块根>/mods/<id>/settings/`
//
// 三处都找不到 = 这个 mod 没有设置可选，页面上它的名字就只是个名字。
//
// # 安全边界（本地工具也要有）
//
//   - 编辑页能读写的**只有 manifest 里声明过的 target**，多一个字节都不行；
//   - target 的路径必须解析到 scope 根（客户端 / 服务端模块根 / mods 目录）**内部**，
//     出现 `..` 或绝对路径直接拒 —— mod 包是外来 код，不能让它指向任意路径；
//   - 写盘前做 JSON 校验 + 先备份再原子替换。

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// settingsDirName 是 mod 包里放的那层目录名。约定优于配置：不另开字段。
const settingsDirName = "settings"

// settingsManifestName 是 settings 目录里的声明文件名。
const settingsManifestName = "manifest.json"

// SettingsManifest 是 `settings/manifest.json` 的结构。
type SettingsManifest struct {
	Schema int `json:"schema"`
	// Title 是编辑页的标题（弹窗头上显示的那行）。
	Title string `json:"title"`
	// Subtitle 是一句说明。
	Subtitle string `json:"subtitle,omitempty"`
	// Entry 是编辑页 HTML 的文件名（相对 settings 目录），缺省用 index.html。
	Entry string `json:"entry,omitempty"`
	// Targets 声明这个编辑页要读写的全部文件。没声明的路径一律不许碰。
	Targets []SettingTarget `json:"targets"`
}

// SettingTarget 是一个"可编辑文件"的声明。
type SettingTarget struct {
	// Key 是编辑页用来指认这份配置的短名（`GET/POST /api/settings?key=<Key>`）。
	Key string `json:"key"`
	// Label 给人看的名字。
	Label string `json:"label"`
	// Scope 决定路径基准：client（客户端根）/ module（服务端模块根）/ mods（mod 目录）。
	Scope string `json:"scope"`
	// Path 是相对该基准的路径，**必须相对且不能含 ..**。
	Path string `json:"path"`
	// Note 是一句显示在表单旁边的说明（比如"100 = 1.00 倍"）。
	Note string `json:"note,omitempty"`
	// HotReload 表示改完之后多久生效（给人看的文案由前端拼）。
	HotReload bool `json:"hotReload,omitempty"`
	// Optional 表示文件不存在不算错（首次由服务端/mod 自己生成）。
	Optional bool `json:"optional,omitempty"`
	// Template 是文件不存在时可一键生成的初始内容（必须是合法 JSON）。
	Template string `json:"template,omitempty"`
}

// settingsSource 是一个"能按相对路径读文件"的来源（目录或 zip 包）。
type settingsSource struct {
	desc string
	dir  string // 目录型：settings 目录的绝对路径
	zip  string // zip 型：zip 文件路径 + 包内前缀
	pre  string
}

// read 从来源里读一个相对 settings 目录的文件。
func (src settingsSource) read(rel string) ([]byte, bool, error) {
	rel = strings.TrimPrefix(filepath.ToSlash(rel), "/")
	if strings.Contains(rel, "..") {
		return nil, false, fmt.Errorf("settings 里出现非法相对路径 %q", rel)
	}
	if src.zip != "" {
		return readZipEntry(src.zip, src.pre+"/"+rel)
	}
	p := filepath.Join(src.dir, filepath.FromSlash(rel))
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return b, true, nil
}

// readZipEntry 读 zip 里的一个成员（不存在返回 false，不是错误 —— 那是"这个来源没有 settings"）。
func readZipEntry(zipPath, name string) ([]byte, bool, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, false, err
	}
	defer r.Close()
	// zip 里的分隔符是 '/'
	name = strings.TrimPrefix(name, "/")
	for _, f := range r.File {
		if f.Name == name {
			b, err := readZipFile(f, 1<<20) // 1 MB 上限足够：编辑页 + manifest 都是小文本文件
			if err != nil {
				return nil, false, err
			}
			return b, true, nil
		}
	}
	return nil, false, nil
}

// settingsSources 按顺序列出某 mod 可能的 settings 来源。
func settingsSources(s *Server, mod Mod) []settingsSource {
	var out []settingsSource

	// 1) mods 目录里扫到的那一项
	full := filepath.Join(s.Store.Dir, filepath.FromSlash(mod.DirName))
	if fi, err := os.Stat(full); err == nil {
		if fi.IsDir() {
			out = append(out, settingsSource{desc: "mods 目录", dir: filepath.Join(full, settingsDirName)})
		} else if strings.EqualFold(filepath.Ext(full), ".zip") {
			// zip 里的成员路径形如 `<包内顶层>/settings/...`，前缀要先探出来。
			if pre := zipModPrefix(full, mod.ID); pre != "" {
				out = append(out, settingsSource{desc: "mod 包", zip: full, pre: pre})
			} else {
				out = append(out, settingsSource{desc: "mod 包", zip: full, pre: ""})
			}
		}
	}

	// 2) modkit 注册表里记的安装源（目录或 zip 都可能是）
	if root := registrySourceRoot(s, mod.ID); root != "" {
		if fi, err := os.Stat(root); err == nil {
			if fi.IsDir() {
				out = append(out, settingsSource{desc: "安装源目录", dir: filepath.Join(root, settingsDirName)})
			} else if strings.EqualFold(filepath.Ext(root), ".zip") {
				pre := zipModPrefix(root, mod.ID)
				out = append(out, settingsSource{desc: "安装源包", zip: root, pre: pre})
			}
		}
	}

	// 3) 服务端已落的 Go 包旁（server 层 mod）
	if env := s.Env(); env.ModuleDir != "" {
		out = append(out, settingsSource{
			desc: "服务端包",
			dir:  filepath.Join(env.ModuleDir, "mods", mod.ID, settingsDirName),
		})
	}
	return out
}

// zipModPrefix 猜 zip 里这个 mod 的顶层前缀：形态 B 的包是 `<包>/<mod>/mod.json`。
// 猜不出来返回空（那时按"包根即 mod 根"处理）。
func zipModPrefix(zipPath, modID string) string {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return ""
	}
	defer r.Close()
	for _, f := range r.File {
		n := filepath.ToSlash(f.Name)
		if !strings.HasSuffix(n, "/"+manifestName) {
			continue
		}
		top := strings.SplitN(n, "/", 2)[0]
		if top == modID || strings.TrimSuffix(strings.TrimPrefix(top, ""), "") == modID {
			return top + "/" + settingsDirName
		}
	}
	// 只有一个顶层目录时，形态 A（包根就是 mod 根）就用它
	tops := map[string]bool{}
	for _, f := range r.File {
		n := filepath.ToSlash(f.Name)
		if i := strings.Index(n, "/"); i > 0 {
			tops[n[:i]] = true
		}
	}
	if len(tops) == 1 {
		for t := range tops {
			return t + "/" + settingsDirName
		}
	}
	return ""
}

// registrySourceRoot 从 modkit 注册表里读这个 mod 的安装源路径。
//
// 注册表位置固定为 `<客户端>/.launcher-mods/modkit/registry.json`（既有约定，
// 由 modkit install 维护）。读不到、或这一项没有 sourceRoot 就返回空 ——
// "没有 hint"不是错误，只是少一个候选来源。
func registrySourceRoot(s *Server, modID string) string {
	env := s.Env()
	if env.ClientDir == "" {
		return ""
	}
	raw, err := os.ReadFile(filepath.Join(env.ClientDir, ".launcher-mods", "modkit", "registry.json"))
	if err != nil {
		return ""
	}
	var reg struct {
		Mods []struct {
			ID         string `json:"id"`
			SourceRoot string `json:"sourceRoot"`
		} `json:"mods"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		return ""
	}
	for _, m := range reg.Mods {
		if m.ID == modID {
			return m.SourceRoot
		}
	}
	return ""
}

// SettingsBundle 是一次"打开这个 mod 的编辑页"需要的全部东西。
type SettingsBundle struct {
	ModID    string               `json:"modId"`
	ModName  string               `json:"modName"`
	From     string               `json:"from"`
	Manifest SettingsManifest     `json:"manifest"`
	Page     string               `json:"page,omitempty"`
	Targets  []SettingsTargetView `json:"targets"`
	Problems []string             `json:"problems,omitempty"`
}

// SettingsTargetView 是一个 target 的当前状态 + 内容。
type SettingsTargetView struct {
	SettingTarget
	Resolved string `json:"resolved"` // 解析出来的绝对路径（页面上显示"改的是哪个文件"）
	Exists   bool   `json:"exists"`
	Content  string `json:"content,omitempty"`
	Error    string `json:"error,omitempty"`
}

// loadSettingsBundle 找到某个 mod 的 settings 并把它与现有文件内容一起读出来。
func loadSettingsBundle(s *Server, mod Mod) (*SettingsBundle, error) {
	var (
		manifest SettingsManifest
		page     []byte
		found    bool
		from     string
	)
	for _, src := range settingsSources(s, mod) {
		raw, ok, err := src.read(settingsManifestName)
		if err != nil {
			continue
		}
		if !ok {
			continue
		}
		if err := json.Unmarshal(raw, &manifest); err != nil {
			return nil, fmt.Errorf("%s 里的 %s 解析失败：%w", src.desc, settingsManifestName, err)
		}
		if manifest.Schema != 0 && manifest.Schema != 1 {
			return nil, fmt.Errorf("%s 的 settings schema 只能是 1（实际 %d）", src.desc, manifest.Schema)
		}
		entry := manifest.Entry
		if entry == "" {
			entry = "index.html"
		}
		if b, ok, err := src.read(entry); err == nil && ok {
			page = b
		} else if err != nil {
			return nil, fmt.Errorf("读编辑页 %s 失败：%w", entry, err)
		}
		found, from = true, src.desc
		break
	}
	if !found {
		return nil, nil // 这个 mod 没有设置 —— 不是错误
	}

	bundle := &SettingsBundle{
		ModID: mod.ID, ModName: mod.Name, From: from,
		Manifest: manifest, Page: string(page),
	}
	if bundle.Page == "" {
		bundle.Problems = append(bundle.Problems,
			fmt.Sprintf("找不到编辑页 %s（就只剩下面这几份配置文件可以看）", orEntry(manifest.Entry)))
	}
	if len(manifest.Targets) == 0 {
		bundle.Problems = append(bundle.Problems, "manifest 里没有声明任何可编辑文件")
	}
	seen := map[string]bool{}
	for _, t := range manifest.Targets {
		view := SettingsTargetView{SettingTarget: t}
		if t.Key == "" {
			view.Error = "缺 key"
		} else if seen[t.Key] {
			view.Error = "key 重复"
		} else {
			seen[t.Key] = true
			abs, err := resolveTargetPath(s, t)
			if err != nil {
				view.Error = err.Error()
			} else {
				view.Resolved = abs
				raw, err := os.ReadFile(abs)
				switch {
				case err == nil:
					view.Exists = true
					view.Content = string(raw)
				case os.IsNotExist(err):
					view.Exists = false
				default:
					view.Error = err.Error()
				}
			}
		}
		bundle.Targets = append(bundle.Targets, view)
	}
	return bundle, nil
}

func orEntry(e string) string {
	if e == "" {
		return "index.html"
	}
	return e
}

// resolveTargetPath 把一个 target 声明解析成绝对路径。
//
// 这是本文件的**安全闸门**：绝对路径、`..`、以及解析后逃出 scope 根的一律拒绝。
// mod 包可以是别人做的，这里不信它写什么。
func resolveTargetPath(s *Server, t SettingTarget) (string, error) {
	if t.Path == "" {
		return "", fmt.Errorf("target %q 没有 path", t.Key)
	}
	rel := filepath.FromSlash(t.Path)
	if filepath.IsAbs(rel) || strings.Contains(filepath.ToSlash(rel), "../") {
		return "", fmt.Errorf("target %q 的 path 必须是相对路径且不能含 ..", t.Key)
	}
	env := s.Env()
	var base string
	switch strings.ToLower(strings.TrimSpace(t.Scope)) {
	case "client":
		base = env.ClientDir
		if base == "" {
			return "", fmt.Errorf("target %q 作用域是 client，但没找到客户端目录", t.Key)
		}
	case "module", "server":
		base = env.ModuleDir
		if base == "" {
			return "", fmt.Errorf("target %q 作用域是服务端，但没找到服务端模块根", t.Key)
		}
	case "mods":
		base = s.Store.Dir
	default:
		return "", fmt.Errorf("target %q 的 scope %q 不认识（可用：client / module / mods）", t.Key, t.Scope)
	}
	abs, err := filepath.Abs(filepath.Join(base, rel))
	if err != nil {
		return "", err
	}
	rootAbs, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	if !withinRoot(rootAbs, abs) {
		return "", fmt.Errorf("target %q 的路径逃出了 %s 根目录", t.Key, t.Scope)
	}
	return abs, nil
}

// withinRoot 判断 abs 是否落在 root 内部（含 root 自身）。
func withinRoot(root, abs string) bool {
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

// writeTarget 写一个 target 的内容：先校验 JSON，再备份，最后原子替换。
func writeTarget(s *Server, mod Mod, key, content string) (string, error) {
	bundle, err := loadSettingsBundle(s, mod)
	if err != nil {
		return "", err
	}
	if bundle == nil {
		return "", fmt.Errorf("mod %s 没有提供设置", mod.ID)
	}
	target, ok := bundle.Manifest.targetByKey(key)
	if !ok {
		return "", fmt.Errorf("mod %s 没有声明名为 %q 的可编辑文件", mod.ID, key)
	}
	abs, err := resolveTargetPath(s, target)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", fmt.Errorf("内容为空，不写（要清空请写 {} 或 [] ）")
	}
	// 现在约定 target 都是 JSON：写坏一个 JSON 会让服务端起不来。
	var probe any
	if err := json.Unmarshal([]byte(trimmed), &probe); err != nil {
		return "", fmt.Errorf("不是合法 JSON，拒绝写入：%w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("建目录失败：%w", err)
	}
	// 备份：只留一份 .bak，多了没意义（这不是文档，是"刚才那一版"）
	if old, err := os.ReadFile(abs); err == nil {
		_ = os.WriteFile(abs+".bak", old, 0o644)
	}
	tmp := abs + ".tmp"
	if err := os.WriteFile(tmp, []byte(trimmed), 0o644); err != nil {
		return "", fmt.Errorf("写临时文件失败：%w", err)
	}
	if err := os.Rename(tmp, abs); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("替换失败：%w", err)
	}
	return abs, nil
}

// ModSettingsHint 是列表里"这个名字能不能点"的依据。
//
// 它刻意**只读 manifest 不读文件内容**：列表分页可能有几十项，为了显示一个
// "有设置"就去把每份配置读一遍不值得。真正的内容留到打开编辑页时再读。
type ModSettingsHint struct {
	Title   string `json:"title"`
	From    string `json:"from,omitempty"`
	Targets int    `json:"targets"`
}

// listmodWithSettings 给列表里的每一项补上 settings 提示。
func (s *Server) modsWithSettings(items []Mod) []ModView {
	out := make([]ModView, 0, len(items))
	for _, m := range items {
		out = append(out, ModView{Mod: m, Settings: s.settingsHint(m)})
	}
	return out
}

// ModView 是列表接口返回的一项（多带一个 settings 提示）。
type ModView struct {
	Mod
	Settings *ModSettingsHint `json:"settings,omitempty"`
}

// settingsHint 找出这个 mod 的 settings 摘要；没有就返回 nil。
func (s *Server) settingsHint(mod Mod) *ModSettingsHint {
	for _, src := range settingsSources(s, mod) {
		raw, ok, err := src.read(settingsManifestName)
		if err != nil || !ok {
			continue
		}
		var m SettingsManifest
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		title := m.Title
		if title == "" {
			title = "设置"
		}
		return &ModSettingsHint{Title: title, From: src.desc, Targets: len(m.Targets)}
	}
	return nil
}

// findMod 按 id 或列表相对路径（DirName）定位一项。
func (s *Server) findMod(idOrKey string) (*Mod, error) {
	list, _, err := s.Store.List()
	if err != nil {
		return nil, err
	}
	idOrKey = strings.TrimSpace(idOrKey)
	for i := range list {
		if list[i].ID == idOrKey || list[i].DirName == idOrKey {
			return &list[i], nil
		}
	}
	return nil, nil
}

// handleSettings 是编辑页的读接口：`GET /api/settings?key=<DirName 或 id>`。
//
// 返回 Editing Bundle：manifest + 页面 HTML + 每份 target 的当前内容。
// 这个 mod 没有设置时返回 `{"none":true}` —— 前端据此显示"这个 mod 没有可编辑项"。
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		fail(w, 400, "没有指定 mod（key = 列表里的 id 或相对路径）")
		return
	}
	mod, err := s.findMod(key)
	if err != nil {
		fail(w, 500, "读 mods 目录失败：%v", err)
		return
	}
	if mod == nil {
		fail(w, 404, "列表里没有这一项：%s", key)
		return
	}
	s.refreshEnv() // target 路径依赖客户端/服务端根，每次重新认一遍
	bundle, err := loadSettingsBundle(s, *mod)
	if err != nil {
		fail(w, 500, "%v", err)
		return
	}
	if bundle == nil {
		writeJSON(w, 200, map[string]any{"none": true, "modId": mod.ID, "modName": mod.Name})
		return
	}
	writeJSON(w, 200, bundle)
}

// handleSettingsPage 返回编辑页本身的 HTML：`GET /api/settings/page?key=…`。
//
// 为什么要单独一个 URL 而不是让前端用 `iframe.srcdoc` 塞进去：
// srcdoc 里的 iframe 的 `location.search` 是空的，编辑页读不到"我是哪个 mod"，
// 于是它没法自己去拉数据。给它一个真实 URL 之后，`?key=` 就在那儿，
// 编辑页可以 `fetch("/api/settings?key=…")` —— 同源，不需要任何跨文档魔法。
func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		fail(w, 400, "没有指定 mod")
		return
	}
	mod, err := s.findMod(key)
	if err != nil {
		fail(w, 500, "读 mods 目录失败：%v", err)
		return
	}
	if mod == nil {
		fail(w, 404, "列表里没有这一项：%s", key)
		return
	}
	s.refreshEnv()
	bundle, err := loadSettingsBundle(s, *mod)
	if err != nil {
		fail(w, 500, "%v", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if bundle == nil || strings.TrimSpace(bundle.Page) == "" {
		_, _ = w.Write([]byte(noSettingsPageHTML(mod.Name)))
		return
	}
	_, _ = w.Write([]byte(bundle.Page))
}

// noSettingsPageHTML 是"这个 mod 有 manifest 但没有编辑页"时的占位 scaffolding。
func noSettingsPageHTML(modName string) string {
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>没有编辑页</title>` +
		`<style>body{margin:0;padding:28px;text-align:center;color:#6b7280;` +
		`font:13.5px/1.7 "Segoe UI",system-ui,"Microsoft YaHei",sans-serif}</style></head><body>` +
		`<div>` + modName + ` 声明了可编辑文件，但没找到它的编辑页（index.html）。` +
		`<br>下面这几份配置仍然可以在宿主页面上看到路径。</div></body></html>`
}

// handleSettingsSave 是编辑页的写接口：`POST /api/settings?key=…&target=<Key>`。
// body 是配置文件的**原文**（text/plain 或 application/json 都收）。
//
// 安全：只认 manifest 里声明过的 target，路径必须落在对应 scope 根内，
// 内容必须是合法 JSON，写前留一份 .bak、写用临时文件原子替换。
func (s *Server) handleSettingsSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		fail(w, 405, "只支持 POST")
		return
	}
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	target := strings.TrimSpace(r.URL.Query().Get("target"))
	if key == "" {
		fail(w, 400, "没有指定 mod")
		return
	}
	if target == "" {
		fail(w, 400, "没有指定要保存哪一份配置（target = manifest 里的 key）")
		return
	}
	mod, err := s.findMod(key)
	if err != nil {
		fail(w, 500, "读 mods 目录失败：%v", err)
		return
	}
	if mod == nil {
		fail(w, 404, "列表里没有这一项：%s", key)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		fail(w, 400, "读请求正文失败：%v", err)
		return
	}
	s.refreshEnv()
	abs, err := writeTarget(s, *mod, target, string(body))
	if err != nil {
		fail(w, 400, "%v", err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "saved": abs, "key": target})
}

// targetByKey 在 manifest 里找一个 target。
func (m SettingsManifest) targetByKey(key string) (SettingTarget, bool) {
	for _, t := range m.Targets {
		if t.Key == key {
			return t, true
		}
	}
	return SettingTarget{}, false
}
