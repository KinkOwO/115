package character

import (
	"dfolan/internal/catalog"

	"testing"
)

func TestKnightPreviewSkillsEligibilityAndLearning(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog("../../configs/skills.next27.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}

	// 1. Skill 61: Tech Tree: Demon Soldier (<12::techtree_demon_soldier_name>)
	// Chaos preview passive skill with growtype maximum level [1, 0, 1, 0, 0, 0].
	d61, ok := l.index[12][61]
	if !ok {
		t.Fatal("Knight skill 61 missing from catalog")
	}
	// ForAdvancement(0) must be eligible for unadvanced Knight
	if !d61.ForAdvancement(0) {
		t.Fatal("unadvanced Knight cannot learn preview skill 61")
	}
	// Advancement 1 (Elven Knight) cannot learn skill 61
	if d61.ForAdvancement(1) {
		t.Fatal("Elven Knight should not be able to learn Chaos skill 61")
	}
	// Advancement 2 (Chaos) can learn skill 61
	if !d61.ForAdvancement(2) {
		t.Fatal("Chaos should be able to learn skill 61")
	}

	// Learning rank 1 at level 1: cost should be 0 SP, no error
	cost, err := d61.Cost(1, 0, 1, map[uint16]byte{})
	if err != nil || cost != 0 {
		t.Fatalf("unadvanced Knight learning skill 61 rank 1: cost=%d err=%v", cost, err)
	}
	// Learning rank 2 must fail because cap[0] is 1
	if _, err = d61.Cost(1, 0, 2, map[uint16]byte{}); err == nil {
		t.Fatal("unadvanced Knight learned rank 2 of skill 61 exceeding cap 1")
	}

	// 2. Skill 63: Summon Demon Soldier (<12::summon_demon_soldier_name>)
	// Chaos preview active skill, required level 15, requires skill 61 rank 1.
	d63, ok := l.index[12][63]
	if !ok {
		t.Fatal("Knight skill 63 missing from catalog")
	}
	if !d63.ForAdvancement(0) {
		t.Fatal("unadvanced Knight cannot learn preview skill 63")
	}
	// Missing prerequisite skill 61
	if _, err = d63.Cost(15, 0, 1, map[uint16]byte{}); err == nil {
		t.Fatal("learned skill 63 without prerequisite skill 61")
	}
	// Level < 15 must fail
	if _, err = d63.Cost(14, 0, 1, map[uint16]byte{61: 1}); err == nil {
		t.Fatal("level 14 learned level 15 skill 63")
	}
	// Level 15 with prerequisite learned must succeed
	if _, err = d63.Cost(15, 0, 1, map[uint16]byte{61: 1}); err != nil {
		t.Fatalf("level 15 unadvanced Knight learning skill 63 failed: %v", err)
	}

	// 3. Post-advancement skill filtering in knownSkills:
	// A character who learned skill 61 as unadvanced (advancement 0)
	svc := &Service{Catalog: c, Learning: l}
	role := Character{Profession: 12, ConfigVersion: c.Source.SaveIdentity()}
	prof := c.Professions[12]

	baseState := State{
		Level:         15,
		Advancement:   0,
		SourceSHA256:  prof.RawSHA256,
		InitialSkills: prof.InitialSkills,
		LearnedSkills: [2]map[uint16]byte{
			0: {61: 1},
		},
	}

	// At advancement 0: skill 61 is known
	known0, err := svc.knownSkills(role, baseState, 0)
	if err != nil {
		t.Fatal(err)
	}
	if known0[61] != 1 {
		t.Fatalf("unadvanced Knight missing learned skill 61: %+v", known0)
	}

	// After advancing to Elven Knight (advancement 1): skill 61 must be removed
	adv1State := baseState
	adv1State.Advancement = 1
	known1, err := svc.knownSkills(role, adv1State, 0)
	if err != nil {
		t.Fatal(err)
	}
	if known1[61] != 0 {
		t.Fatalf("Elven Knight retained foreign Chaos skill 61: %+v", known1)
	}

	// After advancing to Chaos (advancement 2): skill 61 must be retained
	adv2State := baseState
	adv2State.Advancement = 2
	known2, err := svc.knownSkills(role, adv2State, 0)
	if err != nil {
		t.Fatal(err)
	}
	if known2[61] != 1 {
		t.Fatalf("Chaos lost matching skill 61: %+v", known2)
	}
}
