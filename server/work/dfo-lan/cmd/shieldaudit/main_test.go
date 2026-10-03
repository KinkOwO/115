package main

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestWindowParserRejectsUnknownConditions(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[shield]"}, {Type: 3, Text: "[item index]"}, {Type: 0, Value: 113370003}, {Type: 3, Text: "[get condition]"}, {Type: 6, Text: "level"}, {Type: 3, Text: "[get shield level]"}, {Type: 0, Value: 1}, {Type: 3, Text: "[/shield]"}, {Type: 3, Text: "[/shield]"}}
	rows, e := parseWindow(cells)
	if e != nil || len(rows) != 1 || rows[0].Item != 113370003 || rows[0].RequiredLevel != 1 {
		t.Fatal(rows, e)
	}
	cells[4].Text = "unknown"
	if _, e = parseWindow(cells); e == nil {
		t.Fatal("unknown condition silently exported")
	}
}
