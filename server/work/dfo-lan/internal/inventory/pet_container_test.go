package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestPetConsumableGrantAndSavedMigration(t *testing.T) {
	c := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}, Items: map[uint32]catalog.LootItem{24: {ID: 24, Kind: "stackable", StackableType: "[feed]"}}}
	r := BagRules{Source: "test", MissingStackLimit: 1000}
	bag := Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 65, Template: 24, Amount: 5}}}
	migrated, moved, err := SweepPetConsumables(bag, c, r)
	if err != nil || !moved || len(migrated.Items) != 0 || len(migrated.PetItems) != 1 || migrated.PetItems[0].Slot != 376 {
		t.Fatalf("migration = %+v, %v, %v", migrated, moved, err)
	}
	migrated, slot, err := migrated.Add(c, r, 24, 1500)
	if err != nil || slot != 376 || migrated.PetItems[0].Amount != 1505 {
		t.Fatalf("grant = %+v, %d, %v", migrated, slot, err)
	}
	state, err := SaveBag(json.RawMessage(`{}`), migrated)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := ReadBag(state)
	if err != nil || len(restored.PetItems) != 1 {
		t.Fatalf("restore = %+v, %v", restored, err)
	}
	body, err := PetContainerBody(restored, true)
	if err != nil || body[0] != 7 || binary.LittleEndian.Uint16(body[1:3]) != 1 {
		t.Fatalf("pet body = %d bytes, %v", len(body), err)
	}
	move := protocol.ItemMoveRequest{SourceList: 7, SourceSlot: 376, SourceItem: 24, DestinationList: 7, DestinationSlot: 377, Selection: 0xffffffff, Count: 5}
	restored, err = restored.MovePetStackRequest(c, r, move)
	if err != nil || len(restored.PetItems) != 2 {
		t.Fatalf("pet move = %+v, %v", restored, err)
	}
	out := protocol.ItemMoveRequest{SourceList: 7, SourceSlot: 377, SourceItem: 24, DestinationList: 0, DestinationSlot: 65, Selection: 0xffffffff, Count: 5}
	restored, err = restored.MovePetStackRequest(c, r, out)
	if err != nil || len(restored.Items) != 1 || restored.Items[0].Amount != 5 {
		t.Fatalf("pet withdraw = %+v, %v", restored, err)
	}
	back := protocol.ItemMoveRequest{SourceList: 0, SourceSlot: 65, SourceItem: 24, DestinationList: 7, DestinationSlot: 378, Selection: 0xffffffff, Count: 5}
	restored, err = restored.MovePetStackRequest(c, r, back)
	if err != nil || len(restored.Items) != 0 {
		t.Fatalf("pet deposit = %+v, %v", restored, err)
	}
}
