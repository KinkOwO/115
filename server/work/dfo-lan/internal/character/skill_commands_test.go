package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
	"reflect"
	"testing"
)

func TestSavedSkillCommandsOverrideSourceOnEntry(t *testing.T) {
	prof := catalog.Profession{
		RawSHA256:         "source",
		InitialSkills:     []int32{70, 1, 1},
		SkillCommands:     map[uint16][]uint32{70: {0, 1, 6}},
		InitialSkillSlots: map[uint16]uint16{70: 0},
	}
	s := Service{Catalog: catalog.Characters{Professions: map[byte]catalog.Profession{1: prof}}}
	role := storage.Character{Profession: 1}
	state := State{Level: 1, SourceSHA256: "source", InitialSkills: prof.InitialSkills, SkillCommands: []byte{1, 70, 0, 1, 8}}
	rows, err := s.skillRows(role, state, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !reflect.DeepEqual(rows[0].Commands, []uint32{8}) {
		t.Fatalf("custom command was not projected: %+v", rows)
	}
	state.SkillCommands = []byte{0}
	rows, err = s.skillRows(role, state, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || !reflect.DeepEqual(rows[0].Commands, []uint32{0, 1, 6}) {
		t.Fatalf("reset did not restore the source command: %+v", rows)
	}
}
