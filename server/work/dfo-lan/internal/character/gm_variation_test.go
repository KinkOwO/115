package character

import (
	"bytes"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

func TestGMVariationsCurrentSourceAndLosslessSave(t *testing.T) {
	s, st, _, req := boostVPFixture(t)
	for id, d := range s.Learning.index[0] {
		d.Fields["[variation point]"] = []pvf.Token{{Type: 3, Text: "[option1]"}, {Type: 3, Text: "[intension2]"}}
		s.Learning.index[0][id] = d
	}
	raw, _ := json.Marshal(st)
	var doc map[string]json.RawMessage
	_ = json.Unmarshal(raw, &doc)
	doc["future"] = json.RawMessage(`9007199254740993`)
	doc["skill_variations"] = json.RawMessage(`[{"future_vp":9007199254740995},{"future_tree":123}]`)
	raw, _ = json.Marshal(doc)
	role := Character{ID: 1, AccountID: 1, Profession: 0, ConfigVersion: s.Learning.Source.SaveIdentity(), State: raw}
	view, err := s.GMVariationView(role)
	if err != nil || !view.Unlocked || len(view.Candidates) != 5 || view.Remaining != 5 {
		t.Fatalf("view %+v %v", view, err)
	}
	edit := GMVariationEdit{Intensions: make([]protocol.SkillVariation, 3), Options: req.Options}
	next, err := s.GMEditVariations(role, edit)
	if err != nil {
		t.Fatal(err)
	}
	var out State
	_ = json.Unmarshal(next, &out)
	if out.TechniquePoints != [2]uint16{0, 22} || out.SkillPoints != st.SkillPoints {
		t.Fatalf("point contract %+v", out)
	}
	for _, value := range []string{"9007199254740993", "9007199254740995", "future_tree"} {
		if !bytes.Contains(next, []byte(value)) {
			t.Fatalf("lost %s", value)
		}
	}
	edit.Options = append([]protocol.SkillVariation(nil), req.Options...)
	edit.Options[0].ID = 9999
	if _, err = s.GMEditVariations(role, edit); err == nil {
		t.Fatal("unlearned skill accepted")
	}
	edit.Options[0] = edit.Options[1]
	if _, err = s.GMEditVariations(role, edit); err == nil {
		t.Fatal("duplicate skill accepted")
	}
	edit.Options = append([]protocol.SkillVariation(nil), req.Options...)
	edit.Options[0].Choice = 2
	if _, err = s.GMEditVariations(role, edit); err == nil {
		t.Fatal("variant absent in source accepted")
	}
	edit.Options = make([]protocol.SkillVariation, 5)
	next, err = s.GMEditVariations(role, edit)
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(next, &out)
	if out.TechniquePoints[0] != 5 || out.SkillVariations[0].Options[0].Choice != 3 || out.SkillVariations[0].Options[0].Empty != 1 {
		t.Fatal("reset failed", out)
	}
	st.Awakening = 2
	role.State, _ = json.Marshal(st)
	if _, err = s.GMEditVariations(role, edit); err == nil {
		t.Fatal("VP unlocked without third awakening")
	}
}
