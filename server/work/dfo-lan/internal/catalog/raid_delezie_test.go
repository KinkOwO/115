package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"reflect"
	"testing"
)

func delezieFixture() *pvf.CTPTable {
	t := &pvf.CTPTable{Path: "mode.ctp"}
	add := func(parent int, name string, cells ...pvf.CTPCell) int {
		i := len(t.Records)
		t.Records = append(t.Records, pvf.CTPRecord{Index: i, Parent: parent, Name: name, Cells: cells})
		return i
	}
	n := func(v float64) pvf.CTPCell { return pvf.CTPCell{Kind: "float", Float: v} }
	s := func(v string) pvf.CTPCell { return pvf.CTPCell{Kind: "name", Name: v} }
	phase := add(-1, "[PHASE]")
	dungeons := add(phase, "[DUNGEONS]")
	info := add(dungeons, "[DUNGEON INFO]")
	add(info, "[INDEX]", n(123))
	locations := add(phase, "[LOCATIONS]")
	loc := add(locations, "[LOCATION INFO]")
	add(loc, "[LOCATION INDEX]", n(0))
	add(loc, "[MOVABLE LOCATION]", n(9))
	trig := add(phase, "[TRIGGER]")
	or := add(trig, "[OR]")
	add(or, "[IF]", s("[X]"), s("[==]"), n(0))
	add(or, "[IF]", s("[Y]"), s("[>]"), n(4))
	behavior := add(phase, "[BEHAVIOR]")
	add(behavior, "[SET]", s("[X]"), n(1))
	for _, row := range t.Records {
		if row.Parent < 0 {
			continue
		}
		parent := &t.Records[row.Parent]
		parent.Refs = append(parent.Refs, pvf.CTPRef{Name: row.Name, Values: []uint64{uint64(row.Index)}})
	}
	return t
}

func TestDeleziePhaseFollowsSourceAndKeepsModeLocalReferences(t *testing.T) {
	table := delezieFixture()
	parse := func() []DeleziePhaseRules {
		tree, e := pvf.NewCTPTree(table)
		if e != nil {
			t.Fatal(e)
		}
		phases, e := parseDeleziePhases(tree)
		if e != nil {
			t.Fatal(e)
		}
		return phases
	}
	first := parse()[0]
	if _, ok := first.Dungeons[123]; !ok {
		t.Fatal("source dungeon missing")
	}
	if first.Locations[0].UndeclaredMovable[0] != 9 {
		t.Fatal("mode-external location hidden or fabricated")
	}
	if first.Events[0].Trigger[0].Name != "[OR]" {
		t.Fatal("nested condition collapsed")
	}
	// Source replacement changes membership; no maintained dungeon constant.
	table.Records[3].Cells[0].Float = 456
	second := parse()[0]
	if _, ok := second.Dungeons[123]; ok {
		t.Fatal("stale dungeon survived source change")
	}
	if _, ok := second.Dungeons[456]; !ok {
		t.Fatal("changed source ignored")
	}
}

func TestDelezieChannelBindingsFollowSourceAndRejectCrossMode(t *testing.T) {
	table := &pvf.CTPTable{Path: "header.ctp", Records: []pvf.CTPRecord{
		{Index: 0, Parent: -1, Name: "[RAID LIST WINDOW DIFFICULTY INFO]", Refs: []pvf.CTPRef{{Name: "[INFO]", Values: []uint64{1}}}},
		{Index: 1, Parent: 0, Name: "[INFO]", Refs: []pvf.CTPRef{{Name: "[CHANNEL TYPE]", Values: []uint64{2}}, {Name: "[DIFFICULTY]", Values: []uint64{3}}}},
		{Index: 2, Parent: 1, Name: "[CHANNEL TYPE]", Cells: []pvf.CTPCell{{Kind: "float", Float: 42}}},
		{Index: 3, Parent: 1, Name: "[DIFFICULTY]", Cells: []pvf.CTPCell{{Kind: "name", Name: "single"}}},
	}}
	parse := func() map[uint32][]DelezieDifficulty {
		tree, e := pvf.NewCTPTree(table)
		if e != nil {
			t.Fatal(e)
		}
		out, e := delezieChannelModes(tree)
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	bindings := parse()
	r := &DelezieRaidRules{ChannelModes: bindings, Modes: map[DelezieDifficulty]*DelezieModeRules{DelezieSingle: {Difficulty: DelezieSingle}}}
	if _, e := r.ModeForChannel(42, DelezieSingle); e != nil {
		t.Fatal(e)
	}
	if _, e := r.ModeForChannel(42, DelezieNormal); e == nil {
		t.Fatal("normal mode leaked into single channel")
	}
	table.Records[2].Cells[0].Float = 55
	changed := parse()
	if len(changed[42]) > 0 || changed[55][0] != DelezieSingle {
		t.Fatal("channel binding constant survived source change")
	}
	table.Records[3].Cells[0].Name = "unknown"
	tree, _ := pvf.NewCTPTree(table)
	if _, e := delezieChannelModes(tree); e == nil {
		t.Fatal("unknown difficulty silently replaced")
	}
}

func TestDelezieModeNativePVF(t *testing.T) {
	file := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if file == "" {
		t.Skip("native archive required")
	}
	a, e := OpenTestArchiveCached(file, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if e != nil {
		t.Fatal(e)
	}
	dir, e := ImportChannelDirectory(a)
	if e != nil {
		t.Fatal(e)
	}
	entries, e := ImportRaidEntrances(a, &dir)
	if e != nil {
		t.Fatal(e)
	}
	r := entries[112].Delezie
	if r == nil || entries[120].Delezie != r {
		t.Fatal("missing shared source rules")
	}
	if len(r.Tables) != 18 {
		t.Fatalf("canonical CTP directory has %d imported tables, expected current native 18", len(r.Tables))
	}
	if len(r.Modes[DelezieHard].Phases[0].Dungeons[100004604].Rows) != 2 {
		t.Fatal("repeated hard-mode dungeon metadata discarded")
	}
	if len(r.SymbolIDs) == 0 || len(r.TimerDefinitions) == 0 {
		t.Fatal("missing native symbol/timer declarations")
	}
	for _, item := range []struct {
		channel    uint32
		difficulty DelezieDifficulty
	}{{112, DelezieNormal}, {112, DelezieHard}, {120, DelezieSingle}, {120, DelezieMatch}} {
		m, e := r.ModeForChannel(item.channel, item.difficulty)
		if e != nil {
			t.Fatal(e)
		}
		if len(m.Phases) != 1 || len(m.ObjectDefinitions) == 0 || len(m.BuffDefinitions) == 0 || len(m.Phases[0].Events) == 0 || len(m.ScalarExpressions) == 0 {
			t.Fatal("empty source mode")
		}
		if m.ObjectGraph == nil || len(m.PhaseTimerReferences) == 0 || len(m.ObjectGraph.AIs) == 0 {
			t.Fatal("missing timer or object graph bindings")
		}
		summons, err := r.SummonProgram(m)
		if err != nil || !reflect.DeepEqual(summons, m.SummonActions) {
			t.Fatal("paired summon program did not follow native Source", summons, err)
		}
		expectedActions := []int{2, 2, 1, 1}[item.difficulty]
		expectedPairs := []int{6, 19, 1, 7}[item.difficulty]
		pairs, tails := 0, 0
		for _, action := range summons {
			pairs += len(action.Pairs)
			tails += len(action.IgnoredTail)
		}
		if len(summons) != expectedActions || pairs != expectedPairs {
			t.Fatalf("current native summon vectors: actions=%d pairs=%d", len(summons), pairs)
		}
		t.Logf("mode=%s summons=%d pairs=%d unusedCells=%d", item.difficulty.Name(), len(summons), pairs, tails)
		for _, ref := range m.PhaseTimerReferences {
			main, ok := r.TimerDefinitions[ref.MainName]
			if !ok || main.ID != ref.Key.Main || main.Subtypes[ref.SubName] != ref.Key.Sub {
				t.Fatalf("timer reference escaped declaration scope: %+v", ref)
			}
		}
		t.Logf("mode=%s phaseTimers=%d objectTimers=%d presets=%d combos=%d AIs=%d objectAIs=%d", item.difficulty.Name(), len(m.PhaseTimerReferences), len(m.ObjectTimerReferences), len(m.ObjectGraph.Presets), len(m.ObjectGraph.Combos), len(m.ObjectGraph.AIs), len(m.ObjectGraph.ObjectAIs))
		t.Logf("channel=%d mode=%s dungeons=%d locations=%d objects=%d buffs=%d events=%d symbols=%d timers=%d", item.channel, item.difficulty.Name(), len(m.Phases[0].Dungeons), len(m.Phases[0].Locations), len(m.ObjectDefinitions), len(m.BuffDefinitions), len(m.Phases[0].Events), len(r.SymbolIDs), len(r.TimerDefinitions))
	}
	if _, e := r.ModeForChannel(112, DelezieSingle); e == nil {
		t.Fatal("single allowed in normal channel")
	}
	if _, e := r.ModeForChannel(120, DelezieHard); e == nil {
		t.Fatal("hard allowed in single/match channel")
	}
	if r.Modes[DelezieNormal].Objects == r.Modes[DelezieSingle].Objects {
		t.Fatal("mode object definitions merged")
	}
}
