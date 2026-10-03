package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/savecontract"

	"encoding/json"
	"fmt"
	"testing"
)

func TestAwakeningStagesPreserveInventoryAndRanks(t *testing.T) {
	prof := catalog.Profession{RawSHA256: "hash", AwakeningSkills: map[byte]map[byte][]int32{1: {1: {86, 1}, 2: {245, 1}, 3: {112, 1}}}}
	s := Service{Catalog: catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: "version"}, Professions: map[byte]catalog.Profession{0: prof}}, Learning: &LearningCatalog{index: map[byte]map[uint16]LearningDefinition{0: {}}}}
	for _, id := range []uint16{86, 245, 112} {
		s.Learning.index[0][id] = LearningDefinition{Fields: map[string][]pvf.Token{"[required level]": {{Type: 0, Value: 50}}, "[skill fitness growtype]": {{Type: 0, Value: 1}}, "[type]": {{Type: 6, Text: "[active]"}}}}
	}
	r := Character{ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"level":115,"advancement":1,"source_sha256":"hash","inventory":{"gold":123,"worn":[{"slot":12,"template":101000013}]},"learned_skills":[{"86":7},{}]}`)}
	if _, e := s.ApplyAwakening(r, 2); e == nil {
		t.Fatal("stage skipped")
	}
	for stage := byte(1); stage <= 3; stage++ {
		var e error
		r.State, e = s.ApplyAwakening(r, stage)
		if e != nil {
			t.Fatal(e)
		}
		var st State
		json.Unmarshal(r.State, &st)
		wire, e := st.WireAdvancement()
		if e != nil || wire != 1+stage*16 || st.Advancement != 1 || st.LearnedSkills[0][86] != 7 {
			t.Fatal(st, e)
		}
		var raw map[string]json.RawMessage
		json.Unmarshal(r.State, &raw)
		if string(raw["inventory"]) != `{"gold":123,"worn":[{"slot":12,"template":101000013}]}` {
			t.Fatal("inventory changed")
		}
		before := string(r.State)
		r.State, e = s.ApplyAwakening(r, stage)
		if e != nil || before != string(r.State) {
			t.Fatal("duplicate changed state")
		}
	}
}

func TestSourceAwakeningGrants(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.awakening-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog("../../configs/skills.release.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	s := Service{Catalog: c, Learning: l}
	count := 0
	for job, prof := range c.Professions {
		for adv, stages := range prof.AwakeningSkills {
			if len(stages[1]) == 0 || len(stages[2]) == 0 || len(stages[3]) == 0 {
				continue
			}
			t.Run(fmt.Sprintf("job%d_adv%d", job, adv), func(t *testing.T) {
				state := State{Level: 115, Advancement: adv, AllJobsPilot: true, Attributes: prof.InitialAttributes, InitialSkills: prof.InitialSkills, SourceSHA256: prof.RawSHA256}
				raw, _ := json.Marshal(state)
				role := Character{Name: "AwakeTest", WireID: 503, Profession: job, ConfigVersion: c.Source.SaveIdentity(), State: raw}
				for stage := byte(1); stage <= 3; stage++ {
					role.State, e = s.ApplyAwakening(role, stage)
					if e != nil {
						t.Fatalf("stage%d: %v", stage, e)
					}
					if _, e = s.EntrySkills(role); e != nil {
						t.Fatal(e)
					}
					if _, e = s.EntryAddition(role); e != nil {
						t.Fatal(e)
					}
					if _, e = s.EntryBasicProbe(role, [2]byte{}); e != nil {
						t.Fatal(e)
					}
					count++
				}
			})
		}
	}
	if count < 100 {
		t.Fatal("insufficient source coverage", count)
	}
	t.Logf("validated %d source awakening transitions", count)
}

func TestAwakenedSkillLearningUsesOwnJobAndStage(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.awakening-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog("../../configs/skills.release.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	d := l.index[0][86]
	st := State{Level: 115, Advancement: 1, Awakening: 1}
	if _, e := d.costForState(st, 2, map[uint16]byte{86: 1}); e != nil {
		t.Fatal(e)
	}
	st.Awakening = 0
	if _, e := d.costForState(st, 2, nil); e == nil {
		t.Fatal("unawakened learned awakening skill")
	}
	st.Advancement = 2
	st.Awakening = 3
	if _, e := d.costForState(st, 2, nil); e == nil {
		t.Fatal("other growtype learned awakening skill")
	}
	if d.ForAdvancement(1) {
		t.Fatal("shared base cap mutated")
	}
	d = l.index[0][114]
	st.Advancement = 1
	if !d.ForAwakening(1, 3) || d.ForAwakening(1, 2) {
		t.Fatal("third awakening eligibility")
	}
	if cost, e := d.costForState(st, 1, nil); e != nil || cost != 200 {
		t.Fatal("third awakening first cost", cost, e)
	}
	if cost, e := d.costForState(st, 2, nil); e != nil || cost != 100 {
		t.Fatal("third awakening later cost", cost, e)
	}
}
