package main

import (
	"encoding/binary"
	"sort"
	"testing"

	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
)

// TestAbyssTablesStayInsideTheirDungeons 钉住「通用掉落 vs 深渊专属」这条边界。
//
// 两件事一起证：
//
//  1. 调律（深渊）奖励表只对**自己声明的副本号**生效。这里故意把一个普通副本
//     接上深渊表（l.Attunement = a）再跑，产物里不允许出现表里的任何一个模板 ——
//     专属奖励不能漏进普通地图。
//  2. 统一包装展开对普通副本是惰性的：源在**没有包装**时不动随机数（见
//     OpenRewardBoxes），所以普通副本的产物与旧行为逐字节相同。
//
// 第 2 条正是这次改动的要害：展开从「只挂在调律那一块」提到「所有来源统一一次」，
// 必须同时证明它没把深渊的东西带进普通地图。
func TestAbyssTablesStayInsideTheirDungeons(t *testing.T) {
	env := loadOmenTestEnv(t)
	a := env.a

	exclusive := map[uint32]bool{}
	for _, id := range a.Templates() {
		exclusive[id] = true
	}

	// 挑一个既不是深渊、也不是奥德赛章节的普通副本；按 ID 排序，让选择是确定的。
	ids := make([]uint32, 0, len(env.dc.Dungeons))
	for id := range env.dc.Dungeons {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var pick uint32
	for _, id := range ids {
		if exclusive[id] {
			continue
		}
		def := env.dc.Dungeons[id]
		if def.Odyssey || def.SourceBoss != 0 || len(def.Mazes) == 0 {
			continue
		}
		s, err := dungeon.Select(env.dc, protocol.DungeonSelection{ID: id, Difficulty: 1, Party: 65535}, 115, nil)
		if err != nil || len(s.Monsters) == 0 {
			continue
		}
		pick = id
		break
	}
	if pick == 0 {
		t.Skip("no ordinary dungeon in this catalog to compare against")
	}

	rules, err := loot.LoadRules("../../configs/drop.compat90.json")
	if err != nil {
		t.Fatal(err)
	}
	tables, err := loot.Parse(env.lc)
	if err != nil {
		t.Fatal(err)
	}

	const runs = 6
	leaked := map[uint32]int{}
	containers := map[uint32]int{}
	rows := 0
	for i := 0; i < runs; i++ {
		s, err := dungeon.Select(env.dc, protocol.DungeonSelection{ID: pick, Difficulty: 1, Party: 65535}, 115, nil)
		if err != nil {
			t.Fatal(err)
		}
		s.Loaded = true
		l := loot.NewSession(env.lc, tables, rules, env.gear, s.RunID, 1, 11, 11)
		// 故意把深渊表接上：它必须对这个副本完全惰性。
		l.Attunement = a
		l.RewardBoxes = env.boxes
		for _, m := range s.Monsters {
			if _, e := s.ConfirmDeath(uint32(m.Entity), 11, 11); e != nil {
				continue
			}
			got, e := l.Death(s, m.Entity)
			if e != nil {
				continue
			}
			for _, r := range got {
				template := binary.LittleEndian.Uint32(r.Item[2:6])
				rows++
				if exclusive[template] {
					leaked[template]++
				}
				if env.boxes.Container(template) {
					containers[template]++
				}
			}
		}
	}
	if rows == 0 {
		t.Fatalf("dungeon %d paid no row in %d runs: the comparison proves nothing", pick, runs)
	}
	if len(leaked) > 0 {
		t.Fatalf("abyss-exclusive templates leaked into ordinary dungeon %d: %v", pick, countByTemplate(leaked))
	}
	if len(containers) > 0 {
		t.Fatalf("ordinary dungeon %d put a wrapped box on the ground: %v", pick, countByTemplate(containers))
	}
	t.Logf("ordinary dungeon %d: %d row(s) over %d run(s), no abyss exclusive, no wrapped box",
		pick, rows, runs)
}
