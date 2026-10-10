package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"testing"
)

func TestDisabledDungeonExperienceOverridesRepeatedWeights(t *testing.T) {
	d := catalog.DungeonDefinition{Script: catalog.ScriptRecord{Cells: []pvf.Token{
		{Type: 3, Text: "[disable exp]"},
		{Type: 3, Text: "[experience increasing point]"}, {Type: 0, Value: 0},
		{Type: 3, Text: "[experience increasing point]"}, {Type: 2, Number: 0.83},
	}}}
	m := protocol.DungeonMonster{Template: 109014482, Level: 115}
	if gain, err := GrowthMonsterGain(catalog.Progression{}, GrowthRules{}, d, m, 115, 0); err != nil || gain != 0 {
		t.Fatal("disabled source requires no experience tables", gain, err)
	}
	d.Script.Cells = d.Script.Cells[1:]
	c := catalog.Progression{DifficultyRates: []float32{1}, MonsterRates: []float32{1}, MonsterExperience: map[uint16]uint64{115: 100}}
	if _, err := GrowthMonsterGain(c, GrowthRules{}, d, m, 115, 0); err == nil {
		t.Fatal("ordinary ambiguous weight was silently accepted")
	}
}

func TestCurrentSourceWithExplicitReferenceRules(t *testing.T) {
	c, e := catalog.LoadProgression(testfixture.ProgressionPath(t))
	if e != nil {
		t.Fatal(e)
	}
	d, e := catalog.LoadDungeons(testfixture.DungeonPath(t, "dungeons.generated.json"))
	if e != nil {
		t.Fatal(e)
	}
	r, e := LoadGrowthRules("../../configs/experience.compat90.json")
	if e != nil {
		t.Fatal(e)
	}
	m := protocol.DungeonMonster{Template: 109014858, Level: 3}
	gain, e := GrowthMonsterGain(c, r, d.Dungeons[3], m, 1, 0)
	if e != nil || gain != 104 {
		t.Fatal("current base72 difficulty1.3 reference penalty1.12", gain, e)
	}
	m.NonCombat = true
	if gain, e = GrowthMonsterGain(c, r, d.Dungeons[3], m, 1, 0); e != nil || gain != 0 {
		t.Fatal("cinematic actor rewarded")
	}
	for _, tc := range []struct {
		total, gain uint64
		level       byte
		sp          uint32
	}{{999, 0, 1, 0}, {999, 1, 2, 30}, {999, 1036, 3, 60}} {
		result, e := AddGrowthExperience(c, r, 1, tc.total, tc.gain)
		if e != nil || result.Level != tc.level || result.SkillPointGain != tc.sp || result.Experience != tc.total+tc.gain {
			t.Fatal(result, e)
		}
	}
	if _, e = AddGrowthExperience(c, r, 1, ^uint64(0), 1); e == nil {
		t.Fatal("experience overflow accepted")
	}
	if _, e = AddGrowthExperience(c, r, 2, 0, 1); e == nil {
		t.Fatal("inconsistent saved level accepted")
	}
	// Same formula with changed configuration proves no hidden rate constant.
	r.Penalty[2] = 2
	m.NonCombat = false
	gain, e = GrowthMonsterGain(c, r, d.Dungeons[3], m, 1, 0)
	if e != nil || gain != 186 {
		t.Fatal(gain, e)
	}
}

func TestSceneActorExperienceDoesNotRequireMonsterTables(t *testing.T) {
	for _, m := range []protocol.DungeonMonster{
		{Template: 55424, Rank: 5, APC: true, Level: 0, Team: 100},
		{Template: 109019135, Rank: 0, Level: 0, Team: 100},
	} {
		gain, e := GrowthMonsterGain(catalog.Progression{}, GrowthRules{}, catalog.DungeonDefinition{}, m, 115, 2)
		if e != nil || gain != 0 {
			t.Fatal(gain, e)
		}
	}
}
