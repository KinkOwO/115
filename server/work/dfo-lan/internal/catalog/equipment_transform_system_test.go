package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// 合成用例：把源里用到的每一种段形状都覆盖一遍（不依赖 PVF，也不依赖导出物）。
const transformSystemSample = `[need materials]
 [info]
  [condition] 115 ` + "`rare`" + `
  [cost]
   [group] 1
    0 25000
    10361512 1
   [/group]
   [group] 2
    10401346 5
    10361512 1
   [/group]
  [/cost]
 [/info]
[/need materials]
[refund materials]
 115 ` + "`rare`" + `   0 1 10361512 1
 115 ` + "`primeval`" + ` 0 2 10361516 1 10403609 1
[/refund materials]
[need amalgamation materials]
 [info]
  [condition] 115 ` + "`unique`" + `
  [cost]
   [group] 1
    0 30000
   [/group]
   [group] 2
    10401346 6
   [/group]
  [/cost]
 [/info]
[/need amalgamation materials]
[refund amalgamation materials]
 115 ` + "`unique`" + ` 0 3 1 10404330 100
[/refund amalgamation materials]
[need primer materials]
 [info]
  [condition] 115 ` + "`rare`" + `
  [cost]
   [group] 1
    0 25000
   [/group]
   [group] 2
    10401346 5
   [/group]
  [/cost]
 [/info]
 [info]
  [condition] 115 ` + "`epic`" + `
  [cost]
   [group] 1
    0 40000
   [/group]
   [group] 2
    10401346 8
   [/group]
  [/cost]
 [/info]
[/need primer materials]
[refund primer materials]
 115 ` + "`rare`" + ` 0 1 10415190 1
 115 ` + "`rare`" + ` 1 0
 115 ` + "`epic`" + ` 0 1 10415190 30
 115 ` + "`epic`" + ` 1 0
[/refund primer materials]
`

func TestParseEquipmentTransformSystemSample(t *testing.T) {
	s, e := ParseEquipmentTransformSystem(transformSystemSample)
	if e != nil {
		t.Fatalf("parse: %v", e)
	}
	if len(s.EquipmentNeed) != 1 || len(s.PrimerNeed) != 2 {
		t.Fatalf("need rows = equipment %d / primer %d, want 1 / 2", len(s.EquipmentNeed), len(s.PrimerNeed))
	}
	// 装备变换 rare：两支付法各自要什么。
	opt, ok := s.Payment(TransformChainEquipment, "rare", 1)
	if !ok || opt.Gold != 25000 || len(opt.Items) != 1 ||
		opt.Items[0].Template != 10361512 || opt.Items[0].Count != 1 {
		t.Fatalf("equipment rare option 1 = %+v ok=%v", opt, ok)
	}
	opt, ok = s.Payment(TransformChainEquipment, "rare", 2)
	if !ok || opt.Gold != 0 || len(opt.Items) != 2 ||
		opt.Items[0].Template != 10401346 || opt.Items[0].Count != 5 ||
		opt.Items[1].Template != 10361512 {
		t.Fatalf("equipment rare option 2 = %+v ok=%v", opt, ok)
	}
	if _, ok := s.Payment(TransformChainEquipment, "rare", 3); ok {
		t.Fatal("option 3 must not exist")
	}
	if _, ok := s.Payment(TransformChainEquipment, "epic", 1); ok {
		t.Fatal("epic is not in this sample's equipment need table")
	}

	// 晶体/誓约变换（CMD2381）：只有金币 / 巡礼之印两支，**没有灵魂项**。
	opt, ok = s.Payment(TransformChainPrimer, "rare", 1)
	if !ok || opt.Gold != 25000 || len(opt.Items) != 0 {
		t.Fatalf("primer rare option 1 = %+v ok=%v, want gold 25000 and no items", opt, ok)
	}
	opt, ok = s.Payment(TransformChainPrimer, "rare", 2)
	if !ok || opt.Gold != 0 || len(opt.Items) != 1 ||
		opt.Items[0].Template != 10401346 || opt.Items[0].Count != 5 {
		t.Fatalf("primer rare option 2 = %+v ok=%v", opt, ok)
	}

	// 返还：分支 0 有料、分支 1 空（源里五档都如此）。
	items, ok := s.RefundFor(TransformChainPrimer, "epic", 0)
	if !ok || len(items) != 1 || items[0].Template != 10415190 || items[0].Count != 30 {
		t.Fatalf("primer epic refund branch 0 = %+v ok=%v", items, ok)
	}
	items, ok = s.RefundFor(TransformChainPrimer, "epic", 1)
	if !ok || len(items) != 0 {
		t.Fatalf("primer epic refund branch 1 = %+v ok=%v, want empty", items, ok)
	}
	// primeval 的装备返还多一件 10403609（太初星辉）。
	items, ok = s.RefundFor(TransformChainEquipment, "primeval", 0)
	if !ok || len(items) != 2 || items[0].Template != 10361516 || items[1].Template != 10403609 {
		t.Fatalf("equipment primeval refund = %+v ok=%v", items, ok)
	}

	// `[refund amalgamation materials]` 多一列标签：必须被识别为「有标签」而不是当成件数。
	if len(s.AmalgamationRefund) != 1 {
		t.Fatalf("amalgamation refund rows = %d, want 1", len(s.AmalgamationRefund))
	}
	row := s.AmalgamationRefund[0]
	if !row.HasTag || row.Tag != 3 || row.Branch != 0 || len(row.Items) != 1 ||
		row.Items[0].Template != 10404330 || row.Items[0].Count != 100 {
		t.Fatalf("amalgamation refund row = %+v", row)
	}
}

// 缺段 / 形状不对必须报错，不能静默给出半张表。
func TestParseEquipmentTransformSystemRejectsBroken(t *testing.T) {
	cases := map[string]string{
		"缺 need materials":   "[need primer materials]\n [info]\n  [condition] 115 `rare`\n  [cost]\n   [group] 1\n    0 1\n   [/group]\n  [/cost]\n [/info]\n[/need primer materials]\n",
		"缺 refund materials": "[need materials]\n [info]\n  [condition] 115 `rare`\n  [cost]\n   [group] 1\n    0 1\n   [/group]\n  [/cost]\n [/info]\n[/need materials]\n[need primer materials]\n [info]\n  [condition] 115 `rare`\n  [cost]\n   [group] 1\n    0 1\n   [/group]\n  [/cost]\n [/info]\n[/need primer materials]\n",
		"缺 condition":        "[need materials]\n [info]\n  [cost]\n   [group] 1\n    0 1\n   [/group]\n  [/cost]\n [/info]\n[/need materials]\n",
		"组行不是模板+数量":          "[need materials]\n [info]\n  [condition] 115 `rare`\n  [cost]\n   [group] 1\n    0\n   [/group]\n  [/cost]\n [/info]\n[/need materials]\n",
		"返还行件数与模板数不符":        "[need materials]\n [info]\n  [condition] 115 `rare`\n  [cost]\n   [group] 1\n    0 1\n   [/group]\n  [/cost]\n [/info]\n[/need materials]\n[refund materials]\n 115 `rare` 0 2 10361512 1\n[/refund materials]\n",
	}
	for name, text := range cases {
		if _, e := ParseEquipmentTransformSystem(text); e == nil {
			t.Fatalf("%s: must be rejected", name)
		}
	}
}

// 稀有度名 ↔ `[rarity]` 码值：由源自身互证（10361512..16 的 .stk [rarity] = 2/3/6/4/8）。
func TestTransformRarityName(t *testing.T) {
	want := map[int32]string{2: "rare", 3: "unique", 6: "legendary", 4: "epic", 8: "primeval"}
	for code, name := range want {
		got, ok := TransformRarityName(code)
		if !ok || got != name {
			t.Fatalf("rarity %d -> %q ok=%v, want %q", code, got, ok, name)
		}
	}
	if _, ok := TransformRarityName(5); ok {
		t.Fatal("rarity 5 has no source name")
	}
}

// 真实源用例：数据来自 runtime/pvfinspect-primer/ 的导出（gitignore，缺失就跳过）。
// 它钉住整张表的关键数值 —— 改口径之前先看这里为什么是这些数。
func TestParseEquipmentTransformSystemRealSource(t *testing.T) {
	path := filepath.Join("..", "..", "runtime", "pvfinspect-primer", "lv1",
		"00-equipmenttransformsystem.cos.txt")
	b, e := os.ReadFile(path)
	if e != nil {
		t.Skipf("真实源不在（先跑 pvfinspect 导出）: %v", e)
	}
	s, e := ParseEquipmentTransformSystem(string(b))
	if e != nil {
		t.Fatalf("parse real source: %v", e)
	}
	// 三条链各 5 / 2 / 5 档（源里 [need amalgamation materials] 只有 unique 与 epic）。
	if len(s.EquipmentNeed) != 5 || len(s.AmalgamationNeed) != 2 || len(s.PrimerNeed) != 5 {
		t.Fatalf("need rows = %d/%d/%d, want 5/2/5",
			len(s.EquipmentNeed), len(s.AmalgamationNeed), len(s.PrimerNeed))
	}
	// 金币阶梯：25000/30000/35000/40000/50000 —— 旧 Go 常量恒 50000 就是在这里出错的。
	gold := map[string]uint32{
		"rare": 25000, "unique": 30000, "legendary": 35000, "epic": 40000, "primeval": 50000,
	}
	for rarity, want := range gold {
		opt, ok := s.Payment(TransformChainPrimer, rarity, 1)
		if !ok || opt.Gold != want || len(opt.Items) != 0 {
			t.Fatalf("primer %s option 1 = %+v ok=%v, want gold %d", rarity, opt, ok, want)
		}
		opt, ok = s.Payment(TransformChainEquipment, rarity, 1)
		if !ok || opt.Gold != want || len(opt.Items) != 1 {
			t.Fatalf("equipment %s option 1 = %+v ok=%v, want gold %d + 1 soul", rarity, opt, ok, want)
		}
	}
	// 晶体变换的返还：10415190（微光灵魂）×1/2/10/30/100，分支 1 全空。
	refund := map[string]uint32{"rare": 1, "unique": 2, "legendary": 10, "epic": 30, "primeval": 100}
	for rarity, count := range refund {
		items, ok := s.RefundFor(TransformChainPrimer, rarity, 0)
		if !ok || len(items) != 1 || items[0].Template != 10415190 || items[0].Count != count {
			t.Fatalf("primer %s refund branch 0 = %+v ok=%v, want 10415190 x%d", rarity, items, ok, count)
		}
		if items, ok = s.RefundFor(TransformChainPrimer, rarity, 1); !ok || len(items) != 0 {
			t.Fatalf("primer %s refund branch 1 = %+v ok=%v, want empty", rarity, items, ok)
		}
	}
	// 巡礼之印支：10401346 × 5/6/7/8/10。
	seals := map[string]uint32{"rare": 5, "unique": 6, "legendary": 7, "epic": 8, "primeval": 10}
	for rarity, count := range seals {
		opt, ok := s.Payment(TransformChainPrimer, rarity, 2)
		if !ok || opt.Gold != 0 || len(opt.Items) != 1 ||
			opt.Items[0].Template != 10401346 || opt.Items[0].Count != count {
			t.Fatalf("primer %s option 2 = %+v ok=%v, want 10401346 x%d", rarity, opt, ok, count)
		}
	}
	// 三个段都必须有原文哈希：ImportEquipmentTransformSystem 会把它写进结构体，
	// 这里只确认 Parse 也会算（两条入口的 SHA256 语义一致）。
	if len(s.SHA256) != 64 || s.Bytes == 0 {
		t.Fatalf("provenance = %q bytes=%d", s.SHA256, s.Bytes)
	}
}
