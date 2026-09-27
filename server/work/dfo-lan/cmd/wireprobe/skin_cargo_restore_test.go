package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func skinState(t *testing.T, skins []uint32, applied uint32) json.RawMessage {
	t.Helper()
	state, e := inventory.SaveBag(json.RawMessage(`{"level":1}`), inventory.Bag{
		Version:     "ordinary-bag-v1",
		WeaponSkin:  applied,
		WeaponSkins: skins,
	})
	if e != nil {
		t.Fatal(e)
	}
	return state
}

// The client keeps the skin storage container as session state, so a relog shows
// an empty window unless entry replays the whole saved tab. Live 2026-09-26: the
// applied skin still drove the real model while the storage window was blank.
func TestSkinCargoRestoreReplaysSavedTab(t *testing.T) {
	got, e := skinCargoRestore(skinState(t, []uint32{101010912, 101010438}, 101010438))
	if e != nil {
		t.Fatal(e)
	}
	want := []byte{byte(protocol.SkinCargoWeaponShape)}
	want = binary.LittleEndian.AppendUint16(want, 2)
	want = binary.LittleEndian.AppendUint32(want, 101010912)
	want = binary.LittleEndian.AppendUint32(want, 101010438)
	want = binary.LittleEndian.AppendUint16(want, 0)
	if !bytes.Equal(got, want) {
		t.Fatalf("cargo %x, want %x", got, want)
	}
}

// An old save has no list, only the applied skin. Entry must still show that one
// rather than an empty window.
func TestSkinCargoRestoreFallsBackToAppliedSkin(t *testing.T) {
	got, e := skinCargoRestore(skinState(t, nil, 101010912))
	if e != nil {
		t.Fatal(e)
	}
	want := []byte{byte(protocol.SkinCargoWeaponShape)}
	want = binary.LittleEndian.AppendUint16(want, 1)
	want = binary.LittleEndian.AppendUint32(want, 101010912)
	want = binary.LittleEndian.AppendUint16(want, 0)
	if !bytes.Equal(got, want) {
		t.Fatalf("cargo %x, want %x", got, want)
	}
}

// A character that replicated nothing must not get a NOTI1545: the entry packet
// list skips empty payloads, so a nil body means the frame is not sent at all.
func TestSkinCargoRestoreSilentWithoutSkins(t *testing.T) {
	got, e := skinCargoRestore(skinState(t, nil, 0))
	if e != nil {
		t.Fatal(e)
	}
	if len(got) != 0 {
		t.Fatalf("cargo %x, want empty", got)
	}
}

// The container push has to sit in the entry sequence as NOTI1545, ahead of the
// final id-2 appearance frame, and a character with nothing replicated must not
// get the frame at all.
func TestEntrySendsSkinCargoOnlyWhenNonEmpty(t *testing.T) {
	body := []byte{byte(protocol.SkinCargoWeaponShape), 1, 0, 0xe0, 0x4d, 0x05, 0x06, 0, 0}
	var found *outboundPacket
	appearance := -1
	all := (entryPayloads{SkinCargo: body}).packets()
	for i, p := range all {
		if p.Name == "skin_cargo_restored" {
			copy := p
			found = &copy
		}
		if p.Name == "actor_appearance_ready" {
			appearance = i
		}
	}
	if found == nil {
		t.Fatal("skin_cargo_restored missing from the entry plan")
	}
	if found.ID != 1545 || !bytes.Equal(found.Payload, body) {
		t.Fatalf("packet=%+v, want id 1545 with the cargo body", found)
	}
	if appearance < 0 || appearance < indexOf(all, "skin_cargo_restored") {
		t.Fatal("actor appearance must stay the final data frame after the cargo push")
	}
	for _, p := range (entryPayloads{}).packets() {
		if p.Name == "skin_cargo_restored" {
			t.Fatal("a character with nothing replicated must not get NOTI1545")
		}
	}
}

// The worn row's golden frame is rebuilt from the client's own per-page selection
// table, so entry has to state the worn skin; without it the first open after a
// relog shows the storage unframed (live 2026-09-27).
func TestSkinSelectionRestoreNamesWornSkin(t *testing.T) {
	got, e := skinSelectionRestore(skinState(t, []uint32{101010912}, 101010912))
	if e != nil {
		t.Fatal(e)
	}
	want := []byte{byte(protocol.SkinCargoWeaponShape)}
	want = binary.LittleEndian.AppendUint32(want, 101010912)
	if !bytes.Equal(got, want) {
		t.Fatalf("selection %x, want %x", got, want)
	}
}

// Nothing applied means nothing to highlight: a zero id is not a selection, and
// the entry list drops the frame entirely.
func TestSkinSelectionRestoreSilentWithoutAppliedSkin(t *testing.T) {
	got, e := skinSelectionRestore(skinState(t, []uint32{101010912}, 0))
	if e != nil {
		t.Fatal(e)
	}
	if len(got) != 0 {
		t.Fatalf("selection %x, want empty", got)
	}
}

// The selection frame rides straight after the container push that fills the same
// page, and still ahead of the final id-2 appearance frame.
func TestEntrySendsSkinSelectionAfterCargo(t *testing.T) {
	cargo := []byte{byte(protocol.SkinCargoWeaponShape), 1, 0, 0xe0, 0x4d, 0x05, 0x06, 0, 0}
	selection := []byte{byte(protocol.SkinCargoWeaponShape), 0xe0, 0x4d, 0x05, 0x06}
	all := (entryPayloads{SkinCargo: cargo, SkinSelection: selection}).packets()
	at := indexOf(all, "skin_cargo_selected")
	cargoAt := indexOf(all, "skin_cargo_restored")
	appearance := indexOf(all, "actor_appearance_ready")
	if at < 0 {
		t.Fatal("skin_cargo_selected missing from the entry plan")
	}
	if all[at].ID != 1546 || !bytes.Equal(all[at].Payload, selection) {
		t.Fatalf("packet=%+v, want id 1546 with the selection body", all[at])
	}
	if cargoAt < 0 || at < cargoAt || appearance < 0 || at > appearance {
		t.Fatalf("order cargo=%d selection=%d appearance=%d", cargoAt, at, appearance)
	}
	for _, p := range (entryPayloads{}).packets() {
		if p.Name == "skin_cargo_selected" {
			t.Fatal("a character with no skin applied must not get NOTI1546")
		}
	}
}

func indexOf(packets []outboundPacket, name string) int {
	for i, p := range packets {
		if p.Name == name {
			return i
		}
	}
	return -1
}
