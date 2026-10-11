package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

func TestBakalModeSourceIsolation(t *testing.T) {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	ts := []pvf.Token{tag("[PHASE MAX]"), {Type: 0, Value: 1}, tag("[PHASE]"), {Type: 0, Value: 11}, tag("[/PHASE]"), tag("[HARD RAID PARTY MAX]"), {Type: 0, Value: 3}, tag("[RAID PHASE OF HARDMODE]"), {Type: 0, Value: 22}, tag("[/RAID PHASE OF HARDMODE]")}
	for mode, want := range map[BakalMode]int32{BakalNormal: 11, BakalHard: 22} {
		got, err := bakalPhaseTokens(ts, mode)
		if err != nil || len(got) != 5 || got[3].Value != want {
			t.Fatalf("mode %d: %v %v", mode, got, err)
		}
	}
	if _, err := bakalPhaseTokens(ts[:len(ts)-1], BakalHard); err == nil {
		t.Fatal("unclosed hard phase accepted")
	}
	if _, err := bakalPhaseTokens(ts[:5], BakalHard); err == nil {
		t.Fatal("normal substituted for missing hard phase")
	}
	if _, err := bakalPhaseTokens(ts, BakalMode(2)); err == nil {
		t.Fatal("unknown mode accepted")
	}
}

func TestBakalHardCurrentPVF(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("current PVF required")
	}
	a, err := OpenTestArchiveCached(p, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := ImportRaidEntrances(a, &d)
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[82]
	r := entry.BakalHard
	if r == nil || r.Mode != BakalHard || len(r.Events) == 0 || len(r.Monsters) == 0 || len(r.Rewards) == 0 {
		t.Fatalf("hard source incomplete: %+v", r)
	}
	if entry.Bakal.Mode != BakalNormal || r == entry.Bakal {
		t.Fatal("mode source alias")
	}
	for _, ch := range []uint32{82, 83} {
		attr, _ := d.Attributes(ch)
		t.Logf("channel %d: %+v", ch, attr)
	}
	t.Logf("hard phase dungeons=%d events=%d monsters=%d reserves=%d timers=%d variables=%v rewards=%v weekly bids=%v", len(r.Dungeons), len(r.Events), len(r.Monsters), len(r.ReservedMonsters), len(r.Timers), r.InitialVariables, r.Rewards, r.Bidding.WeeklyCount)
}
