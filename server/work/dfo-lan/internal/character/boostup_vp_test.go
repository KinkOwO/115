package character

import (
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"strings"
	"testing"
)

func boostVPFixture(t *testing.T) (*Service, State, json.RawMessage, protocol.SkillPurchase) {
	t.Helper()
	version := strings.Repeat("a", 64)
	c := &boostup.Catalog{Steps: []boostup.Step{{Number: 1}, {Number: 2}, {Number: 3, Mission: "skill vp option", MissionCells: []pvf.Token{{Type: 3, Text: "[condition]"}, {Type: 0, Value: 3}}}, {Number: 4, Guide: "dungeon"}}}
	s := &Service{Boost: c, Catalog: catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: version}, Professions: map[byte]catalog.Profession{0: {RawSHA256: version}}}, Learning: &LearningCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}, index: map[byte]map[uint16]LearningDefinition{0: {}}}}
	st := State{Level: 115, Advancement: 1, Awakening: 3, AllJobsPilot: true, SourceSHA256: version, SkillPoints: [2]uint16{77, 88}, TechniquePoints: [2]uint16{11, 22}}
	r := protocol.SkillPurchase{Tree: 0, Options: make([]protocol.SkillVariation, 5)}
	for i := 0; i < 5; i++ {
		id := uint16(301 + i)
		r.Options[i] = protocol.SkillVariation{ID: id, Choice: 1, Status: 1}
		st.InitialSkills = append(st.InitialSkills, int32(id), 1, 1)
		s.Learning.index[0][id] = LearningDefinition{Job: 0, ID: id, Fields: map[string][]pvf.Token{"[type]": {{Type: 6, Text: "[active]"}}, "[skill fitness growtype]": {{Type: 0, Value: 1}}}}
	}
	raw, e := json.Marshal(st)
	if e != nil {
		t.Fatal(e)
	}
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(raw, &fields)
	fields["keep"] = json.RawMessage(`42`)
	raw, _ = json.Marshal(fields)
	raw, e = boostup.WriteState(raw, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 3, Phase: 2, Claimed: map[byte]bool{3: true}}})
	if e != nil {
		t.Fatal(e)
	}
	return s, st, raw, r
}

func TestBoostVPSaveLocalCompletionOnlyAfterAllocation(t *testing.T) {
	s, st, raw, req := boostVPFixture(t)
	st.SkillVariations[0].Options = append([]protocol.SkillVariation(nil), req.Options...)
	// No Enhance rows: an extra reinforcement combination is not required.
	next, done, e := s.completeBoostVPSave(raw, st, req)
	if e != nil || !done {
		t.Fatal(done, e)
	}
	state, e := boostup.ReadState(next)
	if e != nil || state.Training.Step != 4 || state.Training.Phase != 0 {
		t.Fatal(state, e)
	}
	if st.TechniquePoints != [2]uint16{11, 22} || st.SkillPoints != [2]uint16{77, 88} {
		t.Fatal("training minted points")
	}
	// 源的完成条件是 [condition] 3，不是把五格全花完：三点即过。
	st.SkillVariations[0].Options[3] = protocol.SkillVariation{Choice: 3}
	st.SkillVariations[0].Options[4] = protocol.SkillVariation{Choice: 3}
	if out, done, e := s.completeBoostVPSave(raw, st, req); e != nil || !done || string(out) == string(raw) {
		t.Fatal("three allocated points", done, e)
	}
	st.SkillVariations[0].Options[2] = protocol.SkillVariation{Choice: 3}
	for _, mode := range []string{"two-points", "ordinary-skill", "unclaimed", "already-finished"} {
		ss, rr, request := st, raw, req
		switch mode {
		case "two-points":
			ss.SkillVariations[0].Options = append([]protocol.SkillVariation(nil), req.Options...)
			for i := 2; i < 5; i++ {
				ss.SkillVariations[0].Options[i] = protocol.SkillVariation{Choice: 3}
			}
		case "ordinary-skill":
			request.Options = nil
		case "unclaimed":
			a, _ := boostup.ReadState(raw)
			a.Training.Claimed = nil
			rr, _ = boostup.WriteState(raw, a)
		case "already-finished":
			rr = next
		}
		out, done, e := s.completeBoostVPSave(rr, ss, request)
		if e != nil || done || string(out) != string(rr) {
			t.Fatal(mode, done, e)
		}
	}
}
