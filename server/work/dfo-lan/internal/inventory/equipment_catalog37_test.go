package inventory

import (
	"encoding/json"
	"os"
	"testing"
)

// The 37 catalog adds the 1638 templates quests hand out. Those rows exist so
// a quest reward can be granted at all; they must not quietly become monster
// drops, so the bag-usable pool is expected to stay close to the 35 catalog's.
func TestWidenedCatalogKeepsDropPoolSane(t *testing.T) {
	var source struct {
		Source struct {
			Checksum string `json:"checksum"`
		} `json:"source"`
	}
	b, e := os.ReadFile("../../configs/equipment.current37.json")
	if e != nil {
		t.Skip("37 catalog not present")
	}
	if e = json.Unmarshal(b, &source); e != nil {
		t.Fatal(e)
	}
	wide, e := LoadEquipmentCatalog("../../configs/equipment.current37.json", source.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	narrow, e := LoadEquipmentCatalog("../../configs/equipment.current35.json", source.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	before, after := len(narrow.DropPool()), len(wide.DropPool())
	t.Logf("rows %d -> %d, drop pool %d -> %d",
		len(narrow.Rows), len(wide.Rows), before, after)
	if len(wide.Rows) <= len(narrow.Rows) {
		t.Fatal("widened catalog is not wider")
	}
	// Every row the narrow catalog could grant must still be grantable.
	for _, r := range narrow.Rows {
		if _, err := wide.Basic(r.ID); err != nil {
			if _, was := narrow.Basic(r.ID); was == nil {
				t.Fatalf("template %d lost its basic acceptance", r.ID)
			}
		}
	}
	if after > before*3 {
		t.Fatalf("drop pool grew out of proportion: %d -> %d", before, after)
	}

	// Quest 21650's reward. Live capture 20260912T011904 refused it four times
	// after the template was imported, because it is bound gear and Basic only
	// accepts what a monster may drop.
	if _, err := wide.Reward(100261068); err != nil {
		t.Fatal("quest 21650's reward is still not grantable:", err)
	}
	if _, err := wide.Basic(100261068); err == nil {
		t.Fatal("bound gear leaked into the drop-pool rule")
	}
	grantable, pooled := 0, 0
	for _, r := range wide.Rows {
		if _, err := wide.Reward(r.ID); err == nil {
			grantable++
		}
		if _, err := wide.Basic(r.ID); err == nil {
			pooled++
		}
	}
	t.Logf("grantable %d of %d rows, pool-eligible %d", grantable, len(wide.Rows), pooled)
	if grantable <= pooled {
		t.Fatal("the reward rule is no wider than the pool rule")
	}
}
