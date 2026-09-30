package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"testing"
)

// Job 9 (demonic swordman) and job 10 (creator mage) declare [max grow count] 1
// with a single [growtype 1] block, so their characters never leave advancement
// 0 and their [awakening 1..3] grants sit in the growtype-0 column.
// AwakeningSkillGrants exposes that block as advancement index 0.
//
// Live evidence 2026-09-26: the dark swordman (adv 0, level 110) sends CMD2177
// stage 1 and the server answered "awakening must progress sequentially" while
// the client kept re-showing the 1st-awakening prompt.
func TestBranchlessProfessionAwakensFromGrowtypeZeroColumn(t *testing.T) {
	branchless := catalog.Profession{
		RawSHA256:       "hash",
		AwakeningSkills: map[byte]map[byte][]int32{0: {1: {263, 1}, 2: {256, 1, 255, 1}, 3: {269, 1}}},
	}
	// A profession that keeps a change-of-job branch: advancement 0 there still
	// means "unadvanced", so it must stay refused.
	branched := catalog.Profession{
		RawSHA256:         "hash",
		AdvancementGrowth: map[byte]map[string]float32{1: {"[hp max]": 60}},
		AwakeningSkills:   map[byte]map[byte][]int32{1: {1: {86, 1}, 2: {245, 1}, 3: {112, 1}}},
	}
	s := Service{
		Catalog: catalog.Characters{
			Source:      pvf.ArchiveSnapshot{Checksum: "version"},
			Professions: map[byte]catalog.Profession{9: branchless, 0: branched},
		},
		Learning: &LearningCatalog{index: map[byte]map[uint16]LearningDefinition{9: {}, 0: {}}},
	}
	for _, id := range []uint16{263, 256, 255, 269} {
		s.Learning.index[9][id] = LearningDefinition{Fields: map[string][]pvf.Token{"[required level]": {{Type: 0, Value: 50}}}}
	}
	for _, id := range []uint16{86, 245, 112} {
		s.Learning.index[0][id] = LearningDefinition{Fields: map[string][]pvf.Token{"[required level]": {{Type: 0, Value: 50}}}}
	}

	// The projection must accept the pair the domain now produces.
	if wire, err := (State{Advancement: 0, Awakening: 3}).WireAdvancement(); err != nil || wire != 48 {
		t.Fatalf("adv=0 stage=3 wire: got %d err %v, want 48", wire, err)
	}
	// Out-of-range values must still be rejected.
	if _, err := (State{Advancement: 0, Awakening: 4}).WireAdvancement(); err == nil {
		t.Fatal("stage 4 accepted")
	}

	role := storage.Character{Name: "DarkSword", WireID: 501, Profession: 9, ConfigVersion: "version", State: json.RawMessage(`{"level":110,"advancement":0,"source_sha256":"hash"}`)}
	for stage := byte(1); stage <= 3; stage++ {
		raw, err := s.ApplyAwakening(role, stage)
		if err != nil {
			t.Fatalf("branchless stage %d refused: %v", stage, err)
		}
		role.State = raw
		var st State
		if err = json.Unmarshal(raw, &st); err != nil {
			t.Fatal(err)
		}
		if st.Advancement != 0 || st.Awakening != stage {
			t.Fatalf("stage %d persisted %d/%d", stage, st.Advancement, st.Awakening)
		}
		if wire, e := st.WireAdvancement(); e != nil || wire != stage<<4 {
			t.Fatalf("stage %d wire: got %d err %v, want %d", stage, wire, e, stage<<4)
		}
	}

	// The level gate still applies.
	low := storage.Character{Name: "Low", Profession: 9, ConfigVersion: "version", State: json.RawMessage(`{"level":49,"advancement":0,"source_sha256":"hash"}`)}
	if _, err := s.ApplyAwakening(low, 1); err == nil {
		t.Fatal("stage 1 accepted below level 50")
	}
	// Skipping a stage is still refused.
	skip := storage.Character{Name: "Skip", Profession: 9, ConfigVersion: "version", State: json.RawMessage(`{"level":115,"advancement":0,"source_sha256":"hash"}`)}
	if _, err := s.ApplyAwakening(skip, 2); err == nil {
		t.Fatal("stage 2 accepted before stage 1")
	}
	// Source mismatch is still refused.
	stale := storage.Character{Name: "Stale", Profession: 9, ConfigVersion: "version", State: json.RawMessage(`{"level":115,"advancement":0,"source_sha256":"other"}`)}
	if _, err := s.ApplyAwakening(stale, 1); err == nil {
		t.Fatal("source mismatch accepted")
	}

	// A branched profession at advancement 0 must still refuse awakening.
	unadvanced := storage.Character{Name: "Unadvanced", Profession: 0, ConfigVersion: "version", State: json.RawMessage(`{"level":115,"advancement":0,"source_sha256":"hash"}`)}
	if _, err := s.ApplyAwakening(unadvanced, 1); err == nil {
		t.Fatal("unadvanced branched profession awakened")
	}
}

// A branchless profession whose catalog entry carries an empty growtype-0
// column (the state before characters.*.json is re-exported) must not be
// treated as awakenable — the gate needs the grants, not just the absence of
// a growth table.
func TestBranchlessGateRequiresGrowtypeZeroGrants(t *testing.T) {
	noGrants := catalog.Profession{RawSHA256: "hash", AwakeningSkills: map[byte]map[byte][]int32{}}
	s := Service{
		Catalog:  catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: "version"}, Professions: map[byte]catalog.Profession{9: noGrants}},
		Learning: &LearningCatalog{index: map[byte]map[uint16]LearningDefinition{9: {}}},
	}
	role := storage.Character{Name: "NoGrants", Profession: 9, ConfigVersion: "version", State: json.RawMessage(`{"level":115,"advancement":0,"source_sha256":"hash"}`)}
	if _, err := s.ApplyAwakening(role, 1); err == nil {
		t.Fatal("awakening accepted without growtype-0 grants")
	}
}

// Locks the release catalog to the inner-PVF source: characters.skycastle-release.json
// must keep the growtype-0 awakening grants for job 9/10. Re-exporting the
// catalog without the fixed AwakeningSkillGrants silently drops them and both
// branchless professions lose awakening again. Values are verbatim from
// character/swordman/dsswordman.chr and character/mage/creatormage.chr,
// [growtype 1] -> [awakening 1..3] -> [awakening skill].
func TestReleaseCatalogKeepsBranchlessAwakeningGrants(t *testing.T) {
	c, err := catalog.LoadCharacters("../../configs/characters.skycastle-release.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		job    byte
		grants map[byte][]int32
	}{
		{9, map[byte][]int32{1: {263, 1}, 2: {256, 1, 255, 1}, 3: {269, 1}}},
		{10, map[byte][]int32{1: {273, 1, 268, 1}, 2: {274, 1, 261, 1, 262, 1}, 3: {407, 1}}},
	} {
		prof, ok := c.Professions[tc.job]
		if !ok {
			t.Fatalf("job %d missing from the release catalog", tc.job)
		}
		if len(prof.AdvancementGrowth) != 0 {
			t.Fatalf("job %d unexpectedly has a change-of-job branch", tc.job)
		}
		for stage, want := range tc.grants {
			got := prof.AwakeningSkills[0][stage]
			if len(got) != len(want) {
				t.Fatalf("job %d stage %d grants %v, want %v", tc.job, stage, got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("job %d stage %d grants %v, want %v", tc.job, stage, got, want)
				}
			}
		}
	}
}

// Exercise both branchless jobs against the catalogs actually loaded by the
// launcher, including the entry packets used immediately after awakening and
// after reselecting a saved character. Unknown save fields must survive.
func TestBranchlessReleaseAwakeningPreservesSaveAndBuildsEntry(t *testing.T) {
	s, c := loadAwakeningGrantFixture(t)
	for _, job := range []byte{9, 10} {
		t.Run(fmt.Sprintf("job%d", job), func(t *testing.T) {
			prof := c.Professions[job]
			state := State{Level: 115, AllJobsPilot: true, SourceSHA256: prof.RawSHA256, Attributes: prof.InitialAttributes, InitialSkills: prof.InitialSkills}
			raw, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err = json.Unmarshal(raw, &fields); err != nil {
				t.Fatal(err)
			}
			fields["future_save_field"] = json.RawMessage(`{"value":123,"list":[1,2]}`)
			raw, err = json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			role := storage.Character{Name: "BranchlessTest", WireID: 503, Profession: job, ConfigVersion: c.Source.Checksum, State: raw}
			for stage := byte(1); stage <= 3; stage++ {
				role.State, err = s.ApplyAwakening(role, stage)
				if err != nil {
					t.Fatalf("stage%d: %v", stage, err)
				}
				if err = json.Unmarshal(role.State, &state); err != nil {
					t.Fatal(err)
				}
				for tree := range state.LearnedSkills {
					for grantedStage := byte(1); grantedStage <= stage; grantedStage++ {
						grants := prof.AwakeningSkills[0][grantedStage]
						for i := 0; i < len(grants); i += 2 {
							if state.LearnedSkills[tree][uint16(grants[i])] < byte(grants[i+1]) {
								t.Fatalf("stage%d tree%d missing source grant %v", stage, tree, grants[i:i+2])
							}
						}
					}
				}
				if _, err = s.EntryBasicProbe(role, [2]byte{}); err != nil {
					t.Fatalf("stage%d basic entry: %v", stage, err)
				}
				if _, err = s.EntrySkills(role); err != nil {
					t.Fatalf("stage%d skill entry: %v", stage, err)
				}
				if _, err = s.EntryAddition(role); err != nil {
					t.Fatalf("stage%d addition entry: %v", stage, err)
				}
				if err = json.Unmarshal(role.State, &fields); err != nil {
					t.Fatal(err)
				}
				if string(fields["future_save_field"]) != `{"value":123,"list":[1,2]}` {
					t.Fatal("unknown save field changed")
				}
				before := string(role.State)
				role.State, err = s.ApplyAwakening(role, stage)
				if err != nil || string(role.State) != before {
					t.Fatalf("stage%d repeat changed state: %v", stage, err)
				}
			}
			if state.TechniquePoints[0] != 5 {
				t.Fatal("third awakening lost its VP pool")
			}
		})
	}
}
