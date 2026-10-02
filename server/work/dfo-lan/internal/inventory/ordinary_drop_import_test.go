package inventory

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestOrdinaryDropCandidateCreationAndEligibility(t *testing.T) {
	t.Setenv("DFO_ALLOW_TRADE_EQUIPMENT", "")
	row := EquipmentDefinition{ID: 1, Fields: map[string][]pvf.Token{
		"[grade]": {{Type: 0, Value: 105}}, "[rarity]": {{Type: 0, Value: 1}},
		"[equipment type]": {{Type: 6, Text: "[weapon]"}}, "[attach type]": {{Type: 6, Text: "[free]"}},
		"[durability]": {{Type: 0, Value: 35}},
	}}
	candidate, ok := ordinaryDropCandidate(row)
	if !ok || candidate.Grade != 105 || candidate.Weight != 1 {
		t.Fatalf("compatibility default changed: %+v %v", candidate, ok)
	}
	row.Fields["[creation rate]"] = []pvf.Token{{Type: 0, Value: 900}}
	if candidate, ok = ordinaryDropCandidate(row); !ok || candidate.Weight != 900 {
		t.Fatalf("source weight lost: %+v", candidate)
	}
	for _, rate := range []int32{0, -1} {
		row.Fields["[creation rate]"][0].Value = rate
		if _, ok = ordinaryDropCandidate(row); ok {
			t.Fatalf("nonpositive explicit rate %d awarded", rate)
		}
	}
	delete(row.Fields, "[creation rate]")
	row.Fields["[equipment type]"][0].Text = "[hair avatar]"
	if _, ok = ordinaryDropCandidate(row); ok {
		t.Fatal("avatar entered main equipment pool")
	}
	row.Fields["[equipment type]"][0].Text = "[weapon]"
	row.Fields["[attach type]"][0].Text = "[sealing]"
	if _, ok = ordinaryDropCandidate(row); ok {
		t.Fatal("expanded unverified attachment state")
	}
}
