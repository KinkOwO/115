package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
	"encoding/json"
	"testing"
)

func advancementFixture(t *testing.T, advancement, awakening byte) (*Service, storage.Character) {
	t.Helper()
	c, err := catalog.LoadCharacters("../../configs/characters.auto-skills-candidate.json")
	if err != nil {
		t.Fatal(err)
	}
	p := c.Professions[12] // knight: advancement branches 1..4, branch 4 is dragon knight
	state := State{Level: 50, Advancement: advancement, Awakening: awakening, Attributes: p.InitialAttributes, InitialSkills: p.InitialSkills, SourceSHA256: p.RawSHA256}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{Profession: 12, ConfigVersion: c.Source.Checksum, State: raw}
	return &Service{Catalog: c}, role
}

func decodeState(t *testing.T, raw json.RawMessage) State {
	t.Helper()
	var st State
	if err := json.Unmarshal(raw, &st); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestApplyAdvancementToDragonKnight(t *testing.T) {
	s, role := advancementFixture(t, 0, 0)
	raw, err := s.ApplyAdvancement(role, 4)
	if err != nil {
		t.Fatal(err)
	}
	st := decodeState(t, raw)
	if st.Advancement != 4 || st.Awakening != 0 {
		t.Fatalf("got advancement=%d awakening=%d, want 4/0", st.Advancement, st.Awakening)
	}
}

func TestApplyAdvancementResetsAwakening(t *testing.T) {
	s, role := advancementFixture(t, 2, 3)
	raw, err := s.ApplyAdvancement(role, 4)
	if err != nil {
		t.Fatal(err)
	}
	st := decodeState(t, raw)
	if st.Advancement != 4 || st.Awakening != 0 {
		t.Fatalf("got advancement=%d awakening=%d, want 4/0", st.Advancement, st.Awakening)
	}
}

func TestApplyAdvancementIdempotentOnSameBranch(t *testing.T) {
	s, role := advancementFixture(t, 4, 0)
	raw, err := s.ApplyAdvancement(role, 4)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != string(role.State) {
		t.Fatal("selecting the current branch must be a no-op")
	}
}

func TestApplyAdvancementRejectsInvalidTarget(t *testing.T) {
	s, role := advancementFixture(t, 0, 0)
	if _, err := s.ApplyAdvancement(role, 0); err == nil {
		t.Fatal("advancement 0 (base profession) must be rejected")
	}
	if _, err := s.ApplyAdvancement(role, 5); err == nil {
		t.Fatal("a branch the profession does not ship must be rejected")
	}
}
