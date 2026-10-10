package character

import (
	"context"
	"dfolan/internal/catalog"
	"encoding/binary"
	"encoding/json"
	"errors"
	"testing"
)

func TestAllActorProjectionsReadCurrentProfileFrames(t *testing.T) {
	professions, err := catalog.LoadCharacters("../../configs/characters.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	current := [2]uint32{20078, 50017}
	s := &Service{Catalog: professions, ProfileFrames: func(ctx context.Context, role Character) ([2]uint32, error) {
		if role.ID != 2 || role.AccountID != 1 {
			t.Fatal("wrong selection owner")
		}
		return current, nil
	}}
	role := Character{ID: 2, AccountID: 1, WireID: 2, Name: "FrameTest", Profession: 0, State: json.RawMessage(`{"level":115,"advancement":0}`)}
	for _, project := range []func(Character, [2]byte) ([]byte, error){s.EntryBasicProbe, s.AppearanceProbe} {
		for _, frames := range [][2]uint32{{20078, 50017}, {20000, 50000}, {500020001, 500050001}} {
			current = frames
			p, e := project(role, [2]byte{4, 7})
			if e != nil {
				t.Fatal(e)
			}
			if binary.LittleEndian.Uint32(p[5+0x2c:]) != frames[0] || binary.LittleEndian.Uint32(p[5+0x30:]) != frames[1] {
				t.Fatal("entry/equipment/party projection used stale skin state")
			}
		}
	}
	want := errors.New("selection read failed")
	s.ProfileFrames = func(context.Context, Character) ([2]uint32, error) { return [2]uint32{}, want }
	if _, e := s.EntryBasicProbe(role, [2]byte{}); !errors.Is(e, want) {
		t.Fatal("entry masked failed selection read")
	}
	if _, e := s.AppearanceProbe(role, [2]byte{}); !errors.Is(e, want) {
		t.Fatal("appearance masked failed selection read")
	}
}
