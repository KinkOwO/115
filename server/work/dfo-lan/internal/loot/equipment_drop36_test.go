package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"testing"
)

// Monsters must be able to drop wearable gear: before36 the equipment
// category was rolled and then discarded, so a character could clear the
// whole low-level chain and never own a single piece of equipment.
func TestEquipmentCategoryDropsBagUsableGear(t *testing.T) {
	c, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	tables, e := Parse(c)
	if e != nil {
		t.Fatal(e)
	}
	rules, e := LoadRules("../../configs/drop.current36.json")
	if e != nil {
		t.Fatal(e)
	}
	gear, e := inventory.LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	pool := gear.DropPool()
	if len(pool) == 0 {
		t.Fatal("equipment drop pool is empty")
	}
	usable := map[uint32]bool{}
	for _, d := range pool {
		if _, err := gear.Basic(d.ID); err != nil {
			t.Fatalf("pool offers gear the bag rejects: %d: %v", d.ID, err)
		}
		usable[d.ID] = true
	}

	// Sweep a deterministic seed range at a low monster level and require the
	// equipment category to actually yield gear from the source grade window.
	dropped, stacked := 0, 0
	for seed := uint32(0); seed < 4000; seed++ {
		out, err := Roll(c, tables, rules, pool, seed, 5, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range out.Awards {
			switch {
			case a.Template == 0:
			case usable[a.Template] && c.Items[a.Template].Kind != "stackable":
				if a.Amount != 1 {
					t.Fatalf("gear award amount %d", a.Amount)
				}
				dropped++
			default:
				stacked++
			}
		}
		for _, k := range out.SkippedKinds {
			if k == "equipment_dictionary_pending" {
				t.Fatal("equipment category is still being discarded")
			}
		}
	}
	if dropped == 0 {
		t.Fatal("no equipment dropped across the sampled seeds")
	}
	if stacked == 0 {
		t.Fatal("stackable drops regressed")
	}
	t.Logf("equipment drops=%d stackable drops=%d pool=%d", dropped, stacked, len(pool))
}
