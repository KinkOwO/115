package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestPotionMembershipAndDungeonLimitFollowSource(t *testing.T) {
	s := ScriptRecord{Cells: []pvf.Token{section("[potion list]"), number(7), section("[/potion list]"), section("[dungeon list]"), number(100), number(8), section("[/dungeon list]")}}
	r, err := ParseStackableDungeonLimits(s)
	if err != nil || !r.Potions[7] || r.Potions[8] || r.Dungeons[100] != 8 {
		t.Fatal(r, err)
	}
	s.Cells[1].Value = 9
	s.Cells[5].Value = 5
	r, err = ParseStackableDungeonLimits(s)
	if err != nil || r.Potions[7] || !r.Potions[9] || r.Dungeons[100] != 5 {
		t.Fatal("parallel source values", r, err)
	}
}
