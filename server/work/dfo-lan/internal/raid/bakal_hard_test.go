package raid

import (
	"dfolan/internal/catalog"
	"os"
	"testing"
	"time"
)

func TestNativeHardBakalSourceFullPhase(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("current PVF required")
	}
	a, err := catalog.OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := catalog.ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := catalog.ImportRaidEntrances(a, &dir)
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[82]
	entry.Bakal = entry.BakalHard
	entry.PartyMax = entry.HardPartyMax
	r := entry.Bakal
	now := time.Now()
	s, err := PrepareBakalOpening(entry, []Member{{Actor: 9, Position: 1}}, now.Add(-time.Duration(r.StartDelay)*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Activate(now); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Tick(now.Add(10 * time.Second)); err != nil {
		t.Fatal("hard timer execution", err)
	}
	for _, kind := range []string{"skasa", "sparazzi", "hisma"} {
		found := false
		for slot, m := range s.Monsters() {
			if m.Kind != kind {
				continue
			}
			found = true
			loc := r.Locations[slot]
			if _, err = s.EnterDungeon(loc.Dungeon, now.Add(11*time.Second), true); err != nil {
				t.Fatal(kind, err)
			}
			if err = s.DefeatMonster(slot, m.ID, now.Add(12*time.Second)); err != nil {
				t.Fatal(kind, err)
			}
		}
		if !found {
			t.Fatal("hard source dragon missing", kind)
		}
	}
	var bossSlot uint32
	for slot, m := range s.Monsters() {
		if m.Kind == "bakal" {
			bossSlot = slot
		}
	}
	if bossSlot == 0 {
		t.Fatal("hard source Bakal missing")
	}
	m, _ := s.InitialMonster(bossSlot)
	if _, err = s.EnterDungeon(r.Locations[bossSlot].Dungeon, now.Add(13*time.Second), true); err != nil {
		t.Fatal(err)
	}
	hp := r.InitialVariables["[MONSTER MAX HP]"] * r.Stage.HealthPercent / 100
	if err = s.ReportHealth(bossSlot, hp, now.Add(14*time.Second)); err != nil {
		t.Fatal(err)
	}
	if s.SymbolValues()[r.Symbols["[BAKAL PHASE]"]] != 1 {
		t.Fatal("hard half-health phase did not execute")
	}
	if err = s.DefeatMonster(bossSlot, m.ID, now.Add(15*time.Second)); err != nil {
		t.Fatal(err)
	}
	second, ok := s.InitialMonster(bossSlot)
	if !ok || second.ID != m.SecondID || second.Grid != m.SecondGrid {
		t.Fatal("hard second actor missing")
	}
	if err = s.DefeatMonster(bossSlot, second.ID, now.Add(16*time.Second)); err != nil {
		t.Fatal(err)
	}
	final := s.FinalDungeon()
	if final == 0 {
		t.Fatal("hard ending source not selected")
	}
	if _, err = s.EnterDungeon(final, now.Add(17*time.Second), true); err != nil {
		t.Fatal(err)
	}
	if err = s.ClearFinalDungeon(final, now.Add(18*time.Second)); err != nil || s.Ended() != "clear" {
		t.Fatal("hard clear", err)
	}
	run, clears, ready := s.SettlementIdentity()
	if !ready || run == "" || clears < r.MinimumClearCount {
		t.Fatal("hard eligible owned settlement missing", clears)
	}
	t.Logf("hard77-event source: three dragons, half-health, second actor, ending, clear and%d confirmed dungeons", clears)
}
