package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/savecontract"
	"dfolan/internal/testfixture"
	"dfolan/internal/world"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
)

func specialWarpFixture(t *testing.T) *worldSession {
	t.Helper()
	c, e := catalog.LoadWorld(testfixture.CatalogPath(t, "world"))
	if e != nil {
		t.Fatal(e)
	}
	g, e := catalog.LoadOdysseyGrowth("../../configs/odyssey-growth-release.json")
	if e != nil {
		t.Fatal(e)
	}
	req := append([]byte{0, 4, 0, 0, 0}, []byte("test")...)
	req = append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0)
	for len(req)%8 != 0 {
		req = append(req, 0)
	}
	return &worldSession{account: 7, level: 38, service: &world.Service{Catalog: c}, progression: &character.ProgressionService{Odyssey: g},
		role:  database.Character{ID: 14, AccountID: 7, WireID: 14, Request: req, ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"odyssey_completed_dungeons":[100004934,100004935,100004936,100004937,100004938]}`)},
		state: database.WorldState{Position: database.WorldPosition{Town: 40, Area: 4, X: 849, Y: 263}}}
}

func TestSpecialWarpPreparationAndDarkelfDestination(t *testing.T) {
	w := specialWarpFixture(t)
	old := w.state
	plan, e := w.prepareSpecialWarp(nil)
	if e != nil || len(plan) != 1 {
		t.Fatal(plan, e)
	}
	if plan[0].Kind != 0 || plan[0].ID != 365 || hex.EncodeToString(plan[0].Payload) != "04010e0000000000" {
		t.Fatal(plan)
	}
	if w.state != old {
		t.Fatal("preparation changed position")
	}
	for i := 0; i < 32; i++ {
		if !retainRequestBody(2261, map[uint16]int{2261: 99}) {
			t.Fatal("preparation stopped being decoded")
		}
		p, e := w.prepareSpecialWarp(nil)
		if e != nil || len(p) != 0 {
			t.Fatal("duplicate animation", p, e)
		}
	}
	r := protocol.AreaChangeRequest{Town: 41, Area: 2, X: 569, Y: 218, Flag: 5, PreviousTown: 40, PreviousArea: 4}
	next, e := w.areaTransition(r)
	if e != nil || next.Town != 41 || next.Area != 2 {
		t.Fatal(next, e)
	}
	// [MERGE-20260928-JOURNAL-LANDING] 原来这里是 `r.X = 1`（篡改落点坐标）。
	// 41/2 正是奥德赛日志的第 3 站（fixture 的 State 已通关前两站 934..938），
	// 而落点坐标不是站点的身份 —— 客户端从传送门/地图选择器出发时报的是它自己的
	// 默认落点（实机 2026-09-28「前往天界」即如此）。X=1 落在 41/2 的 walkable
	// 矩形 [17,170,800,110] 加 WalkableTolerance=128 之内，是合法落点。
	// 所以这里改为篡改**站点**，那才是必须被拒的东西。
	r.Town = 99
	if _, e = w.areaTransition(r); e == nil {
		t.Fatal("altered target admitted")
	}
	r.Town = 41
	r.X = 569
	w.role.State = json.RawMessage(`{"odyssey_completed_dungeons":[100004934,100004935,100004936,100004937]}`)
	if _, e = w.areaTransition(r); e == nil {
		t.Fatal("missing clear bypassed")
	}
	w = specialWarpFixture(t)
	w.specialWarpPending = true
	if e = w.handle(36, []byte{1}, nil, nil); e == nil || w.specialWarpPending {
		t.Fatal("malformed move retained pending")
	}
	w.specialWarpPending = true
	clearSelectedWorld(w)
	if w.specialWarpPending {
		t.Fatal("pending survived character switch")
	}
	t.Log("MODIFIED: CMD2261 empty -> NOTI365 04010e0000000000; no position mutation; source Darkelf41/2 admitted after confirmed clears")
}

func TestSpecialWarpRejectsInvalidContext(t *testing.T) {
	for _, kind := range []string{"body", "role", "owner", "actor", "dungeon", "selection", "geometry", "level"} {
		t.Run(kind, func(t *testing.T) {
			w := specialWarpFixture(t)
			var p []byte
			switch kind {
			case "body":
				p = []byte{0}
			case "role":
				w.role.ID = 0
			case "owner":
				w.role.AccountID++
			case "actor":
				w.role.WireID = 65535
			case "dungeon":
				w.activeDungeon = &dungeon.Session{}
			case "selection":
				w.selectingDungeon = true
			case "geometry":
				w.state.Position.X = 65535
			case "level":
				w.level = 0
			}
			if plan, e := w.prepareSpecialWarp(p); e == nil || len(plan) != 0 || w.specialWarpPending {
				t.Fatal(plan, e)
			}
		})
	}
}

func TestTownMapTeleportTransition(t *testing.T) {
	c, e := catalog.LoadWorld(testfixture.CatalogPath(t, "world"))
	if e != nil {
		t.Fatal(e)
	}
	w := &worldSession{
		account: 1,
		level:   50,
		service: &world.Service{Catalog: c},
		role:    database.Character{ID: 11, AccountID: 1, WireID: 11},
		state:   database.WorldState{Position: database.WorldPosition{Town: 39, Area: 2, X: 320, Y: 306}},
	}

	// 1. 普通走门：39/2 (后街) 到 40/0 (西海岸) 无门户边，TailFlags 为 0 时必须拒绝
	rWalk := protocol.AreaChangeRequest{
		Town: 40, Area: 0, X: 388, Y: 180, Flag: 5,
		PreviousTown: 39, PreviousArea: 2, TailFlags: [2]byte{0, 0},
	}
	if _, err := w.areaTransition(rWalk); err == nil {
		t.Fatal("walk transition without portal should be rejected")
	}

	// 2. 地图传送点传送：TailFlags[1] == 5，无直通门户边也放行
	rTeleport := protocol.AreaChangeRequest{
		Town: 40, Area: 0, X: 388, Y: 180, Flag: 5,
		PreviousTown: 39, PreviousArea: 2, TailFlags: [2]byte{0, 5},
	}
	next, err := w.areaTransition(rTeleport)
	if err != nil || next.Town != 40 || next.Area != 0 || next.X != 388 || next.Y != 180 {
		t.Fatalf("map teleport failed: %+v, err: %v", next, err)
	}

	// 3. 特殊传送预备态 (CMD 2261 触发的 specialWarpPending) 也放行
	w.state.Position = database.WorldPosition{Town: 39, Area: 2, X: 320, Y: 306}
	if _, err := w.prepareSpecialWarp(nil); err != nil {
		t.Fatal(err)
	}
	next, err = w.areaTransition(rWalk)
	if err != nil || next.Town != 40 || next.Area != 0 {
		t.Fatalf("specialWarpPending teleport failed: %+v, err: %v", next, err)
	}
	// 验证 pending 状态已被单次消费
	if _, err := w.areaTransition(rWalk); err == nil {
		t.Fatal("specialWarpPending should be consumed after single transition")
	}

	// 4. 月光酒馆传送点 (39/4) 落点偏差容差测试 (落点 y=198，距行走区 68 像素)
	w.state.Position = database.WorldPosition{Town: 40, Area: 3, X: 152, Y: 153}
	rBar := protocol.AreaChangeRequest{
		Town: 39, Area: 4, X: 355, Y: 198, Flag: 5,
		PreviousTown: 40, PreviousArea: 3, TailFlags: [2]byte{0, 5},
	}
	next, err = w.areaTransition(rBar)
	if err != nil || next.Town != 39 || next.Area != 4 || next.X != 355 || next.Y != 198 {
		t.Fatalf("moonlight bar teleport failed: %+v, err: %v", next, err)
	}

	// 5. 过期来源区域拒绝
	rStale := protocol.AreaChangeRequest{
		Town: 40, Area: 0, X: 388, Y: 180, Flag: 5,
		PreviousTown: 39, PreviousArea: 99, TailFlags: [2]byte{0, 5},
	}
	if _, err := w.areaTransition(rStale); err == nil {
		t.Fatal("stale previous area should be rejected")
	}

	// 6. 等级不足拒绝
	w.state.Position = database.WorldPosition{Town: 39, Area: 2, X: 320, Y: 306}
	w.level = 1
	if _, err := w.areaTransition(rTeleport); !errors.Is(err, world.ErrLevel) {
		t.Fatalf("under-level teleport should return ErrLevel, got %v", err)
	}
	w.level = 50

	// 7. 越界坐标拒绝
	rOOB := protocol.AreaChangeRequest{
		Town: 40, Area: 0, X: 60000, Y: 60000, Flag: 5,
		PreviousTown: 40, PreviousArea: 3, TailFlags: [2]byte{0, 5},
	}
	if _, err := w.areaTransition(rOOB); err == nil {
		t.Fatal("out-of-bounds teleport coordinate should be rejected")
	}

	// 8. 副本中拒绝城镇传送
	w.activeDungeon = &dungeon.Session{}
	if _, err := w.areaTransition(rTeleport); err == nil {
		t.Fatal("teleport inside dungeon should be rejected")
	}
	w.activeDungeon = nil

	// 9. 点击 "Teleport to Seria's Room" (TailFlags == [0, 0], Flag == 5)
	// 从西海岸 (40/0) 传送进入赛丽亚房间 (38/1)
	w.state.Position = database.WorldPosition{Town: 40, Area: 0, X: 403, Y: 181}
	rSeria := protocol.AreaChangeRequest{
		Town: 38, Area: 1, X: 557, Y: 210, Flag: 5,
		PreviousTown: 40, PreviousArea: 0, TailFlags: [2]byte{0, 0},
	}
	next, err = w.areaTransition(rSeria)
	if err != nil || next.Town != 38 || next.Area != 1 || next.X != 557 || next.Y != 210 {
		t.Fatalf("teleport to seria room failed: %+v, err: %v", next, err)
	}
	if next.Return == nil || next.Return.Town != 40 || next.Return.Area != 0 || next.Return.X != 403 || next.Return.Y != 181 {
		t.Fatalf("teleport to seria room did not save return origin: %+v", next.Return)
	}

	// Live CMD36 from Seria's right-hand map selector: Flag=5, TailFlags[0]=5.
	// The selected West Coast landing must win over the stamped Hendon origin.
	quickRoom := next
	quickRoom.Return = &database.WorldReturn{Town: 39, Area: 0, X: 3494, Y: 314}
	w.state.Position = quickRoom
	quickBody, err := hex.DecodeString("28000000000000007f01bd00052600000001000005000000")
	if err != nil {
		t.Fatal(err)
	}
	quickRequest, err := protocol.DecodeAreaChangeRequest(quickBody)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := w.areaTransition(quickRequest)
	if err != nil || selected.Town != 40 || selected.Area != 0 || selected.X != 383 || selected.Y != 189 || selected.Return != nil {
		t.Fatalf("Seria map selector ignored chosen destination: %+v %v", selected, err)
	}

	// 10. 从赛丽亚房间走到底部光圈返回西海岸，验证 Return 坐标被权威恢复
	w.state.Position = next
	rReturn := protocol.AreaChangeRequest{
		Town: 40, Area: 0, X: 746, Y: 157, Flag: 5,
		PreviousTown: 38, PreviousArea: 1, TailFlags: [2]byte{0, 0},
	}
	returned, err := w.areaTransition(rReturn)
	if err != nil || returned.Town != 40 || returned.Area != 0 || returned.X != 403 || returned.Y != 181 || returned.Return != nil {
		t.Fatalf("return from seria room failed: %+v, err: %v", returned, err)
	}
}

func TestNativeEpisodeTownReturnFromSavedPosition(t *testing.T) {
	c, err := catalog.LoadWorld(testfixture.CatalogPath(t, "world"))
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{
		account: 7,
		level:   94,
		service: &world.Service{Catalog: c},
		role:    database.Character{ID: 14, AccountID: 7, WireID: 14},
		state:   database.WorldState{Position: database.WorldPosition{Town: 55, Area: 0, X: 862, Y: 345}},
	}
	r := protocol.AreaChangeRequest{Town: 38, Area: 0, X: 2092, Y: 220, Flag: 5, PreviousTown: 55, PreviousArea: 0}
	if !w.npcMoveTeleport(r) || !w.episodeTownReturn(r) {
		t.Fatal("NPC move and episode return source rules not indexed")
	}
	next, err := w.areaTransition(r)
	if err != nil || next.Town != 38 || next.Area != 0 || next.X != 2092 || next.Y != 220 {
		t.Fatalf("episode town exit refused: %+v, %v", next, err)
	}
	r.Town = 40
	if _, err := w.areaTransition(r); err == nil {
		t.Fatal("unrelated destination admitted from episode town")
	}
	for _, tc := range []struct{ episodeTown, returnTown, returnArea uint32 }{
		{75, 22, 4}, {82, 22, 4}, {149, 6, 2},
	} {
		w.state.Position = database.WorldPosition{Town: tc.episodeTown, Area: 0}
		r = protocol.AreaChangeRequest{Town: tc.returnTown, Area: tc.returnArea, Flag: 5, PreviousTown: tc.episodeTown}
		if !w.episodeTownReturn(r) {
			t.Fatalf("episode %d return NPC destination refused", tc.episodeTown)
		}
	}
}
