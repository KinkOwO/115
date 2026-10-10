package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestDungeonTypeComesFromNativeScript(t *testing.T) {
	for _, kind := range []string{"boundary of attunement", "normal", "bakal"} {
		d, err := ParseDungeon(17, ScriptRecord{Path: "contents/2026/skyofathousandseasofborder/dungeon/source-test.dgn", Cells: []pvf.Token{
			section("[minimum required level]"), number(115), section("[basis level]"), number(145), section("[dungeon type]"), label(kind),
			section("[maze info]"), section("[size]"), number(1), number(1), section("[map specification]"), label("boss"), number(0), number(0), number(81), section("[/map specification]"),
			section("[start map]"), number(0), number(0), section("[/start map]"), section("[boss map]"), number(0), number(0), section("[/boss map]"),
		}})
		if err != nil || d.Type != kind || d.Bakal != (kind == "bakal") {
			t.Fatalf("kind=%s type=%s error=%v", kind, d.Type, err)
		}
	}
}
