package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestFameProjectionSeparatesLevelSectionsAndInheritance(t *testing.T) {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	integer := func(n int32) pvf.Token { return pvf.Token{Type: 0, Value: n} }
	base := equipmentDefinitionFromScript(1, catalog.ScriptRecord{Path: "equipment/base.equ", SHA256: strings.Repeat("1", 64), Cells: []pvf.Token{
		tag("[fame value]"), integer(7), tag("[fame value]"), integer(11),
		tag("[level section ability]"), integer(20), tag("[fame value]"), integer(18), tag("[/level section ability]"),
		tag("[level section ability]"), integer(100), tag("[fame value]"), integer(748), tag("[/level section ability]"),
	}})
	child := equipmentDefinitionFromScript(2, catalog.ScriptRecord{Path: "equipment/child.equ", SHA256: strings.Repeat("2", 64), Cells: []pvf.Token{
		tag("[import script]"), {Type: 6, Text: "equipment/1.equ"}, tag("[add fame value]"), integer(3),
	}})
	c, err := NewEquipmentCatalog(EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}, Rows: []EquipmentDefinition{base, child}}, "test")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []uint32{1, 2} {
		for _, row := range []struct {
			level byte
			value int32
		}{{1, 11}, {19, 11}, {20, 18}, {99, 18}, {100, 748}, {115, 748}} {
			d, err := c.FameDefinition(id, row.level)
			if err != nil {
				t.Fatal(err)
			}
			got := d.Fields["[fame value]"]
			if len(got) != 1 || got[0].Value != row.value {
				t.Fatalf("id=%d level=%d: %+v", id, row.level, got)
			}
			if id == 2 && d.Fields["[add fame value]"][0].Value != 3 {
				t.Fatal("child override lost")
			}
		}
	}
	// A source edit must change the result, without changing any lookup constants.
	c.index[1].fameLevels[1].fields["[fame value]"][0].Value = 901
	d, err := c.FameDefinition(2, 115)
	if err != nil || d.Fields["[fame value]"][0].Value != 901 {
		t.Fatal("source edit not consumed", err)
	}
}

// Opt-in audit reads only the actual PVF; no player state or generated content input.
func TestFameCurrentPVFLevelSamples(t *testing.T) {
	path := os.Getenv("DFO_FAME_AUDIT_PVF")
	if path == "" {
		t.Skip("set DFO_FAME_AUDIT_PVF for the actual source audit")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 2 << 30}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	indexScript, err := catalog.ResolveScript(a, "list/equipment.lst")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := catalog.ParseIndex(indexScript.Cells)
	if err != nil {
		t.Fatal(err)
	}
	index := catalog.ItemIndex{Source: a.Snapshot(), Items: map[uint32]catalog.ItemIndexEntry{}, IndexHashes: map[string]string{indexScript.Path: indexScript.SHA256}}
	for _, row := range rows {
		p := row.Path
		if !strings.HasPrefix(p, "equipment/") {
			p = "equipment/" + p
		}
		index.Items[row.ID] = catalog.ItemIndexEntry{ID: row.ID, Path: p, Kind: "equipment"}
	}
	full, err := OpenPVFEquipmentCatalog(a, index)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	c := &EquipmentCatalog{Source: a.Snapshot(), Full: full}
	for _, id := range []uint32{101001127, 101021029, 100280978} {
		raw, err := c.Definition(id)
		if err != nil {
			t.Fatal(err)
		}
		if len(raw.fameLevels) == 0 {
			t.Fatalf("sample %d has no source level sections", id)
		}
		for _, level := range []byte{1, 15, 19, 20, 25, 30, 39, 40, 50, 99, 100, 105, 115} {
			d, err := c.FameDefinition(id, level)
			if err != nil {
				t.Fatal(err)
			}
			values := d.Fields["[fame value]"]
			if len(values) > 1 {
				t.Fatalf("unprojected multiple values: id=%d level=%d %+v", id, level, values)
			}
			var value int32
			if len(values) == 1 {
				if values[0].Type != 0 {
					t.Fatal("non-integer fame", values)
				}
				value = values[0].Value
			}
			report, _ := json.Marshal(map[string]any{"id": id, "path": raw.Path, "script_sha256": raw.SHA256, "level": level, "value": value, "sections": len(raw.fameLevels), "pvf_sha256": a.Snapshot().Checksum})
			t.Log(string(report))
		}
	}
}
