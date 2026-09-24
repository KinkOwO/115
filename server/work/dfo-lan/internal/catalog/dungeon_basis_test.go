package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestTrainingRoomBasisFallback(t *testing.T) {
	maze := []pvf.Token{
		section("[maze info]"), section("[size]"), number(1), number(1),
		section("[start map]"), number(0), number(0),
		section("[boss map]"), number(-1), number(-1),
		section("[map specification]"), label("map"), number(0), number(0), number(36250),
	}
	for _, tc := range []struct {
		name   string
		levels []pvf.Token
		want   uint32
	}{
		{"recommended", []pvf.Token{section("[recommended level]"), number(110), number(115)}, 110},
		{"minimum", nil, 100},
		{"explicit", []pvf.Token{section("[recommended level]"), number(110), section("[basis level]"), number(105)}, 105},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cells := append([]pvf.Token{section("[minimum required level]"), number(100)}, tc.levels...)
			cells = append(cells, maze...)
			d, err := ParseDungeon(5000, ScriptRecord{Path: "dungeon/poongjintrainingroom/a.dgn", Cells: cells})
			if err != nil || d.BasisLevel != tc.want || !d.NoFatigue {
				t.Fatalf("d=%+v err=%v", d, err)
			}
		})
	}
	if _, err := ParseDungeon(5000, ScriptRecord{Cells: maze}); err == nil {
		t.Fatal("missing minimum level accepted")
	}
}
