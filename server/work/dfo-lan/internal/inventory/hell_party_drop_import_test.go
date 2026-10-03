package inventory

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestHellEpicCandidateDoesNotBroadenOrdinaryPool(t *testing.T) {
	row := EquipmentDefinition{ID: 100, Fields: map[string][]pvf.Token{"[grade]": {{Type: 0, Value: 60}}, "[rarity]": {{Type: 0, Value: 4}}, "[creation rate]": {{Type: 0, Value: 20}}, "[durability]": {{Type: 0, Value: 35}}, "[equipment type]": {{Type: 6, Text: "[weapon]"}}, "[attach type]": {{Type: 6, Text: "[free]"}}}}
	got, ok := hellPartyDropCandidate(row)
	if !ok || got.Rarity != 4 || got.Weight != 20 || got.Durability != 35 {
		t.Fatal("source epic was filtered", got)
	}
	if _, ok := ordinaryDropCandidate(row); ok {
		t.Fatal("Hell acceptance broadened ordinary pool")
	}
	delete(row.Fields, "[creation rate]")
	if _, ok := hellPartyDropCandidate(row); ok {
		t.Fatal("missing native creation rate invented a weight")
	}
}
