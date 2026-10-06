package main

import (
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"dfolan/internal/world"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// 实机 2026-10-06 12:15:57 / 13:36:52 的 665 Go 按钮 CMD36 原帧（会话
// …_195540_767216、…_212930_180742）：请求 241/1@143,173，flag 5，tail 0，
// 来源 38/1。源里这一行写的就是 `[go contents town area] 241 1 143 173`。
const boostChallengeGoFrame = "f1000000010000008f00ad00052600000001000000000000"

func boostChallengeGoSession(t *testing.T, unlocked bool) (*worldSession, protocol.AreaChangeRequest) {
	t.Helper()
	cat, e := catalog.LoadWorld(testfixture.CatalogPath(t, "world"))
	if e != nil {
		t.Fatal(e)
	}
	state, e := json.Marshal(boostup.State{Version: 1, Challenge: &boostup.ChallengeState{
		Version: 1, Enrolled: true,
		Rows: map[byte]boostup.ChallengeProgress{0: {Unlocked: unlocked}},
	}})
	if e != nil {
		t.Fatal(e)
	}
	w := &worldSession{
		service: &world.Service{Catalog: cat},
		level:   115,
		account: 7,
		boostup: &boostup.Catalog{Town: 222, Challenges: []boostup.ChallengeDefinition{{
			Index: 0, Kind: "clear endkeeper of order", Goal: 10, Repeat: 1,
			GoTarget: [4]uint32{241, 1, 143, 173}, HasGoTarget: true,
		}}},
		role:  database.Character{ID: 1, AccountID: 7, State: json.RawMessage(`{"boost_up115":` + string(state) + `}`)},
		state: database.WorldState{Position: database.WorldPosition{Town: 38, Area: 1, X: 557, Y: 210, Return: &database.WorldReturn{Town: 38, Area: 0, X: 1677, Y: 222}}},
	}
	b, e := hex.DecodeString(boostChallengeGoFrame)
	if e != nil {
		t.Fatal(e)
	}
	r, e := protocol.DecodeAreaChangeRequest(b)
	if e != nil {
		t.Fatal(e)
	}
	return w, r
}

// 665 Go 必须落在源写的 `[go contents town area]`，不再被 Seria 离场存档改写。
func TestBoostChallengeGoTeleportUsesSourceTarget(t *testing.T) {
	w, r := boostChallengeGoSession(t, true)
	next, e := w.areaTransition(r)
	if e != nil {
		t.Fatal(e)
	}
	if next.Town != 241 || next.Area != 1 || next.X != 143 || next.Y != 173 {
		t.Fatalf("source target lost: %+v", next)
	}
	if next.Return != nil {
		t.Fatal("destination is not a seria room, no return stamp expected")
	}
}

// 落点以源为准：客户端把 x/y 换成别的值也不能改目的地。
func TestBoostChallengeGoTeleportIgnoresClientCoordinates(t *testing.T) {
	w, r := boostChallengeGoSession(t, true)
	r.X, r.Y = 5000, 6000
	next, e := w.areaTransition(r)
	if e != nil || next.Town != 241 || next.Area != 1 || next.X != 143 || next.Y != 173 {
		t.Fatal(next, e)
	}
}

// 未解锁的行没有传送授权：请求回到原有判定（Seria 离场改写），不新增权限。
func TestBoostChallengeGoTeleportRequiresUnlockedRow(t *testing.T) {
	w, r := boostChallengeGoSession(t, false)
	next, e := w.areaTransition(r)
	if e != nil || next.Town != 38 || next.Area != 0 || next.X != 1677 || next.Y != 222 {
		t.Fatalf("locked row must keep the prior behaviour: %+v %v", next, e)
	}
}

// 源里没登记的目标区域不走这一支：241/0 仍然按普通门控改写回离场存档。
func TestBoostChallengeGoTeleportSkipsUnlistedTarget(t *testing.T) {
	w, r := boostChallengeGoSession(t, true)
	r.Area = 0
	if next, e := w.areaTransition(r); e != nil || next.Town != 38 || next.Area != 0 {
		t.Fatalf("unlisted target changed behaviour: %+v %v", next, e)
	}
}

// 副本内不是城镇角色，ownedTownTeleport 的既有门禁优先。
func TestBoostChallengeGoTeleportRefusesDungeonActor(t *testing.T) {
	w, r := boostChallengeGoSession(t, true)
	w.activeDungeon = &dungeon.Session{}
	if next, e := w.areaTransition(r); e == nil && next.Town == 241 {
		t.Fatal("in-dungeon actor accepted", next)
	}
}
