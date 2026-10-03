package quest

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestTournamentSceneTriggerDoesNotWriteQuestProgress(t *testing.T) {
	q := sceneQuest()
	service := &Service{Catalog: catalog.QuestCatalog{Quests: map[uint32]catalog.QuestDefinition{q.ID: q}}}
	run := &dungeon.Session{Loaded: true, Room: catalog.DungeonRoom{Map: 100008683}, Tournament: &dungeon.TournamentRun{CurrentRound: 2}}
	// A nil Store ensures the first-round trigger returns before any write
	// or quest refresh. It must not consume the quest during the next fight.
	active, err := service.SceneTrigger(context.Background(), character.Character{}, run, uint16(q.ID))
	if err != nil || active != nil {
		t.Fatalf("unfinished tournament trigger: active=%v err=%v", active, err)
	}
	run.MarkSceneCompleted()
	if run.Completed() {
		t.Fatal("scene completion bypassed tournament combat")
	}
}

func TestTournamentSceneTriggerWaitsForFourRoundsAndBossCheck(t *testing.T) {
	a := catalog.OpenNativeArchive(t)
	c := catalog.LoadNativeFullDungeons(t)
	overlay, err := catalog.ImportTournamentQuestMaps(a, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.ApplyTournamentQuestMaps(&c, overlay); err != nil {
		t.Fatal(err)
	}
	quests, err := catalog.ImportQuests(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		dungeon uint32
		quest   uint16
		arena   int32
	}{{100003298, 13784, 100008699}, {100003299, 13785, 100008700}} {
		run, err := dungeon.Select(c, protocol.DungeonSelection{ID: tc.dungeon, Difficulty: 3, Party: 65535, Quest: uint32(tc.quest)}, 115, map[uint16]bool{tc.quest: true})
		if err != nil {
			t.Fatal(err)
		}
		run.Loaded = true
		q := quests.Quests[uint32(tc.quest)]
		if q.Kind != "[clear map]" || len(q.ObjectiveCells) != 1 || q.ObjectiveCells[0].Value != tc.arena {
			t.Fatalf("quest %d source objective changed: %+v", tc.quest, q)
		}
		for round, opponent := range run.Tournament.Opening.Path {
			if changed, err := run.ConfirmDeath(uint32(opponent.Entity), 10, 10); err != nil || !changed {
				t.Fatalf("quest %d round %d death: changed=%t err=%v", tc.quest, round+1, changed, err)
			}
			if _, applies, err := SceneClearObjective(q, run); applies || err != nil {
				t.Fatalf("quest %d round %d CMD33 bypassed completion: applies=%t err=%v", tc.quest, round+1, applies, err)
			}
			run.MarkSceneCompleted()
			if run.Completed() || run.Tournament.CurrentRound != byte(round+2) {
				t.Fatalf("quest %d round %d scene trigger ended tournament", tc.quest, round+1)
			}
		}
		final := run.Tournament.Opening.Path[3].Entity
		if err := run.BossCheck(protocol.BossCheckRequest{Actor: 10, Target: final}, 10); err != nil || !run.Completed() {
			t.Fatalf("quest %d final boss check: completed=%t err=%v", tc.quest, run.Completed(), err)
		}
		if objective, applies, err := SceneClearObjective(q, run); err != nil || !applies || objective != uint32(tc.arena) {
			t.Fatalf("quest %d completed objective: %d %t %v", tc.quest, objective, applies, err)
		}
	}
}

func sceneQuest() catalog.QuestDefinition {
	return catalog.QuestDefinition{ID: 3191, Kind: "[clear map]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 100008683}}}
}

func TestSceneClearObjectiveAppliesOnFinalLayer(t *testing.T) {
	run := &dungeon.Session{Loaded: true, Room: catalog.DungeonRoom{Map: 100008683, Boss: true}}
	objective, applies, err := SceneClearObjective(sceneQuest(), run)
	if err != nil || !applies || objective != 100008683 {
		t.Fatalf("final layer trigger rejected: %d %t %v", objective, applies, err)
	}
}

func TestSceneClearObjectiveSilentForOtherKinds(t *testing.T) {
	run := &dungeon.Session{Loaded: true, Room: catalog.DungeonRoom{Map: 100008683}}
	for _, d := range []catalog.QuestDefinition{
		{ID: 1, Kind: "[meet npc]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 12}}},
		{ID: 2, Kind: "[clear map]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 100008683}, {Type: 0, Value: 100008684}}},
		{ID: 3, Kind: "[clear map]", Pending: []string{"unresolved"}, ObjectiveCells: []pvf.Token{{Type: 0, Value: 100008683}}},
		{ID: 4, Kind: "[look cinematic]", ObjectiveCells: []pvf.Token{{Type: 0, Value: 14898}}},
	} {
		if _, applies, err := SceneClearObjective(d, run); applies || err != nil {
			t.Fatalf("quest %d must stay on the silent path: %t %v", d.ID, applies, err)
		}
	}
}

func TestSceneClearObjectiveRequiresOwnedLoadedRunOnObjectiveMap(t *testing.T) {
	d := sceneQuest()
	if _, _, err := SceneClearObjective(d, nil); err == nil {
		t.Fatal("trigger without a run accepted")
	}
	if _, _, err := SceneClearObjective(d, &dungeon.Session{Room: catalog.DungeonRoom{Map: 100008683}}); err == nil {
		t.Fatal("trigger on an unloaded run accepted")
	}
	run := &dungeon.Session{Loaded: true, Room: catalog.DungeonRoom{Map: 100008684}}
	if _, _, err := SceneClearObjective(d, run); err == nil {
		t.Fatal("trigger from an earlier layer map accepted")
	}
}
