package character

import (
	"dfolan/internal/catalog/pvf"

	"encoding/json"

	"testing"
)

func TestBranchlessAwakeningGrantDoesNotDeadlockLearning(t *testing.T) {
	s, c := loadAwakeningGrantFixture(t)
	prof := c.Professions[9]
	raw, err := json.Marshal(State{Level: 115, SourcePath: prof.Path, SourceSHA256: prof.RawSHA256, InitialSkills: prof.InitialSkills})
	if err != nil {
		t.Fatal(err)
	}
	role := Character{Profession: 9, ConfigVersion: c.Source.SaveIdentity(), State: raw}
	for stage := byte(1); stage <= 2; stage++ {
		role.State, err = s.ApplyAwakening(role, stage)
		if err != nil {
			t.Fatal(err)
		}
	}
	var state State
	if err = json.Unmarshal(role.State, &state); err != nil {
		t.Fatal(err)
	}
	known, err := s.knownSkills(role, state, 0)
	if err != nil {
		t.Fatal(err)
	}
	if known[255] != 1 || known[81] != 0 {
		t.Fatal("source no longer reproduces the grant's missing prerequisite", known)
	}
	// Dark Cross (64) is an ordinary learnable skill without prerequisites.
	if _, err = s.Learning.index[9][64].costForState(state, 1, known); err != nil {
		t.Fatal(err)
	}
	known[64] = 1
	if err = s.validateLearningPrerequisites(9, known, map[uint16]byte{64: 1}, nil); err != nil {
		t.Fatal("unrelated purchase blocked by source grant 255 -> 81", err)
	}
}

// Use a private PostgreSQL schema to exercise the same awakening event and
// learning transaction as the gateway, without changing any player records.

func TestLearningPrerequisitesProtectChangesAndRefunds(t *testing.T) {
	s := Service{Learning: &LearningCatalog{index: map[byte]map[uint16]LearningDefinition{9: {
		255: {Fields: map[string][]pvf.Token{"[pre required skill]": {{Value: 81}, {Value: 1}}}},
		77:  {Fields: map[string][]pvf.Token{"[pre required skill]": {{Value: 8}, {Value: 1}}}},
	}}}}
	for _, tc := range []struct {
		name    string
		known   map[uint16]byte
		changes map[uint16]byte
		reduced map[uint16]bool
		refused bool
	}{
		{"new_skill_missing_prerequisite", map[uint16]byte{77: 1}, map[uint16]byte{77: 1}, nil, true},
		{"source_grant_upgrade_missing_prerequisite", map[uint16]byte{255: 2}, map[uint16]byte{255: 2}, nil, true},
		{"refund_breaks_dependent", map[uint16]byte{255: 1, 81: 0}, map[uint16]byte{81: 0}, map[uint16]bool{81: true}, true},
		{"refund_preserves_dependent", map[uint16]byte{255: 1, 81: 1}, map[uint16]byte{81: 1}, map[uint16]bool{81: true}, false},
		{"unrelated_refund_with_old_gap", map[uint16]byte{255: 1, 64: 0}, map[uint16]byte{64: 0}, map[uint16]bool{64: true}, false},
		{"new_skill_and_prerequisite", map[uint16]byte{77: 1, 8: 1}, map[uint16]byte{77: 1, 8: 1}, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := s.validateLearningPrerequisites(9, tc.known, tc.changes, tc.reduced)
			if (err != nil) != tc.refused {
				t.Fatalf("refused=%v, want %v: %v", err != nil, tc.refused, err)
			}
		})
	}
}
