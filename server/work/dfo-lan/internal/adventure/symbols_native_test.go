package adventure

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestAdventureSymbolsFollowSourceIndices(t *testing.T) {
	block := func(id int32) []pvf.Token {
		return []pvf.Token{{Type: 3, Text: "[adv symbol]"}, {Type: 3, Text: "[index]"}, {Type: 0, Value: id}, {Type: 3, Text: "[/adv symbol]"}}
	}
	cells := append(block(0), block(987654)...)
	r, err := readSymbols(cells)
	if err != nil || !r[0] || !r[987654] || len(r) != 2 {
		t.Fatal(r, err)
	}
	cells[len(cells)-2].Value = 123
	r, err = readSymbols(cells)
	if err != nil || r[987654] || !r[123] {
		t.Fatal("changed source not reflected", r, err)
	}
	if _, err := readSymbols(append(block(0), block(0)...)); err == nil {
		t.Fatal("duplicate source accepted")
	}
	if _, err := readSymbols(block(123)); err == nil {
		t.Fatal("missing default accepted")
	}
}
