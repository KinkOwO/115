package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

// 真实内层归档对照：把秘宝精度解析器压到当次源上。
//
//	DFO_PVF_CORE_TEST_ARCHIVE=D:\115us\server\work\client-build\Script.inner.pvf
//	go test ./internal/catalog/ -run SoleEquipmentNative -count=1
//
// 未设置环境变量时跳过（与其它 *_native_test.go 同一约定）。
func openSoleArchive(t *testing.T) *pvf.Archive {
	t.Helper()
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native sole equipment parity")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestSoleEquipmentRulesNative(t *testing.T) {
	a := openSoleArchive(t)
	rules, err := ImportSoleEquipmentRules(a)
	if err != nil {
		t.Fatalf("import rules: %v", err)
	}
	// 源当前登记三件秘宝：Venus 100354181 / Nabel 100391142 / Diregie 100346156。
	if len(rules.Items) != 3 {
		t.Fatalf("items = %d, want 3 (%v)", len(rules.Items), rules.Templates())
	}
	for _, template := range []uint32{100354181, 100391142, 100346156} {
		info, ok := rules.Info(template)
		if !ok {
			t.Fatalf("%d is missing from the source", template)
		}
		if info.MaxQuality != 100 {
			t.Fatalf("%d [max quality] = %d, want 100", template, info.MaxQuality)
		}
		if len(info.Groups) != 2 {
			t.Fatalf("%d groups = %d, want 2", template, len(info.Groups))
		}
		// 组 1 的最后一项必须是金币（源把 [group] 1 的第三项写成 `0 4000000`）。
		high := info.Groups[1]
		if len(high) == 0 || !high[len(high)-1].Gold() {
			t.Fatalf("%d group 1 = %+v, want a trailing gold entry", template, high)
		}
	}

	// 逐字段对照源（实机三件秘宝的面板成本，见 analysis/tasks/next150 的表格）。
	cases := []struct {
		template uint32
		low      []SoleEquipmentMaterial
	}{
		{100354181, []SoleEquipmentMaterial{{10404807, 8}, {10404719, 20}, {10401346, 800}}},
		{100391142, []SoleEquipmentMaterial{{10407241, 8}, {10404719, 20}, {10401346, 800}}},
		{100346156, []SoleEquipmentMaterial{{10413524, 30}, {10361515, 40}, {10401346, 800}}},
	}
	for _, c := range cases {
		info, _ := rules.Info(c.template)
		got := info.Groups[0]
		if len(got) != len(c.low) {
			t.Fatalf("%d group 0 = %+v, want %+v", c.template, got, c.low)
		}
		for i := range c.low {
			if got[i] != c.low[i] {
				t.Fatalf("%d group 0[%d] = %+v, want %+v", c.template, i, got[i], c.low[i])
			}
		}
		// 两个材料组必须各给一套成本：组 0 = 实物（第三项是材料）、组 1 = 金币替代。
		// 组号由客户端请求的 selector 决定（protocol.SoleMaterialGroupForSelector）。
		lowCost, ok := rules.Materials(c.template, 0)
		if !ok || len(lowCost) == 0 {
			t.Fatalf("%d group 0 missing (ok=%v)", c.template, ok)
		}
		highCost, ok := rules.Materials(c.template, 1)
		if !ok || len(highCost) == 0 {
			t.Fatalf("%d group 1 missing (ok=%v)", c.template, ok)
		}
		if !highCost[len(highCost)-1].Gold() {
			t.Fatalf("%d group 1 = %+v, want gold tail", c.template, highCost)
		}
		if lowCost[len(lowCost)-1].Gold() {
			t.Fatalf("%d group 0 = %+v, want material tail", c.template, lowCost)
		}
	}
}

// 反证：秘宝清单文件（.lst）里只有三个部位文件，而规则表的 [item index] 是模板号 ——
// 解析器只认 `[infos]`，不依赖 .lst（少了 .lst 也必须在）。
func TestSoleEquipmentParseRejectsMissingSections(t *testing.T) {
	if _, err := ParseSoleEquipmentRules("[infos]\n[/infos]\n"); err == nil {
		t.Fatal("an empty [infos] must be rejected")
	}
	body := "[infos]\n [info]\n  [item index] 1\n [info]\n[/infos]\n"
	if _, err := ParseSoleEquipmentRules(body); err == nil {
		t.Fatal("[info] without [max quality] must be rejected")
	}
}

// CMD2289 秘宝制作：源 `[create need materials]` 的逐项对照。
//
// 真源与精度提升同一个文件（etc/115lvability/soleequipmentsystem.cos），但**是另一段**：
// 制作成本与"加精度"成本是两套独立表。下面三件配方取自文档 §2，必须与源逐项一致。
func TestSoleCreateMaterialsNative(t *testing.T) {
	a := openSoleArchive(t)
	rules, err := ImportSoleEquipmentRules(a)
	if err != nil {
		t.Fatalf("import rules: %v", err)
	}
	cases := []struct {
		template uint32
		want     []SoleEquipmentMaterial
	}{
		// Venus 100354181：五项的实物组，末项 10401346 ×20000。
		{100354181, []SoleEquipmentMaterial{{10404718, 1}, {10404807, 1000}, {10404719, 2000}, {10361516, 5}, {10401346, 20000}}},
		// Nabel 100391142（文档 §2 只列了前四项，末项同 Venus 的 10401346 ×20000）。
		{100391142, []SoleEquipmentMaterial{{10406144, 1}, {10407241, 1000}, {10404719, 2000}, {10361516, 5}, {10401346, 20000}}},
		// Diregie 100346156（同样以 10401346 ×20000 收尾）。
		{100346156, []SoleEquipmentMaterial{{10413157, 1}, {10413524, 1200}, {10361515, 500}, {10361516, 25}, {10401346, 20000}}},
	}
	for _, c := range cases {
		info, ok := rules.Info(c.template)
		if !ok {
			t.Fatalf("%d is missing from the source", c.template)
		}
		if len(info.CreateGroups) != 2 {
			t.Fatalf("%d create groups = %d, want 2 (%+v)", c.template, len(info.CreateGroups), info.CreateGroups)
		}
		low, ok := rules.CreateMaterials(c.template, 0)
		if !ok || len(low) != len(c.want) {
			t.Fatalf("%d create group 0 = %+v (ok=%v), want %+v", c.template, low, ok, c.want)
		}
		for i := range c.want {
			if low[i] != c.want[i] {
				t.Fatalf("%d create group 0[%d] = %+v, want %+v", c.template, i, low[i], c.want[i])
			}
		}
		high, ok := rules.CreateMaterials(c.template, 1)
		if !ok || len(high) != len(low) {
			t.Fatalf("%d create group 1 = %+v (ok=%v), want the same width as group 0", c.template, high, ok)
		}
		// 组 0 末项是实物、组 1 末项是金币：源里两组**只差最后一项**。
		if low[len(low)-1].Gold() {
			t.Fatalf("%d create group 0 = %+v, want a material tail", c.template, low)
		}
		if !high[len(high)-1].Gold() {
			t.Fatalf("%d create group 1 = %+v, want a gold tail", c.template, high)
		}
		for i := 0; i < len(low)-1; i++ {
			if low[i] != high[i] {
				t.Fatalf("%d create groups differ before the last entry: %+v vs %+v", c.template, low, high)
			}
		}
	}
	// Venus 组 1 的金币数量就是源里的 1 亿（`0 100000000`：模板 0 = 金币，不是"零个模板 0"）。
	if high, ok := rules.CreateMaterials(100354181, 1); !ok || high[len(high)-1].Amount != 100000000 {
		t.Fatalf("Venus create group 1 = %+v, want a trailing gold of 100000000", high)
	}
	// 不猜、不回落：不存在的模板与组号都必须 false。
	if _, ok := rules.CreateMaterials(999999, 0); ok {
		t.Fatal("an unregistered template must not resolve")
	}
	if _, ok := rules.CreateMaterials(100354181, 7); ok {
		t.Fatal("a missing create group must not resolve")
	}
	// 制作演出时长（客户端播，服务端只用于回包时机对照实验）。
	// ⚠️ 这两个值写在节标题**同一行**（`[create movie time] 18000 18000`），读节内行会得到 0
	// —— 2026-10-02 那次延迟实验静默失效就栽在这里，这里钉死防复发。
	for template, want := range map[uint32][2]int{
		100354181: {18000, 1200},
		100391142: {13500, 1500},
		100346156: {18800, 2100},
	} {
		info, ok := rules.Info(template)
		if !ok {
			t.Fatalf("%d is missing from the source", template)
		}
		if info.CreateMovieTime != want[0] || info.CreateWaitTime != want[1] {
			t.Fatalf("%d create movie/wait = %d/%d, want %d/%d",
				template, info.CreateMovieTime, info.CreateWaitTime, want[0], want[1])
		}
	}
}
