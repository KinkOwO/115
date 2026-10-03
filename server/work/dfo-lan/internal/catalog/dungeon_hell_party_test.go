package catalog

import (
	"dfolan/internal/catalog/pvf"
	"reflect"
	"testing"
)

func TestParseDungeonRepeatedHellDeclarations(t *testing.T) {
	declaration := []pvf.Token{
		section("[hell dungeon]"), number(1),
		section("[seal door map index]"), number(60056),
		section("[seal door pos]"), number(1), number(2),
	}
	for _, tc := range []struct {
		name   string
		second []pvf.Token
		want   bool
	}{
		{"same room in another maze", declaration, true},
		{"different enabled value", []pvf.Token{section("[hell dungeon]"), number(0)}, false},
		{"different seal map", []pvf.Token{section("[seal door map index]"), number(60050)}, false},
		{"different seal coordinate", []pvf.Token{section("[seal door pos]"), number(3), number(0)}, false},
		{"empty repeated field", []pvf.Token{section("[hell dungeon]")}, false},
		{"malformed repeated field", []pvf.Token{section("[hell dungeon]"), number(1), number(1)}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cells := []pvf.Token{section("[minimum required level]"), number(63), section("[basis level]"), number(63)}
			cells = append(cells, section("[maze info]"))
			cells = append(cells, declaration...)
			cells = append(cells, section("[maze info]"))
			cells = append(cells, tc.second...)
			before := append([]pvf.Token(nil), cells...)
			d, err := ParseDungeon(92, ScriptRecord{Cells: cells})
			if err != nil {
				t.Fatal(err)
			}
			if (d.HellParty != nil) != tc.want {
				t.Fatalf("HellParty=%+v want supported=%t", d.HellParty, tc.want)
			}
			if tc.want && (d.HellParty.SealMap != 60056 || d.HellParty.SealPosition != [2]byte{1, 2}) {
				t.Fatalf("source room changed: %+v", d.HellParty)
			}
			if !reflect.DeepEqual(cells, before) {
				t.Fatal("source cells changed")
			}
		})
	}
}
