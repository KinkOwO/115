package raid

import (
	"dfolan/internal/catalog"
	"reflect"
	"testing"
)

func asrahanCombatFixture() *catalog.AsrahanRaidRules {
	r := asrahanKernelFixture()
	r.Normal.Events[0].Behavior = append(r.Normal.Events[0].Behavior, asrahanTokens("[SET DUNGEON STATE]", 11, "open")...)
	r.Groups = []catalog.AsrahanSymbolGroup{{Header: asrahanTokens(1, 11, -1), Fields: asrahanTokens("[HP]", -1, "[DAMAGE1]", "[DAMAGE2]", "[DAMAGE3]", "[COUNT]", "[ONE]", 21, -1, -1, -1, -1, -1, "[MAX]", -1, -1, -1)}}
	return r
}

func TestAsrahanSourceCombatOwnerRejectsStaleReportsAndReusedRuns(t *testing.T) {
	s, now := newAsrahanKernel(t, asrahanCombatFixture(), nil)
	if err := s.BeginCombat("run", "fight", now, 1, 11, 21, 0); err == nil {
		t.Fatal("combat before source initialization")
	}
	if err := s.Start("run", now); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][3]uint32{{2, 11, 21}, {1, 12, 21}, {1, 11, 22}} {
		if err := s.BeginCombat("run", "fight", now, args[0], args[1], args[2], 0); err == nil {
			t.Fatal("foreign group/dungeon/monster obtained ownership", args)
		}
	}
	if err := s.BeginCombat("run", "fight", now, 1, 11, 21, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.BeginCombat("run", "other", now, 1, 11, 21, 0); err == nil {
		t.Fatal("one party owns two fights")
	}
	if err := s.VerifiedCombatDamage("run", "fight", now, 20); err != nil || s.View().Symbols["[HP]"] != 80 {
		t.Fatal(err, s.View())
	}
	before := s.View()
	for _, report := range []struct {
		run, combat string
		damage      int32
	}{{"foreign", "fight", 30}, {"run", "foreign", 30}, {"run", "fight", 19}, {"run", "fight", -1}} {
		if err := s.VerifiedCombatDamage(report.run, report.combat, now, report.damage); err == nil || !reflect.DeepEqual(before, s.View()) {
			t.Fatal("invalid report changed source", report, err)
		}
	}
	s.DrainEffects()
	if err := s.VerifiedCombatDamage("run", "fight", now, 20); err != nil || len(s.DrainEffects()) != 0 {
		t.Fatal("duplicate damage replayed effects", err)
	}
	if err := s.EndCombat("run", "fight", now); err != nil {
		t.Fatal(err)
	}
	if err := s.BeginCombat("run", "fight", now, 1, 11, 21, 0); err == nil {
		t.Fatal("retired combat ID reused")
	}
	if err := s.BeginCombat("run", "retry", now, 1, 11, 21, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifiedCombatDamage("run", "fight", now, 90); err == nil {
		t.Fatal("old fight changed retry symbols")
	}
	if err := s.VerifiedCombatDamage("run", "retry", now, 30); err != nil || s.View().Symbols["[HP]"] != 70 {
		t.Fatal(err, s.View())
	}
}

func TestAsrahanCombatDamageAndEffectsRollBackTogether(t *testing.T) {
	r := asrahanCombatFixture()
	r.Normal.Events = append(r.Normal.Events, catalog.AsrahanSourceEvent{Trigger: asrahanTokens("[ON CHANGE SYMBOL]", "[HP]", "[IF]", "[HP]", "[<]", 100), Behavior: asrahanTokens("[INIT DIVINE BEAST]")})
	fail := true
	bindings := map[string]AsrahanActionBinding{"[INIT DIVINE BEAST]": func(AsrahanView, catalog.AsrahanInstruction) (AsrahanActionResult, error) {
		if fail {
			return AsrahanActionResult{Symbols: []AsrahanSymbolChange{{"[FOREIGN]", 1}}}, nil
		}
		return AsrahanActionResult{}, nil
	}}
	s, now := newAsrahanKernel(t, r, bindings)
	if err := s.Start("run", now); err != nil {
		t.Fatal(err)
	}
	if err := s.BeginCombat("run", "fight", now, 1, 11, 21, 0); err != nil {
		t.Fatal(err)
	}
	s.DrainEffects()
	before := s.View()
	if err := s.VerifiedCombatDamage("run", "fight", now, 20); err == nil || !reflect.DeepEqual(before, s.View()) || len(s.DrainEffects()) != 0 {
		t.Fatal("partial damage/cascade committed", err)
	}
	fail = false
	if err := s.VerifiedCombatDamage("run", "fight", now, 20); err != nil || s.View().Symbols["[HP]"] != 80 {
		t.Fatal("rollback lost combat owner", err)
	}
}
