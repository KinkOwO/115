package main

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/loot"
)

// 拆不拆的判据（2026-10-08 修正，见 analysis/tasks/next179）：
// **只有声明了客户端开箱入口的才原样落地，其余一律由服务端代开。**
//
//	ClientOpenPath（`[oath item booster]` / `[lottery ani info]`，如光辉意志）⇒ 原样落地，玩家自己开
//	两个标记都没有的一般礼盒（如 `10419728` 史诗砝码的史诗档）⇒ **代开**
//	带 `[instantly open]`（波动箱 / 虚无之魂 / 太初星蕴石）⇒ 代开
//
// 旧判据「只有 `[instantly open]` 才代开」的漏洞：`10419728` 这类两个标记都没有的盒子被原样落地，
// 而客户端没有开箱入口 ⇒ 玩家拿到死盒子、掉不出装备（业主 2026-10-08 实机反馈）。
func TestServerUnwrapUnlessTheClientHasAnOpenPath(t *testing.T) {
	pool := func(item uint32) []catalog.BoosterRewardPool {
		return []catalog.BoosterRewardPool{{
			DrawCount:  1,
			Candidates: []catalog.BoosterRewardCandidate{{Template: item, Weight: 1000, Count: 1}},
		}}
	}
	cat := &BoosterCatalog{
		Definitions: map[uint32]catalog.BoosterDefinition{
			1001: {Template: 1001, Type: "[booster]", Pools: pool(2001), InstantlyOpen: true},
			1002: {Template: 1002, Type: "[booster]", Pools: pool(2002), ClientOpenPath: true},
			1003: {Template: 1003, Type: "[booster]", Pools: pool(2003)},
		},
		Items: map[uint32]catalog.ItemIndexEntry{
			1001: {ID: 1001, Kind: "stackable", StackableType: "[booster]"},
			1002: {ID: 1002, Kind: "stackable", StackableType: "[booster]"},
			1003: {ID: 1003, Kind: "stackable", StackableType: "[booster]"},
			2001: {ID: 2001, Kind: "stackable"},
			2002: {ID: 2002, Kind: "stackable"},
			2003: {ID: 2003, Kind: "stackable"},
		},
	}
	legacy := boosterBoxSource{catalog: cat}
	byMarker := boosterBoxSource{catalog: cat, serverUnwrap: true}

	// 旧行为保持不变（回归保护）：三个都当包装拆。
	for _, id := range []uint32{1001, 1002, 1003} {
		if _, ok := legacy.RewardBox(id); !ok {
			t.Fatalf("旧行为下 %d 应当可开", id)
		}
	}

	// 新判据：即开的代开、两个标记都没有的也要代开、只有客户端有入口的留给玩家。
	if _, ok := byMarker.RewardBox(1001); !ok {
		t.Fatal("带 [instantly open] 的应当由服务端代开")
	}
	if _, ok := byMarker.RewardBox(1003); !ok {
		t.Fatal("两个标记都没有的一般礼盒必须代开 —— 否则是玩家开不了的死盒子（10419728 就是这个）")
	}
	if _, ok := byMarker.RewardBox(1002); ok {
		t.Fatal("声明了客户端开箱入口的不该由服务端代开")
	}
	// 留给玩家的那个必须三件事同时成立：RewardBox 不认、Container 不认、Item 认。
	if byMarker.Container(1002) {
		t.Fatal("不能算「本 build 打不开的盒子」—— 那会被直接丢掉")
	}
	if !byMarker.Item(1002) {
		t.Fatal("它应当作为普通物品落地")
	}

	// 端到端：玩家自开的原样落地；另两个必须变成内容物。
	out, _, _ := loot.OpenRewardBoxes(1, byMarker, []loot.Award{{Template: 1002, Amount: 1}})
	if len(out) != 1 || out[0].Template != 1002 {
		t.Fatalf("玩家自开的罐子必须原样落地：%+v", out)
	}
	for _, id := range []uint32{1001, 1003} {
		out, _, _ = loot.OpenRewardBoxes(1, byMarker, []loot.Award{{Template: id, Amount: 1}})
		if len(out) != 1 || out[0].Template != id+1000 {
			t.Fatalf("模板 %d 必须被代开成内容物：%+v", id, out)
		}
	}
}
