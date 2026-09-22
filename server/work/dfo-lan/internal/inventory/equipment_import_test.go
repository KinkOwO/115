package inventory

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

// 真源里大量装备是"薄壳"：只写 [name]/[attach type]/[usable period]/[value]/
// [import script]，靠继承另一件的基础字段。例如
//
//	equipment/character/common/jacket/cloth/100050791.equ
//	  [import script] `character/common/jacket/cloth/100050666.equ`
//
// 自身不带 [rarity]/[equipment type]/[durability]。不跟随这条链，发放会以
// "special equipment reward requires additional source state" 或
// "missing source equipment durability" 失败 —— 实机诊断
// （cmd/wireprobe/selection_box_audit_test.go）显示 78 个自选盒 / 543 件装备因此发不出去。
func TestRewardFollowsImportScript(t *testing.T) {
	const src = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	full, err := OpenFullEquipmentCatalog("../../configs/equipment-full", src)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	c := &EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: src}, Full: full}

	const shell, base = uint32(100050791), uint32(100050666)
	d, err := c.Definition(shell)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Fields["[rarity]"]) > 0 || len(d.Fields["[durability]"]) > 0 {
		t.Fatalf("样本已变：%d 不再是薄壳（rarity %d 值 / durability %d 值）",
			shell, len(d.Fields["[rarity]"]), len(d.Fields["[durability]"]))
	}
	if got := importTarget(d.Fields["[import script]"]); got != base {
		t.Fatalf("[import script] 指向 %d，期望 %d", got, base)
	}
	want, err := c.Reward(base)
	if err != nil {
		t.Fatalf("基础装备 %d 本身发不出去: %v", base, err)
	}
	got, err := c.Reward(shell)
	if err != nil {
		t.Fatalf("薄壳装备 %d 发放失败（[import script] 没被跟随）: %v", shell, err)
	}
	if got != want {
		t.Fatalf("薄壳 %d 的耐久 %d 与基础 %d 的 %d 不一致", shell, got, base, want)
	}
}
