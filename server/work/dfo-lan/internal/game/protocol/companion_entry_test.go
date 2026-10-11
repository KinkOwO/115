package protocol

import (
	"bytes"
	"testing"
)

func TestCompanionEntryFullEquipmentPreservesSurroundingFields(t *testing.T) {
	s := EntryAdditionProbe{ActorServerID: 65534, Stats: PackedEntryStats{HP: 100, MP: 100, BasePercent: 100}, ExpandEquipFlags: 19, SkillTrees: [2][]EntrySkill{{{ID: 238, Level: 7}}, nil}}
	empty, err := UserInfoAdditionProbe(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range []uint16{0, 12, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 25, 26, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47} {
		template := uint32(100313757)
		if slot == 0 {
			template = 769900045
		}
		if slot == 26 {
			template = 500991107
		}
		row := OrdinaryItem(slot, template, 0)
		s.Worn = append(s.Worn, DetailedWorn{Slot: slot, Template: template, Record: row[:]})
	}
	s.CompanionEquipment = true
	got, err := UserInfoAdditionProbe(s)
	if err != nil {
		t.Fatal(err)
	}
	equipment, err := TagEquipment(s.Worn)
	if err != nil {
		t.Fatal(err)
	}
	const start = 361
	if !bytes.Equal(got[:start], empty[:start]) || !bytes.Equal(got[start:start+len(equipment)], equipment) || !bytes.Equal(got[start+len(equipment):], empty[start+14:]) {
		t.Fatal("equipment insertion shifted identity/stats/skills")
	}
	if equipment[0] != byte(len(s.Worn)) {
		t.Fatal("missing combat equipment")
	}
	s.Worn = append(s.Worn, DetailedWorn{Slot: 48, Template: 100313757})
	if _, err = UserInfoAdditionProbe(s); err == nil {
		t.Fatal("unsupported row silently dropped")
	}
}
