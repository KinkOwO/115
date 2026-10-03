package catalog

import (
	"dfolan/internal/catalog/pvf"
	"reflect"
	"testing"
)

func TestNPCPlaceIndexStableAndDeduplicatedAcrossLoads(t *testing.T) {
	npc := ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[NPC]"}, {Type: 0, Value: 7}, {Type: 6, Text: "npc"}, {Type: 0, Value: 1}, {Type: 0, Value: 2}, {Type: 0, Value: 0}}}
	w := WorldCatalog{Areas: map[string]WorldArea{
		"9/2": {Town: 9, Area: 2, Map: npc, ImportedScripts: []ScriptRecord{npc}},
		"1/3": {Town: 1, Area: 3, Map: npc}, "1/0": {Town: 1, Area: 0, Map: npc},
	}}
	want := []NPCPlace{{1, 0}, {1, 3}, {9, 2}}
	for range 30 {
		w.indexNPCTeleports()
		if !reflect.DeepEqual(w.NPCPlaces[7], want) {
			t.Fatal(w.NPCPlaces)
		}
	}
}
