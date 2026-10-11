package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestSkinSlotSourceIdentityAndActionChanges(t *testing.T) {
	const current = 590714050
	const another = 987654321
	aura := catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[action type]"}, {Type: 6, Text: "[open aura skin slot]"}, {Type: 3, Text: "[/action type]"}}}
	c := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{
		current: {ID: current, Kind: "stackable", Script: aura},
		another: {ID: another, Kind: "stackable", Script: aura},
		// An old known template does not inherit an action absent from source.
		AuraSkinTicket: {ID: AuraSkinTicket, Kind: "stackable"},
	}}
	for _, id := range []uint32{current, another} {
		if mask, ok := SkinSlotMaskForTicket(id, c); !ok || mask != ExpandAuraSkin {
			t.Fatalf("source-defined ticket %d was not recognized", id)
		}
	}
	if _, ok := SkinSlotMaskForTicket(AuraSkinTicket, c); ok {
		t.Fatal("legacy ID bypassed missing source action")
	}
	changed := c.Items[current]
	changed.Script.Cells[1].Text = "[open creature skin slot]"
	c.Items[current] = changed
	if mask, ok := SkinSlotMaskForTicket(current, c); !ok || mask != ExpandCreatureSkin {
		t.Fatal("source action change ignored")
	}
	delete(c.Items, current)
	if _, ok := SkinSlotMaskForTicket(current, c); ok {
		t.Fatal("absent source template accepted")
	}
}
