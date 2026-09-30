package loot

import (
	"testing"

	"dfolan/internal/catalog"
)

// [MERGE-20260928-DUNGEON-GROUP-INDEX] 115 深渊三个真副本走的是**另一条**路径：
// 它们在 etc/dungeondropinfo.cos 里有条目（带品级与率），所以优先命中 RollDungeonGroups，
// 不落到 [normal group index]。
//
// 数据（只读观察）：
//
//	100003295/100003296/100003297  [type] dgn_hell
//	组 10900 (epic, 53 物) / 21030 (epic, 11) / 10012..10026 11085~11087 (stackable)
func TestAbyssHellThreeRealDungeons(t *testing.T) {
	cat, e := catalog.LoadLoot("../../configs/loot.level150.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []uint32{100003295, 100003296, 100003297} {
		if !cat.IsAbyssDungeon(id) {
			t.Errorf("副本 %d 应被识别为深渊(dgn_hell)", id)
		}
		entries, ok := cat.DropInfoByID(id)
		if !ok || len(entries) == 0 {
			t.Errorf("副本 %d 在 dungeondropinfo 里没有条目", id)
			continue
		}
		kinds := map[string]int{}
		for _, en := range entries {
			kinds[en.Grade]++
		}
		t.Logf("副本 %d：dungeondropinfo 条目 %d 条，品级分布 %v", id, len(entries), kinds)
	}

	// 组表必须可读：10900 / 21030 是两个 epic 大组。
	for _, gid := range []uint32{10900, 21030} {
		g, ok := cat.DropGroupByID(gid)
		if !ok {
			t.Errorf("深渊组 %d 不可读", gid)
			continue
		}
		t.Logf("组 %d: explicit=%d smart=%d", gid, len(g.Explicit), len(g.Smart))
		if len(g.Explicit)+len(g.Smart) == 0 {
			t.Errorf("深渊组 %d 是空的", gid)
		}
	}

	// 走 dungeondropinfo 路径实际抽一次。
	//
	// 难度必须按副本各自的档位给：三个副本的 [rate list] 是错开的 ——
	// 100003295 的率在档 0，100003296 在档 1，100003297 在档 2。全用 0 抽会让
	// 后两个看起来「0 件」，那是调用参数错，不是掉落错。
	for _, tc := range []struct {
		id         uint32
		difficulty int
	}{
		{100003295, 0},
		{100003296, 1},
		{100003297, 2},
	} {
		produced := 0
		for seed := uint32(1); seed <= 50; seed++ {
			out, ok, e := RollDungeonGroups(cat, Rules{}, seed*2654435761,
				DungeonGroupDropRequest{DungeonID: tc.id, MonsterKind: "boss", Difficulty: tc.difficulty, Rarity: -1})
			if e != nil {
				t.Fatalf("副本 %d 难度 %d: %v", tc.id, tc.difficulty, e)
			}
			if !ok {
				t.Fatalf("副本 %d 应命中 dungeondropinfo 路径", tc.id)
			}
			produced += len(out.Awards)
		}
		t.Logf("副本 %d：50 个种子 boss 难度%d 共产出 %d 件", tc.id, tc.difficulty, produced)
		if produced == 0 {
			t.Errorf("副本 %d 难度 %d 一件都没出", tc.id, tc.difficulty)
		}
	}
}
