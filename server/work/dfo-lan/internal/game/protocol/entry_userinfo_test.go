package protocol

import (
	"encoding/binary"
	"testing"
)

// Offsets here are counted independently from the native reader sequence in
// docs/protocol/entry-userinfo.md, not by decoding through the serializer.
func TestEntryBasicNativeBoundaries(t *testing.T) {
	for _, name := range []string{"LanTest01", "角色一"} {
		p, err := UserInfoBasicProbe(EntryBasicProbe{ActorServerID: 503, Context: [2]byte{4, 7}, Character: CharacterRow{Name: name, Profession: 3, Level: 1}})
		if err != nil {
			t.Fatal(err)
		}
		if len(p) != 307+len(name) {
			t.Fatalf("minimum actor size=%d, want=%d", len(p), 307+len(name))
		}
		if p[0] != 0 || binary.LittleEndian.Uint16(p[1:]) != 1 || p[3] != 4 || p[4] != 7 {
			t.Fatal("actor notification header")
		}
		if binary.LittleEndian.Uint16(p[165:]) != 503 || binary.LittleEndian.Uint32(p[167:]) != uint32(len(name)) || string(p[171:171+len(name)]) != name {
			t.Fatal("fixed prefix shifted actor or UTF-8 name")
		}
		if p[171+len(name)] != 3 || p[173+len(name)] != 1 {
			t.Fatal("profession/level offset")
		}
		if p[232+len(name)] != 1 || p[263+len(name)] != 1 || p[277+len(name)] != 0xff {
			t.Fatal("optional fields shifted the native flag boundaries")
		}
	}
}

func TestEntryBasicRejectsInvalidActor(t *testing.T) {
	for _, id := range []uint16{0, 0xffff} {
		if _, err := UserInfoBasicProbe(EntryBasicProbe{ActorServerID: id, Character: CharacterRow{Name: "LanTest01", Level: 1}}); err == nil {
			t.Fatal("accepted reserved actor ID")
		}
	}
}

func TestEntryBasicWithCreature(t *testing.T) {
	name := "LanTest01"
	p, err := UserInfoBasicProbe(EntryBasicProbe{
		ActorServerID: 503,
		Context:       [2]byte{4, 7},
		Character: CharacterRow{
			Name:           name,
			Profession:     3,
			Level:          1,
			CreatureItemID: 63000,
			CreatureName:   "Faras",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	wantLen := 307 + len(name) + len("Faras")
	if len(p) != wantLen {
		t.Fatalf("actor with creature size=%d, want=%d", len(p), wantLen)
	}
}
