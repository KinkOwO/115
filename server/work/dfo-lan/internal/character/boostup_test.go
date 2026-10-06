package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"encoding/json"
	"testing"
)

func TestBoostLevelUsesSourceGrowthAwakeningAndPreservesAssets(t *testing.T) {
	s, g, _, _, role := adminFixture(t)
	c, e := catalog.LoadCharacters("../../configs/characters.awakening-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	s.Catalog = c
	g.Professions = c
	var before map[string]json.RawMessage
	if e = json.Unmarshal(role.State, &before); e != nil {
		t.Fatal(e)
	}
	next, e := s.BoostLevel(role, g, 115)
	if e != nil {
		t.Fatal(e)
	}
	var st State
	if e = json.Unmarshal(next.State, &st); e != nil {
		t.Fatal(e)
	}
	if st.Level != 115 || st.Awakening != 3 || st.Experience != g.Catalog.Thresholds[113] || st.SkillPoints[0] != st.SkillPoints[1] {
		t.Fatal("incomplete level/awakening/SP", st.Level, st.Awakening, st.Experience, st.SkillPoints)
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(next.State, &fields); e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"inventory", "foreign_payload"} {
		if !bytes.Equal(before[key], fields[key]) {
			t.Fatal("asset/unknown state changed", key)
		}
	}
	unchanged, e := s.BoostLevel(next, g, 115)
	if e != nil || !bytes.Equal(next.State, unchanged.State) {
		t.Fatal("at-cap training reset existing progression/skills", e)
	}
	if _, e = s.BoostLevel(next, g, 114); e == nil {
		t.Fatal("capsule downgraded character")
	}
	if _, e = s.EntrySkills(next); e != nil {
		t.Fatal("post boost skills not renderable", e)
	}
	t.Logf("source experience=%d SP=%v awakening=%d", st.Experience, st.SkillPoints, st.Awakening)
}
