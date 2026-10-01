package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/savecontract"

	"encoding/json"
	"strings"
	"testing"
)

func TestRoleDetailsPrewarmLeavesSaveUnchangedAndRefusesMissingLearnedSkill(t *testing.T) {
	sha := strings.Repeat("1", 64)
	c, err := newLearningCatalog(LearningCatalog{Source: pvf.ArchiveSnapshot{Checksum: sha}, Rows: []LearningDefinition{{Job: 1, ID: 10, Path: "skill/test.skl", SHA256: sha}}}, sha)
	if err != nil {
		t.Fatal(err)
	}
	s := &Service{Learning: c, Catalog: catalog.Characters{Professions: map[byte]catalog.Profession{1: {RawSHA256: sha}}}}
	state := State{Level: 30, SourceSHA256: sha, InitialSkills: []int32{10, 1, 1}}
	state.LearnedSkills[1] = map[uint16]byte{10: 2}
	state.SkillSlots[1] = map[uint16]uint16{10: 7}
	state.SkillPoints[1] = 123
	raw, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	role := Character{Profession: 1, ConfigVersion: savecontract.Identity(), State: raw}
	original := append([]byte(nil), raw...)
	if err = s.PrepareRoleDetails(role); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(role.State, original) {
		t.Fatal("prewarm changed persisted skills/slots/SP")
	}
	state.LearnedSkills[1][999] = 1
	role.State, _ = json.Marshal(state)
	if err = s.PrepareRoleDetails(role); err == nil {
		t.Fatal("missing learned skill silently accepted")
	}
}
