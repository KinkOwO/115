package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestParseSourceMoveMapEvenEnemy(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []pvf.Token
		want bool
	}{
		{"absent", nil, false},
		{"disabled", []pvf.Token{number(0)}, false},
		{"enabled", []pvf.Token{number(1)}, true},
		{"wrong type", []pvf.Token{label("1")}, false},
		{"unsupported value", []pvf.Token{number(2)}, false},
		{"ambiguous", []pvf.Token{number(1), number(0)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cells := []pvf.Token{section("[minimum required level]"), number(110)}
			if tc.body != nil {
				cells = append(cells, section("[move map even enemy]"))
				cells = append(cells, tc.body...)
			}
			cells = append(cells, section("[maze info]"), section("[size]"), number(1), number(1),
				section("[map specification]"), label("boss"), number(0), number(0), number(100006472), section("[/map specification]"),
				section("[start map]"), number(0), number(0), section("[/start map]"),
				section("[boss map]"), number(0), number(0), section("[/boss map]"))
			d, err := ParseDungeon(100002987, ScriptRecord{Cells: cells})
			if err != nil || d.MoveMapEvenEnemy != tc.want {
				t.Fatalf("move permission=%v want=%v err=%v", d.MoveMapEvenEnemy, tc.want, err)
			}
		})
	}
}
