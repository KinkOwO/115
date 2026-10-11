package raid

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"reflect"
	"testing"
	"time"
)

// A short source-shaped phase isolates lifecycle boundaries without waiting
// through native reinforcement and anger events. Native modes run separately.
func lifecycleBakal() (*BakalOpening, time.Time) {
	now := time.Unix(1700000000, 0)
	s := &BakalOpening{
		rules: &catalog.BakalRaidRules{
			TimeLimit:       10,
			Symbols:         map[string]uint32{"[BAKAL ANGER]": 1, "[BAKAL ANGER PER SECOND]": 2, "[BAKAL PARTY1 DAMAGE]": 3, "[BAKAL PHASE]": 4},
			Locations:       map[uint32]catalog.BakalScriptLocation{7: {ID: 7, Dungeon: 8}},
			Slots:           map[uint32]catalog.BakalRaidSlot{7: {ID: 7, Dungeon: 8, Kind: "boss", Maps: []uint32{9}}},
			BuffDefinitions: map[string]catalog.BakalBuffDefinition{"charge": {Kind: "charge", Index: 20, Raid: true}},
		},
		active: true, startedAt: now, clock: now, roster: []Member{{Actor: 1, Position: 1}},
		variables: map[string]int32{"[BAKAL ANGER]": 0, "[BAKAL ANGER PER SECOND]": 0, "[MONSTER MAX HP]": 100, "[BAKAL PARTY1 DAMAGE]": 0},
		monsters:  map[uint32]catalog.BakalRaidMonster{7: {Kind: "bakal", ID: 10}},
		dungeons:  map[uint32]string{8: "open"}, currentDungeon: 8,
		deadlines: map[[2]uint32]time.Time{}, clearCounts: map[uint32]bool{}, raidBuffs: [5]uint32{1},
	}
	return s, now
}

func TestBakalDeadlineCannotBecomeLateClear(t *testing.T) {
	for _, timerOffset := range []time.Duration{9 * time.Second, 10 * time.Second, 11 * time.Second} {
		t.Run(timerOffset.String(), func(t *testing.T) {
			s, now := lifecycleBakal()
			s.rules.Events = []catalog.BakalScriptEvent{{
				Trigger:  []catalog.BakalScriptInstruction{{Op: "[CHECK TIMER END]", Args: []pvf.Token{{Type: 0, Value: 1}, {Type: 0, Value: 7}}}},
				Behavior: []catalog.BakalScriptInstruction{{Op: "[CLEAR PHASE]"}},
			}}
			s.deadlines[[2]uint32{1, 7}] = now.Add(timerOffset)
			effects, err := s.Tick(now.Add(12 * time.Second))
			if err != nil {
				t.Fatal(err)
			}
			want := "fail"
			if timerOffset < 10*time.Second {
				want = "clear"
			}
			if s.Ended() != want {
				t.Fatalf("expired phase became %s; want %s (effects=%+v)", s.Ended(), want, effects)
			}
			if more, err := s.Tick(now.Add(13 * time.Second)); err != nil || len(more) != 0 {
				t.Fatal("terminal phase emitted again", more, err)
			}
		})
	}
}

func TestBakalTerminalAndExpiredCommandsAreInert(t *testing.T) {
	commands := map[string]func(*BakalOpening, time.Time) error{
		"symbol": func(s *BakalOpening, at time.Time) error { return s.ReportSymbol(3, 1, at) },
		"health": func(s *BakalOpening, at time.Time) error { return s.ReportHealth(7, 90, at) },
		"damage": func(s *BakalOpening, at time.Time) error { return s.ReportCurrentDamage(1, at) },
		"death":  func(s *BakalOpening, at time.Time) error { return s.DefeatMonster(7, 10, at) },
		"buff":   func(s *BakalOpening, at time.Time) error { return s.UseRaidBuff(20, at) },
		"giveup": func(s *BakalOpening, at time.Time) error { return s.GiveupDungeon(8, at) },
		"slot":   func(s *BakalOpening, at time.Time) error { _, err := s.AuthorizeSlot(7, 8, 9, at); return err },
		"final": func(s *BakalOpening, at time.Time) error {
			s.rules.Events = []catalog.BakalScriptEvent{{Trigger: []catalog.BakalScriptInstruction{{Op: "[CHECK TIMER END]", Args: []pvf.Token{{Value: 1}, {Value: 7}}}}, Behavior: []catalog.BakalScriptInstruction{{Op: "[CLEAR PHASE]"}}}}
			return s.ClearFinalDungeon(8, at)
		},
	}
	for name, command := range commands {
		for _, status := range []string{"clear", "fail", "expired"} {
			t.Run(name+"/"+status, func(t *testing.T) {
				s, now := lifecycleBakal()
				at := now.Add(time.Second)
				if status == "expired" {
					at = now.Add(10 * time.Second)
				} else {
					s.ended = status
				}
				before := s.cloneScript()
				if err := command(s, at); err == nil {
					t.Fatal("accepted command after phase lifetime")
				}
				if !reflect.DeepEqual(s.variables, before.variables) || !reflect.DeepEqual(s.monsters, before.monsters) || !reflect.DeepEqual(s.effects, before.effects) || s.currentDungeon != before.currentDungeon || s.raidBuffs != before.raidBuffs {
					t.Fatal("rejected command changed phase")
				}
			})
		}
	}
}

func TestBakalActivationFailureRollsBack(t *testing.T) {
	s, now := lifecycleBakal()
	s.active = false
	s.readyAt = now
	s.startedAt = time.Time{}
	s.rules.InitialVariables = map[string]int32{"[BAKAL ANGER]": 4}
	s.rules.Events = []catalog.BakalScriptEvent{{Behavior: []catalog.BakalScriptInstruction{
		{Op: "[SET]", Args: []pvf.Token{{Type: 3, Text: "[BAKAL ANGER]"}, {Type: 0, Value: 5}}},
		{Op: "unsupported"},
	}}}
	before := s.cloneScript()
	if err := s.Activate(now); err == nil {
		t.Fatal("invalid bootstrap accepted")
	}
	if s.active || !s.startedAt.Equal(before.startedAt) || !reflect.DeepEqual(s.variables, before.variables) || !reflect.DeepEqual(s.monsters, before.monsters) || !reflect.DeepEqual(s.effects, before.effects) {
		t.Fatal("failed bootstrap left partial state")
	}
}
