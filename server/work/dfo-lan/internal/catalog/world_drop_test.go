package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorldDropStrictSourceBoundaries(t *testing.T) {
	h := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	n := func(v int32) pvf.Token { return pvf.Token{Value: v} }
	s := ScriptRecord{Path: "etc/worlddrop.etc", SHA256: strings.Repeat("b", 64), Cells: []pvf.Token{h("[world drop]"), n(15), n(7), n(3030), n(850), n(999), n(0), n(-1), h("[/world drop]")}}
	table, err := ParseWorldDropTable(s)
	if err != nil || table.Levels[15].Column2 != 7 || len(table.Levels[15].Items) != 2 {
		t.Fatalf("raw projection: %+v %v", table, err)
	}
	for _, c := range [][]pvf.Token{
		{h("[world drop]"), n(15), n(0), n(1), h("[/world drop]")},
		{h("[world drop]"), n(15), n(0), n(1), n(100), h("[/world drop]")},
		{h("[world drop]"), n(15), n(0), n(-1), n(15), n(0), n(-1), h("[/world drop]")},
		{h("[world drop]"), n(0), n(0), n(-1), h("[/world drop]")},
		{h("[world drop]"), n(15), n(0), n(-1)},
	} {
		s.Cells = c
		if _, err := ParseWorldDropTable(s); err == nil {
			t.Fatal("malformed world table accepted")
		}
	}
	m, err := ParseMonsterItemTable(ScriptRecord{Path: "monster/a.mob", SHA256: s.SHA256, Cells: []pvf.Token{h("[exclude world drop]")}})
	if err != nil || !m.ExcludeWorldDrop || m.Declared {
		t.Fatalf("presence marker: %+v %v", m, err)
	}
}

func TestCurrentPVFWorldDropContainsLiveLevelMaterialsAndConsumables(t *testing.T) {
	p := filepath.Join("..", "..", "..", "client-build", "Script.inner.pvf")
	if _, err := os.Stat(p); err != nil {
		t.Skip("current PVF absent")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1 << 30}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	c := LootCatalog{Source: a.Snapshot()}
	if err := c.EnableWorldDrop(a); err != nil {
		t.Fatal(err)
	}
	if len(c.WorldDrop.Levels) != 200 {
		t.Fatal("world level projection incomplete")
	}
	for _, level := range []uint32{15, 16, 17, 18} {
		row := c.WorldDrop.Levels[level]
		sum := int32(0)
		ids := map[int32]bool{}
		for _, p := range row.Items {
			if p.Template > 0 && p.Value > 0 {
				sum += p.Value
				ids[p.Template] = true
			}
		}
		if row.Column2 != 0 || sum != 7006 || !ids[3030] || !ids[3028] {
			t.Fatalf("level%d raw weights: %+v", level, row)
		}
		if level == 15 && (!ids[1107] || !ids[1113]) || level > 15 && (!ids[1108] || !ids[1114]) {
			t.Fatalf("level%d healing pool absent", level)
		}
	}
}
