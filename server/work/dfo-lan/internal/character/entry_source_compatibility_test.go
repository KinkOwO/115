package character

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestEntrySkillsPreservePayloadAcrossProfessionHashChange(t *testing.T) {
	s, role, state := autoSkillFixture(t)
	state.SkillSlots[0] = map[uint16]uint16{62: 200}
	state.SkillSlots[1] = map[uint16]uint16{62: 201}
	var err error
	role.State, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	saved := append([]byte(nil), role.State...)
	want, err := s.EntrySkills(role)
	if err != nil {
		t.Fatal(err)
	}
	prof := s.Catalog.Professions[role.Profession]
	prof.RawSHA256 = "rebuilt-string-pool"
	s.Catalog.Professions[role.Profession] = prof
	got, err := s.EntrySkills(role)
	if err != nil {
		t.Fatalf("same profession reference rejected after hash change: %v", err)
	}
	if !bytes.Equal(got, want) || !bytes.Equal(role.State, saved) {
		t.Fatal("resource hash change altered skill payload or saved state")
	}
}

func TestEntrySkillsRejectDifferentProfessionReference(t *testing.T) {
	s, role, state := autoSkillFixture(t)
	// Exercise skillRows' own guard independently of automatic-skill grants.
	s.Learning = nil
	for _, path := range []string{"character/wrong.chr", ""} {
		state.SourcePath = path
		var err error
		role.State, err = json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.EntrySkills(role); err == nil {
			t.Fatalf("accepted mismatched profession reference %q", path)
		}
	}
}
