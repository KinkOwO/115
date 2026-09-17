package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestSwordmasterLearningUsesSourceRules(t *testing.T) {
	c, err := catalog.LoadCharacters("../../configs/characters.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	l, err := LoadLearningCatalog("../../configs/skills.next27.json", c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	d := l.index[0][4]
	if d.Path != "skill/swordman/lightswordmastery.skl" {
		t.Fatal("unexpected skill fixture", d.Path)
	}
	for _, tc := range []struct {
		name                     string
		level, advancement, rank int
		allowed                  bool
	}{
		{"swordmaster", 15, 1, 1, true},
		{"low_level", 14, 1, 1, false},
		{"base_job", 115, 0, 1, false},
		{"other_advancement", 115, 2, 1, false},
		{"source_rank_cap", 115, 1, 2, false},
		{"negative_advancement", 115, -1, 1, false},
		{"packed_awakening_not_a_growtype", 115, 17, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cost, err := d.Cost(tc.level, tc.advancement, tc.rank, nil)
			if (err == nil) != tc.allowed {
				t.Fatalf("cost=%d err=%v", cost, err)
			}
			if tc.allowed && cost != 15 {
				t.Fatalf("SP cost=%d want15", cost)
			}
		})
	}
}

func TestAdvancedLearningRetainsPrerequisiteAndSpecialCostChecks(t *testing.T) {
	ints := func(values ...int32) []pvf.Token {
		var out []pvf.Token
		for _, v := range values {
			out = append(out, pvf.Token{Type: 0, Value: v})
		}
		return out
	}
	d := LearningDefinition{Fields: map[string][]pvf.Token{
		"[type]":                   {{Type: 6, Text: "[active]"}},
		"[skill fitness growtype]": ints(1),
		"[growtype maximum level]": ints(0, 3),
		"[required level]":         ints(15), "[maximum level]": ints(3),
		"[purchase cost]": ints(20), "[required level range]": ints(3),
		"[pre required skill]": ints(8, 2),
	}}
	if _, err := d.Cost(18, 1, 2, map[uint16]byte{8: 1}); err == nil {
		t.Fatal("missing prerequisite accepted")
	}
	if _, err := d.Cost(17, 1, 2, map[uint16]byte{8: 2}); err == nil {
		t.Fatal("rank level interval ignored")
	}
	if cost, err := d.Cost(18, 1, 2, map[uint16]byte{8: 2}); err != nil || cost != 20 {
		t.Fatal(cost, err)
	}
	d.Fields["[special purchase cost]"] = ints(1)
	if _, err := d.Cost(18, 1, 2, map[uint16]byte{8: 2}); err == nil {
		t.Fatal("special currency skill accepted as SP purchase")
	}
}
