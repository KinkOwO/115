package loot

import (
	"strings"
	"testing"

	"dfolan/internal/catalog"
)

// [AZURE-CLEAR-REWARD] 蔚蓝号的清关奖励**不由通用卡池决定** —— 副本脚本自己带
// `[disable clear reward]`，并把清单写在 `[difficulty dropitem group list]` 里。
//
// 实机 2026-10-04：结算面板能出、能翻牌，但产物是全局卡池随机出的低阶装备
// （"Old Street Armor Shoes" + 610 Gold）。根因就是这里没被读。
//
// 源（contents/2025/azuremain/dungeon/azuremain.dgn，已逐行核对）：
//
//	[gold card use] 0
//	[disable clear reward]                 ← 关掉通用清关奖励
//	[dungeon clear result]
//	  [reward card] 1
//	[/dungeon clear result]
//	[difficulty dropitem group list]
//	  [group info]
//	    [item index] 10326880 10326884 10403423 10419054
//	    [normal group index] 2 21251 3 1 21291   ← 编码未解，故意不读
//	    [special setinfo reward] 320 10326880 1 21279   ×4
//	    [special custom reward info] 320 10326884 1 21293
//	    [reward multiple info] 2 10 9          ← 第 3 项 ×10 = 面板「1000% efficiency」
//	    [reward multiple info] 3 4 3           ← 第 4 项 ×4  = 面板「400% efficiency」
//	  [/group info]
//	  [custom group info] [contents] `equipment guide` …（同清单）
//	  [custom group info] [contents] `normal`  …（10408729/10419056 变体）
//	  [custom group info] [contents] `matching` …（10408725/10419055 变体）
//	[/difficulty dropitem group list]
//
// 组队界面「Rewards」那一行 4 个图标 = `[item index]` 的 4 个模板，其中后两个带 ×10 / ×4。
//
//	DFO_PVF_CORE_TEST_ARCHIVE=D:\115us\server\work\client-build\Script.inner.pvf \
//	go test ./internal/loot/ -run AzureMainClearRewardIsSourced -count=1 -v
func TestAzureMainClearRewardIsSourced(t *testing.T) {
	a := catalog.OpenNativeArchive(t)
	rec, err := catalog.ReadScript(a, "contents/2025/azuremain/dungeon/azuremain.dgn")
	if err != nil {
		t.Fatal(err)
	}
	blocks, err := catalog.ParseDungeonDropBlocks(rec.Cells)
	if err != nil {
		t.Fatalf("ParseDungeonDropBlocks: %v", err)
	}
	if len(blocks) < 4 {
		t.Fatalf("blocks = %d, want the [group info] plus three [custom group info]", len(blocks))
	}
	if blocks[0].Kind != "[group info]" {
		t.Fatalf("first block kind = %q, want [group info]", blocks[0].Kind)
	}

	index := map[string][]int32{}
	multiples := map[string][][]int32{}
	for _, sec := range blocks[0].Sections {
		switch sec.Header {
		case "[item index]":
			index[sec.Header] = sec.Values
		case "[reward multiple info]":
			multiples[sec.Header] = append(multiples[sec.Header], sec.Values)
		}
	}

	wantItems := []int32{10326880, 10326884, 10403423, 10419054}
	got := index["[item index]"]
	if len(got) != len(wantItems) {
		t.Fatalf("[item index] = %v, want %v", got, wantItems)
	}
	for i := range wantItems {
		if got[i] != wantItems[i] {
			t.Fatalf("[item index][%d] = %d, want %d (got %v)", i, got[i], wantItems[i], got)
		}
	}

	gotMul := multiples["[reward multiple info]"]
	if len(gotMul) != 2 {
		t.Fatalf("[reward multiple info] rows = %v, want exactly 2", gotMul)
	}
	// 第 1 列是 [item index] 的下标，第 2 列是倍率（= 面板上的「N00% efficiency」）。
	wantMul := [][2]int32{{2, 10}, {3, 4}}
	for i, w := range wantMul {
		if len(gotMul[i]) < 2 || gotMul[i][0] != w[0] || gotMul[i][1] != w[1] {
			t.Fatalf("[reward multiple info] row %d = %v, want index %d ×%d", i, gotMul[i], w[0], w[1])
		}
	}
	// 倍率只能落在 [item index] 内，否则就是读错了表。
	for _, row := range gotMul {
		if int(row[0]) >= len(wantItems) {
			t.Fatalf("multiple index %d out of range for %d items", row[0], len(wantItems))
		}
	}

	labels := map[string]bool{}
	for _, b := range blocks[1:] {
		for _, sec := range b.Sections {
			if sec.Header == "[contents]" {
				for _, l := range sec.Labels {
					labels[l] = true
				}
			}
		}
	}
	for _, want := range []string{"equipment guide", "normal", "matching"} {
		if !labels[want] {
			t.Fatalf("missing [custom group info] [contents] %q (have %v)", want, labels)
		}
	}
}

// [AZURE-CLEAR-REWARD] AzureMainClearRewards 把上面那段脚本读成奖单。
// 期望值就是组队界面「Rewards」那一行的 4 个图标（后两个带 x10 / x4 角标）。
func TestAzureMainClearRewardsFromScript(t *testing.T) {
	a := catalog.OpenNativeArchive(t)
	rec, err := catalog.ReadScript(a, "contents/2025/azuremain/dungeon/azuremain.dgn")
	if err != nil {
		t.Fatal(err)
	}
	items, err := AzureMainClearRewards(catalog.DungeonDefinition{Script: rec})
	if err != nil {
		t.Fatalf("AzureMainClearRewards: %v", err)
	}
	want := []Award{
		{Template: 10326880, Amount: 1},
		{Template: 10326884, Amount: 1},
		{Template: 10403423, Amount: 10},
		{Template: 10419054, Amount: 4},
	}
	if len(items) != len(want) {
		t.Fatalf("items = %+v, want %+v", items, want)
	}
	for i := range want {
		if items[i] != want[i] {
			t.Fatalf("items[%d] = %+v, want %+v", i, items[i], want[i])
		}
	}
	// 空脚本必须报错，不能静默发出一份空奖单。
	if _, err := AzureMainClearRewards(catalog.DungeonDefinition{}); err == nil {
		t.Fatal("空脚本应当报错")
	}
}

// [AZURE-CLEAR-RESULT-DIFF] 对照沉月湖的副本脚本原始文本，看 [dungeon clear result]
// 一段的取值差在哪（业主建议的对照法）。
func TestClearResultSectionAzureVsMoon(t *testing.T) {
	a := catalog.OpenNativeArchive(t)
	for _, p := range []string{
		"contents/2025/azuremain/dungeon/azuremain.dgn",
		"contents/2025/moonlake/dungeon/2f/2f_100004137.dgn",
		"contents/2025/moonlake/dungeon/1f/1f_100004136.dgn",
	} {
		txt, err := a.ReadText(p)
		if err != nil {
			t.Logf("%s: %v", p, err)
			continue
		}
		lines := strings.Split(txt, "\n")
		t.Logf("=== %s", p)
		for i, l := range lines {
			if strings.Contains(l, "dungeon clear result") || strings.Contains(l, "reward card") || strings.Contains(l, "result board") {
				t.Logf("  %4d| %s", i+1, strings.TrimRight(l, "\r"))
			}
		}
	}
}
