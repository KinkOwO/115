package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestEntryWeaponSkinHeaderOnlyChangesNativeOverride(t *testing.T) {
	input := EntryBasicProbe{ActorServerID: 1, Character: CharacterRow{Name: "Deemo", Profession: 16, Level: 115}, PartyFrameID: 123, RequestFrameID: 456}
	base, err := UserInfoBasicProbe(input)
	if err != nil {
		t.Fatal(err)
	}
	input.WeaponSkinID = 117020007
	got, err := UserInfoBasicProbe(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(base) != len(got) || binary.LittleEndian.Uint32(got[113:]) != 117020007 {
		t.Fatal("native header field/packet boundary")
	}
	copy(got[113:117], base[113:117])
	if !bytes.Equal(base, got) {
		t.Fatal("override changed unrelated packet fields")
	}
}
