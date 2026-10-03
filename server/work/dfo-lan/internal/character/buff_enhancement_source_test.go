package character

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// The registered item is absent from the small drop/reward catalog. Its type
// must be resolved from the full lazy source, including an import-script chain.
func TestBuffEnhancementLazyEquipmentSource(t *testing.T) {
	s, role := buffFixture(t)
	var data bytes.Buffer
	records := map[uint32]any{}
	for _, entry := range []struct {
		id    uint32
		cells []pvf.Token
	}{
		{100, []pvf.Token{{Type: 3, Text: "[equipment type]"}, {Type: 6, Text: "[title name]"}}},
		{102, []pvf.Token{{Type: 3, Text: "[import script]"}, {Type: 6, Text: "character/common/title/100.equ"}}},
	} {
		raw, err := json.Marshal(catalog.ScriptRecord{Path: fmt.Sprintf("equipment/character/common/title/%d.equ", entry.id), SHA256: role.ConfigVersion, Cells: entry.cells})
		if err != nil {
			t.Fatal(err)
		}
		var packed bytes.Buffer
		z := zlib.NewWriter(&packed)
		if _, err = z.Write(raw); err != nil {
			t.Fatal(err)
		}
		if err = z.Close(); err != nil {
			t.Fatal(err)
		}
		records[entry.id] = map[string]any{"Offset": data.Len(), "Size": packed.Len(), "SHA256": fmt.Sprintf("%x", sha256.Sum256(packed.Bytes()))}
		data.Write(packed.Bytes())
	}
	prefix := filepath.Join(t.TempDir(), "lazy-equipment")
	raw, err := json.Marshal(map[string]any{"Source": s.Equipment.Source, "IndexSHA256": role.ConfigVersion, "Records": records})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(prefix+".index.json", raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(prefix+".data", data.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	full, err := inventory.OpenFullEquipmentCatalog(prefix, role.ConfigVersion)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	s.Equipment.Full = full
	b, err := inventory.ReadBag(role.State)
	if err != nil {
		t.Fatal(err)
	}
	b.Equipment[0].Template = 102
	role.State, err = inventory.SaveBag(role.State, b)
	if err != nil {
		t.Fatal(err)
	}
	role.State, err = s.applyBuffEnhancement(role, protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 0, Slot: 57})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 57, 0}) {
		t.Fatalf("lazy source restore: %x %v", got, err)
	}
}

// Exercise the current native archive through the same compact projection and
// retained view as production, without any exported equipment JSON.
func TestBuffEnhancementNativePVFSource(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native buff equipment lookup")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1 << 30}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	list, err := catalog.ResolveScript(a, "list/equipment.lst")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := catalog.ParseIndex(list.Cells)
	if err != nil {
		t.Fatal(err)
	}
	const title = uint32(500330719) // The title used in the scene restoration fixture.
	index := catalog.ItemIndex{Source: a.Snapshot(), IndexHashes: map[string]string{list.Path: list.SHA256}, Items: map[uint32]catalog.ItemIndexEntry{}}
	for _, row := range rows {
		if row.ID == title {
			index.Items[title] = catalog.ItemIndexEntry{ID: title, Kind: "equipment", Path: row.Path}
		}
	}
	if len(index.Items) != 1 {
		t.Fatal("scene fixture title is absent from the native LIST")
	}
	full, err := inventory.OpenPVFEquipmentCatalog(a, index)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	if err = a.CompactRuntimeStrings(); err != nil {
		t.Fatal(err)
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	s, role := buffFixture(t)
	s.Equipment = &inventory.EquipmentCatalog{Source: index.Source, Full: full}
	b, err := inventory.ReadBag(role.State)
	if err != nil {
		t.Fatal(err)
	}
	b.Equipment[0].Template = title
	role.State, err = inventory.SaveBag(role.State, b)
	if err != nil {
		t.Fatal(err)
	}
	role.State, err = s.applyBuffEnhancement(role, protocol.BuffEnhancementRequest{Skill: 306, Kind: 13, List: 0, Slot: 57})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.BuffEnhancementRestore(role)
	if err != nil || !bytes.Equal(got, []byte{0x32, 1, 1, 1, 13, 0, 57, 0}) {
		t.Fatalf("native PVF restore: %x %v", got, err)
	}
}
