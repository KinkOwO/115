package catalog

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dfolan/internal/catalog/pvf"
)

func TestMonsterItemTableUsesLastNativeSectionAndKeepsRawPairs(t *testing.T) {
	h := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	n := func(v int32) pvf.Token { return pvf.Token{Value: v} }
	s := ScriptRecord{Path: "monster/example.mob", SHA256: strings.Repeat("a", 64), Cells: []pvf.Token{
		h("[item]"), n(1000), n(50), h("[/item]"),
		h("[common champion drop item]"), n(9000), n(700), h("[/common champion drop item]"),
		h("[item]"), n(3012), n(100), n(1004), n(0), n(-1), n(-2), h("[/item]"),
	}}
	table, err := ParseMonsterItemTable(s)
	want := []MonsterItemPair{{3012, 100}, {1004, 0}, {-1, -2}}
	if err != nil || !table.Declared || !reflect.DeepEqual(table.Items, want) || table.SHA256 != s.SHA256 {
		t.Fatalf("native table: %+v %v", table, err)
	}
	// An explicit final empty section clears the previous pool too.
	s.Cells = append(s.Cells, h("[item]"), h("[/item]"))
	table, err = ParseMonsterItemTable(s)
	if err != nil || !table.Declared || len(table.Items) != 0 {
		t.Fatalf("empty replacement: %+v %v", table, err)
	}
	for _, cells := range [][]pvf.Token{
		{h("[item]"), n(1), h("[/item]")},
		{h("[item]"), n(1), n(2)},
		{h("[item]"), n(1), {Type: 1}, h("[/item]")},
		{h("[item]"), h("[other]")},
	} {
		s.Cells = cells
		if _, err := ParseMonsterItemTable(s); err == nil {
			t.Fatal("malformed MOB table accepted")
		}
	}
}

func TestCurrentPVFMonsterItemTablesContainMissingConsumables(t *testing.T) {
	p := os.Getenv("DFO_LOOT_PVF")
	if p == "" {
		p = filepath.Join("..", "..", "..", "client-build", "Script.inner.pvf")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip("current PVF absent")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1 << 30}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tables, err := ImportMonsterItemTables(a, []uint32{1, 2, 70, 71, 70})
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 4 || len(tables[2].Items) != 3 || !reflect.DeepEqual(tables[70].Items, []MonsterItemPair{{1063, 50}, {1002, 100}, {1047, 200}}) {
		t.Fatalf("native pools changed: %+v", tables)
	}
	if _, err := ImportMonsterItemTables(a, []uint32{0xffffffff}); err == nil {
		t.Fatal("unknown MOB used fallback")
	}
	list, err := ResolveScript(a, "list/stackable.lst")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ParseIndex(list.Cells)
	if err != nil {
		t.Fatal(err)
	}
	paths := map[uint32]string{}
	for _, r := range rows {
		paths[r.ID] = r.Path
	}
	for _, id := range []uint32{1000, 1002, 1047, 1063, 3012} {
		p := paths[id]
		if !strings.HasPrefix(p, "stackable/") {
			p = "stackable/" + p
		}
		s, err := ResolveScript(a, p)
		if err != nil {
			t.Fatal(err)
		}
		if len(sectionCells(s.Cells, "[creation rate]")) != 0 {
			t.Fatalf("%d now declares creation rate", id)
		}
		ty := sectionCells(s.Cells, "[stackable type]")
		if len(ty) == 0 || ty[0].Type != 6 {
			t.Fatalf("%d missing type", id)
		}
		t.Logf("native template=%d path=%s type=%s missing creation", id, p, ty[0].Text)
	}
}

func TestCurrentPVFMonsterItemTablesUseRootLISTPaths(t *testing.T) {
	p := os.Getenv("DFO_LOOT_PVF")
	if p == "" {
		p = filepath.Join("..", "..", "..", "client-build", "Script.inner.pvf")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip("current PVF absent")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1 << 30}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	ids := []uint32{1, 109014957, 109014964, 109015032}
	direct, err := ImportMonsterItemTables(a, ids)
	if err != nil {
		t.Fatal(err)
	}
	c := LootCatalog{Source: a.Snapshot()}
	if err := c.EnableMonsterItemDetails(a); err != nil {
		t.Fatal(err)
	}
	defer c.CloseDetails()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		got, known, err := c.MonsterItemTable(id)
		if err != nil || !known || !reflect.DeepEqual(got, direct[id]) {
			t.Fatalf("root LIST view %d: %+v known=%v err=%v", id, got, known, err)
		}
		if id == 1 {
			if !got.Declared || !strings.HasPrefix(got.Path, "monster/") {
				t.Fatalf("legacy MOB binding lost: %+v", got)
			}
		} else if got.Declared || !strings.HasPrefix(got.Path, "contents/2022/new_scenario_renewal/") {
			t.Fatalf("scenario MOB acquired a fabricated pool or wrong path: %+v", got)
		}
	}
}
