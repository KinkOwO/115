package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"testing"
)

// Explicit offline discovery: the source's package rows, not a parallel table,
// define which pieces belong together. The output is an operator audit only.
func TestTownRobotWardrobeSourceAudit(t *testing.T) {
	out := os.Getenv("DFO_ROBOT_WARDROBE_AUDIT")
	if out == "" {
		t.Skip("explicit offline wardrobe source audit")
	}
	a, e := pvf.OpenReadOnly(pvf.Options{Path: `D:\115us\server\work\client-build\Script.inner.pvf`, MaxBytes: 1 << 30}, "45c8db452291abf5f912d23515d1774aa56ced23a5b61ec95ef1f3ec8263c0dc")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	index, e := catalog.ImportItemIndex(a)
	if e != nil {
		t.Fatal(e)
	}
	packs := []catalog.ScriptRecord{}
	for _, it := range index.Items {
		if it.Kind != "stackable" || it.StackableType != "[booster selection]" {
			continue
		}
		s, e := catalog.ResolveScript(a, it.Path)
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, x := range s.Cells {
			if x.Text == "[avatar]" {
				found = true
				break
			}
		}
		if found {
			packs = append(packs, s)
		}
	}
	b, e := json.MarshalIndent(map[string]any{"source": a.Snapshot(), "packages": packs}, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(out, b, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("avatar packages", len(packs))
}
