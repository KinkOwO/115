package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"dfolan/internal/catalog/pvf"
)

func TestCurrentHellEpicSourceKeepsAreaAndListBoundaries(t *testing.T) {
	p := os.Getenv("DFO_LOOT_PVF")
	if p == "" {
		p = filepath.Join("..", "..", "..", "client-build", "Script.inner.pvf")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip("current PVF absent")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: p, MaxBytes: 900 * 1024 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	script, err := ResolveScript(a, "etc/helldropepicitemtable.etc")
	if err != nil {
		t.Fatal(err)
	}
	table, err := ParseHellEpicTable(script)
	if err != nil {
		t.Fatal(err)
	}
	areas := map[uint32]bool{}
	for _, list := range table.Lists {
		areas[list.Area] = true
	}
	if len(areas) != 7 || len(table.Lists) != 63 || table.SHA256 != script.SHA256 {
		t.Fatalf("source partition changed: areas=%d lists=%d", len(areas), len(table.Lists))
	}
	list, ok := table.List(54, 1)
	if !ok || len(list.Items) == 0 || list.Items[0] != (DropWeight{Template: 100060598, Weight: 19297}) || list.TotalWeight == 0 {
		t.Fatalf("area 54/list 1 changed: %+v", list)
	}
	if _, ok := table.List(0, 1); ok {
		t.Fatal("unmapped area fell back into a generic epic pool")
	}
	broken := script
	broken.Cells = append([]pvf.Token(nil), script.Cells[:len(script.Cells)-1]...)
	if _, err := ParseHellEpicTable(broken); err == nil {
		t.Fatal("truncated area accepted")
	}
}
