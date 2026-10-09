package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

// 真实形状取自 100004137 的 `.dgn`（2026-10-09 直读，见 analysis/tasks/next190 §D/§E），
// 这里只保留两个难度块与最小可判字段。
func rewardDeclCells() []pvf.Token {
	cells := []pvf.Token{
		{Type: 3, Text: "[difficulty dropitem group list]"},
		{Type: 3, Text: "[custom group info]"},
		{Type: 3, Text: "[contents]"},
		{Type: 6, Text: "equipment guide"},
		{Type: 3, Text: "[item index]"},
		{Type: 0, Value: 10326880}, {Type: 0, Value: 10326884},
		{Type: 0, Value: 10403422}, {Type: 0, Value: 10419051},
		{Type: 3, Text: "[normal group index]"},
		{Type: 0, Value: 2}, {Type: 0, Value: 21251}, {Type: 0, Value: 3}, {Type: 0, Value: 1}, {Type: 0, Value: 21291},
		{Type: 3, Text: "[fame info]"}, {Type: 0, Value: 0}, {Type: 0, Value: 0},
		{Type: 3, Text: "[special setinfo reward]"}, {Type: 8, Reference: "<2::SetEquipmentReward>"},
		{Type: 0, Value: 320}, {Type: 0, Value: 10326880}, {Type: 0, Value: 1}, {Type: 0, Value: 21279},
		{Type: 3, Text: "[special setinfo reward]"}, {Type: 8, Reference: "<2::SetOathPrimerReward>"},
		{Type: 0, Value: 320}, {Type: 0, Value: 10326880}, {Type: 0, Value: 1}, {Type: 0, Value: 21468},
		{Type: 3, Text: "[special setinfo reward]"}, {Type: 8, Reference: "<2::RareEquipmentReward>"},
		{Type: 0, Value: 320}, {Type: 0, Value: 10326880}, {Type: 0, Value: 1}, {Type: 0, Value: 21470},
		{Type: 3, Text: "[special setinfo reward]"}, {Type: 8, Reference: "<2::WeaponEquipmentReward>"},
		{Type: 0, Value: 138}, {Type: 0, Value: 10326880}, {Type: 0, Value: 1}, {Type: 0, Value: 21310},
		{Type: 3, Text: "[special custom reward info]"}, {Type: 8, Reference: "<2::MonsterCard_ExpectationReward_0>"},
		{Type: 0, Value: 320}, {Type: 0, Value: 10326884}, {Type: 0, Value: 1}, {Type: 0, Value: 21292},
		{Type: 3, Text: "[reward multiple info]"}, {Type: 0, Value: 2}, {Type: 0, Value: 7}, {Type: 0, Value: 6},
		{Type: 3, Text: "[reward multiple info]"}, {Type: 0, Value: 3}, {Type: 0, Value: 3}, {Type: 0, Value: 2},
		{Type: 3, Text: "[/group info]"},
		{Type: 3, Text: "[custom group info]"},
		{Type: 3, Text: "[contents]"},
		{Type: 6, Text: "normal"},
		{Type: 3, Text: "[item index]"},
		{Type: 0, Value: 10326880}, {Type: 0, Value: 10326884},
		{Type: 0, Value: 10408728}, {Type: 0, Value: 10419053},
		{Type: 3, Text: "[normal group index]"},
		{Type: 0, Value: 2}, {Type: 0, Value: 21251}, {Type: 0, Value: 3}, {Type: 0, Value: 1}, {Type: 0, Value: 21291},
		{Type: 3, Text: "[reward multiple info]"}, {Type: 0, Value: 2}, {Type: 0, Value: 7},
		{Type: 3, Text: "[reward multiple info]"}, {Type: 0, Value: 3}, {Type: 0, Value: 3},
		{Type: 3, Text: "[/custom group info]"},
		{Type: 3, Text: "[/difficulty dropitem group list]"},
	}
	return cells
}

func TestRewardDeclarationsReadBothBlocks(t *testing.T) {
	blocks := rewardDeclarations(rewardDeclCells())
	if len(blocks) != 2 {
		t.Fatalf("块数 = %d，期望 2", len(blocks))
	}
	first := blocks[0]
	if first.Contents != "equipment guide" {
		t.Fatalf("第一块 [contents] = %q", first.Contents)
	}
	if len(first.Item) != 4 || first.Item[0] != 10326880 || first.Item[3] != 10419051 {
		t.Fatalf("第一块 [item index] = %v", first.Item)
	}
	// 组索引按「个数+组号」原样保留，解码归 internal/loot。
	wantGroups := []int32{2, 21251, 3, 1, 21291}
	if len(first.Groups) != len(wantGroups) {
		t.Fatalf("第一块 [normal group index] = %v", first.Groups)
	}
	for i, v := range wantGroups {
		if first.Groups[i] != v {
			t.Fatalf("第一块组索引[%d] = %d，期望 %d", i, first.Groups[i], v)
		}
	}
	// 5 条基础产物：名字/率/列表件/数量/组号/来源标签。
	if len(first.Special) != 5 {
		t.Fatalf("基础产物条数 = %d，期望 5", len(first.Special))
	}
	want := []struct {
		name  string
		rate  uint32
		list  uint32
		group uint32
		custom bool
	}{
		{"SetEquipmentReward", 320, 10326880, 21279, false},
		{"SetOathPrimerReward", 320, 10326880, 21468, false},
		{"RareEquipmentReward", 320, 10326880, 21470, false},
		{"WeaponEquipmentReward", 138, 10326880, 21310, false},
		{"MonsterCard_ExpectationReward_0", 320, 10326884, 21292, true},
	}
	for i, w := range want {
		got := first.Special[i]
		if got.Name != w.name || got.Rate != w.rate || got.List != w.list || got.Group != w.group || got.Custom != w.custom || got.Count != 1 {
			t.Fatalf("第 %d 条基础产物 = %+v，期望 %+v", i, got, w)
		}
	}
	// 效率：类别 2 = 装备（普通 7 / 匹配 6）、类别 3 = 誓约·星蕴石（普通 3 / 匹配 2）。
	if len(first.Multiple) != 2 {
		t.Fatalf("效率条数 = %d，期望 2", len(first.Multiple))
	}
	if m := first.Multiple[0]; m.Kind != 2 || m.Normal != 7 || m.Matching != 6 {
		t.Fatalf("效率[0] = %+v", m)
	}
	if m := first.Multiple[1]; m.Kind != 3 || m.Normal != 3 || m.Matching != 2 {
		t.Fatalf("效率[1] = %+v", m)
	}
	// 第二块（[contents] normal）只给单倍数值：Matching 保持 0 =「源里没写」。
	second := blocks[1]
	if second.Contents != "normal" {
		t.Fatalf("第二块 [contents] = %q", second.Contents)
	}
	if len(second.Multiple) != 2 || second.Multiple[0].Normal != 7 || second.Multiple[0].Matching != 0 {
		t.Fatalf("第二块效率 = %+v", second.Multiple)
	}
	if second.Multiple[1].Normal != 3 || second.Multiple[1].Matching != 0 {
		t.Fatalf("第二块效率[1] = %+v", second.Multiple[1])
	}
	if len(second.Special) != 0 {
		t.Fatalf("第二块没有基础产物声明，却读到 %d 条", len(second.Special))
	}
}

// 形状不符必须保持空，绝不用默认值把别的副本行为改掉。
func TestRewardDeclarationsRejectShiftedShapes(t *testing.T) {
	if got := rewardDeclarations(nil); got != nil {
		t.Fatalf("空 cells 应返回 nil，得到 %v", got)
	}
	// 没有 [difficulty dropitem group list]：整段不读。
	noList := []pvf.Token{
		{Type: 3, Text: "[group info]"},
		{Type: 3, Text: "[special setinfo reward]"}, {Type: 8, Reference: "<2::X>"},
		{Type: 0, Value: 1}, {Type: 0, Value: 2}, {Type: 0, Value: 3}, {Type: 0, Value: 4},
	}
	if got := rewardDeclarations(noList); got != nil {
		t.Fatalf("缺 [difficulty dropitem group list] 时应为 nil，得到 %v", got)
	}
	// 基础产物少了最后一格（组号）⇒ 整条丢弃，不是补 0。
	short := []pvf.Token{
		{Type: 3, Text: "[difficulty dropitem group list]"},
		{Type: 3, Text: "[special setinfo reward]"}, {Type: 8, Reference: "<2::X>"},
		{Type: 0, Value: 320}, {Type: 0, Value: 10326880}, {Type: 0, Value: 1},
		{Type: 3, Text: "[/difficulty dropitem group list]"},
	}
	// 没有块开标签时连块都不建 —— 判据是「这条声明没被采纳到哪里」，不是块数。
	for _, b := range rewardDeclarations(short) {
		if len(b.Special) != 0 {
			t.Fatalf("缺格的基础产物不应被采纳：%+v", b)
		}
	}
	// [reward multiple info] 只有一个数（连类别+倍数都凑不齐）⇒ 不采纳。
	one := []pvf.Token{
		{Type: 3, Text: "[difficulty dropitem group list]"},
		{Type: 3, Text: "[reward multiple info]"}, {Type: 0, Value: 7},
		{Type: 3, Text: "[/difficulty dropitem group list]"},
	}
	for _, b := range rewardDeclarations(one) {
		if len(b.Multiple) != 0 {
			t.Fatalf("单值效率不应被采纳：%+v", b)
		}
	}
}

func TestStringKey(t *testing.T) {
	cases := map[string]string{
		"<13::name_10326880>":               "name_10326880",
		"<2::MonsterCard_ExpectationReward_0>": "MonsterCard_ExpectationReward_0",
		"plain":                             "",
		"<no-separator>":                    "",
		"":                                  "",
	}
	for in, want := range cases {
		if got := stringKey(in); got != want {
			t.Fatalf("stringKey(%q) = %q，期望 %q", in, got, want)
		}
	}
}
