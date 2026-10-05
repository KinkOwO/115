package boostup

import "testing"

func TestBoostEnchantedMissionChecksEveryRequiredPart(t *testing.T) {
	r := &WearRequirement{Kinds: []string{"[weapon]", "[coat]"}, Enchanted: true}
	facts := []WornFact{{Slot: 12, Template: 1, Kind: "[weapon]"}, {Slot: 14, Template: 2, Kind: "[coat]"}}
	if ok, e := r.Satisfied(facts, nil, map[uint16]uint32{12: 42}); e != nil || ok {
		t.Fatal("unenchanted coat passed", e)
	}
	if ok, e := r.Satisfied(facts, nil, map[uint16]uint32{12: 42, 14: 43}); e != nil || !ok {
		t.Fatal(ok, e)
	}
}
