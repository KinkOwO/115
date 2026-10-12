package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"reflect"
	"strings"
	"testing"
	"time"
)

type apocalypseTestBoxes map[uint32]loot.RewardBox

func (b apocalypseTestBoxes) RewardBox(id uint32) (loot.RewardBox, bool) {
	v, ok := b[id]
	return v, ok
}
func (b apocalypseTestBoxes) Item(id uint32) bool      { return id == 21 || id == 22 || id == 77 }
func (b apocalypseTestBoxes) Container(id uint32) bool { _, ok := b[id]; return ok }

func prepareApocalypseRewardFixture(t *testing.T, w *worldSession) {
	t.Helper()
	f := ispinsSQLiteRewardWorld(t)
	w.store, w.account, w.role, w.loot, w.dungeons = f.store, f.account, f.role, f.loot, f.dungeons
	w.loot.RewardBoxes = apocalypseTestBoxes{77: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 21, Count: 8, Weight: 1}}}}}}
	op := w.legion.catalog.Operation(int64(w.apocalypse.Choice) + 1)
	if op == nil {
		t.Fatal("fixture operation missing")
	}
	op.Reward.Values = []float64{22, 3, 1000000, float64(*op.Type), 1}
	w.dungeons.LegionContents.SHA256 = strings.Repeat("b", 64)
	w.dungeons.LegionContents.Contents["Apocalypse"] = catalog.LegionContent{Name: "Apocalypse", Rewards: []catalog.LegionRewardRow{{PartyType: "solo", Difficulty: op.Reward.Label, Kind: "basic", Values: []int64{1000000, 77, 1}}}}
	stage := legion.ApocalypseEndpoint(w.apocalypse.Choice)
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.ApocalypseStageDungeons[stage]}, RunID: "terminal-room"}
	w.activeDungeon.MarkSceneCompleted()
	w.apocalypse.Cleared = stage
	w.apocalypse.RunStartedAt = time.Now().Add(-time.Minute)
}

func TestApocalypseSourceRewardOwnsClearAndCommitsSourceProducts(t *testing.T) {
	w := apocalypseRewardSession(t, 0, 4)
	prepareApocalypseRewardFixture(t, w)
	if _, err := w.apocalypseSourceRewards(); err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil || len(bag.Items) != 2 {
		t.Fatal(bag, err)
	}
	amounts := map[uint32]uint32{}
	for _, item := range bag.Items {
		amounts[item.Template] += item.Amount
	}
	if amounts[21] != 8 || amounts[22] != 3 {
		t.Fatal(amounts)
	}
	if _, err := w.apocalypseSourceRewards(); err != nil {
		t.Fatal(err)
	}
	again, _ := inventory.ReadBag(w.role.State)
	if again.Items[0].Amount != bag.Items[0].Amount || again.Items[1].Amount != bag.Items[1].Amount {
		t.Fatal("replay paid twice")
	}
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: legion.ApocalypseStageDungeons[3]}}
	if _, err := w.apocalypseSourceRewards(); err == nil {
		t.Fatal("unfinished combat awarded")
	}
}

func TestApocalypseCardAcknowledgementDoesNotGrantSourceItemsAgain(t *testing.T) {
	w := apocalypseRewardSession(t, 0, 4)
	prepareApocalypseRewardFixture(t, w)
	if _, err := w.apocalypseTerminalReward(legion.CmdRewardEnd, "test"); err != nil {
		t.Fatal(err)
	}
	before, _ := inventory.ReadBag(w.role.State)
	for _, index := range []byte{2, 2, 1} {
		if _, err := w.grantFreeCard(index); err != nil {
			t.Fatal(err)
		}
		after, _ := inventory.ReadBag(w.role.State)
		if !reflect.DeepEqual(before, after) || w.cardReceipt.Index != 2 {
			t.Fatal("presentation paid twice or changed committed pick", after)
		}
	}
}
