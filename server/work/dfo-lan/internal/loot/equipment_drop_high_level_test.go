package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"testing"
)

// High-level dungeons must have gear to drop.
//
// 2026-09-23 实机：奥德赛 100004940（basis level 55）打完 112 次死亡，装备 0 件，
// 日志里留下 `drop_rules_pending / equipment_grade_window_empty`。原因不是概率，
// 而是掉落池里根本没有那个等级的装备：
//   - 装备目录 grade>=22 的 2430 件 rarity 全部 >=1，而掉落池准入当时拒绝 rarity>1；
//   - rarity=0 的 744 件又全在 grade<=20；
//     ⇒ 池子只剩 grade<=20 的 1534 件，等级 >=22 的副本一件都掉不出来。
//
// 这里把 level 53/113 的 BOSS 掉落当作回归闸门：必须掉出装备，且不再有空窗口。
func TestEquipmentDropsAboveLowLevelDungeons(t *testing.T) {
	c, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	tables, e := Parse(c)
	if e != nil {
		t.Fatal(e)
	}
	rules, e := LoadRules("../../configs/drop.compat90.json")
	if e != nil {
		t.Fatal(e)
	}
	gear, e := inventory.LoadEquipmentCatalog("../../configs/equipment.current37.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	pool := gear.DropPool()
	usable := map[uint32]bool{}
	high := 0
	for _, d := range pool {
		usable[d.ID] = true
		if d.Grade >= 22 {
			high++
		}
	}
	if high == 0 {
		t.Fatalf("drop pool carries no gear above grade 21: %d entries", len(pool))
	}
	// diff 1 = 奥德赛的 [designated difficulty] 2（客户端难度 1 起算）。
	for _, level := range []byte{53, 113} {
		dropped, empty := 0, 0
		for seed := uint32(0); seed < 2000; seed++ {
			out, err := RollWithBonus(c, tables, rules, pool, seed*2654435761, level, 3, 1, 0)
			if err != nil {
				t.Fatalf("level %d: %v", level, err)
			}
			for _, a := range out.Awards {
				if usable[a.Template] && c.Items[a.Template].Kind != "stackable" {
					dropped++
				}
			}
			for _, k := range out.SkippedKinds {
				if k == "equipment_grade_window_empty" {
					empty++
				}
			}
		}
		if dropped == 0 {
			t.Fatalf("level %d dropped no gear at all across 2000 seeds", level)
		}
		if empty != 0 {
			t.Fatalf("level %d still hits an empty grade window %d times", level, empty)
		}
		t.Logf("level %d: gear=%d/2000, empty=0, pool=%d (grade>=22: %d)", level, dropped, len(pool), high)
	}
}
