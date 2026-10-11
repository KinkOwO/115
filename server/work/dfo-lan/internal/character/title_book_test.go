package character

import (
	"bytes"
	"context"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type titleBookTestStore struct {
	Store
	role Character
}

func (s *titleBookTestStore) CommitCharacterEvent(_ context.Context, account, id int64, source, key, model string, apply func(Character) (json.RawMessage, json.RawMessage, error)) (Character, bool, error) {
	next, _, err := apply(s.role)
	if err != nil {
		return s.role, false, err
	}
	s.role.State = next
	return s.role, true, nil
}
func titleBookService(t *testing.T, bag inventory.Bag) (*Service, Character) {
	t.Helper()
	checksum := strings.Repeat("a", 64)
	cat, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: checksum}, Rows: []inventory.EquipmentDefinition{{ID: 101, Path: "title.equ", SHA256: checksum, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[title name]"}, {Type: 0, Value: 1}}}}, {ID: 102, Path: "title2.equ", SHA256: checksum, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[title name]"}, {Type: 0, Value: 1}}}}, {ID: 999, Path: "weapon.equ", SHA256: checksum, Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[weapon]"}, {Type: 0, Value: 1}}}}}}, checksum)
	if err != nil {
		t.Fatal(err)
	}
	bag.Version = "ordinary-bag-v1"
	raw, err := inventory.SaveBag(json.RawMessage(`{"level":110,"unrelated":{"kept":7}}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role := Character{ID: 7, AccountID: 1, State: raw}
	store := &titleBookTestStore{role: role}
	return &Service{Equipment: cat, Store: store}, role
}
func TestTitleBookTransferPreservesInstanceAndOtherState(t *testing.T) {
	rec := protocol.OrdinaryItem(51, 101, 123)
	rec[14] = 9
	rec[24] = 7
	item := inventory.BagEquipment{Slot: 51, Template: 101, Record: rec[:], Durability: 2, Period: 12345}
	svc, role := titleBookService(t, inventory.Bag{Gold: 456, Equipment: []inventory.BagEquipment{item}})
	r := protocol.TitleBookTransfer{Space: 0, Slot: 51, Template: 101, Category: 4, BookSlot: 10}
	raw, err := svc.applyTitleBook(role, r, true, [2]uint16{9, 64})
	if err != nil {
		t.Fatal(err)
	}
	role.State = raw
	b, err := inventory.ReadBag(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Equipment) != 0 || !b.OwnsBookTitle(101) || b.Gold != 456 {
		t.Fatal("transfer lost item/currency")
	}
	raw, err = svc.applyTitleBook(role, r, false, [2]uint16{9, 64})
	if err != nil {
		t.Fatal(err)
	}
	b, err = inventory.ReadBag(raw)
	if err != nil {
		t.Fatal(err)
	}
	restored, _ := json.Marshal(b.Equipment[0])
	original, _ := json.Marshal(item)
	if !bytes.Equal(restored, original) {
		t.Fatalf("instance changed: %s / %s", original, restored)
	}
	if !bytes.Contains(raw, []byte(`"kept":7`)) {
		t.Fatal("unrelated state lost")
	}
	if _, err = svc.applyTitleBook(role, r, true, [2]uint16{9, 64}); err == nil {
		t.Fatal("duplicated absent source")
	}
}
func TestTitleBookRejectsStaleOrNonTitle(t *testing.T) {
	svc, role := titleBookService(t, inventory.Bag{Equipment: []inventory.BagEquipment{{Slot: 51, Template: 999}}})
	before := append([]byte(nil), role.State...)
	for _, template := range []uint32{101, 999} {
		if _, err := svc.applyTitleBook(role, protocol.TitleBookTransfer{Slot: 51, Template: template, Category: 4, BookSlot: 10}, true, [2]uint16{9, 64}); err == nil {
			t.Fatal("accepted stale/non-title")
		}
	}
	if !bytes.Equal(before, role.State) {
		t.Fatal("refusal mutated save")
	}
}
func TestTitleBookAppearanceRequiresOwnershipAndPreservesWornStats(t *testing.T) {
	worn := inventory.BagEquipment{Slot: 13, Template: 101, Durability: 7}
	svc, role := titleBookService(t, inventory.Bag{Worn: []inventory.BagEquipment{worn}, TitleBook: map[byte][]inventory.TitleBookEntry{4: {{Equipment: inventory.BagEquipment{Slot: 10, Template: 102}}}}})
	if _, _, err := svc.ApplyTitleAppearance(context.Background(), role, "foreign", 999); err == nil {
		t.Fatal("accepted unowned appearance")
	}
	next, _, err := svc.ApplyTitleAppearance(context.Background(), role, "apply", 102)
	if err != nil {
		t.Fatal(err)
	}
	b, err := inventory.ReadBag(next.State)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(b.Worn[0], worn) {
		t.Fatal("appearance changed real title")
	}
	look, err := wornAppearance(next.State)
	if err != nil || len(look) != 1 || look[0].Item != 102 {
		t.Fatalf("appearance missing: %+v %v", look, err)
	}
	cleared, _, err := svc.ApplyTitleAppearance(context.Background(), next, "clear", 0)
	if err != nil {
		t.Fatal(err)
	}
	look, err = wornAppearance(cleared.State)
	if err != nil || look[0].Item != 101 {
		t.Fatal("clear did not restore real title")
	}
}
