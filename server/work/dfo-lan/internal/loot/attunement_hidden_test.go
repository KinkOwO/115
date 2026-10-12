package loot

import "testing"

// 命运的凝械（hidden drop table）不是掷出来的，是玩家在 CMD 2406 的弹窗里**选**出来的。
// 两个下标的语义由 2026-10-12 三个实机数据点钉住（正文 = 4×u32，前两个有效）：
//
//	(2,2) 首饰 + Hideout's Endless Gold      14:36 会话 …_143513_
//	(2,3) 首饰 + Dragon Arena Uprising       14:41 会话 …_144043_
//	(1,2) 防具 + Hideout's Endless Gold      15:36 会话 …_153521_
//
// 部位不变而套装变 ⇒ 第 1 个 u32 是部位、第 2 个是套装；部位的取值与客户端自己那串
// 下拉文本同序（dstr 101038052..55 = Weapon/Armor/Accessory/Special Equipment = 0..3），
// 和隐藏表 [index] 逐格对得上。下面几条把这个读法钉死。

func TestHiddenSelectionMatchesTheLivePopupPicks(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		maze     uint32
		part     uint32
		set      uint32
		want     uint32
		wantTier string
	}{
		{"accessory hideouts gold", 1, 2, 2, 10410316, "epic"},
		{"accessory dragon arena", 1, 2, 3, 10410317, "epic"},
		{"armor hideouts gold", 1, 1, 2, 10410340, "epic"},
		{"armor deep shadow", 1, 1, 0, 10410338, "epic"},
		{"special magic domain", 1, 3, 11, 10410361, "epic"},
		{"primeval accessory hideouts gold", 2, 2, 2, 10410328, "primeval"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sel, ok := a.HiddenSelection(100005068, c.maze, c.part, c.set)
			if !ok {
				t.Fatalf("maze %d part %d set %d not found", c.maze, c.part, c.set)
			}
			if sel.Carrier != c.want || sel.Tier != c.wantTier {
				t.Fatalf("got carrier %d tier %q, want %d %q", sel.Carrier, sel.Tier, c.want, c.wantTier)
			}
			if sel.Maze != c.maze || sel.Part != c.part || sel.Set != c.set {
				t.Fatalf("subscripts echoed wrong: %+v", sel)
			}
		})
	}
}

// 越界一律「没有这一格」，不回绕：把 set=12 折回 key 0 会让玩家选到的套装和脚下掉的那件
// 无关，而那正是这一整个功能要修的东西。
func TestHiddenSelectionRefusesUndeclaredCells(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name            string
		dungeon         uint32
		maze, part, set uint32
	}{
		{name: "set beyond the twelve", dungeon: 100005068, maze: 1, part: 2, set: 12},
		{name: "weapon row has no set choice", dungeon: 100005068, maze: 1, part: 0, set: 1},
		{name: "primeval offers no armor", dungeon: 100005068, maze: 2, part: 1, set: 0},
		{name: "primeval offers no special gear", dungeon: 100005068, maze: 2, part: 3, set: 0},
		{name: "not an attunement dungeon", dungeon: 100000001, maze: 1, part: 2, set: 2},
	} {
		t.Run(c.name, func(t *testing.T) {
			if sel, ok := a.HiddenSelection(c.dungeon, c.maze, c.part, c.set); ok {
				t.Fatalf("expected not found, got %+v", sel)
			}
		})
	}
}

// 源里声明的每一格都必须能选到（反过来就是：表读漏了一格，实机表现是某个套装永远拿不到）。
func TestHiddenSelectionReachesEveryDeclaredCell(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	declared, found := 0, 0
	for _, dg := range a.Dungeons() {
		tbl := a.byDungeon[dg]
		if tbl == nil {
			t.Fatalf("dungeon %d not loaded", dg)
		}
		for _, h := range tbl.Hidden {
			keys := map[uint32]bool{}
			for _, e := range h.Entries {
				declared++
				if keys[e.Key] {
					t.Fatalf("dungeon %d maze %d index %d declares key %d twice", dg, h.Maze, h.Index, e.Key)
				}
				keys[e.Key] = true
				sel, ok := a.HiddenSelection(dg, h.Maze, h.Index, e.Key)
				if !ok {
					t.Fatalf("dungeon %d maze %d index %d key %d declared but not selectable", dg, h.Maze, h.Index, e.Key)
				}
				if sel.Carrier != e.Item || sel.Tier != e.Tier {
					t.Fatalf("dungeon %d maze %d index %d key %d: got %d/%q, source says %d/%q",
						dg, h.Maze, h.Index, e.Key, sel.Carrier, sel.Tier, e.Item, e.Tier)
				}
				found++
			}
		}
	}
	if found == 0 || found != declared {
		t.Fatalf("reached %d of %d declared hidden cells", found, declared)
	}
}
