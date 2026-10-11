package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// Run the real consume/grant transaction. A whole equipment set must fit in
// visible slots; neither an empty locked slot nor one remaining visible slot
// permits a two-piece set to consume its source box.
func TestBoosterEquipmentCapacityRollback(t *testing.T) {
	const box uint32 = 900001
	const gear uint32 = 900002
	for _, tier := range []byte{0, 1, 2} {
		for _, free := range []uint16{0, 1} {
			t.Run(fmt.Sprintf("tier%d/free%d", tier, free), func(t *testing.T) {
				b := inventory.Bag{Version: "ordinary-bag-v1", Expansion: tier, Gold: 700, Items: []inventory.BagItem{{Slot: 65, Template: box, Amount: 1}}}
				end := uint16(9 + 40 + 8*uint16(tier))
				for s := uint16(9); s < end-free; s++ {
					b.Equipment = append(b.Equipment, inventory.BagEquipment{Slot: s, Template: gear, Durability: 60})
				}
				state, err := inventory.SaveBag(json.RawMessage(`{}`), b)
				if err != nil {
					t.Fatal(err)
				}
				store := newMockBoosterStore(database.Character{ID: 11, AccountID: 1, State: state})
				lootSvc := &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}, Items: map[uint32]catalog.LootItem{
					box: {ID: box, Kind: "stackable", StackableType: "[booster]", StackLimit: 1000}, gear: {ID: gear, Kind: "equipment"},
				}}, BagRules: inventory.BagRules{EquipmentSlots: [2]uint16{9, 64}}}
				cat := &BoosterCatalog{Definitions: map[uint32]BoosterDefinition{box: {Template: box, Type: "[booster]", Pools: []BoosterRewardPool{{DrawCount: 1, Candidates: []BoosterRewardCandidate{{Template: gear, Weight: 1000, Count: 2}}}}}}}
				w := &worldSession{role: store.character, loot: lootSvc}
				req := make([]byte, 8)
				binary.LittleEndian.PutUint16(req, 65)
				binary.LittleEndian.PutUint32(req[2:], 1)
				packets, err := w.openBoosterItem(context.Background(), store, nil, lootSvc, cat, odysseyWeaponChoices{}, req, req)
				if err == nil || len(packets) != 0 || !reflect.DeepEqual(store.character.State, state) || !reflect.DeepEqual(w.role.State, state) || len(store.receipts) != 0 {
					t.Fatal("full bag consumed box or partially granted set", err)
				}
				// Reuse the exact request after a real capacity change / two slots freed.
				if tier < 2 {
					b.Expansion++
				} else {
					b.Equipment = b.Equipment[:len(b.Equipment)-int(2-free)]
				}
				expanded, err := inventory.SaveBag(state, b)
				if err != nil {
					t.Fatal(err)
				}
				store.character.State = expanded
				w.role = store.character
				if _, err = w.openBoosterItem(context.Background(), store, nil, lootSvc, cat, odysseyWeaponChoices{}, req, req); err != nil {
					t.Fatal("box failed after room made", err)
				}
				got, err := inventory.ReadBag(store.character.State)
				if err != nil {
					t.Fatal(err)
				}
				if len(got.Items) != 0 || len(got.Equipment) != len(b.Equipment)+2 || got.Gold != 700 || len(store.receipts) != 1 {
					t.Fatal("reward transaction mismatch")
				}
				for _, row := range got.Equipment {
					if !got.OrdinarySlotUnlocked(row.Slot) {
						t.Fatal("equipment placed in locked slot", row.Slot)
					}
				}
			})
		}
	}
}
