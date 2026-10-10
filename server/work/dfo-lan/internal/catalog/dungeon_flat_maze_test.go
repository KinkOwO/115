package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

func TestDungeonFlatSourceMazeAndExplicitPrecedence(t *testing.T) {
	body := []pvf.Token{section("[size]"), number(4), number(2), section("[map specification]"), label("map"), number(0), number(1), number(10), label("boss"), number(3), number(0), number(11), section("[/map specification]"), section("[start map]"), number(0), number(1), section("[/start map]"), section("[boss map]"), number(3), number(0), section("[/boss map]")}
	cells := append([]pvf.Token{section("[minimum required level]"), number(100)}, body...)
	d, err := ParseDungeon(1, ScriptRecord{Path: "flat.dgn", Cells: cells})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Mazes) != 1 || len(d.Mazes[0].Rooms) != 2 || d.Mazes[0].Start != [2]byte{0, 1} || d.Mazes[0].Boss != [2]byte{3, 0} || len(d.Mazes[0].Pending) != 0 {
		t.Fatalf("flat source lost: %+v", d.Mazes)
	}
	// Explicit mazes own their fields; a file header cannot add a second maze.
	cells = append(cells, section("[maze info]"))
	explicit := append([]pvf.Token(nil), body...)
	explicit[7].Value = 88
	cells = append(cells, explicit...)
	d, err = ParseDungeon(1, ScriptRecord{Path: "explicit.dgn", Cells: cells})
	if err != nil || len(d.Mazes) != 1 || d.Mazes[0].Rooms[0].Map != 88 {
		t.Fatal("flat header changed explicit maze", d.Mazes, err)
	}
}

func TestDungeonDoesNotInventAbsentFlatMaze(t *testing.T) {
	if _, err := ParseDungeon(1, ScriptRecord{Cells: []pvf.Token{section("[minimum required level]"), number(100)}}); err == nil {
		t.Fatal("invented a maze without source room definitions")
	}
}

func TestNativeSiroccoFlatDungeons(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("current PVF required")
	}
	a, err := OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := ResolveScript(a, "etc/raid/siroco/siroco.etc")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[uint32]bool{}
	var ids []uint32
	for _, block := range scriptWarpBlocks(s.Cells, "[DUNGEON INFO]") {
		v := sectionCells(block, "[INDEX]")
		if len(v) != 1 || v[0].Type != 0 || v[0].Value <= 0 {
			t.Fatal("invalid source dungeon")
		}
		id := uint32(v[0].Value)
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) != 123 {
		t.Fatalf("source coverage changed: %d", len(ids))
	}
	c, err := ImportRuntimeDungeons(a, ids)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Dungeons) != len(ids) || len(c.Skipped) != 0 {
		t.Fatalf("native flat maze still skipped: imported=%d skipped=%v", len(c.Dungeons), c.Skipped)
	}
	for id, d := range c.Dungeons {
		if len(d.Mazes) != 1 || len(d.Mazes[0].Rooms) == 0 || len(d.Mazes[0].Pending) != 0 {
			t.Fatalf("incomplete native flat maze %d: %+v", id, d.Mazes)
		}
	}
}
