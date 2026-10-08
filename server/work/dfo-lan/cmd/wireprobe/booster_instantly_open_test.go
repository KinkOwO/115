package main

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/loot"
)

// 源标记规则（next176 §19.3）：**只有带 `[instantly open]` 的 booster 由服务端代开**，
// 没带标记的是「玩家自己在客户端开」的那种（光辉意志/星蕴石袖珍罐，常带抽奖演出），
// 必须**原样落地** —— 所以它要同时满足三件事：RewardBox 不认、Container 不认、Item 认。
//
// 判据完全来自源（catalog.BoosterDefinition.InstantlyOpen），不再靠硬编名单。
func TestInstantlyOpenMarkerDecidesWhetherTheServerUnwraps(t *testing.T) {
	pool := func(item uint32) []catalog.BoosterRewardPool {
		return []catalog.BoosterRewardPool{{
			DrawCount:  1,
			Candidates: []catalog.BoosterRewardCandidate{{Template: item, Weight: 1000, Count: 1}},
		}}
	}
	cat := &BoosterCatalog{
		Definitions: map[uint32]catalog.BoosterDefinition{
			1001: {Template: 1001, Type: "[booster]", Pools: pool(2001), InstantlyOpen: true},
			1002: {Template: 1002, Type: "[booster]", Pools: pool(2002)},
		},
		Items: map[uint32]catalog.ItemIndexEntry{
			1001: {ID: 1001, Kind: "stackable", StackableType: "[booster]"},
			1002: {ID: 1002, Kind: "stackable", StackableType: "[booster]"},
			2001: {ID: 2001, Kind: "stackable"},
			2002: {ID: 2002, Kind: "stackable"},
		},
	}
	legacy := boosterBoxSource{catalog: cat}
	byMarker := boosterBoxSource{catalog: cat, instantlyOpenOnly: true}

	// 旧行为保持不变（回归保护）：两个都当包装拆。
	for _, id := range []uint32{1001, 1002} {
		if _, ok := legacy.RewardBox(id); !ok {
			t.Fatalf("旧行为下 %d 应当可开", id)
		}
	}

	// 源标记规则：即开的照拆；玩家自开的三个断言缺一不可。
	if _, ok := byMarker.RewardBox(1001); !ok {
		t.Fatal("带 [instantly open] 的应当由服务端代开")
	}
	if _, ok := byMarker.RewardBox(1002); ok {
		t.Fatal("没有 [instantly open] 的不该由服务端代开")
	}
	if byMarker.Container(1002) {
		t.Fatal("没有 [instantly open] 的不能算「打不开的盒子」—— 那会被直接丢掉")
	}
	if !byMarker.Item(1002) {
		t.Fatal("它应当作为普通物品落地")
	}

	// 端到端：走一遍展开，罐子必须留在产物里；即开的必须变成内容物。
	out, _, _ := loot.OpenRewardBoxes(1, byMarker, []loot.Award{{Template: 1002, Amount: 1}})
	if len(out) != 1 || out[0].Template != 1002 {
		t.Fatalf("玩家自开的罐子必须原样落地：%+v", out)
	}
	out, _, _ = loot.OpenRewardBoxes(1, byMarker, []loot.Award{{Template: 1001, Amount: 1}})
	if len(out) != 1 || out[0].Template != 2001 {
		t.Fatalf("即开的必须被代开成内容物：%+v", out)
	}
}
