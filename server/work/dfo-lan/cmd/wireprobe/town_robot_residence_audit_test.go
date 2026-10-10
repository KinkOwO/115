package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"testing"
)

func TestTownRobotResidenceSourceAudit(t *testing.T) {
	out := os.Getenv("DFO_ROBOT_RESIDENCE_AUDIT")
	if out == "" {
		t.Skip("explicit offline residence source audit")
	}
	a, e := pvf.OpenReadOnly(pvf.Options{Path: `D:\115us\server\work\client-build\Script.inner.pvf`, MaxBytes: 1 << 30}, "45c8db452291abf5f912d23515d1774aa56ced23a5b61ec95ef1f3ec8263c0dc")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	c, e := catalog.ImportWorldRuntime(a)
	if e != nil {
		t.Fatal(e)
	}
	homes := robotResidenceAreas(c)
	if len(homes) < 2 {
		t.Fatal("source public town distribution absent")
	}
	rows := []any{}
	seen := map[int]bool{}
	for _, home := range homes {
		ids := robotResidentIDs(c, 69, home.Town, home.Area)
		for _, id := range ids {
			if seen[id] {
				t.Fatal("duplicate resident")
			}
			seen[id] = true
		}
		rows = append(rows, map[string]any{"town": home.Town, "area": home.Area, "map_path": home.MapPath, "minimum_level": home.MinimumLevel, "map_sha256": home.Map.SHA256, "robot_ids": ids})
	}
	if len(seen) != 69 {
		t.Fatal("source lost careers")
	}
	raw, e := json.MarshalIndent(map[string]any{"source": c.Source, "town_index_sha256": c.TownIndex.SHA256, "homes": rows, "careers": len(seen)}, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(out, raw, 0600); e != nil {
		t.Fatal(e)
	}
	t.Log("source homes", len(homes), "unique careers", len(seen))
}
