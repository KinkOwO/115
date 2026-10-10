package character

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestAvatarEntryKeepsInventoryInstanceOutsideMode1(t *testing.T) {
	record := make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint16(record, 1)
	binary.LittleEndian.PutUint32(record[2:], 517560000)
	binary.LittleEndian.PutUint32(record[6:], 12345)
	binary.LittleEndian.PutUint32(record[24:], 517562678)
	bag := inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{
		{Slot: 1, Template: 517560000, Durability: 9, Record: record, AvatarOptions: []byte{1, 2}, AvatarSockets: []byte{3, 4}},
		{Slot: 1, Template: 517562678, Group: 1},
	}}
	state, err := inventory.SaveBag(json.RawMessage(`{"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), state...)
	service := Service{DetailedWornCandidate: true}
	packet, err := service.EntryAddition(Character{WireID: 503, State: state})
	if err != nil {
		t.Fatalf("rich avatar instance blocked entry: %v", err)
	}
	// The native mode1 contract is identical to the already confirmed
	// clone/look header and extension layout, regardless of NOTI13 instance.
	expected, err := protocol.DetailedEquipment([]protocol.DetailedWorn{{Slot: 1, Template: 517560000, Durability: 9, HeaderTemplateA: 517562678, AvatarOptions: []byte{1, 2}, AvatarSockets: []byte{3, 4}}})
	if err != nil || !bytes.Contains(packet, expected) {
		t.Fatalf("native detailed avatar block changed: %v", err)
	}
	if !bytes.Equal(state, before) {
		t.Fatal("entry projection changed stored instance")
	}
	loaded, err := inventory.ReadBag(state)
	if err != nil || !bytes.Equal(loaded.Worn[0].Record, record) {
		t.Fatal("stored inventory record lost")
	}
	inventoryPacket, err := inventory.EquipmentPayload(3, loaded.WornBaseItems(), true)
	expectedRow := inventory.EquipmentRow(loaded.WornBaseItems()[0])
	if err != nil || !bytes.Contains(inventoryPacket, expectedRow[:]) {
		t.Fatalf("NOTI13 lost avatar instance: %v", err)
	}
}
