package inventory

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

func TestAvatarSelectionPersistsNativeField(t *testing.T) {
	s, role := wearFixture(t)
	s.Rules.Special = true
	s.Catalog.index[40601] = EquipmentDefinition{ID: 40601, Fields: map[string][]pvf.Token{
		"[equipment type]": {{Type: 6, Text: "[coat avatar]"}}, "[avatar type select]": {{Type: 0, Value: 7}}}}
	b, e := ReadBag(role.State)
	if e != nil {
		t.Fatal(e)
	}
	b.Special = map[byte][]BagEquipment{1: {{Slot: 4, Template: 40601}}}
	role.State, e = SaveBag(role.State, b)
	if e != nil {
		t.Fatal(e)
	}
	r := protocol.AvatarOptionRequest{Location: 2, Slot: 4, Template: 40601, Option: 15}
	raw, e := s.SelectAvatarOption(role, r)
	if e != nil {
		t.Fatal(e)
	}
	saved, e := ReadBag(raw)
	if e != nil {
		t.Fatal(e)
	}
	row := EquipmentRow(saved.Special[1][0])
	if row[11] != 15 || row[12] != 0 {
		t.Fatal("native field not persisted")
	}
	before, _ := json.Marshal(b.Equipment)
	after, _ := json.Marshal(saved.Equipment)
	if string(before) != string(after) {
		t.Fatal("ordinary items changed")
	}
	role.State = raw
	if _, e = s.SelectAvatarOption(role, r); e == nil {
		t.Fatal("free reselection accepted")
	}
	r.Template++
	if _, e = s.SelectAvatarOption(role, r); e == nil {
		t.Fatal("stale template accepted")
	}
}
