package character

import (
	"bytes"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

func TestCompanionReadinessUsesCurrentWornAndNeverChangesState(t *testing.T) {
	s := &Service{WearRules: inventory.WearRules{Slots: map[string]uint16{"[weapon]": 12}}}
	for _, weapon := range []uint32{0, 101010912} {
		t.Run(string(rune('a'+weapon%17)), func(t *testing.T) {
			raw := json.RawMessage(`{"level":115,"future_field":{"keep":true}}`)
			worn := []inventory.BagEquipment{{Slot: 1, Template: 769900045}}
			if weapon != 0 {
				worn = append(worn, inventory.BagEquipment{Slot: 12, Template: weapon})
			}
			var err error
			raw, err = inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Worn: worn})
			if err != nil {
				t.Fatal(err)
			}
			role := Character{ID: 7, Name: "Companion", State: raw}
			before := append([]byte(nil), raw...)
			r, err := s.CompanionReadiness(role)
			if err != nil || r.Weapon != weapon || !r.WeaponSlotKnown || r.Level != 115 || (weapon == 0 && len(r.Warnings) != 1) || (weapon != 0 && len(r.Warnings) != 0) {
				t.Fatal(r, err)
			}
			if !bytes.Equal(before, role.State) {
				t.Fatal("readiness modified saved equipment")
			}
		})
	}
	role := Character{State: []byte(`{"level":115}`)}
	r, err := (&Service{}).CompanionReadiness(role)
	if err != nil || r.WeaponSlotKnown || len(r.Warnings) != 0 {
		t.Fatal("guessed a weapon slot without native rules", r, err)
	}
}
