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
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWeaponSkinDungeonSwitchAndOwnershipSQLite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	store := fixture.Storage()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "weapon-skin-dungeon")
	if err != nil {
		t.Fatal(err)
	}
	weapon := inventory.BagEquipment{Slot: 12, Template: 101010438, Durability: 37}
	raw, err := inventory.SaveBag(json.RawMessage(`{"level":115,"future":{"keep":true},"inventory":{"future_bag":{"keep":true}}}`), inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{weapon}, WeaponSkins: []uint32{101010912, 101010438}})
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "weaponfix", Profession: 0, ConfigVersion: strings.Repeat("a", 64), Request: []byte{0}, State: raw}, 24)
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{store: store, role: role, characters: &character.Service{Store: store}, activeDungeon: &dungeon.Session{}}
	request := func(skin uint32) []byte {
		p := make([]byte, 88)
		binary.LittleEndian.PutUint32(p, 4)
		binary.LittleEndian.PutUint32(p[8:], skin)
		return p
	}
	for _, skin := range []uint32{101010912, 101010438, 101010912, 0, 101010912} {
		plan, err := w.syncSkin(request(skin), func(map[string]any) {})
		if err != nil {
			t.Fatal(err)
		}
		if len(plan) != 2 || plan[0].ID != 1546 || plan[1].Kind != 1 || plan[1].ID != 1565 || !bytes.Equal(plan[1].Payload, protocol.WeaponSkinSelectionEcho(skin)) {
			t.Fatalf("wrong native refresh: %+v", plan)
		}
		for _, p := range plan {
			if p.Kind == 0 && p.ID == 2 {
				t.Fatal("dungeon actor reconstructed")
			}
		}
		bag, err := inventory.ReadBag(w.role.State)
		if err != nil || bag.WeaponSkin != skin {
			t.Fatalf("save mismatch: %+v %v", bag, err)
		}
		got, _ := json.Marshal(bag.Worn[0])
		want, _ := json.Marshal(weapon)
		if !bytes.Equal(got, want) || !bytes.Contains(w.role.State, []byte(`"future_bag":{"keep":true}`)) {
			t.Fatal("real item/unknown state changed")
		}
	}
	before := append([]byte(nil), w.role.State...)
	plan, err := w.syncSkin(request(999999), func(map[string]any) {})
	if err != nil || len(plan) != 1 || plan[0].ID != 1546 || !bytes.Equal(before, w.role.State) {
		t.Fatalf("unowned selection not safely refused: %+v %v", plan, err)
	}
	persisted, err := fixture.CharacterState(ctx, role.ID)
	if err != nil || !bytes.Equal(before, persisted) {
		t.Fatal("refusal wrote persisted save")
	}
	plan, err = w.syncSkin(request(101010912), func(map[string]any) {})
	if err != nil || len(plan) != 0 {
		t.Fatal("duplicate sync rebuilt weapon")
	}
	if _, err := store.CharacterEventReceipt(ctx, account, role.ID, fmt.Sprintf("weapon-skin-v2:%d:5:999999", role.ID)); err == nil {
		t.Fatal("refusal recorded success receipt")
	}
	// Town apply/clear must finish after the weapon-avatar appearance rebuild.
	cat, err := catalog.LoadCharacters("../../configs/characters.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	w.characters.Catalog = cat
	w.activeDungeon = nil
	for _, skin := range []uint32{101010438, 0, 101010912} {
		plan, err = w.syncSkin(request(skin), func(map[string]any) {})
		if err != nil || len(plan) != 3 || plan[1].Kind != 0 || plan[1].ID != 2 || plan[2].Kind != 1 || plan[2].ID != 1565 || !bytes.Equal(plan[2].Payload, protocol.WeaponSkinSelectionEcho(skin)) {
			t.Fatalf("town apply/clear left avatar above skin: %+v %v", plan, err)
		}
		if got := binary.LittleEndian.Uint32(plan[1].Payload[5+0x6c:]); got != skin {
			t.Fatalf("town model override=%d, combat override=%d", got, skin)
		}
	}
}

func TestWeaponSkinRestoredAfterEntryActor(t *testing.T) {
	state := skinState(t, []uint32{101010912}, 101010912)
	echo, err := weaponSkinActorRestore(state)
	if err != nil {
		t.Fatal(err)
	}
	plan := (entryPayloads{WeaponSkinActor: echo, Basic: []byte{1}, Worn: []byte{2}, WornSlots: []byte{3}, WornUpdate: []byte{4}, SkinCargo: []byte{5}}).packets()
	found, last := -1, -1
	for i, p := range plan {
		if p.Kind == 0 && (p.ID == 2 || p.ID == 13 || p.ID == 14 || p.ID == 1545) {
			last = i
		}
		if p.ID == 1565 {
			found = i
			if p.Kind != 1 || !bytes.Equal(p.Payload, protocol.WeaponSkinSelectionEcho(101010912)) {
				t.Fatal("wrong entry echo")
			}
		}
	}
	if found <= last {
		t.Fatal("weapon selection preceded actor/worn/cargo restoration")
	}
	if echo, err := weaponSkinActorRestore(skinState(t, nil, 0)); err != nil || echo != nil {
		t.Fatal("empty save emitted a cosmetic")
	}
}

func TestWeaponSkinRefreshFollowsAllEquipmentRebuilds(t *testing.T) {
	w := &worldSession{characters: &character.Service{}, activeDungeon: &dungeon.Session{}, role: database.Character{State: skinState(t, []uint32{101010912}, 101010912)}}
	base := []outboundPacket{{Name: "clone", Kind: 0, ID: 2, Payload: []byte{1}}, {Name: "ordinary gear", Kind: 0, ID: 14, Payload: []byte{2}}}
	for _, move := range []protocol.ItemMoveRequest{{SourceList: 0, DestinationList: 3, DestinationSlot: 12}, {SourceList: 3, SourceSlot: 12, DestinationList: 0}, {SourceList: 3, DestinationList: 3, SourceSlot: 12, DestinationSlot: 24}} {
		plan, err := appendDungeonWeaponSkinAfterMove(base, w, move)
		if err != nil || len(plan) != 4 || plan[3].Kind != 1 || plan[3].ID != 1565 {
			t.Fatalf("worn move missed final weapon refresh: %+v %v", plan, err)
		}
	}
	if plan, err := appendDungeonWeaponSkinAfterMove(base, w, protocol.ItemMoveRequest{SourceList: 0, DestinationList: 0}); err != nil || len(plan) != 2 {
		t.Fatal("bag move refreshed actor")
	}
	if plan, err := appendDungeonWeaponSkinAfterMove(nil, w, protocol.ItemMoveRequest{SourceList: 3}); err != nil || len(plan) != 0 {
		t.Fatal("duplicate/refused move refreshed actor")
	}
	w.activeDungeon = nil
	if plan, err := appendDungeonWeaponSkinAfterMove(base, w, protocol.ItemMoveRequest{SourceList: 3}); err != nil || len(plan) != 4 {
		t.Fatal("town equipment missed final weapon override")
	}
}
