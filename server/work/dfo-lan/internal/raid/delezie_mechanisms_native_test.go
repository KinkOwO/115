package raid_test

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/raid"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestDelezieMechanismCoresNativePVF(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("explicit current native PVF required")
	}
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"), DerivedCacheDir: "-"})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	directory, err := source.ChannelDirectory()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := source.RaidEntrances(directory)
	if err != nil {
		t.Fatal(err)
	}
	rules := entries[112].Delezie
	header, err := rules.HeaderMechanismProgram()
	if err != nil || len(header.Symbols) != 31 || len(header.Timers) != 3 || !reflect.DeepEqual(header, rules.HeaderMechanismBindings) {
		t.Fatal("shared header references missing", header, err)
	}
	hardReferences := 0
	for _, ref := range header.Timers {
		if ref.Kind == "[HARD 1 PHASE END]" || ref.Kind == "[HARD 2 PHASE END]" {
			hardReferences++
		}
	}
	if hardReferences != 2 {
		t.Fatal("shared hard phase-end references discarded")
	}
	spawnLocation := func(mode *catalog.DelezieModeRules, id uint32) uint32 {
		t.Helper()
		definition, ok := mode.ObjectDefinitions[id]
		if !ok || !definition.HasStaticHP {
			t.Fatalf("missing static native definition %d", id)
		}
		for _, loc := range definition.CreatableLocations {
			if _, ok := mode.Phases[0].Locations[loc]; ok {
				return loc
			}
		}
		t.Fatalf("object %d lacks source phase spawn location", id)
		return 0
	}
	for difficulty := catalog.DelezieNormal; difficulty <= catalog.DelezieMatch; difficulty++ {
		t.Run(difficulty.Name(), func(t *testing.T) {
			mode := rules.Modes[difficulty]
			bindings, err := rules.MechanismProgram(mode)
			if err != nil {
				t.Fatal(err)
			}
			if difficulty == catalog.DelezieNormal {
				if len(bindings.Symbols) != 31 || len(bindings.Timers) != 3 {
					t.Fatalf("current native bindings: symbols=%d timers=%d", len(bindings.Symbols), len(bindings.Timers))
				}
			} else if len(bindings.Symbols) != 0 || len(bindings.Timers) != 0 {
				t.Fatal("mode-local bindings borrowed a different phase table")
			}
			if !reflect.DeepEqual(bindings, mode.MechanismBindings) {
				t.Fatal("Source mode preparation lost mechanism bindings")
			}
			for _, ref := range bindings.Symbols {
				if rules.SymbolIDs[ref.Symbol] != ref.ID {
					t.Fatal("mechanism symbol escaped source", ref)
				}
			}
			program, err := rules.TimerProgram(mode)
			if err != nil {
				t.Fatal(err)
			}
			var positive []int
			nonpositive := 0
			for row, ref := range program.Phase {
				if ref.HasValue {
					if ref.Value > 0 {
						positive = append(positive, row)
					} else {
						nonpositive++
					}
				}
			}
			sort.Ints(positive)
			if len(positive) == 0 {
				t.Fatal("mode has no source positive timer input")
			}
			for _, row := range positive {
				clock, err := raid.NewDelezieTimers(rules, mode)
				if err != nil {
					t.Fatal(err)
				}
				windows, err := clock.ArmPhase([]int{row})
				if err != nil {
					t.Fatal(err)
				}
				ref := program.Phase[row]
				if windows[0].Key != ref.Key || windows[0].Value != ref.Value {
					t.Fatal("armed window ignored source", windows, ref)
				}
				if ended, err := clock.AdvanceRaw(uint64(ref.Value - 1)); err != nil || len(ended) != 0 {
					t.Fatal("early raw expiry", ended, err)
				}
				if ended, err := clock.AdvanceRaw(uint64(ref.Value)); err != nil || len(ended) != 1 {
					t.Fatal("raw expiry missing", ended, err)
				}
			}
			script, err := raid.NewDelezieScript(rules, mode, map[string]int32{"[RAID INIT]": 0})
			if err != nil {
				t.Fatal(err)
			}
			var supported []catalog.DelezieScriptEvent
			for _, event := range mode.Phases[0].Events {
				if script.ValidateEvent(event.TriggerRow) == nil {
					supported = append(supported, event)
				}
			}
			expected := 0
			if difficulty == catalog.DelezieNormal {
				expected = 2
			} else if difficulty == catalog.DelezieHard {
				expected = 1
			}
			if len(supported) != expected {
				t.Fatalf("current complete scalar/timer subset=%d expected=%d", len(supported), expected)
			}
			if len(supported) > 0 {
				matched, changes, err := script.ExecuteEvent(supported[0].TriggerRow)
				if err != nil || !matched || script.Values()["[RAID INIT]"] != 1 {
					t.Fatal("source initialization event failed", matched, changes, err)
				}
				expectedScalars, expectedTimers := 41, 2
				if difficulty == catalog.DelezieHard {
					expectedScalars, expectedTimers = 17, 1
				}
				if len(changes.Scalars) != expectedScalars || len(changes.Timers) != expectedTimers {
					t.Fatal("initial event partially executed", changes)
				}
				if matched, _, err := script.ExecuteEvent(supported[0].TriggerRow); err != nil || matched {
					t.Fatal("initialization replayed", matched, err)
				}
			}
			if difficulty == catalog.DelezieNormal {
				event := supported[1]
				ref := program.Phase[event.Trigger[0].Index]
				window := script.Timers()[ref.Key]
				if _, err := script.AdvanceRaw(window.Deadline - 1); err != nil {
					t.Fatal(err)
				}
				if matched, _, err := script.ExecuteEvent(event.TriggerRow); err != nil || matched {
					t.Fatal("early pollution event", matched, err)
				}
				if _, err := script.AdvanceRaw(window.Deadline); err != nil {
					t.Fatal(err)
				}
				matched, changes, err := script.ExecuteEvent(event.TriggerRow)
				if err != nil || !matched || len(changes.Scalars) != 1 || len(changes.Timers) != 1 {
					t.Fatal("pollution event failed", matched, changes, err)
				}
				step := mode.ScalarExpressions[changes.Scalars[0].Row].Operand.Value
				if script.Values()[changes.Scalars[0].Symbol] != step || changes.Timers[0].Deadline != window.Deadline+uint64(window.Value) {
					t.Fatal("pollution increment/rearm ignored source", changes)
				}
				if matched, _, err := script.ExecuteEvent(event.TriggerRow); err != nil || matched {
					t.Fatal("pollution tick replayed", matched, err)
				}
				if _, err := script.AdvanceRaw(changes.Timers[0].Deadline); err != nil {
					t.Fatal(err)
				}
				if matched, _, err := script.ExecuteEvent(event.TriggerRow); err != nil || !matched || script.Values()[changes.Scalars[0].Symbol] != 2*step {
					t.Fatal("second pollution tick failed", matched, err)
				}
			}
			objects, err := raid.NewDelezieObjects(rules, mode, 0)
			if err != nil {
				t.Fatal(err)
			}
			var linked uint32
			for id := range mode.ObjectGraph.ObjectAIs {
				if definition := mode.ObjectDefinitions[id]; definition.HasStaticHP {
					linked = id
					break
				}
			}
			if linked == 0 {
				t.Fatal("mode has no static live AI object")
			}
			created, err := objects.Spawn([]raid.DelezieObjectSpawn{{Definition: linked, Location: spawnLocation(mode, linked)}})
			if err != nil {
				t.Fatal(err)
			}
			ai, err := objects.AIProgram(created[0].Key)
			if err != nil || !reflect.DeepEqual(ai, mode.ObjectGraph.AIs[mode.ObjectGraph.ObjectAIs[linked]]) {
				t.Fatal("live object AI program differs from native source", ai, err)
			}
			if difficulty != catalog.DelezieNormal {
				if _, err := objects.CombineResolved([]raid.DelezieResolvedCombination{{RecipeRow: rules.Modes[catalog.DelezieNormal].ObjectGraph.Combos[0].Row, Materials: [2]uint32{created[0].Key, created[0].Key}, Location: created[0].Location}}); err == nil {
					t.Fatal("foreign normal combination accepted")
				}
			}
			t.Logf("mode=%s mechanismSymbols=%d specialTimerRefs=%d positivePhaseRefs=%d unresolvedNonpositiveRefs=%d completeScalarTimerEvents=%d totalPhaseEvents=%d liveAI=%d wallClockConnected=false fullBattleConnected=false", difficulty.Name(), len(bindings.Symbols), len(bindings.Timers), len(positive), nonpositive, len(supported), len(mode.Phases[0].Events), ai.ID)
		})
	}
	mode := rules.Modes[catalog.DelezieNormal]
	for _, recipe := range mode.ObjectGraph.Combos {
		objects, err := raid.NewDelezieObjects(rules, mode, 0)
		if err != nil {
			t.Fatal(err)
		}
		materials, err := objects.Spawn([]raid.DelezieObjectSpawn{{Definition: recipe.Materials[0], Location: spawnLocation(mode, recipe.Materials[0])}, {Definition: recipe.Materials[1], Location: spawnLocation(mode, recipe.Materials[1])}})
		if err != nil {
			t.Fatal(err)
		}
		request := raid.DelezieResolvedCombination{RecipeRow: recipe.Row, Materials: [2]uint32{materials[0].Key, materials[1].Key}, Location: spawnLocation(mode, recipe.Result)}
		result, err := objects.CombineResolved([]raid.DelezieResolvedCombination{request})
		if err != nil || len(result) != 1 || result[0].Created.Definition != recipe.Result || result[0].Created.HP != mode.ObjectDefinitions[recipe.Result].StaticHP || len(objects.Snapshot()) != 1 {
			t.Fatal("native combination result invalid", recipe, result, err)
		}
		if _, err := objects.AIProgram(result[0].Created.Key); err != nil {
			t.Fatal("native combination result lacks bound AI", recipe, err)
		}
		if _, err := objects.CombineResolved([]raid.DelezieResolvedCombination{request}); err == nil {
			t.Fatal("native consumed materials replayed")
		}
	}
	t.Logf("normal resolved-source-recipes=%d automaticCollisionConnected=false completeSynthesisEffects=false", len(mode.ObjectGraph.Combos))
}
