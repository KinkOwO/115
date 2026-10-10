package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"testing"
)

func TestBossCompletionPreflightAndReplay(t *testing.T) {
	run := &dungeon.Session{Loaded: true, Room: catalog.DungeonRoom{Boss: true}, Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3}}, Dead: map[uint16]bool{}}
	w := &worldSession{role: database.Character{WireID: 3}, activeDungeon: run}
	p := make([]byte, 16)
	binary.LittleEndian.PutUint16(p, 3)
	binary.LittleEndian.PutUint16(p[2:], 4096)
	plan, e := w.bossCheck(p, nil)
	if e != nil || len(plan) != 0 {
		t.Fatal("early boss check completed", e)
	}
	if _, e = run.ConfirmDeath(4096, 3, 3); e != nil {
		t.Fatal(e)
	}
	plan, e = w.completeDungeon()
	if e != nil || len(plan) != 2 || plan[0].Kind != 0 || plan[0].ID != 115 || plan[1].ID != 31 {
		t.Fatal("wrong completion family/order", e)
	}
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	ready, e := preparePackets(keys, plan)
	if e != nil {
		t.Fatal(e)
	}
	for _, p := range ready {
		plain, e := wire.DecryptPayload(keys, p.ID, p.Raw[16:])
		if e != nil || !bytes.Equal(plain[:len(p.Payload)], p.Payload) {
			t.Fatal("boss frame mismatch", e)
		}
	}
	w.completionSent = true
	if plan, e = w.bossCheck(p, nil); e != nil || len(plan) != 0 {
		t.Fatal("completion replay reopened results", e)
	}
}

func TestOculusClosingCompletionWithoutBossIdentity(t *testing.T) {
	const finalMap uint32 = 100000294
	monsters := []protocol.DungeonMonster{{Entity: 0x1021, Template: 109010976, Team: 100}, {Entity: 0x1022, Template: 109010748, Team: 100}}
	c := catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{finalMap: {SHA256: "map"}}, TerminalScenes: []catalog.DungeonTerminalScene{{
		Source: "source", Dungeon: 291100432, Quest: 12147, Position: [2]byte{1, 2},
		ObjectiveMap: 292106830, FinalMap: finalMap, XMin: 741, XMax: 741, YMin: 346, YMax: 346,
		DungeonSHA256: "dungeon", MapSHA256: "map",
	}}}
	c.Source.Checksum = "source"
	run := &dungeon.Session{
		Definition: catalog.DungeonDefinition{ID: 291100432, Script: catalog.ScriptRecord{SHA256: "dungeon"}},
		Maze:       catalog.DungeonMaze{Quest: 12147, Layers: []catalog.DungeonLayer{{Position: [2]byte{1, 2}, Maps: []uint32{finalMap}}}},
		Room:       catalog.DungeonRoom{X: 1, Y: 2, Map: finalMap, Boss: true}, Loaded: true,
		Monsters: monsters, Visited: map[uint32][]protocol.DungeonMonster{292106830: {}, finalMap: monsters}, Dead: map[uint16]bool{},
	}
	record := protocol.DungeonRoomTransition{Dungeon: 291100432, Position: [2]byte{1, 2}, LayerChange: true,
		Record: [18]byte{0, 0, 0, 0, 4, 5, 0xe5, 0x02, 0x5a, 0x01}}
	next, err := run.MoveScene(c, record)
	if err != nil {
		t.Fatal(err)
	}
	next.Loaded = true
	next.TryComplete()
	w := &worldSession{activeDungeon: next}
	plan, err := w.completeDungeon()
	if err != nil || len(plan) != 1 || plan[0].ID != 31 {
		t.Fatalf("expected clear enable without a fictitious boss check: plan=%+v err=%v", plan, err)
	}
	if maps := next.ClearedMaps(); len(maps) != 2 || maps[1] != 292106830 {
		t.Fatalf("quest objective map missing from completed run: %v", maps)
	}
}

func TestElvenmereTeleportFlow(t *testing.T) {
	// 1. 无活动副本或非 Elvenmere 副本必须拒绝
	w := &worldSession{}
	req := make([]byte, 25)
	if _, err := w.elvenmereTeleport(req); err == nil {
		t.Fatal("accepted teleport without active dungeon")
	}

	w.activeDungeon = &dungeon.Session{
		Definition: catalog.DungeonDefinition{ID: 7},
	}
	if _, err := w.elvenmereTeleport(req); err == nil {
		t.Fatal("accepted elvenmere teleport in ordinary dungeon 7")
	}

	// 2. Elvenmere 副本合法 25 字节请求正常响应，下发 ACK 与 NOTI 2193
	w.activeDungeon = &dungeon.Session{
		Definition: catalog.DungeonDefinition{ID: 100003126},
		Extra:      1,
	}
	plan, err := w.elvenmereTeleport(req)
	if err != nil {
		t.Fatalf("elvenmere teleport failed: %v", err)
	}
	if len(plan) != 2 || plan[0].ID != 2015 || plan[0].Kind != 1 || plan[1].ID != 2193 || plan[1].Kind != 0 {
		t.Fatalf("unexpected elvenmere teleport plan: %+v", plan)
	}
	if w.activeDungeon.Extra != 2 {
		t.Fatalf("expected floor to advance to 2, got %d", w.activeDungeon.Extra)
	}
	if len(plan[1].Payload) != 203 || plan[1].Payload[2] != 2 {
		t.Fatalf("expected NOTI 2193 payload with current floor 2, got %v", plan[1].Payload[:5])
	}

	// 3. 报文长度不足 25 字节必须拒绝
	if _, err := w.elvenmereTeleport(make([]byte, 10)); err == nil {
		t.Fatal("accepted short payload")
	}
}

func TestElvenmerePlayerDeathFailClear(t *testing.T) {
	// 1. 普通副本角色死亡：下发 ACK 40 与 NOTI 32，不附带 NOTI 33
	wOrdinary := &worldSession{
		role: database.Character{ID: 1, WireID: 3},
		activeDungeon: &dungeon.Session{
			RunID:      "run-100",
			Loaded:     true,
			Definition: catalog.DungeonDefinition{ID: 7},
		},
	}
	deathReq := make([]byte, 16)
	planOrd, err := wOrdinary.playerDeath(deathReq)
	if err != nil {
		t.Fatalf("ordinary player death failed: %v", err)
	}
	if len(planOrd) != 2 || planOrd[0].ID != 40 || planOrd[1].ID != 32 {
		t.Fatalf("unexpected ordinary death plan: %+v", planOrd)
	}

	// 2. Elvenmere 特殊爬塔副本（ID 100003126）角色死亡：
	// 原生禁止复活币，单人死亡即代表通关失败，必须追加下发 NOTI 33 (FAIL_CLEAR_DUNGEON)
	wElvenmere := &worldSession{
		role: database.Character{ID: 1, WireID: 3},
		activeDungeon: &dungeon.Session{
			RunID:      "run-200",
			Loaded:     true,
			Definition: catalog.DungeonDefinition{ID: 100003126},
		},
	}
	planElv, err := wElvenmere.playerDeath(deathReq)
	if err != nil {
		t.Fatalf("elvenmere player death failed: %v", err)
	}
	if len(planElv) != 3 {
		t.Fatalf("expected 3 packets for elvenmere death, got %d: %+v", len(planElv), planElv)
	}
	if planElv[0].ID != 40 || planElv[0].Kind != 1 {
		t.Fatalf("expected ACK 40 at index 0, got %+v", planElv[0])
	}
	if planElv[1].ID != 32 || planElv[1].Kind != 0 {
		t.Fatalf("expected NOTI 32 at index 1, got %+v", planElv[1])
	}
	if planElv[2].ID != 33 || planElv[2].Kind != 0 || planElv[2].Name != "dungeon_fail_clear" {
		t.Fatalf("expected NOTI 33 dungeon_fail_clear at index 2, got %+v", planElv[2])
	}
	if len(planElv[2].Payload) != 1 || planElv[2].Payload[0] != 0 {
		t.Fatalf("expected 1-byte payload with 0 for NOTI 33, got %x", planElv[2].Payload)
	}

	// 3. 玩家死亡后退出地下城回城：必须下发 NOTI 32 (state=1) 恢复满血满蓝并解除 Ghost 死亡状态
	planLeave, err := wElvenmere.leaveDungeon()
	if err != nil {
		t.Fatalf("leave dungeon failed: %v", err)
	}
	var foundRevive bool
	for _, p := range planLeave {
		if p.Name == "town_actor_revived" && p.ID == 32 && p.Kind == 0 {
			if len(p.Payload) >= 3 && p.Payload[2] == 1 {
				foundRevive = true
				break
			}
		}
	}
	if !foundRevive {
		t.Fatalf("expected town_actor_revived NOTI 32 (state=1) in leave plan: %+v", planLeave)
	}
	if wElvenmere.pilotDeath.Dead {
		t.Fatal("expected pilotDeath.Dead to be reset to false after leaveDungeon")
	}
}
