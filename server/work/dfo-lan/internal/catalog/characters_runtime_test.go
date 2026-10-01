package catalog

import (
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func characterRuntimeFixture() (Characters, CharacterRuntimePolicy) {
	checksum := strings.Repeat("a", 64)
	c := Characters{Source: pvf.ArchiveSnapshot{Checksum: checksum}, Professions: map[byte]Profession{0: {ID: 0, InitialAttributes: map[string]float32{"hp": 100}, InitialSkills: []int32{5, 1, 1}, InitialSections: map[string][]pvf.Token{"hp": {{Type: 0, Value: 100}}}, BaseGrowth: map[string]float32{"hp": 2}, AdvancementSkills: map[byte][]int32{1: {20, 1, 1}}, AdvancementGrowth: map[byte]map[string]float32{1: {"hp": 3}}, AwakeningSkills: map[byte]map[byte][]int32{1: {1: {30, 1, 1}}}, InitialSkillSlots: map[uint16]uint16{5: 9}, AdvancementSkillSlots: map[byte]map[uint16]uint16{1: {20: 2}}, SkillCommands: map[uint16][]uint32{5: {1, 2}}, CreateEquipmentBySlot: map[string]map[byte]uint32{"weapon": {0: 10}}, Growth: []map[string]float32{{"hp": 2}}, PresetSkills: [][]int32{{5}}, SlotSkills: [][]int32{{20, 1, 1}}}}}
	p := CharacterRuntimePolicy{Version: 1, SourceChecksum: checksum, InitialSkillSlots: map[byte]map[uint16]uint16{0: {5: 1}}}
	return c, p
}
func TestCharacterRuntimePreservesRawSourceAndOwnsProjection(t *testing.T) {
	source, policy := characterRuntimeFixture()
	runtime, err := ProjectCharacterRuntime(source, policy)
	if err != nil {
		t.Fatal(err)
	}
	p := runtime.Professions[0]
	if len(p.Growth) != 0 || len(p.PresetSkills) != 0 || len(p.SlotSkills) != 0 || len(p.SkillCommands) != 0 || len(p.AdvancementSkillSlots) != 0 || p.InitialSkillSlots[5] != 1 {
		t.Fatal("effective compatibility policy changed")
	}
	if len(source.Professions[0].Growth) != 1 || source.Professions[0].InitialSkillSlots[5] != 9 || len(source.Professions[0].SkillCommands) != 1 {
		t.Fatal("source data was discarded or modified")
	}
	policy.InitialSkillSlots[0][5] = 0
	p.InitialAttributes["hp"] = 200
	p.InitialSections["hp"][0].Value = 200
	p.InitialSkills[0] = 1
	p.AdvancementSkills[1][0] = 1
	p.AdvancementGrowth[1]["hp"] = 200
	p.AwakeningSkills[1][1][0] = 1
	p.CreateEquipmentBySlot["weapon"][0] = 20
	raw := source.Professions[0]
	if p.InitialSkillSlots[5] != 1 || raw.InitialAttributes["hp"] != 100 || raw.InitialSections["hp"][0].Value != 100 || raw.InitialSkills[0] != 5 || raw.AdvancementSkills[1][0] != 20 || raw.AdvancementGrowth[1]["hp"] != 3 || raw.AwakeningSkills[1][1][0] != 30 || raw.CreateEquipmentBySlot["weapon"][0] != 10 {
		t.Fatal("runtime projection shares mutable source or policy")
	}
}
func TestCharacterRuntimeRejectsPolicyOutsideSourceGrants(t *testing.T) {
	for _, change := range []func(*CharacterRuntimePolicy){func(p *CharacterRuntimePolicy) { p.SourceChecksum = strings.Repeat("b", 64) }, func(p *CharacterRuntimePolicy) { p.InitialSkillSlots[0] = map[uint16]uint16{7: 1} }, func(p *CharacterRuntimePolicy) { p.InitialSkillSlots[0][5] = 14 }, func(p *CharacterRuntimePolicy) { delete(p.InitialSkillSlots, 0) }, func(p *CharacterRuntimePolicy) { p.Version = 2 }} {
		source, p := characterRuntimeFixture()
		change(&p)
		if _, err := ProjectCharacterRuntime(source, p); err == nil {
			t.Fatal("invalid policy accepted")
		}
	}
	source, p := characterRuntimeFixture()
	p.EnableSourceCommands, p.EnableAdvancementShortcuts = true, true
	runtime, err := ProjectCharacterRuntime(source, p)
	if err != nil {
		t.Fatal(err)
	}
	row := runtime.Professions[0]
	row.SkillCommands[5][0] = 9
	row.AdvancementSkillSlots[1][20] = 9
	if source.Professions[0].SkillCommands[5][0] != 1 || source.Professions[0].AdvancementSkillSlots[1][20] != 2 {
		t.Fatal("enabled projections share source data")
	}
}
