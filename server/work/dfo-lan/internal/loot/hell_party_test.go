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

func hellLootFixture(t *testing.T) (catalog.LootCatalog, *inventory.EquipmentCatalog, *dungeon.Session) {
	t.Helper()
	gear, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}, Rows: []inventory.EquipmentDefinition{{ID: 200, Path: "equipment/test.equ", SHA256: strings.Repeat("b", 64), Fields: map[string][]pvf.Token{"[rarity]": {{Type: 0, Value: 1}}, "[grade]": {{Type: 0, Value: 62}}, "[equipment type]": {{Type: 6, Text: "[coat]"}}, "[attach type]": {{Type: 6, Text: "[free]"}}, "[durability]": {{Type: 0, Value: 35}}}}}}, strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	gear.OrdinaryPool = []inventory.EquipmentDrop{{ID: 200, Grade: 62, Rarity: 1, Weight: 1}}
	gear.HellPartyPool = []inventory.EquipmentDrop{{ID: 200, Grade: 62, Rarity: 1, Weight: 1}}
	c := catalog.LootCatalog{HellPartyDropPercent: 100, HellPartyDrop: &catalog.HellPartyDropTable{Probability: [][7]uint32{{45, 200, 1000, 875, 0, 0, 0}}, Rarity: [][9]uint32{{0, 1000000, 1000000, 1000000, 1000000, 1000001, 1000001, 1000002, 1000003}, {0, 1000000, 1000000, 1000000, 1000000, 1000001, 1000001, 1000002, 1000003}}}}
	rows := []protocol.DungeonMonster{{Entity: 4100, Template: 1050, Level: 62, Team: 100, Hidden: true, SpawnOrder: 1}, {Entity: 4101, Template: 10627, Level: 65, Team: 100, Hidden: true, SpawnOrder: 2, APC: true, Rank: 5, SourceIndex: 10000}, {Entity: 4102, Template: 10627, Level: 65, Team: 100, Hidden: true, SpawnOrder: 2, APC: true, Rank: 5, SourceIndex: 10000}}
	d := &dungeon.Session{RunID: "hell-test", Loaded: true, Difficulty: 1, NextEntity: 4103, HellPosition: &[2]byte{0, 0}, Room: catalog.DungeonRoom{Map: 60051}, Definition: catalog.DungeonDefinition{ID: 87, MinimumLevel: 60, BasisLevel: 62}, Monsters: rows, Dead: map[uint16]bool{}, HellParty: &dungeon.HellPartyRun{Map: 60051, Mode: 1, Rows: rows, Actors: map[uint16]dungeon.HellPartyRunActor{4100: {Group: 125, Order: 1, RewardRolls: 8, HellMonster: true}, 4101: {Group: 44, Order: 2, RewardRolls: 8}, 4102: {Group: 44, Order: 2, RewardRolls: 8}}}}
	return c, gear, d
}

func TestHellRewardOnlyLastOwnedGroupAndDeathReplay(t *testing.T) {
	c, gear, d := hellLootFixture(t)
	s := NewSession(c, Tables{}, Rules{}, gear, d.RunID, 1, 2, 3)
	s.seeds[60051] = 42
	for _, id := range []uint32{4100, 4101, 4102} {
		if fresh, err := d.ConfirmDeath(id, 3, 3); err != nil || !fresh {
			t.Fatal(err)
		}
		rows, err := s.Death(d, uint16(id))
		if err != nil {
			t.Fatal(err)
		}
		if id != 4102 && len(rows) != 0 || id == 4102 && len(rows) != 8 {
			t.Fatalf("wrong group reward: entity=%d rows=%d", id, len(rows))
		}
		if id == 4102 {
			seed, next := s.seeds[60051], d.NextEntity
			again, err := s.Death(d, uint16(id))
			if err != nil || !reflect.DeepEqual(again, rows) || s.seeds[60051] != seed || d.NextEntity != next || len(s.Objects) != 8 {
				t.Fatal("replayed death rerolled/duplicated rewards")
			}
			if rows[0].Item[11] != 35 {
				t.Fatal("Hell gear lost source durability")
			}
		}
	}
	for _, drop := range s.Objects {
		if drop.Award.Template != 200 || drop.Award.Amount != 1 {
			t.Fatal("ordinary/Abyss table contaminated Hell award")
		}
	}
}

func TestHellLastUnownedDeathPaysNothing(t *testing.T) {
	c, gear, d := hellLootFixture(t)
	s := NewSession(c, Tables{}, Rules{}, gear, d.RunID, 1, 2, 3)
	for _, id := range []uint32{4100, 4101, 4102} {
		killer := uint16(3)
		if id == 4102 {
			killer = 65535
		}
		if _, err := d.ConfirmDeath(id, killer, 3); err != nil {
			t.Fatal(err)
		}
		if rows, err := s.Death(d, uint16(id)); err != nil || len(rows) != 0 {
			t.Fatalf("unowned/intermediate death paid: %v", err)
		}
	}
	if !d.HellPartyCleared() || len(s.Objects) != 0 {
		t.Fatal("unowned clear was not acknowledged without reward")
	}
}

func TestHellMultiplierDifficultyAndCreationWeights(t *testing.T) {
	c, _, d := hellLootFixture(t)
	d.Difficulty = 2
	pool := []inventory.EquipmentDrop{{ID: 1, Grade: 62, Rarity: 1, Weight: 1}, {ID: 2, Grade: 62, Rarity: 1, Weight: 9}, {ID: 3, Grade: 62, Rarity: 1}, {ID: 4, Grade: 80, Rarity: 1, Weight: 100}}
	actor := dungeon.HellPartyRunActor{RewardRolls: 1}
	hits, heavy := 0, 0
	for i := uint32(1); i <= 10000; i++ {
		out, err := rollHellParty(c, pool, i*2654435761, d, actor)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range out.Awards {
			hits++
			if a.Template == 2 {
				heavy++
			}
			if a.Template > 2 {
				t.Fatal("zero creation/out of grade gear was selected")
			}
		}
	}
	if hits < 8500 || hits > 9000 || heavy < hits*85/100 || heavy > hits*95/100 {
		t.Fatalf("source probability/weights: %d %d", hits, heavy)
	}
	for _, percent := range []uint32{0, 100} {
		c.HellPartyDropPercent = percent
		d.Difficulty = 3
		out, err := rollHellParty(c, pool, 42, d, actor)
		if err != nil || len(out.Awards) != 0 || out.NextSeed != 42 {
			t.Fatal("zero rate consumed a reward roll")
		}
	}
	if p, err := ParseHellPartyDropPercent(""); err != nil || p != 100 {
		t.Fatal("default is not 1x")
	}
	if _, err := ParseHellPartyDropPercent("-1"); err == nil {
		t.Fatal("negative multiplier accepted")
	}
}
