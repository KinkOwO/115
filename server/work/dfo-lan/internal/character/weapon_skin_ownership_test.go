package character

import (
	"bytes"
	"context"
	"dfolan/internal/inventory"
	"encoding/json"
	"errors"
	"testing"
)

func TestWeaponSkinOwnershipUsesTransactionSave(t *testing.T) {
	state, err := inventory.SaveBag(json.RawMessage(`{"level":115,"future":{"keep":true}}`), inventory.Bag{Version: "ordinary-bag-v1", WeaponSkins: []uint32{101}})
	if err != nil {
		t.Fatal(err)
	}
	role := Character{ID: 7, AccountID: 1, State: state}
	store := &titleBookTestStore{role: role}
	svc := &Service{Store: store}
	stale := role
	stale.State, err = inventory.SaveBag(role.State, inventory.Bag{WeaponSkins: []uint32{999}})
	if err != nil {
		t.Fatal(err)
	}
	if _, changed, err := svc.ApplyWeaponSkin(context.Background(), stale, "foreign", 999); !errors.Is(err, inventory.ErrWeaponSkinNotOwned) || changed {
		t.Fatalf("stale snapshot granted unowned appearance: changed=%v err=%v", changed, err)
	}
	if !bytes.Equal(store.role.State, state) {
		t.Fatal("refusal changed save")
	}
	saved, changed, err := svc.ApplyWeaponSkin(context.Background(), stale, "owned", 101)
	if err != nil || !changed {
		t.Fatalf("owned skin refused: %v", err)
	}
	if !bytes.Contains(saved.State, []byte(`"future":{"keep":true}`)) {
		t.Fatal("unknown state lost")
	}
}

func TestLegacyWeaponSkinRemainsOwnedAfterClearAndSwitch(t *testing.T) {
	for _, next := range []uint32{0, 202} {
		state, err := inventory.SaveBag(json.RawMessage(`{"level":115}`), inventory.Bag{Version: "ordinary-bag-v1", WeaponSkin: 101, WeaponSkins: []uint32{202}})
		if err != nil {
			t.Fatal(err)
		}
		role := Character{ID: 7, AccountID: 1, State: state}
		store := &titleBookTestStore{role: role}
		svc := &Service{Store: store}
		saved, changed, err := svc.ApplyWeaponSkin(context.Background(), role, "change", next)
		if err != nil || !changed {
			t.Fatalf("change failed: %v", err)
		}
		bag, err := inventory.ReadBag(saved.State)
		if err != nil || len(bag.WeaponSkins) != 2 || bag.WeaponSkins[0] != 202 || bag.WeaponSkins[1] != 101 {
			t.Fatalf("legacy skin lost: %+v %v", bag, err)
		}
		if _, changed, err := svc.ApplyWeaponSkin(context.Background(), saved, "return", 101); err != nil || !changed {
			t.Fatalf("legacy skin cannot be reapplied: %v", err)
		}
	}
}
