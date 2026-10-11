package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

func TestAsrahanProgramKeepsUnknownActionOutOfExecution(t *testing.T) {
	r := &AsrahanRaidRules{Symbols: map[string]uint32{"[VALUE]": 1}}
	p := AsrahanRaidPhase{Events: []AsrahanSourceEvent{{Trigger: []pvf.Token{{Type: 3, Text: "[IF]"}, {Type: 3, Text: "[VALUE]"}, {Type: 0}}, Behavior: []pvf.Token{{Type: 3, Text: "[SET]"}, {Type: 3, Text: "[VALUE]"}, {Type: 0, Value: 1}, {Type: 3, Text: "[FUTURE ACTION]"}}}}}
	if _, err := CompileAsrahanPhase(r, p); err == nil {
		t.Fatal("dropped unsupported action from mixed event")
	}
	if p.Events[0].Behavior[len(p.Events[0].Behavior)-1].Text != "[FUTURE ACTION]" {
		t.Fatal("compiler changed source")
	}
}
func TestAsrahanCompleteNativeProgram(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF required")
	}
	a, err := OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := importAsrahanRaidRules(a, "contents/2024/asrahanraid/etc/asrahan.etc")
	if err != nil {
		t.Fatal(err)
	}
	behavior, trigger := map[string]bool{}, map[string]bool{}
	for mode, phase := range map[string]AsrahanRaidPhase{"normal": r.Normal, "hard": r.Hard} {
		groups, err := CompileAsrahanCombatGroups(r, phase)
		if err != nil || len(groups) != len(r.Groups) {
			t.Fatal("incomplete native combat group binding", len(groups), err)
		}
		program, err := CompileAsrahanPhase(r, phase)
		if err != nil {
			t.Fatal(err)
		}
		if len(program) != len(phase.Events) {
			t.Fatal("native event discarded")
		}
		for _, event := range program {
			for _, action := range event.Behavior {
				behavior[action.Op] = true
			}
			for _, action := range event.Trigger {
				trigger[action.Op] = true
			}
		}
		t.Logf("%s compiled complete source events=%d", mode, len(program))
	}
	if len(behavior) != 48 || len(trigger) != 3 {
		t.Fatalf("native grammar coverage=%d/%d", len(behavior), len(trigger))
	}
	t.Logf("native_client_started=false; source_checksum=%s; behavior_types=%d trigger_types=%d", a.Snapshot().Checksum, len(behavior), len(trigger))
}
