package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/progression"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"testing"
)

func advancementProgressionService(t *testing.T) *ProgressionService {
	t.Helper()
	professions, err := catalog.LoadCharacters("../../configs/characters.skycastle-release.json")
	if err != nil {
		t.Fatal(err)
	}
	pc, err := catalog.LoadProgression("../../configs/progression.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	return &ProgressionService{Catalog: pc, Professions: professions, Rules: progression.Rules{LevelCap: 115}}
}

func ordinaryAdvancedRole(t *testing.T, s *ProgressionService, job, advancement byte) storage.Character {
	t.Helper()
	p := s.Professions.Professions[job]
	state := State{
		Level: 15, Experience: s.Catalog.Thresholds[13], SkillPoints: [2]uint16{17, 23},
		Attributes: maps.Clone(p.InitialAttributes), InitialSkills: p.InitialSkills,
		SourcePath: p.Path, SourceSHA256: p.RawSHA256,
	}
	for name, v := range p.BaseGrowth {
		state.Attributes[name] = float32((math.Round(float64(state.Attributes[name])*10) + 14*math.Round(float64(v)*10)) / 10)
	}
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	fields["inventory"] = json.RawMessage(`{"gold":321,"coin":7,"version":"ordinary-bag-v1"}`)
	fields["future_progression_field"] = json.RawMessage(`{"keep":true}`)
	raw, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{Name: "GrowthTest", WireID: 10, Profession: job, ConfigVersion: s.Professions.Source.Checksum, State: raw}
	if advancement != 0 {
		// This is the normal CMD1881/CMD777 domain path, not a pilot repair.
		role.State, err = (&Service{Catalog: s.Professions}).ApplyAdvancement(role, advancement)
		if err != nil {
			t.Fatal(err)
		}
	}
	return role
}

func TestApplyGainAfterOrdinaryAdvancement(t *testing.T) {
	s := advancementProgressionService(t)
	branches := 0
	for job, p := range s.Professions.Professions {
		growthByBranch := maps.Clone(p.AdvancementGrowth)
		if growthByBranch == nil {
			growthByBranch = map[byte]map[string]float32{}
		}
		growthByBranch[0] = p.BaseGrowth
		for advancement, growth := range growthByBranch {
			branches++
			t.Run(fmt.Sprintf("job%d_adv%d", job, advancement), func(t *testing.T) {
				role := ordinaryAdvancedRole(t, s, job, advancement)
				before := decodeState(t, role.State)
				if before.AllJobsPilot || before.SwordmasterPilot {
					t.Fatal("fixture must use ordinary saved state without pilot flags")
				}
				original := bytes.Clone(role.State)
				gains := []uint64{0, 1, s.Catalog.Thresholds[14] - before.Experience}
				for _, gain := range gains {
					t.Run(fmt.Sprintf("gain%d", gain), func(t *testing.T) {
						next, result, err := s.ApplyGain(role, gain)
						if err != nil {
							t.Fatalf("ordinary advancement blocked experience: %v", err)
						}
						got := decodeState(t, next.State)
						levels := 0
						if gain == gains[2] {
							levels = 1
						}
						if result.Level != before.Level+byte(levels) || got.Level != result.Level || got.Experience != before.Experience+gain || result.Experience != got.Experience {
							t.Fatalf("wrong experience/level: result=%+v state=%+v", result, got)
						}
						for name, value := range before.Attributes {
							want := value
							if increment, ok := growth[name]; levels > 0 && ok {
								want = float32((math.Round(float64(value)*10) + math.Round(float64(increment)*10)) / 10)
							}
							if got.Attributes[name] != want {
								t.Fatalf("%s growth=%v, want %v from branch %d", name, got.Attributes[name], want, advancement)
							}
						}
						for i, sp := range before.SkillPoints {
							want := sp
							if levels > 0 {
								want += s.Catalog.SkillPoints[uint16(got.Level)]
							}
							if got.SkillPoints[i] != want {
								t.Fatalf("skill tree %d SP=%d, want %d", i, got.SkillPoints[i], want)
							}
						}
						if got.Advancement != advancement || got.Awakening != before.Awakening || got.AllJobsPilot || got.SwordmasterPilot {
							t.Fatal("experience changed advancement or enabled pilot flags")
						}
						var beforeFields, afterFields map[string]json.RawMessage
						if err = json.Unmarshal(original, &beforeFields); err != nil {
							t.Fatal(err)
						}
						if err = json.Unmarshal(next.State, &afterFields); err != nil {
							t.Fatal(err)
						}
						for _, key := range []string{"inventory", "future_progression_field"} {
							if !bytes.Equal(beforeFields[key], afterFields[key]) {
								t.Fatalf("saved field %q changed", key)
							}
						}
						if !bytes.Equal(role.State, original) {
							t.Fatal("ApplyGain mutated its input")
						}
					})
				}
			})
		}
	}
	if branches < 67 {
		t.Fatalf("incomplete profession coverage: %d branches including base jobs", branches)
	}
	t.Logf("validated %d ordinary source branches, including no-level and level-up gains", branches)
}

func TestApplyGainRejectsUnbackedAdvancement(t *testing.T) {
	for _, name := range []string{"unknown_branch", "missing_branch", "missing_pilot_branch", "foreign_swordmaster_fallback", "source_mismatch"} {
		t.Run(name, func(t *testing.T) {
			s := advancementProgressionService(t)
			role := ordinaryAdvancedRole(t, s, 12, 1)
			state := decodeState(t, role.State)
			p := s.Professions.Professions[12]
			switch name {
			case "unknown_branch":
				state.Advancement = 15
			case "missing_branch", "missing_pilot_branch", "foreign_swordmaster_fallback":
				delete(p.AdvancementGrowth, 1)
				state.AllJobsPilot = name == "missing_pilot_branch"
				if name == "foreign_swordmaster_fallback" {
					state.SwordmasterPilot = true
					p.SwordmasterGrowth = p.BaseGrowth
				}
			case "source_mismatch":
				role.ConfigVersion = "different-source"
			}
			s.Professions.Professions[12] = p
			var err error
			role.State, err = json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			original := bytes.Clone(role.State)
			next, _, err := s.ApplyGain(role, 1)
			if err == nil {
				t.Fatal("accepted growth without this profession's source branch")
			}
			if !bytes.Equal(next.State, original) || !bytes.Equal(role.State, original) {
				t.Fatal("rejected gain changed saved state")
			}
		})
	}
}
