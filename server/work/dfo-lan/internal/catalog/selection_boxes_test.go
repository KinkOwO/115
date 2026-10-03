package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 2026-09-22：装备/装扮自选盒（[booster select category]）不在 booster-catalog.json
// 里——那是固定内容盒——所以服务端查无定义、只回通用失败码，客户端把它显示成
// "库存已满"。目录由 cmd/selectionboximport 从真源 PVF 导出；这里盯住加载校验，
// 以及"玩家的选择必须落在源给出的范围内"这条服务端校验。
func TestSelectionBoxesFlowFixture(t *testing.T) {
	s, err := LoadSelectionBoxes("testdata/selection-flow.json")
	if err != nil {
		t.Fatal(err)
	}
	if s.Model != SelectionBoxModel || len(s.Source.Checksum) != 64 {
		t.Fatalf("model=%q source=%+v", s.Model, s.Source)
	}
	if len(s.Boxes) != 26 {
		t.Fatalf("suspiciously small catalog: %d boxes", len(s.Boxes))
	}

	// 奥德赛武器盒：85 个 (job, growtype) 类别，脚本哈希与既有
	// odyssey-weapon-box-release.json 钉死的常量相同。
	box, ok := s.ByTemplate(10417789)
	if !ok {
		t.Fatal("the Odyssey creation weapon box is missing")
	}
	if len(box.Categories) != 85 {
		t.Fatalf("weapon box categories = %d, want 85", len(box.Categories))
	}
	if box.SHA256 != "d67f5042a5e17ef30581e297f030561de39f92ad5e215d742ec067aeaad96bec" {
		t.Fatalf("weapon box script hash = %s", box.SHA256)
	}
	want := []uint32{101001229, 101011345, 101021099, 101031215, 101040856}
	if len(box.Categories[0].Items) != len(want) {
		t.Fatalf("first category items = %+v", box.Categories[0].Items)
	}
	for i, it := range box.Categories[0].Items {
		if it.Template != want[i] || it.Count != 1 {
			t.Fatalf("first category items = %+v", box.Categories[0].Items)
		}
	}

	items, missing, checked := s.Resolve(10417789, [2]byte{0, 0}, []uint32{101001229})
	if !checked || len(items) != 1 || items[0].Template != 101001229 || items[0].Count != 1 || len(missing) != 0 {
		t.Fatalf("resolve weapon box: items=%+v missing=%v checked=%v", items, missing, checked)
	}
	// 范围外的选择只报告、不拒绝：导出源来自 client-build/Script.inner.pvf，而客户端
	// 加载自己的 Script.pvf（两份不是同一个构建），客户端 UI 给出的选择可能不在导出列表里。
	if _, missing, checked := s.Resolve(10417789, [2]byte{0, 0}, []uint32{101001229, 1}); !checked || len(missing) != 1 || missing[0] != 1 {
		t.Fatalf("out-of-range pick not reported: missing=%v checked=%v", missing, checked)
	}
	// 源里没有的类别 / 只有未建模内容段的类别：报 unchecked（调用方交给既有分发路径）。
	if _, _, checked := s.Resolve(10417789, [2]byte{9, 9}, []uint32{101001229}); checked {
		t.Fatal("an unknown category must stay unchecked")
	}
	if _, _, checked := s.Resolve(999999, [2]byte{0, 0}, []uint32{101001229}); checked {
		t.Fatal("an unknown box must stay unchecked")
	}
	// 索引误标为 [booster selection]、脚本里却只有固定 [booster info] 的盒子。
	if !s.IsFixed(490022952) {
		t.Fatal("the mislabeled fixed box is not recorded")
	}
	// 装扮自选盒（类别里只有 [avatar]）必须报 unchecked，而不是被当成越界。
	if box, ok := s.ByTemplate(10000685); ok {
		if len(box.Categories) != 1 || len(box.Categories[0].Items) != 0 {
			t.Fatalf("costume box shape changed: %+v", box.Categories)
		}
		if _, _, checked := s.Resolve(10000685, box.Categories[0].Category, []uint32{104580113}); checked {
			t.Fatal("a costume-only category must stay unchecked")
		}
	}
}

func TestLoadSelectionBoxesRefusesBrokenArtifacts(t *testing.T) {
	hash := strings.Repeat("a", 64)
	dir := t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	box := `{"template":1,"path":"stackable/x.stk","sha256":"` + hash + `","categories":[{"category":[0,0],"items":[{"template":2,"count":1}]}]}`
	cases := map[string]string{
		"foreign model": `{"model":"other","source":{"checksum":"` + hash + `"},"boxes":{"1":` + box + `}}`,
		"empty boxes":   `{"model":"` + SelectionBoxModel + `","source":{"checksum":"` + hash + `"},"boxes":{}}`,
		"key mismatch":  `{"model":"` + SelectionBoxModel + `","source":{"checksum":"` + hash + `"},"boxes":{"9":` + box + `}}`,
		"zero item":     `{"model":"` + SelectionBoxModel + `","source":{"checksum":"` + hash + `"},"boxes":{"1":{"template":1,"path":"p","sha256":"` + hash + `","categories":[{"category":[0,0],"items":[{"template":0,"count":1}]}]}}}`,
		"bad hash":      `{"model":"` + SelectionBoxModel + `","source":{"checksum":"` + hash + `"},"boxes":{"1":{"template":1,"path":"p","sha256":"nope","categories":[{"category":[0,0],"items":[{"template":2,"count":1}]}]}}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadSelectionBoxes(write(strings.ReplaceAll(name, " ", "-")+".json", body)); err == nil {
				t.Fatal("broken artifact accepted")
			}
		})
	}
}
