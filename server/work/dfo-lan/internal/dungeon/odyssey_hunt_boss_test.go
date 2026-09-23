package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"os"
	"testing"
)

// The chapter target ([hunt boss]) is the only monster an Odyssey level pays
// its currency and its chapter equipment box on. The client reports its death
// with killer FFFF while the closing cinematic clears the room - live runs
// 2026-09-23T10:35:05 (dungeon 100004935) and 10:36:37 (100004936) both did -
// which used to mark it unowned: no loot, no experience, nothing at all for
// the chapter. Every other FFFF death (trash cleared with the boss) keeps the
// ordinary unowned rule.
func TestOdysseyHuntBossDeathIsOwned(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-scenes-release.json")
	if e != nil {
		t.Fatal(e)
	}
	d, ok := c.Dungeons[100004934]
	if !ok {
		t.Fatal("odyssey chapter level missing from catalog")
	}
	if !d.Odyssey || d.HuntBoss == 0 {
		t.Fatalf("dungeon 100004934 odyssey=%v hunt=%d", d.Odyssey, d.HuntBoss)
	}
	// 起始房间的杂兵：被 BOSS 连带清场（killer FFFF）时仍是无主，不发掉落与经验。
	start, e := Select(c, protocol.DungeonSelection{ID: 100004934, Difficulty: byte(d.DesignatedDifficulty), Party: 65535}, 115, nil)
	if e != nil {
		t.Fatal(e)
	}
	start.Loaded = true
	const actor uint16 = 14
	for _, m := range start.Monsters {
		if m.NonCombat || m.APC || m.Rank != 0 {
			continue
		}
		if _, err := start.ConfirmDeath(uint32(m.Entity), actor+1, actor); err == nil {
			t.Fatal("foreign combat killer accepted")
		}
		if _, err := start.ConfirmDeath(uint32(m.Entity), 65535, actor); err != nil {
			t.Fatal(err)
		}
		if !start.Unowned[m.Entity] {
			t.Fatalf("ordinary monster %d cleared with the boss must stay unowned", m.Entity)
		}
		t.Logf("trash %d (%#x) stays unowned under killer FFFF", m.Entity, m.Entity)
		break
	}
	s, e := Select(c, protocol.DungeonSelection{ID: 100004934, Difficulty: byte(d.DesignatedDifficulty), Party: 65535}, 115, nil)
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	for x := s.Room.X + 1; x < 6; x++ {
		next, err := s.Move(c, [2]byte{x, s.Room.Y})
		if err != nil {
			break
		}
		next.Loaded = true
		s = next
	}
	var hunt protocol.DungeonMonster
	found := false
	for _, m := range s.Monsters {
		if m.Rank == 3 && m.Template == d.HuntBoss && !m.NonCombat && !m.APC {
			hunt, found = m, true
		}
	}
	if !found {
		t.Fatal("boss room carries no live hunt boss")
	}
	fresh, e := s.ConfirmDeath(uint32(hunt.Entity), 65535, actor)
	if e != nil || !fresh {
		t.Fatal("hunt boss death refused", e)
	}
	if s.Unowned[hunt.Entity] {
		t.Fatal("hunt boss death marked unowned: chapter would pay no loot")
	}
	if !s.Dead[hunt.Entity] {
		t.Fatal("hunt boss death not recorded")
	}
	t.Logf("hunt boss %d (%#x) stays owned under killer FFFF", hunt.Entity, hunt.Entity)
}

// Every chapter final must really put a live rank-3 lord in front of the
// player: the chapter box roll fires on Rank==3 only. Layered finale scenes
// are part of the source maze, so every room map and every layer map is
// parsed, not just the room the run starts in.
func TestOdysseyChapterFinalsCarryHuntBoss(t *testing.T) {
	raw, e := os.ReadFile("../../configs/odyssey-chapter-drop-release.json")
	if e != nil {
		t.Fatal(e)
	}
	var drop struct {
		Drops []struct {
			Chapter  uint8  `json:"chapter"`
			Final    uint32 `json:"final"`
			Template uint32 `json:"template"`
			Enabled  bool   `json:"enabled"`
		} `json:"drops"`
	}
	if e = json.Unmarshal(raw, &drop); e != nil {
		t.Fatal(e)
	}
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-scenes-release.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, line := range drop.Drops {
		if !line.Enabled {
			continue
		}
		def, ok := c.Dungeons[line.Final]
		if !ok {
			t.Fatalf("chapter %d final %d missing from dungeon catalog", line.Chapter, line.Final)
		}
		if !def.Odyssey || def.HuntBoss == 0 {
			t.Fatalf("chapter %d final %d odyssey=%v hunt=%d", line.Chapter, line.Final, def.Odyssey, def.HuntBoss)
		}
		maps := map[uint32]bool{}
		for _, maze := range def.Mazes {
			for _, room := range maze.Rooms {
				maps[room.Map] = true
			}
			for _, layer := range maze.Layers {
				for _, id := range layer.Maps {
					maps[id] = true
				}
			}
		}
		lord := false
		for id := range maps {
			script, ok := c.Maps[id]
			if !ok {
				continue
			}
			ms, err := fixedMonsters(script, def.BasisLevel)
			if err != nil {
				continue
			}
			for _, m := range ms {
				if m.Rank == 3 && !m.NonCombat && !m.APC && m.Team == 100 {
					lord = true
				}
			}
		}
		if !lord {
			t.Fatalf("chapter %d final %d carries no live rank-3 lord", line.Chapter, line.Final)
		}
	}
}
