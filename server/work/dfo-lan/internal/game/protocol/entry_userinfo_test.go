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
		if p[232+len(name)] != 1 || p[263+len(name)] != 0x03 || p[277+len(name)] != 0xff {
			t.Fatal("optional fields shifted the native flag boundaries")
		}
		// Creature segment {u32 item, dstr name, u8 present}: no creature
		// equipped means all three cells are zero (official sample shape).
		if binary.LittleEndian.Uint32(p[200+len(name):]) != 0 || binary.LittleEndian.Uint32(p[204+len(name):]) != 0 || p[208+len(name)] != 0 {
			t.Fatal("empty creature segment must be all zero")
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
	// The creature segment starts at a fixed offset for the empty-equipment
	// shape: u32 slot-26 template, dstr name, then the u8 present gate the
	// client uses to unhide the town follower (0 would keep it invisible).
	if binary.LittleEndian.Uint32(p[200+len(name):]) != 63000 {
		t.Fatal("creature item id missing from actor segment")
	}
	if binary.LittleEndian.Uint32(p[204+len(name):]) != uint32(len("Faras")) || string(p[208+len(name):208+len(name)+len("Faras")]) != "Faras" {
		t.Fatal("creature name missing from actor segment")
	}
	if p[208+len(name)+len("Faras")] != 1 {
		t.Fatal("creature present byte must be 1 when a creature item id is set")
	}
}
