package progression

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestCurrentSourceWithExplicitReferenceRules(t *testing.T) {
	c, e := catalog.LoadProgression("../../configs/progression.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	d, e := catalog.LoadDungeons("../../configs/dungeons.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	r, e := LoadRules("../../configs/experience.compat90.json")
	if e != nil {
		t.Fatal(e)
	}
	m := protocol.DungeonMonster{Template: 109014858, Level: 3}
	gain, e := MonsterGain(c, r, d.Dungeons[3], m, 1, 0)
	if e != nil || gain != 104 {
		t.Fatal("current base72 difficulty1.3 reference penalty1.12", gain, e)
	}
	m.NonCombat = true
	if gain, e = MonsterGain(c, r, d.Dungeons[3], m, 1, 0); e != nil || gain != 0 {
		t.Fatal("cinematic actor rewarded")
	}
	for _, tc := range []struct {
		total, gain uint64
		level       byte
		sp          uint32
	}{{999, 0, 1, 0}, {999, 1, 2, 30}, {999, 1036, 3, 60}} {
		result, e := AddExperience(c, r, 1, tc.total, tc.gain)
		if e != nil || result.Level != tc.level || result.SkillPointGain != tc.sp || result.Experience != tc.total+tc.gain {
			t.Fatal(result, e)
		}
	}
	if _, e = AddExperience(c, r, 1, ^uint64(0), 1); e == nil {
		t.Fatal("experience overflow accepted")
	}
	if _, e = AddExperience(c, r, 2, 0, 1); e == nil {
		t.Fatal("inconsistent saved level accepted")
	}
	// Same formula with changed configuration proves no hidden rate constant.
	r.Penalty[2] = 2
	m.NonCombat = false
	gain, e = MonsterGain(c, r, d.Dungeons[3], m, 1, 0)
	if e != nil || gain != 186 {
		t.Fatal(gain, e)
	}
}
