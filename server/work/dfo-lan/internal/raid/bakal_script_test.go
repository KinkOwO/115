package raid

import (
	"dfolan/internal/catalog"
	"os"
	"testing"
	"time"
)

func nativeBakalScriptFixture(t *testing.T) (catalog.RaidEntrance, *BakalOpening, time.Time) {
	t.Helper()
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF required")
	}
	a, err := catalog.OpenTestArchiveCached(path, "")
	if err != nil {
		t.Fatal(err)
	}
	d, err := catalog.ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := catalog.ImportRaidEntrances(a, &d)
	if err != nil {
		t.Fatal(err)
	}
	entry := entries[82]
	if len(entry.Bakal.Events) != 79 {
		t.Fatalf("source event coverage changed: %d", len(entry.Bakal.Events))
	}
	if len(entry.Bakal.BuffDefinitions) != 25 {
		t.Fatal("source buffs missing")
	}
	now := time.Unix(1791080000, 0)
	s, err := PrepareBakalOpening(entry, []Member{{Actor: 9, Position: 1}}, now.Add(-3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Activate(now); err != nil {
		t.Fatal(err)
	}
	return entry, s, now
}

func TestNativeBakalScriptTimersPresenceAndRevival(t *testing.T) {
	entry, s, now := nativeBakalScriptFixture(t)
	if s.RaidBuffCounts() != [5]byte{2, 2, 2, 2, 2} {
		t.Fatal("native opening inventory")
	}
	if _, err := s.Tick(now.Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"swan", "steich", "eclair"} {
		found := false
		for _, m := range s.monsters {
			found = found || m.Kind == kind
		}
		if !found {
			t.Fatalf("reserved %s never created", kind)
		}
	}
	if _, err := s.EnterDungeon(100003157, now.Add(11*time.Second), true); err != nil {
		t.Fatal(err)
	}
	if s.variables["[HATCHERY FIRST ENTER]"] != 1 {
		t.Fatal("source hatchery trigger was omitted")
	}
	if _, err := s.Tick(now.Add(11 * time.Second)); err != nil {
		t.Fatal(err)
	}
	awake := 0
	for _, name := range []string{"[SPARAZZI AWAKEN]", "[SKASA AWAKEN]", "[HISMA AWAKEN]"} {
		if s.variables[name] == 1 {
			awake++
		}
	}
	if awake != 1 {
		t.Fatal("first random dragon was not awakened")
	}
	if err := s.DefeatMonster(23, entry.Bakal.MonsterDefinitions["basilisk"].ID, now.Add(12*time.Second)); err != nil {
		t.Fatal(err)
	}
	if s.variables["[IS EXIST BASILISK]"] != 0 {
		t.Fatal("dead gatekeeper still blocks native actors")
	}
	if err := s.DefeatMonster(23, entry.Bakal.MonsterDefinitions["basilisk"].ID, now.Add(12*time.Second)); err == nil {
		t.Fatal("replayed gatekeeper death granted again")
	}
	// Stop both reinforcement generators and enter Bakal after the global
	// anger timer. Otherwise the source failure trigger legitimately ends the
	// phase before the 870-second gatekeeper revival can occur.
	for _, slot := range []uint32{7, 47} {
		loc := entry.Bakal.Locations[slot]
		if _, err := s.EnterDungeon(loc.Dungeon, now.Add(13*time.Second), true); err != nil {
			t.Fatal(err)
		}
		m, _ := s.InitialMonster(slot)
		if err := s.DefeatMonster(slot, m.ID, now.Add(13*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Tick(now.Add(121 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EnterDungeon(100003149, now.Add(121*time.Second), true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Tick(now.Add(883 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if s.variables["[IS EXIST BASILISK]"] != 1 {
		t.Fatalf("source 870s revival did not happen: ended=%s anger=%d clock=%s scheduled=%+v", s.Ended(), s.variables["[BAKAL ANGER]"], s.clock, s.scheduled)
	}
}

func TestNativeBakalScriptThreeDragonsLocksFinalAndDuplicateDeaths(t *testing.T) {
	entry, s, now := nativeBakalScriptFixture(t)
	if _, err := s.EnterDungeon(100003149, now, true); err != nil {
		t.Fatal(err)
	}
	if err := s.ReportHealth(24, 1, now); err != nil {
		t.Fatal(err)
	}
	if s.variables["[BAKAL HP]"] != 5500 {
		t.Fatal("source grade3 lock not enforced")
	}
	if s.variables["[BAKAL PHASE]"] != 0 {
		t.Fatal("source locked grade allowed phase shift")
	}
	if err := s.ReportHealth(24, 1, now); err != nil {
		t.Fatal("repeated locked damage overflowed", err)
	}
	for i, slot := range []uint32{12, 26, 40} {
		loc := entry.Bakal.Locations[slot]
		m := s.monsters[slot]
		at := now.Add(time.Duration(i+1) * time.Second)
		if _, err := s.EnterDungeon(loc.Dungeon, at, true); err != nil {
			t.Fatal(err)
		}
		if err := s.ReportHealth(slot, 5000, at); err != nil {
			t.Fatal(err)
		}
		if err := s.DefeatMonster(slot, m.ID, at); err != nil {
			t.Fatal(err)
		}
		if s.variables["[BAKAL HP UNLOCK GRADE]"] != int32(2-i) {
			t.Fatal("dragon clear did not unlock next source HP grade")
		}
		if err := s.DefeatMonster(slot, m.ID, at); err == nil {
			t.Fatal("duplicate dragon clear accepted")
		}
	}
	if _, err := s.EnterDungeon(100003149, now.Add(5*time.Second), true); err != nil {
		t.Fatal(err)
	}
	m := s.monsters[24]
	if err := s.ReportHealth(24, 5000, now.Add(6*time.Second)); err != nil {
		t.Fatal(err)
	}
	if s.variables["[BAKAL PHASE]"] != 1 {
		t.Fatal("source half-health transition still waits for death")
	}
	if err := s.DefeatMonster(24, m.ID, now.Add(6*time.Second)); err != nil {
		t.Fatal(err)
	}
	actor, exists := s.InitialMonster(24)
	if !exists || actor.ID != m.SecondID || actor.Grid != m.SecondGrid {
		t.Fatal("source second actor/grid not selected")
	}
	if err := s.DefeatMonster(24, m.SecondID, now.Add(7*time.Second)); err != nil {
		t.Fatal(err)
	}
	final := s.FinalDungeon()
	if final != 100003165 {
		t.Fatalf("source final transition missing: %d", final)
	}
	found := false
	for _, e := range s.DrainEffects() {
		found = found || e.Op == "last" && e.ID == final
	}
	if !found {
		t.Fatal("victory did not issue native final routing")
	}
	if _, err := s.EnterDungeon(final, now.Add(8*time.Second), true); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearFinalDungeon(final, now.Add(9*time.Second)); err != nil {
		t.Fatal(err)
	}
	if s.Ended() != "clear" {
		t.Fatal("final clear failed to complete source phase")
	}
}

func TestNativeBakalScriptBuffReportsCannotWriteForeignState(t *testing.T) {
	_, s, now := nativeBakalScriptFixture(t)
	if _, err := s.EnterDungeon(100003150, now, true); err != nil {
		t.Fatal(err)
	}
	if err := s.ReportHealth(40, 5000, now); err == nil {
		t.Fatal("health accepted for foreign dragon")
	}
	if err := s.ReportSymbol(231, 0, now); err == nil {
		t.Fatal("client unlocked Bakal directly")
	}
	if err := s.UseRaidBuff(22, now); err != nil {
		t.Fatal(err)
	}
	if s.RaidBuffCounts()[2] != 1 {
		t.Fatal("buff inventory not consumed")
	}
	if err := s.UseRaidBuff(22, now); err == nil {
		t.Fatal("buff cooldown bypass")
	}
	if err := s.UseRaidBuff(3, now); err == nil {
		t.Fatal("party buff was treated as commander inventory")
	}
}
