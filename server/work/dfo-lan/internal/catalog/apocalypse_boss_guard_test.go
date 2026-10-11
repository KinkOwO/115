package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func guardedNextCells() []pvf.Token {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	text := func(s string) pvf.Token { return pvf.Token{Type: 6, Text: s} }
	expr := func(v int32) pvf.Token { return pvf.Token{Type: 10, Value: v} }
	return []pvf.Token{tag("[TRIGGER]"), tag("[FRAME]"), {Type: 0, Value: 16}, tag("[BEGIN IF]"), tag("[COMPARE VAR]"), expr(1), tag("[==]"), expr(2), tag("[DO BEHAVIOR NAME]"), text("ME"), text("NEXT"), tag("[ELSE IF]"), tag("[COMPARE VAR]"), expr(1), tag("[==]"), expr(3), tag("[DO BEHAVIOR NAME]"), text("ME"), text("NEXT"), tag("[END IF]"), tag("[/TRIGGER]"), tag("[BEHAVIOR]"), text("NEXT"), tag("[LEGION DUNGEON DIRECT MOVE NEXT PHASE]"), text("Apocalypse"), tag("[/BEHAVIOR]")}
}

func TestApocalypseNextBehaviorUsesSourceOperationGuard(t *testing.T) {
	for _, values := range [][2]string{{"2", "3"}, {"7", "9"}} {
		resolve := func(i int) string {
			return map[int]string{1: "getLegionOperationIndex()", 2: values[0], 3: values[1]}[i]
		}
		operations, err := apocalypseNextActionOperations(guardedNextCells(), resolve)
		if err != nil {
			t.Fatal(err)
		}
		m := ApocalypseBossMove{Operations: operations}
		if m.AllowsOperation(1) || m.AllowsOperation(5) || m.AllowsOperation(0) {
			t.Fatal("conditional action became unconditional")
		}
		for _, operation := range operations {
			if !m.AllowsOperation(operation) {
				t.Fatal("source guard lost")
			}
		}
		if values[0] == "7" && (m.AllowsOperation(2) || m.AllowsOperation(3)) {
			t.Fatal("hardcoded operation predicate")
		}
	}
}

func TestApocalypseNextBehaviorRejectsUnsupportedPredicate(t *testing.T) {
	for _, bad := range []map[int]string{{1: "getLegionOperationIndex() + 1", 2: "2", 3: "3"}, {1: "getLegionOperationIndex()", 2: "random()", 3: "3"}, {1: "getLegionOperationIndex()", 2: "0", 3: "3"}} {
		if _, err := apocalypseNextActionOperations(guardedNextCells(), func(i int) string { return bad[i] }); err == nil {
			t.Fatal("unknown predicate accepted")
		}
	}
	cells := guardedNextCells()
	cells[14].Text = "[>=]"
	if _, err := apocalypseNextActionOperations(cells, func(i int) string { return map[int]string{1: "getLegionOperationIndex()", 2: "2", 3: "3"}[i] }); err == nil {
		t.Fatal("unsupported comparison accepted")
	}
	cells = guardedNextCells()
	cells = append(cells, pvf.Token{Type: 3, Text: "[BEHAVIOR]"}, pvf.Token{Type: 6, Text: "NEXT"}, pvf.Token{Type: 3, Text: "[/BEHAVIOR]"})
	if _, err := apocalypseNextActionOperations(cells, func(i int) string { return map[int]string{1: "getLegionOperationIndex()", 2: "2", 3: "3"}[i] }); err == nil {
		t.Fatal("ambiguous dispatch name accepted")
	}
}

func TestApocalypseNextBehaviorRequiresActualTrigger(t *testing.T) {
	cells := guardedNextCells()[21:]
	if _, err := apocalypseNextActionOperations(cells, nil); err == nil {
		t.Fatal("orphan behavior authorized progression")
	}
	// Ordinal zero with an unconditional native frame trigger.
	cells = append([]pvf.Token{{Type: 3, Text: "[TRIGGER]"}, {Type: 3, Text: "[FRAME]"}, {Type: 0, Value: 10}, {Type: 3, Text: "[DO BEHAVIOR]"}, {Type: 6, Text: "ME"}, {Type: 0, Value: 0}, {Type: 3, Text: "[/TRIGGER]"}}, cells...)
	if operations, err := apocalypseNextActionOperations(cells, nil); err != nil || len(operations) != 0 {
		t.Fatal("source unconditional trigger lost", err)
	}
}
