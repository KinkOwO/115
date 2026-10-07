package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"os"
	"testing"
)

// Explicit current-source regression: no player storage, sockets, or game UI.
func TestBorderRewardsCurrentPVF(t *testing.T) {
	path := os.Getenv("DFO_BORDER_PVF")
	if path == "" {
		t.Skip("set DFO_BORDER_PVF for current-source regression")
	}
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "items,equipment,boosters,attunement", ArchivePath: path}, gamedata.CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Equipment.Close()
	if err = c.Attunement.SetBorderMultipliers(5, 5); err != nil {
		t.Fatal(err)
	}
	boxes := boosterBoxSource{catalog: &BoosterCatalog{Definitions: c.Boosters, Items: c.Items.Items}}
	lc := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: c.SourceChecksum}, Items: map[uint32]catalog.LootItem{}}
	for id, item := range c.Items.Items {
		lc.Items[id] = catalog.LootItem{ID: id, Kind: item.Kind}
	}
	eq := &inventory.EquipmentCatalog{Source: c.Items.Source, Full: c.Equipment}
	counts := map[uint32]int{}
	for _, id := range []uint32{100005066, 100005067, 100005068} {
		for seed := uint32(1); seed <= 100; seed++ {
			d := &dungeon.Session{RunID: "native-border", Loaded: true, NextEntity: 1, Dead: map[uint16]bool{1: true}, Monsters: []protocol.DungeonMonster{{Entity: 1, Template: 123, Rank: 3, Level: 1}}}
			d.Definition.ID = id
			d.Definition.SourceBoss = 123
			d.Definition.Script.Cells = []pvf.Token{{Type: 3, Text: "[exclude gold drop]"}, {Type: 3, Text: "[exclude monster random drop]"}}
			s := loot.NewSession(lc, loot.Tables{}, loot.Rules{}, eq, d.RunID, 1, 1, 1)
			s.Attunement = c.Attunement
			s.RewardBoxes = boxes
			grade, err := s.PrepareBorderRewards(d, seed*2654435761)
			if err != nil {
				t.Fatalf("dungeon %d seed %d: %v", id, seed, err)
			}
			rows, err := s.Death(d, 1)
			if err != nil {
				t.Fatal(err)
			}
			actual := uint32(40)
			for _, row := range rows {
				drop := s.Objects[row.Object]
				if lc.Items[drop.Award.Template].Kind == "stackable" {
					continue
				}
				def, err := eq.Definition(drop.Award.Template)
				if err != nil {
					t.Fatal(err)
				}
				r := def.Fields["[rarity]"][0].Value
				var want uint32
				switch r {
				case 0, 1:
					want = 40
				case 2:
					want = 41
				case 3, 5:
					want = 42
				case 6:
					want = 43
				case 4:
					want = 44
				case 8:
					want = 45
				default:
					t.Fatalf("unsupported actual rarity %d", r)
				}
				if want > actual {
					actual = want
				}
			}
			if actual != grade {
				t.Fatalf("announced %d, paid %d", grade, actual)
			}
			counts[grade]++
		}
	}
	if counts[44] == 0 || counts[45] == 0 {
		t.Fatalf("epic/primeval branches unexercised: %v", counts)
	}
	t.Logf("source=%s runs=300 grade_counts=%v", c.SourceChecksum, counts)
}
