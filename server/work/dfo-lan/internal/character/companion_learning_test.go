package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"encoding/json"
	"testing"
)

func TestCompanionLearningUsesOrdinaryCostsWithoutPlayerStore(t *testing.T) {
	c, err := catalog.LoadCharacters("../../configs/characters.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	l, err := LoadLearningCatalog(testfixture.SkillCatalogPath(t, "next27"), c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{Catalog: c, Learning: l}
	p := c.Professions[0]
	state := State{Level: 115, Advancement: 1, SourcePath: p.Path, SourceSHA256: p.RawSHA256, InitialSkills: p.InitialSkills, SkillPoints: [2]uint16{100, 100}}
	raw, _ := json.Marshal(state)
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	fields["inventory"] = json.RawMessage(`{"gold":123,"worn":[{"slot":12,"template":101000013}]}`)
	fields["future_field"] = json.RawMessage(`{"keep":true}`)
	raw, _ = json.Marshal(fields)
	r := Character{ID: -1, Profession: 0, ConfigVersion: c.Source.SaveIdentity(), State: raw}
	before := bytes.Clone(raw)
	// Source fixture skill 4 is swordmaster lightsword mastery (15 SP).
	var req protocol.SkillPurchase
	if err = json.Unmarshal([]byte(`{"Tree":0,"Entries":[{"ID":4,"Delta":1}]}`), &req); err != nil {
		t.Fatal(err)
	}
	next, err := s.ApplySkillPurchase(r, req)
	if err != nil {
		t.Fatal(err)
	}
	var got State
	json.Unmarshal(next, &got)
	if got.LearnedSkills[0][4] != 1 || got.SkillPoints != [2]uint16{85, 100} {
		t.Fatalf("unexpected skill/SP: %v %v", got.LearnedSkills, got.SkillPoints)
	}
	choices, err := s.CompanionSkillChoices(r)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, choice := range choices {
		if choice.ID == 4 {
			found = true
			if choice.NextCost == nil || *choice.NextCost != 15 {
				t.Fatal("source cost missing", choice)
			}
		}
	}
	if !found {
		t.Fatal("learnable source skill missing")
	}
	var after map[string]json.RawMessage
	json.Unmarshal(next, &after)
	for _, key := range []string{"inventory", "future_field"} {
		if !bytes.Equal(fields[key], after[key]) {
			t.Fatalf("changed %s", key)
		}
	}
	if !bytes.Equal(r.State, before) {
		t.Fatal("mutated caller state")
	}
	// A foreign branch and insufficient SP must both refuse the same request.
	for _, invalid := range []State{
		{Level: 115, Advancement: 2, SourcePath: p.Path, SourceSHA256: p.RawSHA256, InitialSkills: p.InitialSkills, SkillPoints: [2]uint16{100, 100}},
		{Level: 115, Advancement: 1, SourcePath: p.Path, SourceSHA256: p.RawSHA256, InitialSkills: p.InitialSkills},
	} {
		r.State, _ = json.Marshal(invalid)
		if _, err = s.ApplySkillPurchase(r, req); err == nil {
			t.Fatal("invalid purchase accepted")
		}
	}
}
