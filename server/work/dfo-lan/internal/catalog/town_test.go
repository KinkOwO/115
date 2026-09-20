package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestTownSourceEntryBounds(t *testing.T) {
	c := TownArea{MinimumLevel: 1, Walkable: [][4]int32{{10, 196, 2670, 142}}}
	for _, tc := range []struct {
		level   byte
		x, y    uint16
		allowed bool
	}{{1, 561, 234, true}, {0, 561, 234, false}, {1, 9, 234, false}, {1, 2680, 234, false}, {1, 561, 338, false}, {1, 2679, 337, true}} {
		if got := c.Allows(tc.level, tc.x, tc.y); got != tc.allowed {
			t.Fatalf("level=%d x=%d y=%d: %v", tc.level, tc.x, tc.y, got)
		}
	}
}

func TestWorldAreaDungeonGatePriority(t *testing.T) {
	// Verify that [dungeon gate] is not overwritten by subsequent [normal] in area definition
	tokens := []pvf.Token{
		{Type: 3, Text: "[area]"},
		{Type: 0, Value: 0},
		{Type: 6, Text: "map/test.map"},
		{Type: 6, Text: "[dungeon gate]"},
		{Type: 6, Text: "[normal]"},
		{Type: 3, Text: "[/area]"},
	}
	areas, err := parseWorldAreas(235, tokens)
	if err != nil {
		t.Fatalf("parseWorldAreas failed: %v", err)
	}
	if len(areas) != 1 {
		t.Fatalf("expected 1 area, got %d", len(areas))
	}
	if areas[0].Kind != "[dungeon gate]" {
		t.Fatalf("expected Kind to be [dungeon gate], got %q", areas[0].Kind)
	}
}

func TestNestedSectionCellsKeepsRowsAfterConditionBlocks(t *testing.T) {
	// 实源布局（elvengard_hendon_storm.map 38/3）：[town movable area] 内嵌
	// [quest condition] 3156 [/quest condition]，其后才是通往 Elvenmere
	// (38/7) 的门户行；旧的 sectionCells 会在子块处失活并吞掉这一行。
	tokens := []pvf.Token{
		{Type: 3, Text: "[town movable area]"},
		{Type: 0, Value: 207}, {Type: 0, Value: 134}, {Type: 0, Value: 120}, {Type: 0, Value: 40}, {Type: 0, Value: 38}, {Type: 0, Value: 0},
		{Type: 0, Value: 938}, {Type: 0, Value: 323}, {Type: 0, Value: 100}, {Type: 0, Value: 80}, {Type: 0, Value: 39}, {Type: 0, Value: 0},
		{Type: 3, Text: "[quest condition]"},
		{Type: 0, Value: 3156},
		{Type: 3, Text: "[/quest condition]"},
		{Type: 0, Value: 24}, {Type: 0, Value: 218}, {Type: 0, Value: 100}, {Type: 0, Value: 120}, {Type: 0, Value: 38}, {Type: 0, Value: 7},
		{Type: 3, Text: "[/town movable area]"},
	}
	rows, err := sourceRectangles(nestedSectionCells(tokens, "[town movable area]"), 6)
	if err != nil {
		t.Fatalf("sourceRectangles failed: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 portal rows, got %d: %v", len(rows), rows)
	}
	last := rows[2]
	if last[0] != 24 || last[1] != 218 || last[4] != 38 || last[5] != 7 {
		t.Fatalf("quest-gated portal row lost or wrong: %v", last)
	}
}

func TestNestedSectionCellsSkipsSubBlockNumbers(t *testing.T) {
	// 子块（如 [quest condition] 10100）内部的数字属于子块，不得计入父段
	// 矩形行（实源：new_hendon_main.map 39/0 的 [virtual movable area]）。
	tokens := []pvf.Token{
		{Type: 3, Text: "[virtual movable area]"},
		{Type: 0, Value: 16}, {Type: 0, Value: 176}, {Type: 0, Value: 3540}, {Type: 0, Value: 280},
		{Type: 3, Text: "[quest condition]"},
		{Type: 0, Value: 10100},
		{Type: 3, Text: "[/quest condition]"},
		{Type: 0, Value: 299}, {Type: 0, Value: 134}, {Type: 0, Value: 150}, {Type: 0, Value: 90},
		{Type: 3, Text: "[/virtual movable area]"},
	}
	rows, err := sourceRectangles(nestedSectionCells(tokens, "[virtual movable area]"), 4)
	if err != nil {
		t.Fatalf("sourceRectangles failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 walkable rows, got %d: %v", len(rows), rows)
	}
	if rows[1][0] != 299 {
		t.Fatalf("row after condition block lost: %v", rows[1])
	}
}
