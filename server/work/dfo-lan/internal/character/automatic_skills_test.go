package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/testfixture"

	"testing"
)

func autoSkillFixture(t *testing.T) (*Service, Character, State) {
	t.Helper()
	c, e := catalog.LoadCharacters("../../configs/characters.auto-skills-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog(testfixture.SkillCatalogPath(t, "release"), c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	p := c.Professions[11]
	return &Service{Catalog: c, Learning: l}, Character{Profession: 11, ConfigVersion: c.Source.SaveIdentity()}, State{Level: 35, Advancement: 2, AllJobsPilot: true, SourcePath: p.Path, SourceSHA256: p.RawSHA256, InitialSkills: p.InitialSkills}
}

func TestAutomaticSpathaNoctis(t *testing.T) {
	s, role, st := autoSkillFixture(t)
	base, err := knownSkills(st, 0)
	if err != nil || base[62] != 0 {
		t.Fatal(base, err)
	}
	t.Log("BASELINE: Spatha Noctis rank0")
	for _, tree := range []int{0, 1} {
		known, err := s.knownSkills(role, st, tree)
		if err != nil || known[62] != 1 {
			t.Fatal(known, err)
		}
		rows, err := s.skillRows(role, st, tree)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, r := range rows {
			if r.ID == 62 {
				found = r.Level == 1 && r.Slot >= 14 && r.Slot < 255
			}
		}
		if !found {
			t.Fatal("automatic skill absent from palette")
		}
		count := 0
		for _, d := range s.Learning.index[11] {
			pre := d.Ints("[pre required skill]")
			for i := 0; i+1 < len(pre); i += 2 {
				if pre[i] == 62 && d.ForAdvancement(2) {
					if _, err = d.costForState(st, 1, known); err == nil {
						count++
					}
				}
			}
		}
		if count == 0 {
			t.Fatal("dependent skills still blocked")
		}
	}
	st.Level = 14
	known, err := s.knownSkills(role, st, 0)
	if err != nil || known[62] != 1 {
		t.Fatal("condition-one grant disappeared from legacy role", known, err)
	}
	st.Level, st.Advancement = 35, 1
	known, err = s.knownSkills(role, st, 0)
	if err != nil || known[62] != 0 {
		t.Fatal("foreign advancement grant", known, err)
	}
	t.Log("MODIFIED: Spatha Noctis rank1; prerequisites available; condition-one grant persists at level14; foreign advancement not granted")
}

func TestAutomaticAllProfessionGrants(t *testing.T) {
	s, role, st := autoSkillFixture(t)
	for job, p := range s.Catalog.Professions {
		for adv := range p.AdvancementSkills {
			role.Profession = job
			st.SourcePath, st.SourceSHA256, st.InitialSkills, st.Advancement, st.Level = p.Path, p.RawSHA256, p.InitialSkills, adv, 115
			if _, err := s.skillRows(role, st, 0); err != nil {
				t.Fatalf("job%d advancement%d: %v", job, adv, err)
			}
		}
	}
}
