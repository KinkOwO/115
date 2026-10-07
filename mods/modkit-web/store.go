// Package main —— modkit 简易 Web 页面的数据层。
//
// 它只干一件事：把 <mods 目录> 当成 mod 列表来读写，全部规则对齐
// mods/MOD-DEVELOPMENT.md（清单模板 / 权限位 / 层动作）与
// mods/MOD-MANAGER-INTEGRATION.md（enabled.json 的确切格式）。
//
// 注意：**权威校验器在 modkit 引擎里**（internal/modkit 的 Manifest2.Validate）。
// 这里是按文档实现的同构子集，用于"导入前先校验、给出人话错误"。以后把这个页面接进
// 启动器时，把 validateManifest 换成引擎的 LoadManifest2 + Validate 即可（见 README）。
package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	manifestName    = "mod.json"
	stagingDirName  = ".modkit-web-staging" // 上传暂存目录（不会被当成 mod）
	enabledName     = "enabled.json"
	enabledSchema   = 1
	schema2         = 2
	maxZipBytes     = 512 << 20  // 单个 zip 上限
	maxEntries      = 20000      // 条目数上限
	maxExtractBytes = 2 << 30    // 解包总字节上限（防 zip 炸弹）
	maxModBytes     = 1 << 30    // 单个 mod 上限
	maxModsPerZip   = 100        // 一次导入的 mod 数上限
)

var (
	idRe     = regexp.MustCompile(`^[a-z0-9.\-]{1,64}$`)
	sha256Re = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
)

// ---------------------------------------------------------------------------
// 清单
// ---------------------------------------------------------------------------

// Op 是 client / pvf / resource 层里的一条动作。
type Op struct {
	Kind    string `json:"kind"`
	Target  string `json:"target"`
	Source  string `json:"source"`
	Script  string `json:"script"`
	Work    string `json:"work"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	Entries []struct {
		Archive string `json:"archive"`
		Entry   string `json:"entry"`
		Source  string `json:"source"`
	} `json:"entries"`
}

// Layer 是四层里的一层。
type Layer struct {
	Package string `json:"package"`
	Hooks   []struct {
		Name  string `json:"name"`
		Since string `json:"since"`
		Note  string `json:"note"`
	} `json:"hooks"`
	Ops []Op `json:"ops"`
}

// Manifest 是 mod.json（schema 2）。
type Manifest struct {
	Schema      int              `json:"schema"`
	ID          string           `json:"id"`
	Version     string           `json:"version"`
	Name        string           `json:"name"`
	Author      string           `json:"author"`
	Description string           `json:"description"`
	Permissions []string         `json:"permissions"`
	Requires    []string         `json:"requires"`
	Layers      map[string]Layer `json:"layers"`
}

var knownLayers = []string{"server", "pvf", "client", "resource"}

// actionCount 报告这一层声明了几条动作（server 层按 hooks 算）。
func (m *Manifest) actionCount(layer string) int {
	l, ok := m.Layers[layer]
	if !ok {
		return 0
	}
	if layer == "server" {
		return len(l.Hooks)
	}
	return len(l.Ops)
}

func (m *Manifest) layerNames() []string {
	out := []string{}
	for _, name := range knownLayers {
		if _, ok := m.Layers[name]; ok {
			out = append(out, name)
		}
	}
	// 未知层也要暴露出来（校验会报错，列表里能看到）
	for name := range m.Layers {
		known := false
		for _, k := range knownLayers {
			if k == name {
				known = true
			}
		}
		if !known {
			out = append(out, name)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// 校验（对齐 mods/MOD-DEVELOPMENT.md §2 / §3 / §11）
// ---------------------------------------------------------------------------

// safeRel 判断清单里写的相对路径是否安全（不许绝对路径、不许 .. 上跳）。
func safeRel(p string) bool {
	if p == "" {
		return false
	}
	if strings.Contains(p, "\\") {
		p = strings.ReplaceAll(p, "\\", "/")
	}
	if strings.HasPrefix(p, "/") || strings.Contains(p, ":") {
		return false
	}
	clean := path.Clean(p)
	if clean == "." || strings.HasPrefix(clean, "../") || clean == ".." {
		return false
	}
	return true
}

// validateManifest 按文档校验一份清单；files 是该 mod 根下的相对路径集合（斜杠分隔）。
// 返回人话问题列表（空 = 通过）。
func validateManifest(m *Manifest, files map[string]bool) []string {
	var bad []string
	add := func(format string, args ...interface{}) { bad = append(bad, fmt.Sprintf(format, args...)) }

	if m.Schema != schema2 {
		if m.Schema == 1 {
			add("清单是旧格式 schema=1：请先用 `modkit migrate` 转成 schema 2 再导入")
		} else {
			add("schema 必须是 %d，实际是 %d", schema2, m.Schema)
		}
	}
	if !idRe.MatchString(m.ID) {
		add("id 非法（只允许小写字母/数字/./-，≤64 字符）：%q", m.ID)
	}
	if strings.TrimSpace(m.Name) == "" {
		add("缺少 name")
	}
	if strings.TrimSpace(m.Version) == "" {
		add("缺少 version")
	}
	for _, dep := range m.Requires {
		if !idRe.MatchString(dep) {
			add("requires 含非法 id：%q", dep)
		}
		if dep == m.ID {
			add("requires 不能依赖自己：%q", dep)
		}
	}

	if len(m.Layers) == 0 {
		add("没有声明任何层（可用层：server / pvf / client / resource）")
	}
	for name, layer := range m.Layers {
		known := false
		for _, k := range knownLayers {
			if k == name {
				known = true
			}
		}
		if !known {
			add("未知层 %q（可用层：server / pvf / client / resource）", name)
			continue
		}
		if m.actionCount(name) == 0 {
			add("%s 层声明了但没有任何动作（server 层要写 hooks，其它层要写 ops）", name)
		}
		if name == "server" {
			if !files["server/mod.go"] && !hasPrefixFile(files, "server/") {
				add("声明了 server 层，但包里没有 server/ 目录")
			}
		}
		for i, op := range layer.Ops {
			where := fmt.Sprintf("%s 层第 %d 条动作", name, i+1)
			if strings.TrimSpace(op.Kind) == "" {
				add("%s 缺少 kind", where)
			}
			for _, p := range []string{op.Source, op.Script, op.Target, op.Work} {
				if p != "" && (strings.Contains(p, "/") || strings.Contains(p, "\\")) && !strings.HasPrefix(p, "{") {
					if !safeRel(p) && (op.Source == p || op.Script == p) {
						add("%s 的路径不安全（不许绝对路径或 ..）：%q", where, p)
					}
				}
			}
			if op.Source != "" && !strings.HasPrefix(op.Source, "{") {
				if !safeRel(op.Source) {
					add("%s 的 source 路径不安全：%q", where, op.Source)
				} else if !files[op.Source] {
					add("%s 引用的文件不在包里：%s", where, op.Source)
				}
			}
			if op.Script != "" && !strings.HasPrefix(op.Script, "{") {
				if !safeRel(op.Script) {
					add("%s 的 script 路径不安全：%q", where, op.Script)
				} else if !files[op.Script] {
					add("%s 引用的脚本不在包里：%s", where, op.Script)
				}
			}
			for _, e := range op.Entries {
				if e.Source == "" {
					add("%s 的 npk.entries 有一条没有 source", where)
					continue
				}
				if !safeRel(e.Source) || !files[e.Source] {
					add("%s 引用的条目文件不在包里：%s", where, e.Source)
				}
			}
		}
	}

	// 权限位（§3 表：漏写等于隐瞒，引擎直接拒绝）
	has := func(p string) bool {
		for _, x := range m.Permissions {
			if x == p {
				return true
			}
		}
		return false
	}
	need := map[string]string{} // 权限 -> 为什么
	for name, layer := range m.Layers {
		switch name {
		case "server":
			if len(layer.Hooks) > 0 && !has("server.hook") {
				need["server.hook"] = "声明了 server 层"
			}
		case "client":
			for _, op := range layer.Ops {
				switch op.Kind {
				case "file.add", "file.replace":
					if !has("client.file.write") {
						need["client.file.write"] = "client 层有 file.add / file.replace"
					}
				case "exe.patch":
					if !has("client.exe.patch") {
						need["client.exe.patch"] = "client 层有 exe.patch"
					}
				}
			}
		case "pvf":
			if len(layer.Ops) > 0 {
				if !has("pvf.merge") {
					need["pvf.merge"] = "声明了 pvf 层"
				}
				if !has("exec.script") {
					need["exec.script"] = "pvf 层要跑 .ps1"
				}
			}
		case "resource":
			for _, op := range layer.Ops {
				switch op.Kind {
				case "npk.add", "npk.replace":
					if !has("resource.npk") {
						need["resource.npk"] = "resource 层有整份 NPK 操作"
					}
				case "npk.entries", "npk.index":
					if !has("resource.index") {
						need["resource.index"] = "resource 层有条目级覆盖 / 重建索引"
					}
				}
			}
		}
	}
	for perm, why := range need {
		add("缺少权限位 %s（理由：%s）", perm, why)
	}

	sort.Strings(bad)
	return bad
}

func hasPrefixFile(files map[string]bool, prefix string) bool {
	for f := range files {
		if strings.HasPrefix(f, prefix) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// 列表 / 启用状态
// ---------------------------------------------------------------------------

// 列表项的两种来源：已装/作者的 mod 目录，或一个 zip 包（包里含 mod.json）。
const (
	KindInstalled = "installed"
	KindPackage   = "package"
)

// Mod 是列表里的一项。
type Mod struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Author      string   `json:"author"`
	Description string   `json:"description"`
	Layers      []string `json:"layers"`
	Enabled     bool     `json:"enabled"`
	AddedAt     string   `json:"addedAt"` // 目录/文件 mtime（导入/落位时间），RFC3339
	DirName     string   `json:"dirName"` // mods/ 下的相对路径（目录或 .zip）
	Kind        string   `json:"kind"`    // installed | package
	SizeBytes   int64    `json:"sizeBytes"`
	Problems    []string `json:"problems,omitempty"`
}

// EnabledFile 是 enabled.json（**禁用名单**，新装的默认启用）。
type EnabledFile struct {
	Schema    int      `json:"schema"`
	Disabled  []string `json:"disabled"`
	UpdatedAt string   `json:"updatedAt,omitempty"`
	UpdatedBy string   `json:"updatedBy,omitempty"`
}

// Store 是 mods 目录的读写入口。
//
// ScanDepth 决定"列表里认几层"：
//  1 = 只认直接子目录（已装 mod 的目录形态：`<mods>/<mod>/mod.json`，默认）
//  2 = 再往下认一层（mod 作者工作区形态：`<mods>/examples/<mod>/mod.json`）
type Store struct {
	Dir       string
	ScanDepth int
}

// NewStore 绑定 mods 目录（不存在就建）。
func NewStore(dir string) (*Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	return &Store{Dir: abs, ScanDepth: 1}, nil
}

// scanModDirs 找出 mods 目录下所有**含 mod.json 的目录**（相对路径，斜杠分隔）。
// 一旦某个目录自己有 mod.json，就不再往里钻（避免把 mod 内部的样例当独立 mod）。
func (s *Store) scanModDirs() ([]string, error) {
	depth := s.ScanDepth
	if depth < 1 {
		depth = 1
	}
	var out []string
	var walk func(rel string, left int) error
	walk = func(rel string, left int) error {
		dir := filepath.Join(s.Dir, filepath.FromSlash(rel))
		ents, err := os.ReadDir(dir)
		if err != nil {
			return nil // 读不了就跳过，不影响其它 mod
		}
		if _, err := os.Stat(filepath.Join(dir, manifestName)); err == nil && rel != "" {
			out = append(out, rel)
			return nil // 已是 mod 根，不再下钻
		}
		if left <= 0 {
			return nil
		}
		for _, e := range ents {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue // 点目录（含上传暂存目录）不算 mod
			}
			child := e.Name()
			if rel != "" {
				child = rel + "/" + e.Name()
			}
			if err := walk(child, left-1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk("", depth); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// scanPackages 找出 mods 目录下的 zip 包（相对路径）。
func (s *Store) scanPackages() ([]string, error) {
	depth := s.ScanDepth
	if depth < 1 {
		depth = 1
	}
	var out []string
	var walk func(rel string, left int) error
	walk = func(rel string, left int) error {
		dir := filepath.Join(s.Dir, filepath.FromSlash(rel))
		ents, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		// 目录自己就是一个 mod 根 → 里面的 zip 不再当包算（那是 mod 自带的产物）
		if rel != "" {
			if _, err := os.Stat(filepath.Join(dir, manifestName)); err == nil {
				return nil
			}
		}
		for _, e := range ents {
			if strings.HasPrefix(e.Name(), ".") {
				continue // 点目录/点文件（含上传暂存）不进列表
			}
			child := e.Name()
			if rel != "" {
				child = rel + "/" + e.Name()
			}
			if e.IsDir() {
				if left > 0 {
					if err := walk(child, left-1); err != nil {
						return err
					}
				}
				continue
			}
			if strings.HasSuffix(strings.ToLower(e.Name()), ".zip") {
				out = append(out, child)
			}
		}
		return nil
	}
	if err := walk("", depth); err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func (s *Store) enabledPath() string { return filepath.Join(s.Dir, enabledName) }

// loadEnabled 读 enabled.json；缺失/损坏都按"全部启用"处理（与服务端语义一致）。
func (s *Store) loadEnabled() (EnabledFile, string) {
	st := EnabledFile{Schema: enabledSchema}
	b, err := os.ReadFile(s.enabledPath())
	if errors.Is(err, os.ErrNotExist) {
		return st, "没有 " + enabledName + "（按全部启用处理）"
	}
	if err != nil {
		return st, "读 " + enabledName + " 失败（按全部启用处理）：" + err.Error()
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return EnabledFile{Schema: enabledSchema}, enabledName + " 解析失败（按全部启用处理）：" + err.Error()
	}
	if st.Disabled == nil {
		st.Disabled = []string{}
	}
	return st, ""
}

func (s *Store) saveEnabled(st EnabledFile, by string) error {
	st.Schema = enabledSchema
	if st.Disabled == nil {
		st.Disabled = []string{}
	}
	sort.Strings(st.Disabled)
	st.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	st.UpdatedBy = by
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := s.enabledPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.enabledPath())
}

// List 扫描 mods 目录（只认含 mod.json 的一级子目录）并按 id 排序。
func (s *Store) List() ([]Mod, string, error) {
	st, note := s.loadEnabled()
	disabled := map[string]bool{}
	for _, id := range st.Disabled {
		disabled[id] = true
	}

	ents, err := s.scanModDirs()
	if err != nil {
		return nil, note, err
	}
	out := []Mod{}
	for _, rel := range ents {
		dir := filepath.Join(s.Dir, filepath.FromSlash(rel))
		m, problems, err := loadManifestInDir(dir)
		if err != nil {
			// 没有 mod.json 的目录不算 mod（可能是别人的东西），跳过
			continue
		}
		item := Mod{
			ID:          m.ID,
			Name:        m.Name,
			Version:     m.Version,
			Author:      m.Author,
			Description: m.Description,
			Layers:      m.layerNames(),
			DirName:     rel,
			Kind:        KindInstalled,
			Problems:    problems,
		}
		if item.ID == "" {
			item.ID = "(清单缺 id)"
		}
		item.Enabled = !disabled[m.ID]
		if fi, err := os.Stat(dir); err == nil {
			item.AddedAt = fi.ModTime().UTC().Format(time.RFC3339)
		}
		item.SizeBytes = dirSize(dir)
		out = append(out, item)
	}

	// zip 包：也算列表项（可导出、可删除），但启停只对已装目录有意义
	pkgs, err := s.scanPackages()
	if err != nil {
		return nil, note, err
	}
	for _, rel := range pkgs {
		full := filepath.Join(s.Dir, filepath.FromSlash(rel))
		cands, err := inspectZip(full)
		if err != nil || len(cands) == 0 {
			continue // 不是 mod 包（比如普通压缩包），不进列表
		}
		c := cands[0]
		m := c.Manifest
		item := Mod{
			ID:          m.ID,
			Name:        m.Name,
			Version:     m.Version,
			Author:      m.Author,
			Description: m.Description,
			Layers:      m.layerNames(),
			DirName:     rel,
			Kind:        KindPackage,
			Enabled:     true, // 包不是"已安装"，没有停用状态
			Problems:    c.Problems,
		}
		if item.ID == "" {
			item.ID = "(清单缺 id)"
		}
		if len(cands) > 1 {
			item.Problems = append([]string{
				fmt.Sprintf("这个包里含 %d 个 mod（形态 B），列表只显示第一个（%s）", len(cands), c.DirName),
			}, item.Problems...)
		}
		if fi, err := os.Stat(full); err == nil {
			item.AddedAt = fi.ModTime().UTC().Format(time.RFC3339)
			item.SizeBytes = fi.Size()
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].DirName < out[j].DirName
	})
	return out, note, nil
}

// loadManifestInDir 读一个 mod 目录的清单并做校验；没有 mod.json 时返回错误。
func loadManifestInDir(dir string) (*Manifest, []string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return nil, nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return &Manifest{}, []string{"mod.json 不是合法 JSON：" + err.Error()}, nil
	}
	files := map[string]bool{}
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err == nil {
			files[filepath.ToSlash(rel)] = true
		}
		return nil
	})
	return &m, validateManifest(&m, files), nil
}

// SetEnabledResult 是一次批量启停的结果。
type SetEnabledResult struct {
	Done  []string `json:"done"`
	Notes []string `json:"notes,omitempty"` // 例如"zip 包已导入成目录"、"包没有启停状态，跳过"
}

// SetEnabled 批量启用/停用（写 enabled.json 的禁用名单）。
//
// 入参是**唯一键 = 相对路径**（Mod.DirName，目录或 .zip），不是 id：
// 同一个 mods 树里"已装目录"和"它的 zip 包"可能同 id，用路径才无歧义。
//
// 对 **zip 包**的语义（业主 2026-10-06 反馈"点启用不生效"后定）：
//   - 启用 = **就地导入成目录**（有同 id 的已装目录就直接启用那个目录），
//     这样"点启用就生效"，而不是回一句"包不能启停"；
//   - 停用 = 明确跳过并写进 Notes 说明（包本来就没有运行状态）。
// 停用已装目录时做依赖检查：被别的**启用中**的 mod 依赖则整批拒绝。
func (s *Store) SetEnabled(keys []string, on bool, by string) (SetEnabledResult, error) {
	res := SetEnabledResult{}
	list, _, err := s.List()
	if err != nil {
		return res, err
	}
	byKey := map[string]Mod{}
	installedByID := map[string]Mod{}
	for _, m := range list {
		byKey[m.DirName] = m
		if m.Kind == KindInstalled && m.ID != "" {
			installedByID[m.ID] = m
		}
	}

	// 先把钥匙解析成"已装目录"的 keys；包按上面的语义就地处理
	dirKeys := make([]string, 0, len(keys))
	for _, k := range keys {
		m, ok := byKey[k]
		if !ok {
			return res, fmt.Errorf("列表里没有这一项：%s", k)
		}
		if m.Kind == KindInstalled {
			dirKeys = append(dirKeys, k)
			continue
		}
		if !on {
			res.Notes = append(res.Notes, k+"：zip 包没有启停状态，已跳过（要生效先导入）")
			continue
		}
		if inst, ok := installedByID[m.ID]; ok {
			dirKeys = append(dirKeys, inst.DirName)
			res.Notes = append(res.Notes, k+"：已有同 id 的已装目录 "+inst.DirName+"，改为启用它")
			continue
		}
		full := filepath.Join(s.Dir, filepath.FromSlash(k))
		one, ierr := s.Import(full, false, false)
		if ierr != nil {
			return res, fmt.Errorf("导入 %s 失败：%w", k, ierr)
		}
		if len(one.Imported) > 0 {
			res.Notes = append(res.Notes, k+"：已导入成目录 "+strings.Join(one.Imported, ", "))
			for _, d := range one.Imported {
				dirKeys = append(dirKeys, d)
			}
			// 导入后重新读一次，拿到新目录的 key
			if l2, _, e2 := s.List(); e2 == nil {
				byKey = map[string]Mod{}
				for _, m2 := range l2 {
					byKey[m2.DirName] = m2
				}
			}
			continue
		}
		res.Notes = append(res.Notes, k+"："+strings.Join(one.Skipped, "；"))
		for _, d := range one.Skipped {
			res.Notes = append(res.Notes, "（"+d+"）")
		}
	}

	byKey = map[string]Mod{}
	if l2, _, e2 := s.List(); e2 == nil {
		for _, m := range l2 {
			byKey[m.DirName] = m
		}
	}
	ids := make([]string, 0, len(dirKeys))
	for _, k := range dirKeys {
		m, ok := byKey[k]
		if !ok || m.Kind != KindInstalled {
			res.Notes = append(res.Notes, k+"：找不到已装目录，已跳过")
			continue
		}
		ids = append(ids, m.ID)
	}
	if len(ids) == 0 {
		return res, nil
	}

	// 停用时做依赖检查
	if !on {
		willDisable := map[string]bool{}
		for _, id := range ids {
			willDisable[id] = true
		}
		for _, m := range byKey {
			if m.Kind != KindInstalled || willDisable[m.ID] || !m.Enabled {
				continue
			}
			for _, d := range m.requiresOf(s.Dir) {
				if willDisable[d] {
					return res, fmt.Errorf("不能停用：%s 正被启用中的 %s 依赖", d, m.ID)
				}
			}
		}
	}

	st, _ := s.loadEnabled()
	set := map[string]bool{}
	for _, id := range st.Disabled {
		set[id] = true
	}
	for _, id := range ids {
		if on {
			delete(set, id)
		} else {
			set[id] = true
		}
	}
	st.Disabled = st.Disabled[:0]
	for id := range set {
		st.Disabled = append(st.Disabled, id)
	}
	if err := s.saveEnabled(st, by); err != nil {
		return res, err
	}
	res.Done = ids
	return res, nil
}

// requiresOf 读某 mod 的 requires（列表项里没带，按需再读一次清单）。
func (m Mod) requiresOf(modsDir string) []string {
	raw, err := os.ReadFile(filepath.Join(modsDir, filepath.FromSlash(m.DirName), manifestName))
	if err != nil {
		return nil
	}
	var man Manifest
	if json.Unmarshal(raw, &man) != nil {
		return nil
	}
	return man.Requires
}

// Delete 批量删除列表项（目录或 zip 包），并从禁用名单里摘掉；被依赖的拒绝。
// 入参同样是唯一键（相对路径）。
func (s *Store) Delete(keys []string) ([]string, error) {
	list, _, err := s.List()
	if err != nil {
		return nil, err
	}
	byKey := map[string]Mod{}
	for _, m := range list {
		byKey[m.DirName] = m
	}
	victim := map[string]bool{}
	targets := make([]Mod, 0, len(keys))
	for _, k := range keys {
		m, ok := byKey[k]
		if !ok {
			return nil, fmt.Errorf("列表里没有这一项：%s", k)
		}
		targets = append(targets, m)
		if m.Kind == KindInstalled { // 只有已装目录才参与依赖关系
			victim[m.ID] = true
		}
	}
	for _, m := range list {
		if m.Kind != KindInstalled || victim[m.ID] {
			continue
		}
		for _, d := range m.requiresOf(s.Dir) {
			if victim[d] {
				return nil, fmt.Errorf("不能删除：%s 正被 %s 依赖", d, m.ID)
			}
		}
	}

	var done []string
	for _, m := range targets {
		full := filepath.Join(s.Dir, filepath.FromSlash(m.DirName))
		if err := os.RemoveAll(full); err != nil {
			return done, fmt.Errorf("删除 %s 失败：%w", full, err)
		}
		done = append(done, m.DirName)
	}
	st, _ := s.loadEnabled()
	keep := st.Disabled[:0]
	for _, id := range st.Disabled {
		if !victim[id] {
			keep = append(keep, id)
		}
	}
	st.Disabled = keep
	if err := s.saveEnabled(st, "modkit-web"); err != nil {
		return done, err
	}
	return done, nil
}

func dirSize(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			n += fi.Size()
		}
		return nil
	})
	return n
}

// ---------------------------------------------------------------------------
// 导入：识别两种 zip 形态 + 校验 + 落盘
// ---------------------------------------------------------------------------

// Candidate 是 zip 里识别出来的一个 mod。
type Candidate struct {
	Prefix   string    `json:"prefix"`   // zip 内前缀（"" = zip 根就是 mod 根）
	DirName  string    `json:"dirName"`  // 建议落位目录名
	Manifest *Manifest `json:"manifest"` // 已解析的清单
	Problems []string  `json:"problems"` // 校验问题（空 = 通过）
	Bytes    int64     `json:"bytes"`    // 该 mod 在包里的字节数
	Exists   bool      `json:"exists"`   // 目标目录已存在
}

// Inspect 只校验不落盘。两种形态都支持：
//   A. zip 根就是 mod 根（根下有 mod.json）
//   B. zip 里是一组 mod 根目录（每个 <dir>/mod.json）
func (s *Store) Inspect(zipPath string) ([]Candidate, error) {
	cands, err := inspectZip(zipPath)
	if err != nil {
		return nil, err
	}
	for i := range cands {
		if cands[i].DirName == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(s.Dir, filepath.FromSlash(cands[i].DirName))); err == nil {
			cands[i].Exists = true
		}
	}
	return cands, nil
}

// inspectZip 是纯粹的"读包 + 校验"（不关心落位目录）。
func inspectZip(zipPath string) ([]Candidate, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("打不开 zip：%w", err)
	}
	defer zr.Close()
	if len(zr.File) == 0 {
		return nil, errors.New("zip 是空的")
	}
	if len(zr.File) > maxEntries {
		return nil, fmt.Errorf("zip 条目过多（%d > %d）", len(zr.File), maxEntries)
	}

	// 收集每个 mod 根前缀下的文件（相对 mod 根）与目录名
	type cand struct {
		prefix string
		dir    string
		files  map[string]bool
		bytes  int64
		manRaw []byte
	}
	cands := map[string]*cand{}
	var total int64
	for _, f := range zr.File {
		name := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(name, "/") || strings.Contains(name, ":") {
			return nil, fmt.Errorf("zip 里有绝对路径条目：%s", f.Name)
		}
		for _, seg := range strings.Split(name, "/") {
			if seg == ".." {
				return nil, fmt.Errorf("zip 里有上跳路径条目：%s", f.Name)
			}
		}
		if f.FileInfo().IsDir() {
			continue
		}
		total += int64(f.UncompressedSize64)
		if total > maxExtractBytes {
			return nil, fmt.Errorf("解包总大小超过上限 %d 字节", int64(maxExtractBytes))
		}
		clean := path.Clean(name)

		// 找出这份文件属于哪个 mod 根
		var prefix string
		switch {
		case strings.HasSuffix(clean, "/"+manifestName):
			prefix = strings.TrimSuffix(clean, manifestName) // "a/b/" 或 ""
		case clean == manifestName:
			prefix = ""
		}
		if prefix != "" && !strings.HasSuffix(prefix, "/") {
			prefix += "/"
		}
		if prefix != "" {
			// 形态 B：<dir>/mod.json
			c := cands[prefix]
			if c == nil {
				c = &cand{prefix: prefix, dir: strings.TrimSuffix(prefix, "/"), files: map[string]bool{}}
				cands[prefix] = c
			}
			if clean == prefix+manifestName {
				raw, err := readZipFile(f, 1<<20)
				if err != nil {
					return nil, err
				}
				c.manRaw = raw
			}
		} else if clean == manifestName {
			// 形态 A：zip 根就是 mod 根
			c := cands[""]
			if c == nil {
				c = &cand{prefix: "", files: map[string]bool{}}
				cands[""] = c
			}
			raw, err := readZipFile(f, 1<<20)
			if err != nil {
				return nil, err
			}
			c.manRaw = raw
		}
	}
	// 第二次遍历：按已确定的前缀归属文件
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		clean := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
		best := ""
		for prefix := range cands {
			if prefix == "" {
				// 形态 A：根下的文件全都算，但排除形态 B 的目录前缀
				continue
			}
			if strings.HasPrefix(clean, prefix) && len(prefix) > len(best) {
				best = prefix
			}
		}
		if best == "" {
			if c := cands[""]; c != nil {
				c.files[clean] = true
				c.bytes += int64(f.UncompressedSize64)
			}
			continue
		}
		c := cands[best]
		rel := strings.TrimPrefix(clean, best)
		c.files[rel] = true
		c.bytes += int64(f.UncompressedSize64)
	}

	if len(cands) == 0 {
		return nil, errors.New("zip 里找不到 mod.json（形态 A：根下直接放；形态 B：每个 mod 一个子目录）")
	}
	if len(cands) > maxModsPerZip {
		return nil, fmt.Errorf("一次导入的 mod 数过多（%d > %d）", len(cands), maxModsPerZip)
	}
	if _, both := cands[""]; both && len(cands) > 1 {
		return nil, errors.New("zip 里既有根目录形态的 mod.json 又有子目录形态的 mod：请只用其中一种")
	}

	out := []Candidate{}
	seenID := map[string]string{}
	seenDir := map[string]string{}
	for prefix, c := range cands {
		if c.manRaw == nil {
			return nil, fmt.Errorf("zip 里的 %s 没有读到 mod.json", prefix)
		}
		if c.bytes > maxModBytes {
			return nil, fmt.Errorf("%s 超过单 mod 上限 %d 字节", prefix, int64(maxModBytes))
		}
		cand := Candidate{Prefix: prefix, DirName: c.dir, Bytes: c.bytes}
		var m Manifest
		if err := json.Unmarshal(c.manRaw, &m); err != nil {
			cand.Manifest = &Manifest{}
			cand.Problems = []string{"mod.json 不是合法 JSON：" + err.Error()}
		} else {
			cand.Manifest = &m
			cand.Problems = validateManifest(&m, c.files)
		}
		if prefix == "" {
			// 根目录形态没有目录名 → 用清单 id 当落位目录名
			cand.DirName = m.ID
		} else {
			// 多 mod 形态取最后一段做目录名（包里可能是 a/b/ 这种嵌套）
			cand.DirName = path.Base(strings.TrimSuffix(prefix, "/"))
		}
		if cand.DirName == "" {
			cand.Problems = append(cand.Problems, "无法确定落位目录名（根目录形态需要清单里有合法 id）")
		} else {
			if prev, dup := seenID[m.ID]; dup && m.ID != "" {
				cand.Problems = append(cand.Problems,
					fmt.Sprintf("同一个包里出现重复 id %q（另一个在 %s）", m.ID, prev))
			} else if m.ID != "" {
				seenID[m.ID] = prefix
			}
			if prev, dup := seenDir[cand.DirName]; dup {
				cand.Problems = append(cand.Problems,
					fmt.Sprintf("落位目录名重复 %q（另一个在 %s）", cand.DirName, prev))
			} else {
				seenDir[cand.DirName] = prefix
			}
		}
		sort.Strings(cand.Problems)
		out = append(out, cand)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DirName < out[j].DirName })
	return out, nil
}

func readZipFile(f *zip.File, cap int64) ([]byte, error) {
	if int64(f.UncompressedSize64) > cap {
		return nil, fmt.Errorf("%s 过大", f.Name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, cap))
}

// ImportReport 是一次导入的结果。
type ImportReport struct {
	Zip        string      `json:"zip"`
	DryRun     bool        `json:"dryRun"`
	Imported   []string    `json:"imported"`
	Skipped    []string    `json:"skipped"`
	Candidates []Candidate `json:"candidates"`
}

// Import 先整体校验、再落盘（任何一份清单不过就整批不落）。
// 目标目录已存在时必须 overwrite=true，否则该 mod 记为 skipped。
func (s *Store) Import(zipPath string, overwrite bool, dryRun bool) (ImportReport, error) {
	rep := ImportReport{Zip: filepath.Base(zipPath), DryRun: dryRun}
	cands, err := s.Inspect(zipPath)
	if err != nil {
		return rep, err
	}
	rep.Candidates = cands

	var bad []string
	for _, c := range cands {
		if len(c.Problems) > 0 {
			bad = append(bad, fmt.Sprintf("%s：%s", c.DirName, strings.Join(c.Problems, "；")))
		}
	}
	if len(bad) > 0 {
		return rep, fmt.Errorf("校验没通过，未导入任何 mod：\n- %s", strings.Join(bad, "\n- "))
	}
	for _, c := range cands {
		if c.Exists && !overwrite {
			rep.Skipped = append(rep.Skipped, c.DirName+"（已存在，未选择覆盖）")
		}
	}
	if dryRun {
		return rep, nil
	}

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return rep, err
	}
	defer zr.Close()

	for _, c := range cands {
		if c.Exists && !overwrite {
			continue
		}
		dst := filepath.Join(s.Dir, c.DirName)
		tmp := dst + ".importing"
		if err := os.RemoveAll(tmp); err != nil {
			return rep, err
		}
		if err := os.MkdirAll(tmp, 0o755); err != nil {
			return rep, err
		}
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			clean := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
			rel := ""
			if c.Prefix == "" {
				// 根目录形态：根下的所有文件都归它（同包多形态已在 Inspect 里拒绝）
				rel = clean
			} else {
				if !strings.HasPrefix(clean, c.Prefix) {
					continue
				}
				rel = strings.TrimPrefix(clean, c.Prefix)
			}
			if rel == "" || !safeRel(rel) {
				continue
			}
			out := filepath.Join(tmp, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				return rep, err
			}
			if err := extractZipFile(f, out); err != nil {
				return rep, fmt.Errorf("解出 %s 失败：%w", f.Name, err)
			}
		}
		if err := os.RemoveAll(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
			return rep, err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return rep, err
		}
		rep.Imported = append(rep.Imported, c.DirName)
	}
	return rep, nil
}

func extractZipFile(f *zip.File, out string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	w, err := os.OpenFile(out, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, io.LimitReader(rc, maxModBytes))
	return err
}

// ImportLocal 把 mods 目录里**已有的 zip 包**就地导入成目录（不需要上传文件）。
// 界面上 [包] 那一行的「导入」按钮走的就是它 —— 包没有启停状态，
// 想让它变成可启停、可被引擎加载的 mod，先落成目录。
func (s *Store) ImportLocal(keys []string, overwrite bool) (ImportReport, error) {
	rep := ImportReport{DryRun: false}
	list, _, err := s.List()
	if err != nil {
		return rep, err
	}
	byKey := map[string]Mod{}
	for _, m := range list {
		byKey[m.DirName] = m
	}
	var bad []string
	for _, k := range keys {
		m, ok := byKey[k]
		if !ok {
			return rep, fmt.Errorf("列表里没有这一项：%s", k)
		}
		if m.Kind != KindPackage {
			return rep, fmt.Errorf("%s 已经是目录了（不用导入）", k)
		}
		full := filepath.Join(s.Dir, filepath.FromSlash(k))
		one, err := s.Import(full, overwrite, false)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s：%v", k, err))
			continue
		}
		rep.Zip = filepath.Base(k)
		rep.Imported = append(rep.Imported, one.Imported...)
		rep.Skipped = append(rep.Skipped, one.Skipped...)
		rep.Candidates = append(rep.Candidates, one.Candidates...)
	}
	if len(bad) > 0 {
		return rep, fmt.Errorf("导入失败：\n- %s", strings.Join(bad, "\n- "))
	}
	return rep, nil
}

// Export 把选中的 mod 打包：**1 个 = zip 根就是 mod 根（形态 A）；多个 = 每个 mod 一个子目录（形态 B）**。
func (s *Store) Export(keys []string, w io.Writer) ([]string, error) {
	list, _, err := s.List()
	if err != nil {
		return nil, err
	}
	byKey := map[string]Mod{}
	for _, m := range list {
		byKey[m.DirName] = m
	}
	dirs := []Mod{}
	for _, k := range keys {
		m, ok := byKey[k]
		if !ok {
			return nil, fmt.Errorf("列表里没有这一项：%s", k)
		}
		dirs = append(dirs, m)
	}
	if len(dirs) == 0 {
		return nil, errors.New("没有选择要导出的 mod")
	}
	// 单个 zip 包：原样拷贝（包里本来是什么形态就还是什么形态）
	if len(dirs) == 1 && dirs[0].Kind == KindPackage {
		src := filepath.Join(s.Dir, filepath.FromSlash(dirs[0].DirName))
		in, err := os.Open(src)
		if err != nil {
			return nil, err
		}
		defer in.Close()
		if _, err := io.Copy(w, in); err != nil {
			return nil, err
		}
		return []string{filepath.Base(src)}, nil
	}

	zw := zip.NewWriter(w)
	defer zw.Close()
	var names []string
	for _, m := range dirs {
		src := filepath.Join(s.Dir, filepath.FromSlash(m.DirName))
		base := ""
		if len(dirs) > 1 {
			base = m.DirName
			if m.Kind == KindPackage {
				base = strings.TrimSuffix(base, filepath.Ext(base))
			}
			base = filepath.ToSlash(base) + "/" // 形态 B
		}
		if m.Kind == KindPackage {
			// 包：把里面的条目解到 base 下，让外层仍是合法的形态 B
			zr, err := zip.OpenReader(src)
			if err != nil {
				return names, err
			}
			for _, f := range zr.File {
				if f.FileInfo().IsDir() {
					continue
				}
				rel := path.Clean(strings.ReplaceAll(f.Name, "\\", "/"))
				if !safeRel(rel) {
					continue
				}
				fw, err := zw.Create(base + rel)
				if err != nil {
					zr.Close()
					return names, err
				}
				in, err := f.Open()
				if err != nil {
					zr.Close()
					return names, err
				}
				_, err = io.Copy(fw, in)
				in.Close()
				if err != nil {
					zr.Close()
					return names, err
				}
			}
			zr.Close()
			names = append(names, m.DirName)
			continue
		}
		err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(src, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if !safeRel(rel) {
				return nil
			}
			fw, err := zw.Create(base + rel)
			if err != nil {
				return err
			}
			in, err := os.Open(p)
			if err != nil {
				return err
			}
			defer in.Close()
			_, err = io.Copy(fw, in)
			return err
		})
		if err != nil {
			return names, err
		}
		names = append(names, m.DirName)
	}
	return names, nil
}
