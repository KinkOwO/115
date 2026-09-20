package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestSourceBossRequiresSeparateStoryDeathAndCheck(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, early := range []bool{false, true} {
		s, e := Select(c, protocol.DungeonSelection{ID: 3, Party: 65535, Quest: 3145}, 1, map[uint16]bool{3145: true})
		if e != nil {
			t.Fatal(e)
		}
		check := protocol.BossCheckRequest{Actor: 3, Target: s.Monsters[0].Entity}
		s.Loaded = true
		if e = s.BossCheck(check, 3); e == nil {
			t.Fatal("nonboss room completion")
		}
		for _, xy := range [][2]byte{{1, 1}, {1, 0}, {2, 0}, {3, 0}} {
			for _, m := range s.Monsters {
				if !m.NonCombat {
					_, e = s.ConfirmDeath(uint32(m.Entity), 3, 3)
					if e != nil {
						t.Fatal(e)
					}
				}
			}
			s, e = s.Move(c, xy)
			if e != nil {
				t.Fatal(e)
			}
			s.Loaded = true
		}
		check.Target = s.Monsters[0].Entity
		if e = s.BossCheck(check, 4); e == nil {
			t.Fatal("wrong actor")
		}
		unknown := check
		unknown.Target = 60000
		if e = s.BossCheck(unknown, 3); e == nil {
			t.Fatal("unknown boss")
		}
		if early {
			if e = s.BossCheck(check, 3); e != nil {
				t.Fatal(e)
			}
		}
		for _, m := range s.Monsters {
			if !m.NonCombat {
				_, e = s.ConfirmDeath(uint32(m.Entity), 3, 3)
				if e != nil {
					t.Fatal(e)
				}
			}
		}
		if !s.RoomCleared() || s.Completed() {
			t.Fatal("ordinary clear bypassed story boss")
		}
		if !early {
			if e = s.BossCheck(check, 3); e != nil {
				t.Fatal(e)
			}
		}
		if s.Completed() {
			t.Fatal("boss check fabricated dummy death")
		}
		_, e = s.ConfirmDeath(uint32(s.Monsters[2].Entity), 3, 3)
		if e != nil {
			t.Fatal(e)
		}
		if !s.Completed() || s.CompletionTarget() != check.Target {
			t.Fatal("final confirmation failed")
		}
		if e = s.BossCheck(check, 3); e != nil {
			t.Fatal("identical replay", e)
		}
		other := check
		other.Target = s.Monsters[2].Entity
		if e = s.BossCheck(other, 3); e == nil {
			t.Fatal("conflicting replay")
		}
		if _, e = s.Move(c, [2]byte{2, 0}); e == nil {
			t.Fatal("left completed boss room by normal gate")
		}
	}
}

func TestOdysseyBossCheckImmediateCompletion(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-scenes-release.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := Select(c, protocol.DungeonSelection{ID: 100004969, Difficulty: 2, Party: 65535}, 89, nil)
	if e != nil {
		t.Fatal(e)
	}
	s.Loaded = true
	// BossCheck in Odyssey should be immediately admitted and complete the dungeon
	check := protocol.BossCheckRequest{Actor: 10, Target: 9999}
	if err := s.BossCheck(check, 10); err != nil {
		t.Fatalf("expected Odyssey BossCheck to pass, got: %v", err)
	}
	if !s.Completed() || s.CompletionTarget() != 9999 {
		t.Fatalf("expected dungeon to be completed with target 9999, got completed=%v target=%d", s.Completed(), s.CompletionTarget())
	}
}
