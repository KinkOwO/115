package character

import (
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"testing"
)

func TestReportedAwakenedSkills(t *testing.T) {
	c, e := LoadLearningCatalog("../../configs/skills.awakening-candidate.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	st := State{Level: 115, Advancement: 1, Awakening: 3}
	for _, id := range []uint16{113, 236, 246, 258} {
		d := c.index[0][id]
		if _, e := d.costForState(st, 1, nil); e != nil {
			t.Errorf("skill %d type=%v: %v", id, d.Fields["[type]"], e)
		}
	}
}

func TestAutoSetSourceLearningAndVariation(t *testing.T) {
	c, e := LoadLearningCatalog("../../configs/skills.awakening-candidate.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := hex.DecodeString("002572000002700000057100000902010012ef00000deb0000150900002961000017ec00001c5b000016490000240800000148000026690000016e00000a6d00002b070000016c00002e2600000a1f00000105010005ba00000a6b00000a43000001310000012100000a1b0000140f0000010e0000010d0000010c00000104000001aa00000141000001110000014400000262000008010101eb000100000000020101000000004800010000000001090001000000010048000100000001004900010000000100eb000100000001000201020000000100baeb9fb30000000000")
	r, e := protocol.DecodeSkillPurchase(raw)
	if e != nil {
		t.Fatal(e)
	}
	st := State{Level: 115, Advancement: 1, Awakening: 3}
	known := map[uint16]byte{86: 1, 91: 1, 239: 1, 245: 1, 112: 1, 169: 1}
	for _, v := range r.Entries {
		known[v.ID] += v.Delta
	}
	for _, v := range r.Entries {
		if _, e := c.index[0][v.ID].costForState(st, int(known[v.ID]), known); e != nil {
			t.Errorf("skill %d rank %d: %v", v.ID, known[v.ID], e)
		}
	}
	s := Service{Learning: c}
	if e := s.applyVariations(0, &st, known, r); e != nil {
		t.Fatal(e)
	}
	st.Advancement = 2
	if e := s.applyVariations(0, &st, known, r); e == nil {
		t.Fatal("cross-job variation accepted")
	}
}
