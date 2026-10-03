package inventory_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"os"
	"testing"
)

// The historical narrow/wide comparison covered the complete 1536-row basic
// policy selection and its PVF quest-reward expansion. Rebuild both directly
// from the pinned current archive rather than carrying full exported tables.
func TestNativeWidenedCatalogKeepsDropPoolSane(t *testing.T) {
	archive := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for the complete native equipment selection test")
	}
	const currentChecksum = "8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934"
	if got := os.Getenv("DFO_PVF_CORE_TEST_SHA256"); got != currentChecksum {
		t.Skip("set DFO_PVF_CORE_TEST_SHA256 to the current 8b2a archive for this integration gate")
	}
	source, err := gamedata.Open(gamedata.Options{
		Mode: gamedata.PVF, ArchivePath: archive, ExpectedChecksum: currentChecksum,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()

	policy, err := inventory.ReadDropPolicy("../../configs/pvf-drop-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	index, err := source.ItemIndex("")
	if err != nil {
		t.Fatal(err)
	}
	narrow, err := source.EquipmentSelection(index, catalog.QuestCatalog{Source: source.Snapshot()}, policy)
	if err != nil {
		t.Fatal(err)
	}
	quests, err := source.Quests("")
	if err != nil {
		t.Fatal(err)
	}
	wide, err := source.EquipmentSelection(index, quests, policy)
	if err != nil {
		t.Fatal(err)
	}
	before, after := len(narrow.DropPool()), len(wide.DropPool())
	t.Logf("native rows %d -> %d, drop pool %d -> %d",
		len(narrow.Rows), len(wide.Rows), before, after)
	if len(wide.Rows) <= len(narrow.Rows) {
		t.Fatal("quest reward selection is not wider than the basic policy selection")
	}
	for _, row := range narrow.Rows {
		if _, err := wide.Basic(row.ID); err != nil {
			if _, was := narrow.Basic(row.ID); was == nil {
				t.Fatalf("template %d lost its basic acceptance", row.ID)
			}
		}
	}
	if after > before*3 {
		t.Fatalf("drop pool grew out of proportion: %d -> %d", before, after)
	}
	// Quest 21650's reward is grantable but bound gear must not enter the drop pool.
	if _, err := wide.Reward(100261068); err != nil {
		t.Fatal("quest 21650's reward is still not grantable:", err)
	}
	if _, err := wide.Basic(100261068); err == nil {
		t.Fatal("bound gear leaked into the drop-pool rule")
	}
	grantable, pooled := 0, 0
	for _, row := range wide.Rows {
		if _, err := wide.Reward(row.ID); err == nil {
			grantable++
		}
		if _, err := wide.Basic(row.ID); err == nil {
			pooled++
		}
	}
	t.Logf("grantable %d of %d native rows, pool-eligible %d", grantable, len(wide.Rows), pooled)
	if grantable <= pooled {
		t.Fatal("the quest reward rule is no wider than the drop-pool rule")
	}
}
