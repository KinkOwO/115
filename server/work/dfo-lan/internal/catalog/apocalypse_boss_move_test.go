package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestApocalypseLastActionMoveIsSourceDeclared(t *testing.T) {
	valid := []pvf.Token{{Type: 3, Text: "[LEGION DUNGEON DIRECT MOVE NEXT PHASE]"}, {Type: 6, Text: "Apocalypse"}, {Type: 3, Text: "[/BEHAVIOR]"}}
	if ok, err := apocalypseLastActionMovesNext(valid); err != nil || !ok {
		t.Fatal("native source action lost", err)
	}
	if ok, err := apocalypseLastActionMovesNext(valid[2:]); err != nil || ok {
		t.Fatal("absent action invented", err)
	}
	for _, cells := range [][]pvf.Token{
		valid[:1],
		{{Type: 3, Text: valid[0].Text}, {Type: 6, Text: "Venus"}},
		{{Type: 3, Text: valid[0].Text}, {Type: 0, Value: 107}},
		{{Type: 3, Text: valid[0].Text}, {Type: 6, Text: "Apocalypse"}, {Type: 0, Value: 1}},
		append(append([]pvf.Token(nil), valid...), valid...),
	} {
		if ok, err := apocalypseLastActionMovesNext(cells); err == nil || ok {
			t.Fatal("unsupported source action accepted", cells)
		}
	}
}
