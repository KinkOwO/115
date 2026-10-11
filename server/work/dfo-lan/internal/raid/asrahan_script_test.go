package raid

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"reflect"
	"testing"
	"time"
)

func asrahanTokens(values ...any) []pvf.Token {
	var out []pvf.Token
	for _, value := range values {
		switch v := value.(type) {
		case int:
			out = append(out, pvf.Token{Type: 0, Value: int32(v)})
		case string:
			kind := uint8(3)
			if v == "hide" || v == "open" {
				kind = 6
			}
			out = append(out, pvf.Token{Type: kind, Text: v})
		}
	}
	return out
}
func asrahanKernelFixture() *catalog.AsrahanRaidRules {
	phase := catalog.AsrahanRaidPhase{TimeoutSeconds: 60, CoinLimit: 2, Dungeons: map[uint32]catalog.AsrahanDungeon{11: {InitialState: "hide"}}}
	phase.Events = []catalog.AsrahanSourceEvent{
		{Trigger: asrahanTokens("[IF]", "[INIT]", 0), Behavior: asrahanTokens("[SET]", "[MAX]", 100, "[SET]", "[HP]", 100, "[SET]", "[INIT]", 1)},
		{Trigger: asrahanTokens("[ON CHANGE SYMBOL]", "[DAMAGE1]"), Behavior: asrahanTokens("[SUM]", "[TOTAL]", "[DAMAGE1]", "[DAMAGE3]", "[SUB MAX]", "[MAX]", "[HP]", "[DAMAGE1]", "[DAMAGE3]")},
	}
	return &catalog.AsrahanRaidRules{Path: "fixture.etc", SHA256: "owned-source", Symbols: map[string]uint32{"[MAX]": 1, "[HP]": 2, "[DAMAGE1]": 3, "[DAMAGE2]": 4, "[DAMAGE3]": 5, "[INIT]": 6, "[TOTAL]": 7, "[COUNT]": 8, "[ONE]": 9}, Normal: phase, Hard: phase}
}
func newAsrahanKernel(t *testing.T, r *catalog.AsrahanRaidRules, bindings map[string]AsrahanActionBinding) (*AsrahanScript, time.Time) {
	t.Helper()
	now := time.Unix(1000, 0)
	s, err := NewAsrahanScript(r, false, "run", now, bindings)
	if err != nil {
		t.Fatal(err)
	}
	return s, now
}
func TestAsrahanOwnedSourceDamageRange(t *testing.T) {
	s, now := newAsrahanKernel(t, asrahanKernelFixture(), nil)
	if err := s.Start("run", now); err != nil {
		t.Fatal(err)
	}
	if err := s.OwnedSymbols("run", now, []AsrahanSymbolChange{{"[DAMAGE1]", 20}, {"[DAMAGE2]", 25}, {"[DAMAGE3]", 30}}); err != nil {
		t.Fatal(err)
	}
	view := s.View()
	if view.Symbols["[HP]"] != 25 || view.Symbols["[TOTAL]"] != 75 {
		t.Fatalf("middle source party damage omitted: %+v", view.Symbols)
	}
	view.Symbols["[HP]"] = 99
	if s.View().Symbols["[HP]"] != 25 {
		t.Fatal("external view modified owned symbols")
	}
	before := s.View()
	if err := s.OwnedSymbols("stale", now, []AsrahanSymbolChange{{"[DAMAGE1]", 90}}); err == nil || !reflect.DeepEqual(before, s.View()) {
		t.Fatal("foreign run updated source")
	}
	if err := s.OwnedSymbols("run", now, []AsrahanSymbolChange{{"[DAMAGE1]", 90}, {"[FOREIGN]", 1}}); err == nil || !reflect.DeepEqual(before, s.View()) {
		t.Fatal("partially committed invalid source report")
	}
	if err := s.Tick("run", now.Add(time.Minute)); err == nil {
		t.Fatal("expired source run accepted a tick")
	}
}
func TestAsrahanRequiresWholeActionBindingAndRollsBack(t *testing.T) {
	r := asrahanKernelFixture()
	r.Normal.Events[0].Behavior = append(r.Normal.Events[0].Behavior, asrahanTokens("[INIT DIVINE BEAST]")...)
	if _, err := NewAsrahanScript(r, false, "run", time.Unix(1000, 0), nil); err == nil {
		t.Fatal("started raid with omitted source action")
	}
	fail := true
	bindings := map[string]AsrahanActionBinding{"[INIT DIVINE BEAST]": func(view AsrahanView, action catalog.AsrahanInstruction) (AsrahanActionResult, error) {
		view.Symbols["[HP]"] = 999
		if fail {
			return AsrahanActionResult{Symbols: []AsrahanSymbolChange{{"[FOREIGN]", 1}}}, nil
		}
		return AsrahanActionResult{Symbols: []AsrahanSymbolChange{{"[HP]", 75}}}, nil
	}}
	s, now := newAsrahanKernel(t, r, bindings)
	before := s.View()
	if err := s.Start("run", now); err == nil || !reflect.DeepEqual(before, s.View()) || len(s.DrainEffects()) != 0 {
		t.Fatal("failed full source initialization published partial state/effects")
	}
	fail = false
	if err := s.Start("run", now); err != nil {
		t.Fatal(err)
	}
	if s.View().Symbols["[HP]"] != 75 {
		t.Fatal("binding result lost or external snapshot leaked")
	}
	if err := s.Start("run", now); err == nil {
		t.Fatal("duplicate initialization accepted")
	}
}
func TestAsrahanSourceTimerRearmInvalidatesOldDueEvent(t *testing.T) {
	r := asrahanKernelFixture()
	r.Normal.Events[0].Behavior = append(r.Normal.Events[0].Behavior, asrahanTokens("[SET]", "[ONE]", 1, "[SET TIMER]", 1, 11, 1, "[SET TIMER]", 2, 11, 1)...)
	r.Normal.Events = append(r.Normal.Events,
		catalog.AsrahanSourceEvent{Trigger: asrahanTokens("[CHECK TIMER END]", 1, 11), Behavior: asrahanTokens("[RESET TIMER]", 2, 11, "[SET TIMER]", 2, 11, 10, "[SET DUNGEON STATE]", 11, "open")},
		catalog.AsrahanSourceEvent{Trigger: asrahanTokens("[CHECK TIMER END]", 2, 11), Behavior: asrahanTokens("[SET]", "[COUNT]", "[+]", "[ONE]")})
	s, now := newAsrahanKernel(t, r, nil)
	if err := s.Start("run", now); err != nil {
		t.Fatal(err)
	}
	if err := s.Tick("run", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if s.View().Symbols["[COUNT]"] != 0 || s.View().Dungeons[11] != "open" {
		t.Fatal("stale timer fired after source rearm")
	}
	if err := s.Tick("run", now.Add(11*time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := s.Tick("run", now.Add(11*time.Second)); err != nil {
		t.Fatal(err)
	}
	if s.View().Symbols["[COUNT]"] != 1 {
		t.Fatal("source timer replayed")
	}
}
