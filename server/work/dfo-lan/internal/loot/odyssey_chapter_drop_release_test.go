package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"testing"
)

// The shipped chapter box table must be loadable and must only claim boxes the
// selection box catalog really defines: ValidateBoxes runs at gateway startup
// and a template it cannot prove aborts the process instead of dropping.
func TestOdysseyChapterDropReleaseConfig(t *testing.T) {
	d, e := LoadOdysseyChapterDrop("../../configs/odyssey-chapter-drop-release.json")
	if e != nil {
		t.Fatal(e)
	}
	boxes, e := catalog.LoadSelectionBoxes("../../configs/selection-boxes-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = d.ValidateBoxes(boxes); e != nil {
		t.Fatal("shipped chapter drop table would abort startup:", e)
	}
	if !d.Enabled() {
		t.Fatal("no chapter drop is enabled: the chapter final lord pays no equipment box")
	}
	enabled := 0
	for _, line := range d.Drops {
		if line.Enabled {
			enabled++
		}
	}
	t.Logf("chapter drop: %d/7 enabled; catalog proves %d selection boxes", enabled, len(boxes.Boxes))
}

// End-to-end on the source's own numbers: the chapter final lord (the
// [hunt boss] row, rank 3) pays the odyssey coin and, once its line is
// enabled, the chapter equipment box. A disabled line stays inert - it must
// not even consume a roll seed.
func TestOdysseyChapterFinalLordDrop(t *testing.T) {
	d, e := LoadOdysseyChapterDrop("../../configs/odyssey-chapter-drop-release.json")
	if e != nil {
		t.Fatal(e)
	}
	coins, e := LoadOdysseyCurrency("../../configs/odyssey-currency.json")
	if e != nil {
		t.Fatal(e)
	}
	lootCatalog, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	rules, e := LoadRules("../../configs/drop.current36.json")
	if e != nil {
		t.Fatal(e)
	}
	tables, e := Parse(lootCatalog)
	if e != nil {
		t.Fatal(e)
	}
	equipment, e := inventory.LoadEquipmentCatalog("../../configs/equipment.current37.json", lootCatalog.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	dc, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-scenes-release.json")
	if e != nil {
		t.Fatal(e)
	}
	checked := 0
	for _, line := range d.Drops {
		if !line.Enabled {
			// A disabled line must not consume a seed and must award nothing.
			award, next, err := d.Roll(0x12345678, line.Final)
			if err != nil || len(award) != 0 || next != 0x12345678 {
				t.Fatalf("disabled chapter %d is not inert: %v %v %d", line.Chapter, award, err, next)
			}
			continue
		}
		def, ok := dc.Dungeons[line.Final]
		if !ok || !def.Odyssey || def.HuntBoss == 0 {
			t.Fatalf("chapter %d final %d is not an odyssey hunt dungeon", line.Chapter, line.Final)
		}
		if def.BasisLevel == 0 || def.BasisLevel > 255 {
			t.Fatalf("chapter %d final %d basis level %d", line.Chapter, line.Final, def.BasisLevel)
		}
		lord := protocol.DungeonMonster{Entity: 4096, Template: def.HuntBoss, Level: byte(def.BasisLevel), Rank: 3, Team: 100}
		run := &dungeon.Session{
			RunID:      "0123456789abcdef0123456789abcdef",
			Loaded:     true,
			Definition: def,
			Room:       catalog.DungeonRoom{Map: 1},
			Monsters:   []protocol.DungeonMonster{lord},
			Dead:       map[uint16]bool{lord.Entity: true},
			NextEntity: lord.Entity + 1,
		}
		session := NewSession(lootCatalog, tables, rules, equipment, run.RunID, 1, 1, 1)
		session.Currency = coins
		session.ChapterDrop = d
		rows, e := session.Death(run, lord.Entity)
		if e != nil {
			t.Fatalf("chapter %d final lord drop: %v", line.Chapter, e)
		}
		box, coin := false, false
		for _, drop := range rows {
			found, ok := session.Objects[drop.Object]
			if !ok {
				continue
			}
			switch found.Award.Template {
			case line.Template:
				box = true
			case coins.Templates[3]:
				coin = true
			}
		}
		if !box {
			t.Fatalf("chapter %d final lord dropped no equipment box %d", line.Chapter, line.Template)
		}
		if !coin {
			t.Fatalf("chapter %d final lord dropped no odyssey coin %d", line.Chapter, coins.Templates[3])
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no enabled chapter checked")
	}
	t.Logf("chapter drop PASS: %d enabled chapters pay box + coin on their final lord", checked)
}
