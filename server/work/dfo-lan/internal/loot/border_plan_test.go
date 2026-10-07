package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"reflect"
	"strings"
	"testing"
)

type borderTestBoxes map[uint32]uint32

func (b borderTestBoxes) RewardBox(id uint32) (RewardBox, bool) {
	item, ok := b[id]
	return RewardBox{Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: item, Weight: 1, Count: 1}}}}}, ok
}
func (b borderTestBoxes) Item(id uint32) bool      { return id == 101 || id == 102 }
func (b borderTestBoxes) Container(id uint32) bool { _, ok := b[id]; return ok }

func borderTestRun(t *testing.T, rarity int32) (*Session, *dungeon.Session) {
	t.Helper()
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	tab := a.byDungeon[100005068]
	tab.Fixed = []attunementFixed{{Maze: 0, Entries: []attunementEntry{{Tier: "epic", Weight: 1000000, Item: 100}}}}
	tab.Additional = []attunementAdditional{{SelectProb: 1000000, DropCount: 2, Entries: []attunementEntry{{Tier: "epic", Weight: 1000000, Item: 100}}}}
	if err = a.SetBorderMultipliers(5, 5); err != nil {
		t.Fatal(err)
	}
	eq, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}, Rows: []inventory.EquipmentDefinition{
		{ID: 101, Path: "test/epic.equ", SHA256: strings.Repeat("a", 64), Fields: map[string][]pvf.Token{"[rarity]": {{Type: 0, Value: rarity}}}},
		{ID: 102, Path: "test/primeval.equ", SHA256: strings.Repeat("b", 64), Fields: map[string][]pvf.Token{"[rarity]": {{Type: 0, Value: 8}}}},
	}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	d := &dungeon.Session{RunID: "border-test", NextEntity: 1, Monsters: []protocol.DungeonMonster{{Entity: 7, Template: 123, Rank: 3, Level: 1}}, Dead: map[uint16]bool{7: true}}
	d.Definition.ID = 100005068
	d.Definition.SourceBoss = 123
	d.Definition.Script.Cells = []pvf.Token{{Type: 3, Text: "[exclude gold drop]"}, {Type: 3, Text: "[exclude monster random drop]"}}
	s := NewSession(catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}}, Tables{}, Rules{}, eq, d.RunID, 1, 2, 3)
	s.Attunement = a
	s.RewardBoxes = borderTestBoxes{100: 101, oathTierCoffer(45): 102}
	return s, d
}

func TestBorderFrozenGradeAndAwardsSurviveRetry(t *testing.T) {
	for _, rarity := range []int32{4, 8} {
		s, d := borderTestRun(t, rarity)
		grade, err := s.PrepareBorderRewards(d, 42)
		want := uint32(44)
		if rarity == 8 {
			want = 45
		}
		if err != nil || grade != want || len(s.borderPlan.awards) != 15 {
			t.Fatalf("grade=%d err=%v", grade, err)
		}
		before := *s.borderPlan
		again, err := s.PrepareBorderRewards(d, 99999)
		if err != nil || again != grade || !reflect.DeepEqual(before, *s.borderPlan) {
			t.Fatal("loading retry rerolled")
		}
		if len(s.Objects) != 0 || s.attunementRolled {
			t.Fatal("preparation published rewards")
		}
		d.Loaded = true
		d.NextEntity = 65534
		if _, err = s.Death(d, 7); err == nil {
			t.Fatal("capacity failure accepted")
		}
		if s.attunementRolled || len(s.Objects) != 0 || !reflect.DeepEqual(before, *s.borderPlan) {
			t.Fatal("capacity failure consumed frozen plan")
		}
		d.NextEntity = 1
		rows, err := s.Death(d, 7)
		if err != nil || len(rows) != 15 {
			t.Fatalf("death: %d %v", len(rows), err)
		}
		for i, row := range rows {
			if s.Objects[row.Object].Award != before.awards[i] {
				t.Fatal("announced award changed")
			}
		}
		repeat, err := s.Death(d, 7)
		if err != nil || !reflect.DeepEqual(rows, repeat) || len(s.Objects) != 15 {
			t.Fatal("death replay duplicated rewards")
		}
	}
}

func TestBorderForecastIncludesIndependentOathReward(t *testing.T) {
	s, d := borderTestRun(t, 4)
	s.OathTier = 45
	grade, err := s.PrepareBorderRewards(d, 42)
	if err != nil || grade != 45 || len(s.borderPlan.awards) != 16 {
		t.Fatalf("oath missing: %d %v", grade, err)
	}
	d.Loaded = true
	if _, err = s.Death(d, 7); err != nil || !s.oathTierRolled {
		t.Fatalf("oath unpublished: %v", err)
	}
}

func TestBorderPlanRejectsRunMazeAndSourceChanges(t *testing.T) {
	s, d := borderTestRun(t, 4)
	if _, err := s.PrepareBorderRewards(d, 42); err != nil {
		t.Fatal(err)
	}
	d.Maze.Index++
	if _, err := s.PrepareBorderRewards(d, 42); err == nil {
		t.Fatal("foreign maze accepted")
	}
	d.Maze.Index--
	s.Catalog.Source.Checksum = "foreign"
	d.Loaded = true
	if _, err := s.Death(d, 7); err == nil || len(s.Objects) != 0 {
		t.Fatal("foreign source published")
	}
	if IsBorderDungeon(100005014) {
		t.Fatal("endkeeper changed")
	}
}
