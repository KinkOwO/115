package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentHellPartyRulesAndPillarReferences(t *testing.T) {
	p := os.Getenv("DFO_LOOT_PVF")
	if p == "" {
		p = filepath.Join("..", "..", "..", "client-build", "Script.inner.pvf")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip("current PVF absent")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1024 * 1024 * 1024}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	imported, err := ImportHellPartyRules(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint16{125, 126, 127, 128, 33, 35, 42, 43, 44} {
		for _, actor := range imported.Groups[id].Actors {
			source, ok := imported.Actors[actor]
			if !ok || source.Unavailable != "" || len(source.SHA256) != 64 || actor.EntityType == 1 && source.Level == 0 {
				t.Fatalf("legacy group %d actor %v unavailable: %+v", id, actor, source)
			}
		}
	}
	if source := imported.Actors[HellPartyActor{Template: 10627, EntityType: 1}]; source.Level != 65 {
		t.Fatalf("source APC level changed: %+v", source)
	}
	for id := uint32(1050); id <= 1053; id++ {
		if !imported.Actors[HellPartyActor{Template: id}].HellMonster {
			t.Fatalf("cosmofiend %d lost [hell monster] flag", id)
		}
	}
	dropScript, err := ResolveScript(a, "etc/itemdropinfo_monster_hell.etc")
	if err != nil {
		t.Fatal(err)
	}
	drops, err := ParseHellPartyDropTable(dropScript)
	if err != nil || len(drops.Probability) != 2 || drops.Probability[1] != ([7]uint32{45, 200, 335, 875, 1000, 1400, 1000}) || drops.Rarity[0][4] != 1000000 {
		t.Fatalf("Hell drop source differs: %+v %v", drops, err)
	}
	s, err := ResolveScript(a, "etc/hellparty.etc")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := ParseHellPartyRules(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules.Difficulties) != 4 || rules.Difficulties[0].Key != "A" || rules.Difficulties[0].Columns != [5]uint32{8, 5, 100, 100, 0} || rules.SHA256 != s.SHA256 {
		t.Fatalf("difficulty source changed: %+v", rules.Difficulties)
	}
	for id, template := range map[uint16]uint32{125: 1050, 126: 1051, 127: 1052, 128: 1053} {
		g := rules.Groups[id]
		if g.Difficulty != "A" || len(g.Actors) != 1 || g.Actors[0] != (HellPartyActor{Template: template, EntityType: 0}) {
			t.Fatalf("group %d: %+v", id, g)
		}
	}
	g := rules.Groups[44]
	if len(g.Actors) != 6 || g.Actors[0] != (HellPartyActor{Template: 10627, EntityType: 1}) || g.Actors[5] != (HellPartyActor{Template: 56717, EntityType: 0}) {
		t.Fatalf("mixed group lost: %+v", g)
	}
	if g := rules.Groups[236]; len(g.Actors) != 1 || g.Actors[0].EntityType != 3 {
		t.Fatalf("unknown native actor type coerced: %+v", g)
	}
	index, err := ResolveScript(a, "list/map.lst")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ParseIndex(index.Cells)
	if err != nil {
		t.Fatal(err)
	}
	var mapScript ScriptRecord
	for _, row := range rows {
		if row.ID == 60051 {
			mapScript, err = ResolveScript(a, row.Path)
			break
		}
	}
	if err != nil || len(mapScript.Cells) == 0 {
		t.Fatalf("seal map unavailable: %v", err)
	}
	pillars, err := ParseHellPartyPillars(mapScript)
	if err != nil {
		t.Fatal(err)
	}
	if len(pillars) != 1 || pillars[0].Object != 30528 || len(pillars[0].Choices) != 18 {
		t.Fatalf("seal choices: %+v", pillars)
	}
	counts := map[uint16]int{}
	for _, choice := range pillars[0].Choices {
		if _, ok := rules.Groups[choice.Group]; !ok {
			t.Fatalf("unresolved group %d", choice.Group)
		}
		counts[choice.Order]++
	}
	if counts[1] != 13 || counts[2] != 5 {
		t.Fatalf("wave partition lost: %+v", counts)
	}
	// A missing group terminator must never consume the next group's actors.
	broken := s
	broken.Cells = append([]pvf.Token(nil), s.Cells...)
	for i, cell := range broken.Cells {
		if cell.Type == 3 && cell.Text == "[/group]" {
			broken.Cells = append(broken.Cells[:i], broken.Cells[i+1:]...)
			break
		}
	}
	if _, err := ParseHellPartyRules(broken); err == nil {
		t.Fatal("unterminated group accepted")
	}
	broken = mapScript
	broken.Cells = append([]pvf.Token(nil), mapScript.Cells...)
	for i, cell := range broken.Cells {
		if cell.Type == 6 && cell.Text == "[hellparty]" {
			broken.Cells[i+3].Value = 10
			break
		}
	}
	if _, err := ParseHellPartyPillars(broken); err == nil {
		t.Fatal("native order limit ignored")
	}
}
