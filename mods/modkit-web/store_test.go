package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// 测试辅助
// ---------------------------------------------------------------------------

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(files[n])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// manifestJSON 造一份清单：client 层一条 file.add + server 层一个 hook。
func manifestJSON(id, name string, extra ...string) string {
	perms := `"permissions": ["server.hook", "client.file.write"]`
	layers := `"layers": {
      "server": {"package": ".", "hooks": [{"name": "server.boot"}]},
      "client": {"ops": [{"kind": "file.add", "target": "demo/a.txt", "source": "client/a.txt", "size": 2, "sha256": "` + strings.Repeat("a", 64) + `"}]}
    }`
	for _, e := range extra {
		switch {
		case strings.HasPrefix(e, "perms="):
			perms = `"permissions": ` + strings.TrimPrefix(e, "perms=")
		case strings.HasPrefix(e, "layers="):
			layers = `"layers": ` + strings.TrimPrefix(e, "layers=")
		}
	}
	return fmt.Sprintf(`{
  "schema": 2,
  "id": %q,
  "version": "1.0.0",
  "name": %q,
  "author": "测试作者",
  "description": "一句话说明",
  %s,
  %s
}`, id, name, perms, layers)
}

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func zipPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "pkg.zip")
}

// ---------------------------------------------------------------------------
// 形态识别 + 校验
// ---------------------------------------------------------------------------

func TestInspectShapeASingleRoot(t *testing.T) {
	s := newStore(t)
	p := zipPath(t)
	writeZip(t, p, map[string]string{
		"mod.json":      manifestJSON("demo.one", "示例一"),
		"client/a.txt":  "hi",
		"server/mod.go": "package mods",
	})
	cands, err := s.Inspect(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 {
		t.Fatalf("期望 1 个候选，得到 %d", len(cands))
	}
	c := cands[0]
	if c.Prefix != "" {
		t.Errorf("根目录形态的 prefix 应为空，得到 %q", c.Prefix)
	}
	if c.DirName != "demo.one" {
		t.Errorf("落位目录名应取清单 id，得到 %q", c.DirName)
	}
	if len(c.Problems) != 0 {
		t.Errorf("不该有问题：%v", c.Problems)
	}
}

func TestInspectShapeBMultiRoot(t *testing.T) {
	s := newStore(t)
	p := zipPath(t)
	writeZip(t, p, map[string]string{
		"one/mod.json":      manifestJSON("demo.one", "示例一"),
		"one/client/a.txt":  "hi",
		"one/server/mod.go": "package mods",
		"two/mod.json":      manifestJSON("demo.two", "示例二"),
		"two/client/a.txt":  "hi",
		"two/server/mod.go": "package mods",
	})
	cands, err := s.Inspect(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 {
		t.Fatalf("期望 2 个候选，得到 %d", len(cands))
	}
	got := []string{cands[0].DirName, cands[1].DirName}
	if got[0] != "one" || got[1] != "two" {
		t.Errorf("落位目录名应为 one/two，得到 %v", got)
	}
}

func TestInspectNestedPrefixUsesLeafDir(t *testing.T) {
	s := newStore(t)
	p := zipPath(t)
	writeZip(t, p, map[string]string{
		"a/b/deep/mod.json":      manifestJSON("demo.deep", "深层"),
		"a/b/deep/client/a.txt":  "hi",
		"a/b/deep/server/mod.go": "package mods",
	})
	cands, err := s.Inspect(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 1 || cands[0].DirName != "deep" {
		t.Fatalf("嵌套形态应取最后一段做目录名，得到 %+v", cands)
	}
}

func TestInspectRejectsMixedShapes(t *testing.T) {
	s := newStore(t)
	p := zipPath(t)
	writeZip(t, p, map[string]string{
		"mod.json":          manifestJSON("demo.root", "根"),
		"client/a.txt":      "hi",
		"server/mod.go":     "package mods",
		"other/mod.json":    manifestJSON("demo.other", "另一个"),
		"other/client/a.txt": "hi",
	})
	if _, err := s.Inspect(p); err == nil {
		t.Fatal("同一个包里混用两种形态应当报错")
	}
}

func TestValidateManifestProblems(t *testing.T) {
	sha := strings.Repeat("a", 64)
	cases := []struct {
		name     string
		manifest string
		files    map[string]string
		want     string
	}{
		{
			name:     "schema 1 提示 migrate",
			manifest: `{"schema":1,"id":"a.b","version":"1","name":"n","permissions":[],"layers":{}}`,
			want:     "migrate",
		},
		{
			name:     "缺 name",
			manifest: `{"schema":2,"id":"a.b","version":"1","permissions":["server.hook"],"layers":{"server":{"hooks":[{"name":"x"}]}}}`,
			want:     "缺少 name",
		},
		{
			name:     "id 非法",
			manifest: `{"schema":2,"id":"A.B","version":"1","name":"n","permissions":["server.hook"],"layers":{"server":{"hooks":[{"name":"x"}]}}}`,
			want:     "id 非法",
		},
		{
			name:     "未知层",
			manifest: `{"schema":2,"id":"a.b","version":"1","name":"n","permissions":[],"layers":{"magic":{"ops":[{"kind":"x"}]}}}`,
			want:     "未知层",
		},
		{
			name:     "声明了层但没动作",
			manifest: `{"schema":2,"id":"a.b","version":"1","name":"n","permissions":["client.file.write"],"layers":{"client":{"ops":[]}}}`,
			want:     "没有任何动作",
		},
		{
			name: "引用的文件不在包里",
			manifest: `{"schema":2,"id":"a.b","version":"1","name":"n","permissions":["client.file.write"],
			  "layers":{"client":{"ops":[{"kind":"file.add","target":"t","source":"client/missing.txt","size":1,"sha256":"` + sha + `"}]}}}`,
			want: "不在包里",
		},
		{
			name: "source 路径不安全",
			manifest: `{"schema":2,"id":"a.b","version":"1","name":"n","permissions":["client.file.write"],
			  "layers":{"client":{"ops":[{"kind":"file.add","target":"t","source":"../evil.txt","size":1,"sha256":"` + sha + `"}]}}}`,
			want: "不安全",
		},
		{
			name: "缺权限位",
			manifest: `{"schema":2,"id":"a.b","version":"1","name":"n","permissions":[],
			  "layers":{"client":{"ops":[{"kind":"file.add","target":"t","source":"client/a.txt","size":1,"sha256":"` + sha + `"}]}}}`,
			want: "缺少权限位 client.file.write",
		},
		{
			name: "requires 依赖自己",
			manifest: `{"schema":2,"id":"a.b","version":"1","name":"n","permissions":["server.hook"],"requires":["a.b"],
			  "layers":{"server":{"hooks":[{"name":"x"}]}}}`,
			want:     "依赖自己",
		},
		{
			name: "声明 server 层但没有 server 目录",
			manifest: `{"schema":2,"id":"a.b","version":"1","name":"n","permissions":["server.hook"],
			  "layers":{"server":{"hooks":[{"name":"x"}]}}}`,
			want:     "没有 server/ 目录",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m Manifest
			if err := json.Unmarshal([]byte(c.manifest), &m); err != nil {
				t.Fatalf("测试清单本身写错了：%v", err)
			}
			files := map[string]bool{"client/a.txt": true}
			problems := validateManifest(&m, files)
			joined := strings.Join(problems, "\n")
			if !strings.Contains(joined, c.want) {
				t.Fatalf("期望问题里含 %q，实际：\n%s", c.want, joined)
			}
		})
	}
}

func TestInspectRejectsTraversalEntry(t *testing.T) {
	s := newStore(t)
	p := zipPath(t)
	writeZip(t, p, map[string]string{
		"mod.json":         manifestJSON("demo.one", "示例一"),
		"client/a.txt":     "hi",
		"server/mod.go":    "package mods",
		"../outside.txt":   "oops",
	})
	if _, err := s.Inspect(p); err == nil {
		t.Fatal("zip 里有上跳路径应当报错")
	}
}

func TestInspectDuplicateIDInOneZip(t *testing.T) {
	s := newStore(t)
	p := zipPath(t)
	writeZip(t, p, map[string]string{
		"one/mod.json":      manifestJSON("demo.same", "同"),
		"one/client/a.txt":  "hi",
		"one/server/mod.go": "package mods",
		"two/mod.json":      manifestJSON("demo.same", "同（重）"),
		"two/client/a.txt":  "hi",
		"two/server/mod.go": "package mods",
	})
	cands, err := s.Inspect(p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range cands {
		for _, pr := range c.Problems {
			if strings.Contains(pr, "重复 id") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("应报重复 id，实际 %+v", cands)
	}
}

// ---------------------------------------------------------------------------
// 导入 / 导出
// ---------------------------------------------------------------------------

func TestImportRefusesBadPackage(t *testing.T) {
	s := newStore(t)
	p := zipPath(t)
	writeZip(t, p, map[string]string{
		"one/mod.json":  `{"schema":2,"id":"demo.bad","version":"1","name":"","permissions":[],"layers":{}}`,
		"one/client/x":  "hi",
	})
	if _, err := s.Import(p, true, false); err == nil {
		t.Fatal("校验不过的包不应被导入")
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "one")); err == nil {
		t.Fatal("校验失败时不应留下目录")
	}
}

func TestImportDryRunWritesNothing(t *testing.T) {
	s := newStore(t)
	p := zipPath(t)
	writeZip(t, p, map[string]string{
		"one/mod.json":      manifestJSON("demo.one", "示例一"),
		"one/client/a.txt":  "hi",
		"one/server/mod.go": "package mods",
	})
	rep, err := s.Import(p, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !rep.DryRun || len(rep.Imported) != 0 {
		t.Fatalf("干跑不应写入：%+v", rep)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "one")); err == nil {
		t.Fatal("干跑不应落盘")
	}
}

func TestImportOverwriteSemantics(t *testing.T) {
	s := newStore(t)
	p1 := zipPath(t)
	writeZip(t, p1, map[string]string{
		"one/mod.json":      manifestJSON("demo.one", "第一版"),
		"one/client/a.txt":  "v1",
		"one/server/mod.go": "package mods",
	})
	if _, err := s.Import(p1, false, false); err != nil {
		t.Fatal(err)
	}

	p2 := zipPath(t)
	writeZip(t, p2, map[string]string{
		"one/mod.json":      manifestJSON("demo.one", "第二版"),
		"one/client/a.txt":  "v2",
		"one/server/mod.go": "package mods",
	})
	rep, err := s.Import(p2, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Skipped) != 1 || len(rep.Imported) != 0 {
		t.Fatalf("已存在且未选择覆盖时应当跳过：%+v", rep)
	}
	if b, _ := os.ReadFile(filepath.Join(s.Dir, "one", "client", "a.txt")); string(b) != "v1" {
		t.Fatalf("不应被覆盖，实际内容 %q", string(b))
	}
	if _, err := s.Import(p2, true, false); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(s.Dir, "one", "client", "a.txt")); string(b) != "v2" {
		t.Fatalf("选择覆盖后应当是 v2，实际 %q", string(b))
	}
}

func TestImportExportRoundTripBothShapes(t *testing.T) {
	s := newStore(t)
	p := zipPath(t)
	writeZip(t, p, map[string]string{
		"one/mod.json":      manifestJSON("demo.one", "示例一"),
		"one/client/a.txt":  "hi-one",
		"one/server/mod.go": "package mods",
		"two/mod.json":      manifestJSON("demo.two", "示例二"),
		"two/client/a.txt":  "hi-two",
		"two/server/mod.go": "package mods",
	})
	if _, err := s.Import(p, false, false); err != nil {
		t.Fatal(err)
	}
	list, _, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("导入后应有 2 个 mod，得到 %d", len(list))
	}
	for _, m := range list {
		if m.Author != "测试作者" || m.AddedAt == "" || m.SizeBytes == 0 {
			t.Errorf("列表项字段不全：%+v", m)
		}
		if !m.Enabled {
			t.Errorf("新导入的 mod 默认应启用：%+v", m)
		}
	}

	// 导出单个 → 形态 A（zip 根就是 mod 根）
	var buf bytes.Buffer
	if _, err := s.Export([]string{"one"}, &buf); err != nil {
		t.Fatal(err)
	}
	single := filepath.Join(t.TempDir(), "single.zip")
	if err := os.WriteFile(single, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(single)
	if err != nil {
		t.Fatal(err)
	}
	hasRoot := false
	for _, f := range zr.File {
		if f.Name == "mod.json" {
			hasRoot = true
		}
	}
	zr.Close()
	if !hasRoot {
		t.Fatal("单个导出应当是根目录形态（根下有 mod.json）")
	}

	// 导出多个 → 形态 B（每个 mod 一个子目录）
	buf.Reset()
	if _, err := s.Export([]string{"one", "two"}, &buf); err != nil {
		t.Fatal(err)
	}
	multi := filepath.Join(t.TempDir(), "multi.zip")
	if err := os.WriteFile(multi, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := newStore(t)
	cands, err := s2.Inspect(multi)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 {
		t.Fatalf("多 mod 导出应能被识别成 2 个，得到 %d", len(cands))
	}
	for _, c := range cands {
		if len(c.Problems) != 0 {
			t.Errorf("导出的包应当校验通过：%v", c.Problems)
		}
	}
	if _, err := s2.Import(multi, false, false); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(s2.Dir, "one", "client", "a.txt"))
	if string(b) != "hi-one" {
		t.Fatalf("往返后内容不一致：%q", string(b))
	}
}

// ---------------------------------------------------------------------------
// 启用 / 停用 / 删除
// ---------------------------------------------------------------------------

func TestEnableDisableWritesDisabledList(t *testing.T) {
	s := newStore(t)
	importOne(t, s, manifestJSON("demo.one", "示例一"))
	if _, err := s.SetEnabled([]string{"one"}, false, "test"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(s.Dir, enabledName))
	if err != nil {
		t.Fatal(err)
	}
	var st EnabledFile
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	if st.Schema != enabledSchema || len(st.Disabled) != 1 || st.Disabled[0] != "demo.one" {
		t.Fatalf("enabled.json 内容不对：%s", string(raw))
	}
	if st.UpdatedBy != "test" {
		t.Errorf("updatedBy 应为 test，实际 %q", st.UpdatedBy)
	}
	list, _, _ := s.List()
	if len(list) != 1 || list[0].Enabled {
		t.Fatalf("停用后列表应显示未启用：%+v", list)
	}

	if _, err := s.SetEnabled([]string{"one"}, true, "test"); err != nil {
		t.Fatal(err)
	}
	list, _, _ = s.List()
	if !list[0].Enabled {
		t.Fatal("重新启用后应显示已启用")
	}
}

func TestDependencyGuards(t *testing.T) {
	s := newStore(t)
	importOne(t, s, manifestJSON("dep.base", "被依赖"))
	// 依赖 dep.base 的 mod：requires 合法即可
	writeZip(t, zipPath(t), nil) // 占位，避免未使用变量
	p := filepath.Join(t.TempDir(), "dep.zip")
	writeZip(t, p, map[string]string{
		"user/mod.json": strings.Replace(
			manifestJSON("demo.user", "使用者"),
			`"description": "一句话说明",`,
			`"description": "一句话说明", "requires": ["dep.base"],`, 1),
		"user/client/a.txt":  "hi",
		"user/server/mod.go": "package mods",
	})
	if _, err := s.Import(p, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetEnabled([]string{"one"}, false, "test"); err == nil {
		t.Fatal("被启用中的 mod 依赖时不应允许停用")
	}
	if _, err := s.Delete([]string{"one"}); err == nil {
		t.Fatal("被依赖时不应允许删除")
	}
	if _, err := s.SetEnabled([]string{"user"}, false, "test"); err != nil {
		t.Fatalf("先停用依赖者应当可以：%v", err)
	}
	if _, err := s.SetEnabled([]string{"one"}, false, "test"); err != nil {
		t.Fatalf("依赖者停用后，被依赖者应可停用：%v", err)
	}
}

func TestDeleteRemovesDirAndEnabledEntry(t *testing.T) {
	s := newStore(t)
	importOne(t, s, manifestJSON("demo.one", "示例一"))
	if _, err := s.SetEnabled([]string{"one"}, false, "test"); err != nil {
		t.Fatal(err)
	}
	done, err := s.Delete([]string{"one"})
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 1 {
		t.Fatalf("应删除 1 项：%v", done)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "one")); err == nil {
		t.Fatal("目录应已删除")
	}
	var st EnabledFile
	raw, _ := os.ReadFile(filepath.Join(s.Dir, enabledName))
	_ = json.Unmarshal(raw, &st)
	if len(st.Disabled) != 0 {
		t.Fatalf("删除后禁用名单应清空：%s", string(raw))
	}
}

func TestDeleteUnknownMod(t *testing.T) {
	s := newStore(t)
	if _, err := s.Delete([]string{"nope"}); err == nil {
		t.Fatal("删除不存在的 mod 应当报错")
	}
}

// ---------------------------------------------------------------------------
// HTTP 层（只测取列表与分页/搜索这一条链）
// ---------------------------------------------------------------------------

func TestListPaginationAndSearch(t *testing.T) {
	s := newStore(t)
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("demo.m%d", i)
		p := filepath.Join(t.TempDir(), fmt.Sprintf("m%d.zip", i))
		writeZip(t, p, map[string]string{
			"m" + fmt.Sprint(i) + "/mod.json":      manifestJSON(id, "名字"+fmt.Sprint(i)),
			"m" + fmt.Sprint(i) + "/client/a.txt":  "hi",
			"m" + fmt.Sprint(i) + "/server/mod.go": "package mods",
		})
		if _, err := s.Import(p, false, false); err != nil {
			t.Fatal(err)
		}
	}
	items, _, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("应有 3 个 mod，得到 %d", len(items))
	}
	// 分页是 HTTP 层做的，这里验证排序稳定（按 id）
	if items[0].ID != "demo.m0" || items[2].ID != "demo.m2" {
		t.Fatalf("排序不对：%v", []string{items[0].ID, items[1].ID, items[2].ID})
	}
}

func importOne(t *testing.T, s *Store, manifest string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "one.zip")
	writeZip(t, p, map[string]string{
		"one/mod.json":      manifest,
		"one/client/a.txt":  "hi",
		"one/server/mod.go": "package mods",
	})
	if _, err := s.Import(p, true, false); err != nil {
		t.Fatal(err)
	}
}

var _ = io.Discard

func TestListIncludesZipPackages(t *testing.T) {
	s := newStore(t)
	// 把一个 zip 包直接放进 mods 目录（不导入）——列表也要能看见它
	pkgDir := filepath.Join(s.Dir, "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(pkgDir, "demo.pack-1.0.0.zip")
	writeZip(t, pkg, map[string]string{
		"mod.json":      manifestJSON("demo.pack", "包形态"),
		"client/a.txt":  "hi",
		"server/mod.go": "package mods",
	})
	list, _, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Kind != KindPackage {
		t.Fatalf("列表应含一个 package 项：%+v", list)
	}
	if list[0].ID != "demo.pack" || list[0].Name != "包形态" || list[0].Author != "测试作者" {
		t.Fatalf("包项元数据应从包内 mod.json 读出：%+v", list[0])
	}
	if list[0].AddedAt == "" || list[0].SizeBytes == 0 {
		t.Errorf("包项缺日期/大小：%+v", list[0])
	}
	// zip 包的启停语义（业主 2026-10-06 反馈"点启用不生效"后定）：
	//   启用 = 就地导入成目录并启用；停用 = 明确跳过并说明。
	res, err := s.SetEnabled([]string{list[0].DirName}, true, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Done) != 1 || res.Done[0] != "demo.pack" {
		t.Fatalf("启用一个包应当把它导入成目录并启用：%+v", res)
	}
	if len(res.Notes) == 0 {
		t.Errorf("应当说明这个包被导入了：%+v", res)
	}
	l2, _, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	var installed *Mod
	for i := range l2 {
		if l2[i].Kind == KindInstalled {
			installed = &l2[i]
		}
	}
	if installed == nil || installed.ID != "demo.pack" || !installed.Enabled {
		t.Fatalf("导入后的目录应当是已装且启用：%+v", l2)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "demo.pack", "mod.json")); err != nil {
		t.Fatalf("导入后的目录应含 mod.json：%v", err)
	}
	res2, err := s.SetEnabled([]string{list[0].DirName}, false, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(res2.Done) != 0 || len(res2.Notes) == 0 {
		t.Fatalf("停用一个包应当跳过并说明：%+v", res2)
	}
	// 单个包导出 = 原样字节拷贝
	var buf bytes.Buffer
	if _, err := s.Export([]string{list[0].DirName}, &buf); err != nil {
		t.Fatal(err)
	}
	orig, err := os.ReadFile(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), orig) {
		t.Fatal("单个包导出应原样拷贝（字节一致）")
	}
	// 删除包
	done, err := s.Delete([]string{list[0].DirName})
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 1 {
		t.Fatalf("应删除 1 项：%v", done)
	}
	if _, err := os.Stat(pkg); err == nil {
		t.Fatal("zip 包应已删除")
	}
}

func TestExportMixedDirAndPackageIsValidShapeB(t *testing.T) {
	s := newStore(t)
	importOne(t, s, manifestJSON("demo.one", "示例一"))
	pkgDir := filepath.Join(s.Dir, "pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeZip(t, filepath.Join(pkgDir, "demo.pack.zip"), map[string]string{
		"mod.json":      manifestJSON("demo.pack", "包形态"),
		"client/a.txt":  "hi",
		"server/mod.go": "package mods",
	})
	var buf bytes.Buffer
	if _, err := s.Export([]string{"one", "pkg/demo.pack.zip"}, &buf); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "mixed.zip")
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	s2 := newStore(t)
	cands, err := s2.Inspect(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 2 {
		t.Fatalf("混合导出应能被识别成 2 个 mod（形态 B），得到 %d", len(cands))
	}
	for _, c := range cands {
		if len(c.Problems) != 0 {
			t.Errorf("混合导出的包应校验通过：%s %v", c.DirName, c.Problems)
		}
	}
}
