package adventure

import (
	"dfolan/internal/catalog/pvf"
	"reflect"
	"testing"
)

func eliteTestCells(mode int32, dungeons ...int32) []pvf.Token {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	out := []pvf.Token{tag("[contents]"), {Type: 0, Value: 1}, tag("[contents key]"),
		{Type: 0, Value: mode}, tag("[dungeons]")}
	for _, id := range dungeons {
		out = append(out, pvf.Token{Type: 0, Value: id})
	}
	return append(out, tag("[/dungeons]"), tag("[/contents]"))
}

func TestEliteBindingsFollowSourceAndDoNotKeepRetiredDungeons(t *testing.T) {
	first, err := readEliteContents(eliteTestCells(2, 10, 20))
	if err != nil {
		t.Fatal(err)
	}
	r := &Rules{EliteContents: first}
	if mode, ok := r.EliteModeForDungeon(10); !ok || mode != 2 {
		t.Fatal("source binding missing", mode, ok)
	}
	second, err := readEliteContents(eliteTestCells(1, 20, 30))
	if err != nil {
		t.Fatal(err)
	}
	r.EliteContents = second
	if _, ok := r.EliteModeForDungeon(10); ok {
		t.Fatal("retired source dungeon survived")
	}
	if mode, ok := r.EliteModeForDungeon(30); !ok || mode != 1 {
		t.Fatal("changed source mode/dungeon ignored", mode, ok)
	}
	if _, ok := (*Rules)(nil).EliteModeForDungeon(30); ok {
		t.Fatal("nil rules supplied a default binding")
	}
}

func TestEliteSourceRejectsAmbiguousOrIncompleteBindings(t *testing.T) {
	valid := eliteTestCells(2, 10)
	for name, cells := range map[string][]pvf.Token{
		"duplicate mode":      append(append([]pvf.Token{}, valid...), valid...),
		"duplicate dungeon":   eliteTestCells(2, 10, 10),
		"cross-mode dungeon":  append(eliteTestCells(2, 10), eliteTestCells(1, 10)...),
		"unclosed block":      valid[:len(valid)-1],
		"missing dungeon end": append(append([]pvf.Token{}, valid[:len(valid)-2]...), valid[len(valid)-1]),
		"empty dungeons":      eliteTestCells(2),
		"invalid dungeon":     eliteTestCells(2, -1),
		"unknown native key":  eliteTestCells(5, 10),
		"nested block":        append(valid[:len(valid)-1:len(valid)-1], valid...),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := readEliteContents(cells); err == nil {
				t.Fatal("invalid source accepted")
			}
		})
	}
}

func TestEliteInstalledRulesOwnTheirSourceSnapshot(t *testing.T) {
	baseline, err := EmbeddedRules()
	if err != nil {
		t.Fatal(err)
	}
	source := *baseline
	source.EliteContents = EliteContents{2: {10, 20}}
	restore, err := InstallRules(&source)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	source.EliteContents[2][0] = 99
	delete(source.EliteContents, 2)
	current, err := Current()
	if err != nil || !reflect.DeepEqual(current.EliteContents, EliteContents{2: {10, 20}}) {
		t.Fatal("caller mutation changed live content bindings", err)
	}
	invalid := source
	invalid.EliteContents = EliteContents{2: {10}, 1: {10}}
	if _, err := InstallRules(&invalid); err == nil {
		t.Fatal("ambiguous source installed")
	}
	if after, _ := Current(); after != current {
		t.Fatal("invalid install replaced active bindings")
	}
}
