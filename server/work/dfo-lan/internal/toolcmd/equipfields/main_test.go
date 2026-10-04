package equipfields

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestParseEquipmentPaths(t *testing.T) {
	paths, err := parseEquipmentPaths([]pvf.Token{
		{Type: 0, Value: 10018}, {Type: 6, Text: "equipment/character/fighter/glove.equ"},
		{Type: 0, Value: 10019}, {Type: 6, Text: "equipment/character/fighter/gauntlet.equ"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[10018] != "equipment/character/fighter/glove.equ" {
		t.Fatalf("unexpected source equipment index: %#v", paths)
	}
}

func TestParseEquipmentPathsRejectsMalformedIndex(t *testing.T) {
	if _, err := parseEquipmentPaths([]pvf.Token{{Type: 0, Value: 10018}}); err == nil {
		t.Fatal("unpaired equipment index cell accepted")
	}
}
