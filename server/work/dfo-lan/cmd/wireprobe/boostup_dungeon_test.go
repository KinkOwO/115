package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestBoostGuideActualGateAndSelectionUseExistingMapEngine(t *testing.T) {
	w := moonWorlds(t, 1)[0]
	w.boostup = &boostup.Catalog{Town: 222, Steps: []boostup.Step{{Number: 1}, {Number: 2, Area: 1, Guide: "dungeon", Dungeon: 100004558}}}
	w.professions = w.characters.Catalog
	var e error
	w.role.State, e = boostup.WriteState(w.role.State, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 2}})
	must115(t, e)
	w.state.Position = database.WorldPosition{Town: 222, Area: 1, X: 200, Y: 200}
	w.service.Catalog.Areas["222/1"] = catalog.WorldArea{Town: 222, Area: 1, Kind: "[gate]", Walkable: [][4]int32{{100, 100, 1000, 1000}}}
	source := w.dungeons.Dungeons[100004136]
	source.ID = 100004558
	source.Tutorial = true
	source.MinimumLevel = 115
	w.dungeons.Dungeons[source.ID] = source
	w.tutorialDungeons = w.dungeons // shared source indexing must NOT turn activity training into birth
	gate := make([]byte, 8)
	binary.LittleEndian.PutUint32(gate, source.ID)
	selectBody := make([]byte, 32)
	binary.LittleEndian.PutUint32(selectBody, source.ID)
	binary.LittleEndian.PutUint16(selectBody[9:], 65535)
	if _, _, e = w.selectDungeon(selectBody); e == nil {
		t.Fatal("training entered without gate")
	}
	plan, e := w.dungeonGate(gate)
	must115(t, e)
	if len(plan) != 2 || plan[0].ID != 15 || plan[1].ID != 27 {
		t.Fatal("guide gate not wired")
	}
	writePartyPackets(t, plan)
	if output := os.Getenv("US115_TEST_BOOST_GATE_VECTOR"); output != "" {
		must115(t, os.WriteFile(output, plan[1].Payload, 0600))
	}
	w.selectingDungeon = true
	run, plan, e := w.selectDungeon(selectBody)
	must115(t, e)
	if run.Definition.ID != source.ID || !run.Definition.Tutorial || w.inTutorial || countPacket(plan, 28) != 1 || countPacket(plan, 29) != 1 {
		t.Fatal("boost confused with birth/ordinary run")
	}
	if run.RunID == "" || len(run.Monsters) == 0 {
		t.Fatal("map engine not used")
	}
	binary.LittleEndian.PutUint32(gate, source.ID+1)
	if _, e = w.dungeonGate(gate); e == nil {
		t.Fatal("another guide accepted")
	}
}

func assertBoostGuideClearSQL(t *testing.T, ctx context.Context, store *database.TestFixture, role database.Character) {
	t.Helper()
	st, e := boostup.ReadState(role.State)
	must115(t, e)
	st.Training = boostup.Training{Step: 2, Phase: 0, Claimed: map[byte]bool{1: true}}
	raw, e := boostup.WriteState(role.State, st)
	must115(t, e)
	role, _, e = store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, "fixture:guide", "test", func(database.Character) (json.RawMessage, json.RawMessage, error) {
		return raw, json.RawMessage(`{}`), nil
	})
	must115(t, e)
	w := moonWorlds(t, 1)[0]
	w.role = role
	w.account = role.AccountID
	w.loot = &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: role.ConfigVersion}}}
	w.professions = catalog.Characters{Professions: map[byte]catalog.Profession{role.Profession: {Job: "[swordman]"}}}
	w.boostup = &boostup.Catalog{Town: 222, Steps: []boostup.Step{{Number: 1}, {Number: 2, Guide: "dungeon", Dungeon: 100004558}}}
	w.state.Position.Town = 222
	d := &dungeon.Session{RunID: strings.Repeat("c", 32), Definition: catalog.DungeonDefinition{ID: 100004558, Tutorial: true}, Room: catalog.DungeonRoom{Boss: true},
		Monsters: []protocol.DungeonMonster{{Entity: 4096, Template: 9, Rank: 3, Team: 100}}, Dead: map[uint16]bool{}}
	d.Definition.Script.Cells = []pvf.Token{{Type: 3, Text: "[dungeon mode script]"}, {Type: 6, Text: "BoostUp"}}
	d.Room.Map = 91
	d.Maze.Rooms = []catalog.DungeonRoom{d.Room}
	// 本树 dungeon.Session 用公开 Loaded 字段（donor 基线为 MarkLoaded 方法）。
	d.Loaded = true
	w.activeDungeon = d
	if _, e = w.completeBoostGuide(); e == nil {
		t.Fatal("uncleared guide completed")
	}
	deathPlan, e := w.monsterDeath(deathBody(4096, role.WireID), func(map[string]any) {})
	must115(t, e)
	if !d.Completed() || countPacket(deathPlan, 31) != 1 || countPacket(deathPlan, 115) != 0 {
		t.Fatal("real training death did not enable clear without C117", deathPlan)
	}
	plan, e := w.completeBoostGuide()
	must115(t, e)
	if len(plan) != 1 || plan[0].ID != 2638 || plan[0].Payload[1] != 2 || plan[0].Payload[2] != 1 {
		t.Fatal("real boss clear did not unlock guide reward")
	}
	_, e = w.completeBoostGuide()
	must115(t, e)
	var count int
	{
		v, e := store.EventCountByKey(ctx, role.ID, "boostup-guide-clear:2")
		must115(t, e)
		count = int(v)
	}
	if count != 1 {
		t.Fatal("duplicate clear effects", count)
	}
	w.resultSent = true
	w.completionSent = true
	plan, e = w.cardStage(69, nil)
	must115(t, e)
	if len(plan) != 1 {
		t.Fatal("training score ack")
	}
	w.cardScrolled = true
	plan, e = w.cardStage(70, nil)
	must115(t, e)
	if len(plan) != 1 || plan[0].ID != 70 {
		t.Fatal("training layout ack")
	}
	w.cardLayoutSent = true
	// donor 基线的 cardSkip 不在本树：本树教学 run 结算不生成奖单（settlement_flow
	// 的 isBoostGuideRun 门禁），因此任何翻牌交互都必须被 cardsReady 拒绝。
	if _, e = w.cardPick([]byte{0, 0, 0, 0}); e == nil {
		t.Fatal("training manufactured free-gold reward")
	}
	if w.cardPlan != nil {
		t.Fatal("training card plan created")
	}
}

// [GAP] donor 基线的 assertBoostMapPickupSQL 钉的是「服务端往房间预放物件」那一半
// （protocol.StartMapItem + NOTI29 物件行 + catalog.BoostMapItems +
// loot.Session.PrepareStartItems + loot.Service.OwnedItemCatalog），新树不存在这些符号，
// 且 NOTI29 已确认无物品记录位置；取证与候选方案见
// analysis/tasks/boostup662-mapobject-noti29-forensics-20261004.md。
