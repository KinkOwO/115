package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"encoding/binary"
	"testing"
	"time"
)

func venusSourceRewardFixture(t *testing.T) *worldSession {
	w := ispinsSQLiteRewardWorld(t)
	w.venus = &venusRun{choice: 1, stage: 2, entered: true, cleared: [4]bool{true, true, true, false}}
	w.activeDungeon.Definition.ID = legion.VenusStageDungeons[2]
	w.loot.RewardBoxes = apocalypseTestBoxes{77: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Count: 8, Weight: 1}}}}}, 78: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 22, Count: 3, Weight: 1}}}}}}
	w.dungeons.LegionContents.Contents["Venus Legion"] = catalog.LegionContent{Name: "Venus Legion", PhaseReward: 1, Dungeons: []catalog.LegionDungeon{{Dungeon: legion.VenusStageDungeons[2]}}, Rewards: []catalog.LegionRewardRow{{PartyType: "solo", Difficulty: "expert", Kind: "basic", Values: []int64{1, 77, 1}}}, NativeOperations: &catalog.NativeLegionOperations{Definitions: map[uint16]catalog.NativeLegionOperation{2: {Rewards: []catalog.NativeLegionReward{{Difficulty: "expert", Template: 78, Amount: 1, Weight: 1}}}}}}
	return w
}

func TestVenusSourceSettlementRollsBackPartialGrantAndRetries(t *testing.T) {
	w := venusSourceRewardFixture(t)
	w.loot.BagRules.Slots["[etc]"] = [2]uint16{68, 68}
	if packets, err := w.completeVenusStage(); err == nil || len(packets) > 0 {
		t.Fatal("partial settlement published", err)
	}
	bag, _ := inventory.ReadBag(w.role.State)
	if len(bag.Items) != 0 {
		t.Fatal("partial items persisted", bag)
	}
	w.loot.BagRules.Slots["[etc]"] = [2]uint16{68, 69}
	packets, err := w.completeVenusStage()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range packets {
		if p.ID == legion.NotiIspinsBasicClearReward && (binary.LittleEndian.Uint32(p.Payload[1600:]) != 21 || binary.LittleEndian.Uint32(p.Payload[1604:]) != 8) {
			t.Fatal("display does not match source")
		}
	}
	bag, _ = inventory.ReadBag(w.role.State)
	if len(bag.Items) != 2 {
		t.Fatal(bag)
	}
	before := bag.Items[0].Amount
	if _, err = w.completeVenusStage(); err != nil {
		t.Fatal(err)
	}
	bag, _ = inventory.ReadBag(w.role.State)
	if bag.Items[0].Amount != before {
		t.Fatal("duplicate Venus reward")
	}
}

func TestVenusNewRunClearsTerminalState(t *testing.T) {
	r := venusRun{finalDone: true, storyFinished: true, potionUsed: [4]int{8, 8, 8, 8}}
	r.resetRun()
	if r.finalDone || r.storyFinished || r.potionUsed != [4]int{} {
		t.Fatal("new run inherited final state", r)
	}
}

func TestVenusFourthPhaseStopPaysExpertWithoutKillingFourthBoss(t *testing.T) {
	w := venusSourceRewardFixture(t)
	w.venus.choice, w.venus.stage = 2, 3
	w.venus.clearElapsed[2] = 37 * time.Second
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[3]}, RunID: "fourth-attempt", StartedAt: time.Now().Add(-time.Second)}
	packets, _, err := w.venusEndAtPhase4(nil)
	if err != nil {
		t.Fatal(err)
	}
	if w.activeDungeon.Completed() || w.venus.cleared[3] || w.venus.choice != 1 || w.venus.stage != 2 || !w.venus.rewardSent {
		t.Fatal("downshift forged a fourth clear", w.venus)
	}
	if len(packets) < 4 || packets[0].ID != legion.CmdVenusEndAtPhase4 {
		t.Fatal(packets)
	}
	seen := false
	for _, p := range packets {
		if p.Name == "venus_basic_clear_reward" && binary.LittleEndian.Uint64(p.Payload[7760:]) != 37000 {
			t.Fatal("downshift used the fourth fight's clock")
		}
		if p.Name == "venus_phase4_expert_result" {
			seen = p.Payload[2] == 1 && binary.LittleEndian.Uint32(p.Payload[11:]) == 2
		}
	}
	if !seen {
		t.Fatal("expert endpoint projection missing")
	}
	again, _, err := w.venusEndAtPhase4(nil)
	if err != nil || len(again) != 1 {
		t.Fatal("stop replay was not idempotent", again, err)
	}
}

func TestVenusFourthPhaseStopRequiresThreeOwnedClears(t *testing.T) {
	for stage := 0; stage < 3; stage++ {
		w := venusSourceRewardFixture(t)
		w.venus.choice, w.venus.stage = 2, 3
		w.venus.cleared[stage] = false
		if p, _, err := w.venusEndAtPhase4(nil); err == nil || len(p) > 0 || w.venus.downshift {
			t.Fatal("unearned downgrade", stage, err)
		}
	}
}

func TestVenusFourthPhaseStopFullBagRetainsPendingExpertReward(t *testing.T) {
	w := venusSourceRewardFixture(t)
	w.venus.choice, w.venus.stage = 2, 3
	w.venus.clearElapsed[2] = 20 * time.Second
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[3]}, RunID: "pending-fourth", StartedAt: time.Now().Add(-time.Second)}
	w.loot.BagRules.Slots["[etc]"] = [2]uint16{68, 68}
	if p, _, err := w.venusEndAtPhase4(make([]byte, 8)); err == nil || len(p) > 0 || !w.venus.downshift || w.venus.rewardSent {
		t.Fatal("pending stop was lost", err)
	}
	w.loot.BagRules.Slots["[etc]"] = [2]uint16{68, 69}
	if _, _, err := w.venusEndAtPhase4(make([]byte, 8)); err != nil || !w.venus.rewardSent {
		t.Fatal("pending stop did not retry", err)
	}
}

func TestVenusTerminalEndCannotBypassSourceSettlement(t *testing.T) {
	w := venusSourceRewardFixture(t)
	w.venus.cleared[2] = false
	p := make([]byte, 24)
	p[13], p[17] = 106, 2
	if packets, _, err := w.venusRewardEnd(p); err == nil || len(packets) > 0 || w.venus.finalDone {
		t.Fatal("unpaid terminal end accepted", err)
	}
}
