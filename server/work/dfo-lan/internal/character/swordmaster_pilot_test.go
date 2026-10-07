package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/testfixture"

	"encoding/hex"
	"encoding/json"
	"testing"
)

func pilotFixture(t *testing.T) (*Service, Character) {
	t.Helper()
	c, err := catalog.LoadCharacters("../../configs/characters.swordmaster-pilot.json")
	if err != nil {
		t.Fatal(err)
	}
	l, err := LoadLearningCatalog(testfixture.SkillCatalogPath(t, "next27"), c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	p := c.Professions[0]
	state, _ := json.Marshal(State{Level: 1, Attributes: p.InitialAttributes, InitialSkills: p.InitialSkills, SourceSHA256: p.RawSHA256})
	var fields map[string]json.RawMessage
	json.Unmarshal(state, &fields)
	fields["future_field"] = json.RawMessage(`{"keep":true}`)
	state, _ = json.Marshal(fields)
	req, _ := hex.DecodeString("000b0000006e6f726d616c5f74657374000000000000ff000100000000000000")
	return &Service{Catalog: c, Learning: l, Rules: Rules{SwordmasterPilot: true}}, Character{Name: "normal_test", WireID: 1, Profession: 0, Request: req, State: state, ConfigVersion: c.Source.SaveIdentity()}
}

func TestSwordmasterPilotRoundTripAndGrowth(t *testing.T) {
	s, role := pilotFixture(t)
	original := append([]byte(nil), role.State...)
	updated, err := s.RepairSwordmasterPilot(role)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.RepairSwordmasterPilot(updated)
	if err != nil || !bytes.Equal(again.State, updated.State) {
		t.Fatal("repair not idempotent", err)
	}
	if !bytes.Equal(role.State, original) || !bytes.Contains(updated.State, []byte(`"future_field":{"keep":true}`)) {
		t.Fatal("state preservation failed")
	}
	if _, err = s.EntryBasicProbe(updated, [2]byte{}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EntryAddition(updated); err != nil {
		t.Fatal(err)
	}
	if _, err = s.EntrySkills(updated); err != nil {
		t.Fatal(err)
	}
	pc, err := catalog.LoadProgression(testfixture.ProgressionPath(t))
	if err != nil {
		t.Fatal(err)
	}
	ps := ProgressionService{Catalog: pc, Professions: s.Catalog, Rules: GrowthRules{LevelCap: 115}}
	next, result, err := ps.ApplyGain(updated, pc.Thresholds[13])
	if err != nil || result.Level != 15 {
		t.Fatal(result, err)
	}
	var state State
	json.Unmarshal(next.State, &state)
	if state.Attributes["[hp max]"] != 520+14*60 || state.Attributes["[mp max]"] != 480+14*30 {
		t.Fatal("wrong profession growth", state.Attributes)
	}
	if state.SkillPoints[0] == 0 || state.Advancement != 1 {
		t.Fatal("missing SP/advancement")
	}
	if _, err = s.EntrySkills(next); err != nil {
		t.Fatal(err)
	}
}

func TestSwordmasterPilotRefusesUnverifiedStates(t *testing.T) {
	for _, kind := range []string{"disabled", "other_job", "different_name", "source", "leveled", "unknown_mode"} {
		t.Run(kind, func(t *testing.T) {
			s, role := pilotFixture(t)
			switch kind {
			case "disabled":
				s.Rules.SwordmasterPilot = false
			case "other_job":
				role.Profession = 1
			case "different_name":
				role.Name = "different"
			case "source":
				role.ConfigVersion = "different"
			case "leveled":
				var state State
				json.Unmarshal(role.State, &state)
				state.Level = 2
				role.State, _ = json.Marshal(state)
			case "unknown_mode":
				role.Request[26] = 9
			}
			if _, err := s.RepairSwordmasterPilot(role); err == nil {
				t.Fatal("unverified repair accepted")
			}
		})
	}
}
