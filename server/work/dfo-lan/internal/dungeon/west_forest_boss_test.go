package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestWestForestBossMonsterNPCOption(t *testing.T) {
	// The two [monster] rows in source map 91757. The NPC association is
	// followed by a numeric id and then the boss rank of the same row.
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	option := func(s string) pvf.Token { return pvf.Token{Type: 6, Text: s} }
	num := func(v int32) pvf.Token { return pvf.Token{Type: 0, Value: v} }
	first := []pvf.Token{num(107000195), num(1), num(0), num(587), num(288), num(0), num(1), num(1), option("[fixed]"), option("[NPC]"), num(1020), option("[boss]")}
	second := []pvf.Token{num(63821), num(1), num(0), num(722), num(-303), num(0), num(1), num(1), option("[fixed]"), option("[cinematic]"), option("[displayhuntdummy]"), option("[boss]")}
	cells := append([]pvf.Token{tag("[monster]")}, first...)
	cells = append(cells, second...)
	cells = append(cells, tag("[/monster]"))
	script := catalog.ScriptRecord{Path: "map/cataclysm/act13/78_west_forest/91757.map", Cells: cells}
	monsters, err := fixedMonsters(script, 69)
	if err != nil {
		t.Fatal(err)
	}
	if len(monsters) != 2 || monsters[0].SourceIndex != 0 || monsters[0].Template != 107000195 || monsters[0].Rank != 3 || monsters[0].NonCombat || monsters[1].SourceIndex != 1 || monsters[1].Template != 63821 || !monsters[1].NonCombat {
		t.Fatalf("source monster rows changed: %+v", monsters)
	}

	cells[11] = option("[boss]") // NPC's numeric operand must still be present.
	if _, err := fixedMonsters(script, 69); err == nil {
		t.Fatal("accepted NPC option without numeric operand")
	}
}
