package protocol

import (
	"bytes"
	"dfolan/internal/profileskin"
	"encoding/hex"
	"testing"
)

func TestProfileSkinRestoreCurrentUSReader(t *testing.T) {
	cargo, selected, err := ProfileSkinRestore(profileskin.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	// Official category-0 public rows, encoded with the US category selector;
	// US 0x1444eff40 consumes BOTH owned counts, not just the first vector.
	wantCargo, _ := hex.DecodeString("000300204e00000000000050c300000000000060ea0000000000000000")
	wantSelection, _ := hex.DecodeString("00204e000050c3000060ea00000000")
	if !bytes.Equal(cargo, wantCargo) || !bytes.Equal(selected, wantSelection) {
		t.Fatalf("cargo=%x selected=%x", cargo, selected)
	}
	state := profileskin.Defaults()
	state.Selected[2] = 60001
	if _, _, err = ProfileSkinRestore(state); err == nil {
		t.Fatal("unowned selection serialized")
	}
	state.Owned = append(state.Owned, profileskin.Owned{ID: 60001})
	_, selected, err = ProfileSkinRestore(state)
	if err != nil || selected[9] != 0x61 {
		t.Fatalf("stored selection not consumed: %x %v", selected, err)
	}
	state.Owned[0].Expires = 1
	if _, _, err = ProfileSkinRestore(state); err == nil {
		t.Fatal("unsupported timed entitlement serialized")
	}
}
