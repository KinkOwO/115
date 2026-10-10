package character

// adminFixture and InitialState115 reconstruct the donor-tree admin creation
// fixture that the Starter Boost capsule tests are pinned against. The donor
// production admin-bootstrap (direct level-115 character creation) does not
// exist in this tree and the capsule flow itself never calls it, so both
// helpers stay test-only and the donor assertions remain intact.

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"encoding/json"
	"testing"
)

func boostAdminStateJSON(st State) (json.RawMessage, error) {
	raw, e := json.Marshal(st)
	if e != nil {
		return nil, e
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(raw, &fields); e != nil {
		return nil, e
	}
	fields["inventory"] = json.RawMessage(`{"gold":123,"worn":[]}`)
	fields["foreign_payload"] = json.RawMessage(`{"keep":42}`)
	return json.Marshal(fields)
}

func adminFixture(t *testing.T) (*Service, *ProgressionService, State, json.RawMessage, Character) {
	t.Helper()
	c, e := catalog.LoadCharacters("../../configs/characters.awakening-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	// configs/skills.awakening-candidate.json 在本树已退休，同一角色源用
	// testfixture 的历史技能投影（awakening_test.go 的口径）。
	l, e := LoadLearningCatalog(testfixture.SkillCatalogPath(t, "release"), c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	p, e := catalog.LoadProgression(testfixture.ProgressionPath(t))
	if e != nil {
		t.Fatal(e)
	}
	prof := c.Professions[0]
	st := State{Level: 1, Advancement: 1, AllJobsPilot: true, Attributes: prof.InitialAttributes, InitialSkills: prof.InitialSkills, SourcePath: prof.Path, SourceSHA256: prof.RawSHA256}
	raw, e := boostAdminStateJSON(st)
	if e != nil {
		t.Fatal(e)
	}
	req := append([]byte{0, 10, 0, 0, 0}, []byte("BoostAdmin")...)
	for len(req)%8 != 0 {
		req = append(req, 0)
	}
	role := Character{ID: 1, AccountID: 1, WireID: 503, Name: "BoostAdmin", Profession: 0, Request: req, ConfigVersion: c.Source.SaveIdentity(), State: raw}
	return &Service{Catalog: c, Learning: l}, &ProgressionService{Catalog: p, Professions: c, Rules: GrowthRules{LevelCap: 115}}, st, raw, role
}

// Test-only stand-in for the donor admin creation path: a freshly created
// level-115 character before any capsule-driven awakening grants.
func (s *Service) InitialState115(req protocol.CreateRequest) (json.RawMessage, error) {
	prof := s.Catalog.Professions[req.Profession]
	st := State{Level: 115, Advancement: 0, AllJobsPilot: true, Attributes: prof.InitialAttributes, InitialSkills: prof.InitialSkills, SourcePath: prof.Path, SourceSHA256: prof.RawSHA256, CreationOptions: req.Options}
	return boostAdminStateJSON(st)
}
