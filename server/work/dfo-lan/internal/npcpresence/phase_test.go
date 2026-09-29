package npcpresence

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestPhaseCacheDoesNotLowerWhenCompletedSnapshotChanges(t *testing.T) {
	rules := []PhaseRule{{Kind: 2, Value: 90, Index: 3, Resolved: true}}
	var c PhaseCache
	if phase, _ := c.EvaluateCompleted(80, rules, QuestSet{Known: true}); phase != nil {
		t.Fatal("missing reset guessed phase")
	}
	c.SuccessfulSelectionReset()
	phase, gaps := c.EvaluateCompleted(80, rules, QuestSet{Known: true, IDs: map[uint32]bool{90: true}})
	if phase == nil || *phase != 3 || len(gaps) != 0 {
		t.Fatalf("advance: %v %v", phase, gaps)
	}
	phase, _ = c.EvaluateCompleted(80, rules, QuestSet{Known: true})
	if phase == nil || *phase != 3 {
		t.Fatal("completed restore lowered cache")
	}
	c.SuccessfulSelectionReset()
	phase, _ = c.EvaluateCompleted(80, rules, QuestSet{Known: true})
	if phase == nil || *phase != -1 {
		t.Fatal("selection did not reset")
	}
}

func TestUnknownResolveCannotBeRepairedByCompletedSetAlone(t *testing.T) {
	var c PhaseCache
	c.SuccessfulSelectionReset()
	rules := []PhaseRule{{Kind: 2, Value: 90, Index: 1, Resolved: true}}
	c.EvaluateCompleted(80, rules, QuestSet{})
	if phase, _ := c.EvaluateCompleted(80, rules, QuestSet{Known: true}); phase != nil {
		t.Fatal("forgot earlier unknown phase advance")
	}
	c.ObservePhase(80, -1)
	if phase, _ := c.EvaluateCompleted(80, rules, QuestSet{Known: true}); phase == nil || *phase != -1 {
		t.Fatal("observed phase not used")
	}
}

func TestMixedOrDuplicatePhaseConditionsStayUnknown(t *testing.T) {
	for _, rules := range [][]PhaseRule{
		{{Kind: 3, Value: 1, Index: 0, Resolved: true}},
		{{Kind: 2, Value: 1, Index: 0, Resolved: true}, {Kind: 2, Value: 1, Index: 1, Resolved: true}},
	} {
		var c PhaseCache
		c.SuccessfulSelectionReset()
		if phase, _ := c.EvaluateCompleted(80, rules, QuestSet{Known: true}); phase != nil {
			t.Fatal("unsupported phase chosen")
		}
	}
}

func TestPhaseSourceProjectionPreservesUnsupportedMultiCondition(t *testing.T) {
	cells := []pvf.Token{tag("[phase shift]"), tag("[condition]"), word("[completed quest]"), num(90), tag("[phase Index]"), num(2), tag("[/phase shift]"), tag("[phase shift]"), tag("[multi condition]"), num(1), tag("[phase Index]"), num(3), tag("[/phase shift]")}
	rules, gaps := ProjectPhaseRules(cells)
	if len(rules) != 2 || !rules[0].Resolved || rules[0].Kind != 2 || rules[0].Value != 90 || rules[0].Index != 2 || rules[1].Resolved || len(gaps) != 1 {
		t.Fatalf("rules: %+v %v", rules, gaps)
	}
}
