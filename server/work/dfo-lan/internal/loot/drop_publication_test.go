package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"reflect"
	"strings"
	"testing"
)

func TestAttunementCapacityFailureDoesNotConsumeRewards(t *testing.T) {
	for _, id := range []uint32{100005066, 100005014} {
		t.Run(fmtDungeonID(id), func(t *testing.T) {
			a, err := LoadAttunementRewards(attunementConfig)
			if err != nil {
				t.Fatal(err)
			}
			d := &dungeon.Session{RunID: "audit", Loaded: true, NextEntity: 65534, Dead: map[uint16]bool{1: true}, Monsters: []protocol.DungeonMonster{{Entity: 1, Template: 123, Level: 1, Rank: 3}}}
			d.Definition.ID = id
			d.Definition.SourceBoss = 123
			d.Definition.Script.Cells = []pvf.Token{{Type: 3, Text: "[exclude gold drop]"}, {Type: 3, Text: "[exclude monster random drop]"}}
			s := NewSession(catalog.LootCatalog{}, Tables{}, Rules{}, nil, "audit", 1, 1, 1)
			s.Attunement = a
			s.OathTier = 45
			if id == 100005014 {
				s.Omen = NewOmenLedger(a)
				s.Omen.Set(1, 4)
			}
			s.seeds[d.Room.Map] = 42
			_, err = s.Death(d, 1)
			if err == nil || !strings.Contains(err.Error(), "drop identity exhausted") {
				t.Fatalf("unexpected error %v", err)
			}
			if s.attunementRolled || s.omenRolled || s.oathTierRolled || len(s.Objects) > 0 || s.seeds[d.Room.Map] != 42 {
				t.Fatal("failed publication consumed reward state")
			}
			if s.Omen != nil {
				if s.Omen.Held(1) != 4 {
					t.Fatal("omen consumed on failure")
				}
				if _, ok := s.Omen.Last(1); ok {
					t.Fatal("failed omen published")
				}
			}
			d.NextEntity = 1
			rows, err := s.Death(d, 1)
			if err != nil || len(rows) == 0 {
				t.Fatalf("retry lost rewards: %d %v", len(rows), err)
			}
			if !s.attunementRolled || !s.oathTierRolled {
				t.Fatal("successful publication uncommitted")
			}
			if s.Omen != nil {
				out, ok := s.Omen.Last(1)
				if !ok || out.Seq != 1 || !out.Paid || s.Omen.Held(1) != 0 {
					t.Fatal("omen settlement not committed exactly once")
				}
			}
			again, err := s.Death(d, 1)
			if err != nil || !reflect.DeepEqual(rows, again) {
				t.Fatal("retry duplicates/rerolls rewards")
			}
		})
	}
}
func fmtDungeonID(id uint32) string {
	if id == 100005014 {
		return "with_omen"
	}
	return "border"
}
func TestOmenPreviewRejectsConcurrentChange(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	o := NewOmenLedger(a)
	o.Set(1, 0)
	out, _, err := o.preview(1, 100005014, 42)
	if err != nil {
		t.Fatal(err)
	}
	o.Set(1, 2)
	if o.commit(1, out) == nil {
		t.Fatal("concurrent clear overwritten")
	}
	if o.Held(1) != 2 {
		t.Fatal("concurrent state lost")
	}
}

func TestDisabledOmenPublicationRemainsInert(t *testing.T) {
	o := NewOmenLedger(nil)
	out, awards, err := o.preview(1, 100005066, 42)
	if err != nil || len(awards) != 0 || out.Seed != 42 {
		t.Fatal("disabled omen preview changed")
	}
	if err = o.commit(1, out); err != nil {
		t.Fatal(err)
	}
	if o.seq != 0 || len(o.last) != 0 {
		t.Fatal("disabled omen acquired settlement state")
	}
}
