package inventory

import (
	"encoding/json"
	"math/rand"
	"os"
	"testing"
)

func loadRandomOptionCatalogForTest(t *testing.T) (*RandomOptionCatalog, string) {
	t.Helper()
	var header struct {
		Source struct {
			Checksum string `json:"checksum"`
		} `json:"source"`
	}
	b, e := os.ReadFile("../../configs/randomoption.current37.json")
	if e != nil {
		t.Skip("random option catalog not present")
	}
	if e = json.Unmarshal(b, &header); e != nil {
		t.Fatal(e)
	}
	c, e := LoadRandomOptionCatalog("../../configs/randomoption.current37.json", header.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	return c, header.Source.Checksum
}

func TestRandomOptionCatalogLoads(t *testing.T) {
	c, _ := loadRandomOptionCatalogForTest(t)
	if c.GroupCount() == 0 {
		t.Fatal("no option groups")
	}
	if _, e := LoadRandomOptionCatalog("../../configs/randomoption.current37.json", "wrong"); e == nil {
		t.Fatal("source mismatch accepted")
	}
}

// Live anchor: the 2026-09-21 session sent CMD393 for slot 10 holding item
// 100310840, an amulet the full catalog flags [random option] 1, rarity 2,
// minimum level 15.
func TestRandomOptionRollLiveAnchor(t *testing.T) {
	c, source := loadRandomOptionCatalogForTest(t)
	full, e := OpenFullEquipmentCatalog("../inventory/testdata/equipment-flow", source)
	if e != nil {
		t.Fatal("equipment test fixture unavailable", e)
	}
	defer full.Close()
	def, e := full.Definition(100310840)
	if e != nil {
		t.Fatal(e)
	}
	target, ok := magicSealTarget(def)
	if !ok || target.Rarity != 2 || target.Level != 15 || target.Group != "amulet" {
		t.Fatalf("live anchor definition drifted: %+v ok=%v", target, ok)
	}
	rng := rand.New(rand.NewSource(1))
	seenTypes := map[byte]bool{}
	seenCounts := map[byte]bool{}
	for i := 0; i < 200; i++ {
		roll, e := c.Roll(def, rng)
		if e != nil {
			t.Fatal(e)
		}
		if roll.Gold != 0 {
			t.Fatalf("current table charges gold: %d", roll.Gold)
		}
		if roll.OptionType < 1 || roll.OptionType > 3 || roll.Count < 1 || roll.Count > 3 {
			t.Fatalf("roll shape: %+v", roll)
		}
		seenTypes[roll.OptionType] = true
		seenCounts[roll.Count] = true
		ids := map[byte]bool{}
		for j := 0; j < int(roll.Count); j++ {
			o := roll.Options[j]
			if o.ID == 0 || ids[o.ID] {
				t.Fatalf("option ids invalid: %+v", roll)
			}
			ids[o.ID] = true
		}
	}
	if len(seenTypes) == 1 {
		t.Fatal("roll never varied the option type")
	}
	t.Logf("types %v counts %v", seenTypes, seenCounts)
}

func TestRandomOptionGroupChoiceCoverage(t *testing.T) {
	c, _ := loadRandomOptionCatalogForTest(t)
	// Every rarity/quantity row the live accessory groups can draw must exist.
	for _, group := range []string{"amulet", "wrist", "ring"} {
		for quantity := int32(1); quantity <= 3; quantity++ {
			if _, ok := c.groupChoices[randomOptionChoiceKey{2, group, quantity}]; !ok {
				t.Fatalf("missing group choice rarity2 %s quantity %d", group, quantity)
			}
		}
	}
}

func TestUnsealRandomOptionBag(t *testing.T) {
	c, source := loadRandomOptionCatalogForTest(t)
	full, e := OpenFullEquipmentCatalog("../inventory/testdata/equipment-flow", source)
	if e != nil {
		t.Fatal("equipment test fixture unavailable", e)
	}
	defer full.Close()
	equipment := &EquipmentCatalog{Full: full}
	b := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 10, Template: 100310840}}}
	rng := rand.New(rand.NewSource(7))
	updated, row, roll, e := b.UnsealRandomOption(equipment, c, 10, rng)
	if e != nil {
		t.Fatal(e)
	}
	if RandomOptionBlockEmpty(row) {
		t.Fatal("updated record still has an empty option block")
	}
	if row[MagicSealedOffset] != 0 || row[RandomOptionCountOffset] != roll.Count || row[RandomOptionTypeOffset] != roll.OptionType {
		t.Fatalf("record block mismatch: %+v", roll)
	}
	if row[RandomOptionSpecialOffset] != RandomOptionSpecialNone {
		t.Fatal("special index not none")
	}
	// The persisted bag row must reproduce the same record.
	persisted := EquipmentRow(updated.Equipment[0])
	if len(updated.Equipment) != 1 || persisted != row {
		t.Fatal("persisted record drifted")
	}
	// A second unseal of the same item refuses instead of rerolling.
	if _, _, _, e = updated.UnsealRandomOption(equipment, c, 10, rng); e == nil {
		t.Fatal("re-unseal accepted")
	}
	// Unknown slot refuses.
	if _, _, _, e = b.UnsealRandomOption(equipment, c, 11, rng); e == nil {
		t.Fatal("unseal of empty slot accepted")
	}
	// Ordinary unflagged gear refuses.
	plain := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: 10000}}}
	if _, _, _, e = plain.UnsealRandomOption(equipment, c, 9, rng); e == nil {
		t.Fatal("unseal of unflagged equipment accepted")
	}
}
