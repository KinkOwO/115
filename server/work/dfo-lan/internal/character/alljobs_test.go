package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/testfixture"

	"encoding/json"
	"fmt"
	"testing"
)

func TestAllSourceAdvancementsEntryAndGrowth(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.alljobs-pilot.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog(testfixture.SkillCatalogPath(t, "next27"), c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	pc, e := catalog.LoadProgression("../../configs/progression.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	s := Service{Catalog: c, Learning: l}
	ps := ProgressionService{Catalog: pc, Professions: c, Rules: GrowthRules{LevelCap: 115}}
	count := 0
	for job, p := range c.Professions {
		for adv, growth := range p.AdvancementGrowth {
			count++
			t.Run(fmt.Sprintf("job%d_adv%d", job, adv), func(t *testing.T) {
				st := State{Level: 1, Advancement: adv, AllJobsPilot: true, Attributes: p.InitialAttributes, InitialSkills: p.InitialSkills, SourceSHA256: p.RawSHA256}
				raw, _ := json.Marshal(st)
				r := Character{Name: "JobTest", WireID: 1, Profession: job, State: raw, ConfigVersion: c.Source.SaveIdentity()}
				if _, e := s.EntryBasicProbe(r, [2]byte{}); e != nil {
					t.Fatal(e)
				}
				if _, e := s.EntrySkills(r); e != nil {
					t.Fatal(e)
				}
				next, result, e := ps.ApplyGain(r, pc.Thresholds[0])
				if e != nil || result.Level != 2 {
					t.Fatal(result, e)
				}
				var got State
				json.Unmarshal(next.State, &got)
				if got.Attributes["[hp max]"] != p.InitialAttributes["[hp max]"]+growth["[hp max]"] {
					t.Fatal("wrong growth")
				}
			})
		}
	}
	if count < 50 {
		t.Fatalf("incomplete source coverage: %d", count)
	}
	t.Logf("validated %d source advancement branches", count)
}
