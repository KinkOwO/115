package inventory

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestFlipGearCandidateFollowsSourceLevelRarityAndGrantability(t *testing.T) {
	row := EquipmentDefinition{ID: 987654, Fields: map[string][]pvf.Token{
		"[minimum level]": {{Type: 0, Value: 115}}, "[rarity]": {{Type: 0, Value: 3}},
		"[equipment type]": {{Type: 6, Text: "[coat]"}, {Type: 0, Value: 0}}, "[durability]": {{Type: 0, Value: 37}},
	}}
	if !flipGearCandidate(row, 115, []int32{2, 3}) {
		t.Fatal("source gear rejected")
	}
	row.Fields["[minimum level]"][0].Value = 114
	if flipGearCandidate(row, 115, []int32{2, 3}) {
		t.Fatal("wrong source level accepted")
	}
	row.Fields["[minimum level]"][0].Value = 115
	row.Fields["[rarity]"][0].Value = 4
	if flipGearCandidate(row, 115, []int32{2, 3}) {
		t.Fatal("wrong source rarity accepted")
	}
	row.Fields["[rarity]"][0].Value = 2
	row.Fields["[equipment type]"][0].Text = "[hair avatar]"
	if flipGearCandidate(row, 115, []int32{2, 3}) {
		t.Fatal("avatar accepted as main equipment")
	}
	row.Fields["[equipment type]"][0].Text = "[coat]"
	delete(row.Fields, "[durability]")
	if flipGearCandidate(row, 115, []int32{2, 3}) {
		t.Fatal("ungrantable source gear accepted")
	}
}
