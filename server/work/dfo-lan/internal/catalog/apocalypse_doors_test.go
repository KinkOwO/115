package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestApocalypseDoorInitialActionFollowsBasicSource(t *testing.T) {
	for _, index := range []int32{0, 2} {
		cells := []pvf.Token{{Type: 3, Text: "[SET CUSTOM ACTION]"}, {Type: 0, Value: index}}
		got, err := apocalypseInitialCustomAction(cells, 3)
		if err != nil || got != int(index) {
			t.Fatal(got, err)
		}
	}
	for _, cells := range [][]pvf.Token{
		nil,
		{{Type: 3, Text: "[SET CUSTOM ACTION]"}},
		{{Type: 3, Text: "[SET CUSTOM ACTION]"}, {Type: 10}},
		{{Type: 3, Text: "[SET CUSTOM ACTION]"}, {Type: 0, Value: 3}},
		{{Type: 3, Text: "[SET CUSTOM ACTION]"}, {Type: 0, Value: -1}},
		{{Type: 3, Text: "[SET CUSTOM ACTION]"}, {Type: 0}, {Type: 3, Text: "[SET CUSTOM ACTION]"}, {Type: 0}},
	} {
		if _, err := apocalypseInitialCustomAction(cells, 3); err == nil {
			t.Fatal("ambiguous initial action accepted", cells)
		}
	}
}

func doorSource() (ScriptRecord, ScriptRecord, map[int]string, []LegionDungeon) {
	obj := ScriptRecord{Path: "content/door.obj", Cells: []pvf.Token{
		{Type: 3, Text: "[create var]"}, {Type: 6, Text: "o:move_to_dungeon"}, {Type: 10, Value: 10},
		{Type: 6, Text: "o:wrap_dungeon_direct_move"}, {Type: 10, Value: 11}, {Type: 3, Text: "[/create var]"},
	}}
	act := ScriptRecord{Path: "content/action/check.act", Cells: []pvf.Token{{Type: 3, Text: "[COMPARE VAR]"}, {Type: 10, Value: 12}}}
	pool := map[int]string{10: "700", 11: "1", 12: "isApocalypseShortcutOpened(3)"}
	return obj, act, pool, []LegionDungeon{{Dungeon: 800}, {Dungeon: 700}}
}

func TestApocalypseDoorUsesSourceFormulaAndDestinationOrder(t *testing.T) {
	obj, act, pool, destinations := doorSource()
	resolve := func(i int) string { return pool[i] }
	door, err := parseApocalypseDoor(600, obj, act, resolve, destinations)
	if err != nil || door.Dungeon != 700 || door.Slot != 3 || door.Target != 1 || door.Object != 600 {
		t.Fatal(door, err)
	}
	pool[10], pool[12] = "800", "isApocalypseShortcutOpened(0)"
	changed, err := parseApocalypseDoor(601, obj, act, resolve, destinations)
	if err != nil || changed.Dungeon != 800 || changed.Slot != 0 || changed.Target != 0 || changed.Object != 601 {
		t.Fatal("source mutations did not affect binding", changed, err)
	}
}

func TestApocalypseDoorRefusesAmbiguousOrUnsupportedSource(t *testing.T) {
	for _, mutate := range []func(*ScriptRecord, *ScriptRecord, map[int]string, *[]LegionDungeon){
		func(o, a *ScriptRecord, p map[int]string, d *[]LegionDungeon) { p[10] = "getDungeon()" },
		func(o, a *ScriptRecord, p map[int]string, d *[]LegionDungeon) { p[10] = "-1" },
		func(o, a *ScriptRecord, p map[int]string, d *[]LegionDungeon) { p[11] = "0" },
		func(o, a *ScriptRecord, p map[int]string, d *[]LegionDungeon) {
			p[12] = "isApocalypseShortcutOpened(4)"
		},
		func(o, a *ScriptRecord, p map[int]string, d *[]LegionDungeon) { o.Cells[2].Type = 0 },
		func(o, a *ScriptRecord, p map[int]string, d *[]LegionDungeon) { a.Cells[0].Type = 6 },
		func(o, a *ScriptRecord, p map[int]string, d *[]LegionDungeon) {
			*d = append(*d, LegionDungeon{Dungeon: 700})
		},
		func(o, a *ScriptRecord, p map[int]string, d *[]LegionDungeon) { *d = nil },
		func(o, a *ScriptRecord, p map[int]string, d *[]LegionDungeon) { o.Cells = append(o.Cells, o.Cells...) },
		func(o, a *ScriptRecord, p map[int]string, d *[]LegionDungeon) {
			p[13] = "isApocalypseShortcutOpened(2)"
			a.Cells = append(a.Cells, pvf.Token{Type: 3, Text: "[COMPARE VAR]"}, pvf.Token{Type: 10, Value: 13})
		},
	} {
		obj, act, pool, destinations := doorSource()
		mutate(&obj, &act, pool, &destinations)
		if _, err := parseApocalypseDoor(600, obj, act, func(i int) string { return pool[i] }, destinations); err == nil {
			t.Fatal("unsupported source accepted")
		}
	}
}
