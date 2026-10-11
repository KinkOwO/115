package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func TestWeaponSkinForgeSnapshotAndNativeCompletionSQLite(t *testing.T) {
	for _, tc := range []struct {
		name          string
		mode          uint32
		owned, noMold bool
	}{
		{"normal", 2, false, false}, {"steel", 1, false, false},
		{"duplicate", 1, true, false}, {"refused", 1, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			f, err := database.OpenTestFixture(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			st := f.Storage()
			if err := st.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			account, err := st.DevelopmentAccount(ctx, "forge-"+tc.name)
			if err != nil {
				t.Fatal(err)
			}
			cat, err := catalog.LoadCharacters("../../configs/characters.generated.json")
			if err != nil {
				t.Fatal(err)
			}
			bag := inventory.Bag{Version: "ordinary-bag-v1", Gold: 1234,
				Equipment: []inventory.BagEquipment{{Slot: 41, Template: 101010438}, {Slot: 42, Template: 101010438}},
				Worn:      []inventory.BagEquipment{{Slot: 12, Template: 101010912, Durability: 37}},
				Items:     []inventory.BagItem{{Slot: 168, Template: 10308836, Amount: 2}, {Slot: 169, Template: 10308357, Amount: 2}},
			}
			if tc.owned {
				bag.WeaponSkins = []uint32{101010438}
			}
			if tc.noMold {
				bag.Items = nil
			}
			raw, err := inventory.SaveBag(json.RawMessage(`{"level":115,"future":{"keep":true}}`), bag)
			if err != nil {
				t.Fatal(err)
			}
			role, err := st.CreateCharacter(ctx, database.Character{AccountID: account, Name: "forge", Profession: 0, ConfigVersion: strings.Repeat("a", 64), Request: []byte{0}, State: raw}, 24)
			if err != nil {
				t.Fatal(err)
			}
			w := &worldSession{store: st, role: role, characters: &character.Service{Store: st, Catalog: cat, Equipment: shapeTestEquipment(t), WeaponSkinRecipes: shapeTestRecipes(), WeaponSkinBagRules: shapeTestBagRules()}}
			req := []byte{4, 0, 0, 0, 0, 0, 41, 0}
			binary.LittleEndian.PutUint32(req[1:], tc.mode)
			plan, err := w.makeSkin(req, func(map[string]any) {})
			if tc.noMold {
				if err == nil || len(plan) != 0 || !bytes.Equal(w.role.State, raw) {
					t.Fatal("refusal reported completion or changed state")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			after, err := inventory.ReadBag(w.role.State)
			if err != nil {
				t.Fatal(err)
			}
			wantSnapshot, err := protocol.InventoryRestore(after.Rows(), after.Expansion)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan) != 2 || plan[0].Kind != 0 || plan[0].ID != 13 || !bytes.Equal(plan[0].Payload, wantSnapshot) {
				t.Fatal("selected bag icon did not get authoritative restore")
			}
			last := plan[len(plan)-1]
			wantAck := []byte{1}
			if last.Kind != 1 || last.ID != 1592 || !bytes.Equal(last.Payload, wantAck) {
				t.Fatalf("wrong native completion: %+v", last)
			}
			if len(after.Equipment) != 1 || after.Equipment[0].Slot != 42 {
				t.Fatal("wrong weapon consumed")
			}
			for _, it := range after.Items {
				want := uint32(2)
				if it.Template == shapeTestRecipes()[tc.mode].Material || it.WeaponShape != 0 {
					want = 1
				}
				if it.Amount != want {
					t.Fatal("wrong mold debit/product amount")
				}
			}

			if after.Gold != 1234 || len(after.Worn) != 1 || after.Worn[0].Template != 101010912 || !bytes.Contains(w.role.State, []byte(`"future":{"keep":true}`)) {
				t.Fatal("unrelated player state changed")
			}
			persisted, err := f.CharacterState(ctx, role.ID)
			if err != nil || !bytes.Equal(persisted, w.role.State) {
				t.Fatal("response disagrees with committed save")
			}
		})
	}
}

func TestWeaponSkinOverridesAvatarAfterTownAndDungeonMoves(t *testing.T) {
	for _, inDungeon := range []bool{false, true} {
		for _, slot := range []uint16{10, 12} {
			for _, clearing := range []bool{false, true} {
				bag := inventory.Bag{Version: "ordinary-bag-v1", WeaponSkin: 101010438, WeaponSkins: []uint32{101010438}, Worn: []inventory.BagEquipment{{Slot: 10, Template: 400070177}, {Slot: 12, Template: 101010912}}}
				raw, err := inventory.SaveBag(json.RawMessage(`{"level":115}`), bag)
				if err != nil {
					t.Fatal(err)
				}
				w := &worldSession{characters: &character.Service{}, role: database.Character{State: raw}}
				if inDungeon {
					w.activeDungeon = &dungeon.Session{}
				}
				move := protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3, DestinationSlot: slot}
				if clearing {
					move = protocol.ItemMoveRequest{SourceList: 3, SourceSlot: slot, DestinationList: 1}
				}
				base := []outboundPacket{{"move", 1, 19, []byte{1}}, {"worn", 0, 14, []byte{2}}, {"clone/appearance", 0, 2, []byte{3}}}
				plan, err := appendDungeonWeaponSkinAfterMove(base, w, move)
				if err != nil {
					t.Fatal(err)
				}
				if len(plan) != 5 || !bytes.Equal(plan[4].Payload, protocol.WeaponSkinSelectionEcho(101010438)) || plan[4].Kind != 1 || plan[4].ID != 1565 {
					t.Fatalf("avatar/weapon move lost native override: dungeon=%v slot=%d clearing=%v", inDungeon, slot, clearing)
				}
				if !bytes.Equal(w.role.State, raw) {
					t.Fatal("visual override rewrote equipped item")
				}
			}
		}
	}
}
