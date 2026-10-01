package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/managementdata"
	"encoding/json"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
)

func TestNativeNameUsesActualReferenceAndLiteral(t *testing.T) {
	ix := &ItemIndex{byID: map[uint32]int{}, namesClient: NameTable{"name_1": "wrong ID name", "name_99": "actual alias", "chn_name_2": "中文引用"}}
	ix.push(IndexItem{ID: 1, Kind: "equipment", Name: "native alias", NameKey: "name_99", NativeName: true})
	ix.push(IndexItem{ID: 2, Kind: "equipment", Name: "native chn", NameKey: "chn_name_2", NativeName: true})
	ix.push(IndexItem{ID: 3, Kind: "stackable", Name: "literal name", NativeName: true})
	if ix.items[0].Name != "actual alias" || ix.items[1].Name != "中文引用" || ix.items[2].Name != "literal name" {
		t.Fatal(ix.items)
	}
}

func TestCatalogMetadataUsesPreparedGrantCatalog(t *testing.T) {
	s := &server{index: &ItemIndex{items: []ItemEntry{{ID: 7, Kind: "stackable", Type: "[material]"}, {ID: 9, Kind: "equipment", Type: "[coat]"}}}, loot: catalog.LootCatalog{Items: map[uint32]catalog.LootItem{0: {Kind: "stackable"}, 7: {Kind: "stackable"}, 8: {Kind: "equipment"}}}}
	w := httptest.NewRecorder()
	s.handleCatalogMetadata(w, httptest.NewRequest("GET", "/api/catalog-metadata", nil))
	var got struct {
		Items      map[uint32][2]string `json:"items"`
		Stackables []uint32             `json:"stackables"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Items) != 2 || got.Items[9][1] != "[coat]" || len(got.Stackables) != 1 || got.Stackables[0] != 7 {
		t.Fatal(got)
	}
}

func TestNativeGMSourceOnly(t *testing.T) {
	archive := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE and DFO_PVF_CORE_TEST_SHA256")
	}
	p := paths{itemIndex: "does-not-exist/index.json", equipCatalog: "does-not-exist/equipment.json", equipmentSlots: "does-not-exist/slots.json", lootCatalog: "does-not-exist/loot.json", namesClient: "../../../../../gm-tool/configs/names.client.json", namesZH: "does-not-exist/translation.json", namesEN: "does-not-exist/english.json", bagRules: "../../configs/inventory.next29.json"}
	f := managementdata.Flags{Mode: "pvf", ArchivePath: archive, Checksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"), DropPolicy: "../../configs/pvf-drop-policy.json"}
	d, err := prepareNativeGMData(p, f)
	if err != nil {
		t.Fatal(err)
	}
	defer d.awarder.Equipment.Full.Close()
	if d.index.Count() != 599771 || d.index.Source() != "pvf" {
		t.Fatal("native LIST scope changed", d.index.Count(), d.index.Source())
	}
	if entry, ok := d.index.Get(0); !ok || entry.Grantable {
		t.Fatal("gold exposed as grant item", entry)
	}
	if entry, ok := d.index.Get(100050791); !ok || entry.Kind != "equipment" {
		t.Fatal("native equipment missing", entry)
	}
	// The old GM export used another subset and extraction path. Audit all old
	// records against native IDs and displayed scalar fields before recording
	// any source-scope differences in the migration inventory.
	b, err := os.ReadFile("../../../../../gm-tool/configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}
	var old struct {
		Items []IndexItem `json:"items"`
	}
	if err := json.Unmarshal(b, &old); err != nil {
		t.Fatal(err)
	}
	missing, kind, grade, rarity := 0, 0, 0, 0
	nameDiff, samples := 0, 0
	for _, it := range old.Items {
		got, ok := d.index.Get(it.ID)
		if !ok {
			missing++
			continue
		}
		if got.Kind != it.Kind {
			kind++
		}
		if got.Grade != it.Grade {
			grade++
			if samples < 10 {
				t.Logf("grade %d kind=%s old=%d native=%d level=%d type=%q", it.ID, it.Kind, it.Grade, got.Grade, got.Level, got.Type)
				samples++
			}
		}
		if got.Rarity != it.Rarity {
			rarity++
			t.Logf("rarity %d kind=%s old=%d native=%d", it.ID, it.Kind, it.Rarity, got.Rarity)
		}
		if oldName := d.index.namesClient[d.index.nameKeyFor(it.ID)]; oldName != "" && oldName != got.Name {
			nameDiff++
		}
	}
	if missing != 0 || kind != 0 || grade != 0 || rarity != 0 {
		t.Fatal("legacy item scalar metadata changed", missing, kind, grade, rarity)
	}
	slots := loadSlotMap("../../../../../gm-tool/configs/equipment.slots.json")
	slotMissing, slotKind, slotType, slotLevel := 0, 0, 0, 0
	levelSamples, typeSamples := 0, 0
	for key, row := range slots {
		n, err := strconv.ParseUint(key, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		got, ok := d.index.Get(uint32(n))
		if !ok {
			slotMissing++
			continue
		}
		if got.Kind != "equipment" {
			slotKind++
			continue
		}
		if got.Type != row.Cell {
			slotType++
			if typeSamples < 5 {
				t.Logf("slot %s old=%s native=%s", key, row.Cell, got.Type)
				typeSamples++
			}
		}
		if got.Level != row.Level {
			slotLevel++
			if levelSamples < 8 {
				def, err := d.awarder.Equipment.Full.Definition(uint32(n))
				t.Logf("level %s old=%d native=%d type=%s path=%s cells=%+v err=%v", key, row.Level, got.Level, got.Type, def.Path, def.Fields["[minimum level]"], err)
				levelSamples++
			}
		}
	}
	t.Logf("old_slots=%d absent_native=%d non_equipment=%d type_diff=%d level_diff=%d", len(slots), slotMissing, slotKind, slotType, slotLevel)
	t.Logf("old ID-generated overlay name differences=%d", nameDiff)
	t.Logf("native=%d old=%d absent_native=%d kind_diff=%d grade_diff=%d rarity_diff=%d; all native source scripts decoded; no exported metadata or storage read", d.index.Count(), len(old.Items), missing, kind, grade, rarity)
}
