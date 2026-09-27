package inventory

import "testing"

// The saved list is what entry replays into the skin storage window, so it has
// to survive the round trip intact.
func TestWeaponSkinStorageKeepsSavedOrder(t *testing.T) {
	b := Bag{WeaponSkins: []uint32{101010912, 101010438}}
	got := b.WeaponSkinStorage()
	if len(got) != 2 || got[0] != 101010912 || got[1] != 101010438 {
		t.Fatalf("storage=%v, want [101010912 101010438]", got)
	}
}

// Saves written before the list existed only carry the applied skin. Falling
// back to it is what keeps "applied yesterday, storage empty today" from
// happening on an old character.
func TestWeaponSkinStorageFallsBackToAppliedSkin(t *testing.T) {
	b := Bag{WeaponSkin: 101010912}
	got := b.WeaponSkinStorage()
	if len(got) != 1 || got[0] != 101010912 {
		t.Fatalf("storage=%v, want [101010912]", got)
	}
}

// The client keys a storage row by the skin id alone, so a template replicated
// from two different bag slots must not produce two rows.
func TestWeaponSkinStorageDeduplicates(t *testing.T) {
	b := Bag{WeaponSkins: []uint32{101010912, 0, 101010912, 101010438}}
	got := b.WeaponSkinStorage()
	if len(got) != 2 || got[0] != 101010912 || got[1] != 101010438 {
		t.Fatalf("storage=%v, want [101010912 101010438]", got)
	}
}

// The applied skin is always in the tab - it was picked from there - so a save
// that carries a list without it (an applied skin predating the list, or a list
// built before this field existed) must still show it instead of dropping it.
func TestWeaponSkinStorageAddsAppliedSkinMissingFromList(t *testing.T) {
	b := Bag{WeaponSkins: []uint32{101010438}, WeaponSkin: 101010912}
	got := b.WeaponSkinStorage()
	if len(got) != 2 || got[0] != 101010438 || got[1] != 101010912 {
		t.Fatalf("storage=%v, want [101010438 101010912]", got)
	}
}

// Nothing replicated and nothing applied: entry must send no NOTI1545 at all.
func TestWeaponSkinStorageEmptyWhenNothingReplicated(t *testing.T) {
	if got := (Bag{}).WeaponSkinStorage(); len(got) != 0 {
		t.Fatalf("storage=%v, want empty", got)
	}
}
