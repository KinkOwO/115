package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/savecontract"
	"dfolan/internal/storage"
	"dfolan/internal/testfixture"
	"dfolan/internal/world"
	"encoding/json"
	"errors"
	"testing"
)

// 实机缺陷（2026-09-21，角色 test-jh，45 级）：在赫顿玛尔后街（39/5）点击第 1 章
// "雪山的主人"的"移动"按钮，客户端先发 special warp 预备、再发 CMD 36，服务端回
// code 8（日志 `area_refused town=43 area=1 reason="destination level requirement
// not met"`），界面提示"您必须是45级别才能进入斯顿雪域"。
//
// 根因是源数据的两个门槛：stormpass 的 [permission] 同时给出 [need level] 50 与
// [odyssey enter level] 45，客户端在奥德赛模式下按 45 判定（提示里的数字就是它），
// 服务端此前只读 [need level]，于是把 45 级奥德赛角色挡在门外。
func TestOdysseyStormPassJournalTeleportGate(t *testing.T) {
	cat, err := catalog.LoadWorld(testfixture.CatalogPath(t, "world"))
	if err != nil {
		t.Fatal(err)
	}
	growth, err := catalog.LoadOdysseyGrowth("../../configs/odyssey-growth-release.json")
	if err != nil {
		t.Fatal(err)
	}
	req := append([]byte{0, 4, 0, 0, 0}, []byte("test")...)
	req = append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0)
	for len(req)%8 != 0 {
		req = append(req, 0)
	}
	// 日志第 6 个节点（43/1 斯顿雪域）之前的所有节点都必须已通关。
	state, err := json.Marshal(map[string]any{"odyssey_completed_dungeons": []uint32{
		100004934, 100004935, 100004936, 100004937, 100004938,
		100004939, 100004940, 100004941, 100004942,
	}})
	if err != nil {
		t.Fatal(err)
	}
	session := func(level byte, odysseyRole bool) *worldSession {
		role := storage.Character{ConfigVersion: savecontract.Identity(), State: state}
		if odysseyRole {
			role.Request = req
		}
		// 与 worldSession.enter 一致：准入模式取角色自身的创建标记。
		return &worldSession{
			service:     &world.Service{Catalog: cat},
			progression: &character.ProgressionService{Odyssey: growth},
			level:       level,
			odyssey:     character.CreatedAsOdyssey(role),
			role:        role,
			state:       storage.WorldState{Position: storage.WorldPosition{Town: 39, Area: 5, X: 341, Y: 267}},
		}
	}
	// 客户端日志传送形态：CMD 36 Flag=5 且尾部标志全 0。
	journal := protocol.AreaChangeRequest{Town: 43, Area: 1, X: 396, Y: 403, PreviousTown: 39, PreviousArea: 5, Flag: 5}
	// 客户端"移动"按钮的真实序列：先 CMD 2261 预备，再 CMD 36。
	specialWarp := journal
	specialWarp.Flag = 0
	specialWarp.TailFlags = [2]byte{}

	// 1. 奥德赛启动器（DFO_ODYSSEY_MODE=1）：日志传送分支生效。
	t.Setenv("DFO_ODYSSEY_MODE", "1")
	w := session(45, true)
	w.specialWarpPending = true
	next, err := w.areaTransition(journal)
	if err != nil || next.Town != 43 || next.Area != 1 {
		t.Fatalf("45 级奥德赛角色传送斯顿雪域被拒: %+v %v", next, err)
	}
	// 奥德赛门槛本身仍然生效：44 级不得到 45 级节点。
	if _, err = session(44, true).areaTransition(journal); !errors.Is(err, world.ErrLevel) {
		t.Fatalf("44 级奥德赛角色越级进入斯顿雪域: %v", err)
	}

	// 2. 普通场景启动器（DFO_ODYSSEY_MODE=0）：日志分支不生效，但仍必须按角色
	//    自身的奥德赛标记放行 special warp 传送，否则客户端提示 45、服务端要 50。
	w = session(45, true)
	w.specialWarpPending = true
	if next, err = w.areaTransition(specialWarp); err != nil || next.Town != 43 {
		t.Fatalf("场景模式下 45 级奥德赛角色传送斯顿雪域被拒: %+v %v", next, err)
	}

	// 3. 普通角色的 [need level] 50 不受影响，且必须是 code 8 的等级拒绝。
	w = session(45, false)
	w.specialWarpPending = true
	if _, err = w.areaTransition(specialWarp); !errors.Is(err, world.ErrLevel) {
		t.Fatalf("普通 45 级角色绕过了 [need level] 50: %v", err)
	}
}

// 实机缺陷（2026-09-22，角色 gugugaga，20 级奥德赛，角色 11）：在赫顿玛尔（39/0）
// 点日志「移动」去西海岸（40/0），客户端走完 CMD2261 → NOTI365 → CMD36，服务端回
// code 8（日志 `area_refused town=40 area=0 reason="destination level requirement
// not met"` ×3），界面提示 DSTR 30069「You must be Level 15 to go to West Coast」。
// 40/0 的 [permission] 是 [need level] 15 / [odyssey enter level] 35：客户端在 20 级
// 就发出请求且提示数字是 15，说明 odyssey 值只能下调、不能上抬入门门槛（高出的值
// 约束的是该区域 [phase] 变体）；服务端按 replace 语义误用 35 拦截。日志传送白名单
// 不含 40/0，所以实机走的是 special warp 分支。
func TestOdysseyWestCoastTeleportGate(t *testing.T) {
	cat, err := catalog.LoadWorld(testfixture.CatalogPath(t, "world"))
	if err != nil {
		t.Fatal(err)
	}
	req := append([]byte{0, 8, 0, 0, 0}, []byte("gugugaga")...)
	req = append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0)
	for len(req)%8 != 0 {
		req = append(req, 0)
	}
	role := storage.Character{ID: 11, AccountID: 7, WireID: 11, Request: req, State: json.RawMessage(`{"level":20}`)}
	w := &worldSession{
		service: &world.Service{Catalog: cat},
		role:    role,
		level:   20,
		odyssey: character.CreatedAsOdyssey(role),
		state:   storage.WorldState{Position: storage.WorldPosition{Town: 39, Area: 0, X: 3284, Y: 259}},
	}
	if !w.odyssey {
		t.Fatal("fixture is not an odyssey character")
	}
	// 实机包形态：special warp 预备后 CMD 36 Flag=5、尾标志全 0、目的地 40/0。
	w.specialWarpPending = true
	r := protocol.AreaChangeRequest{Town: 40, Area: 0, X: 388, Y: 180, Flag: 5, PreviousTown: 39, PreviousArea: 0}
	next, err := w.areaTransition(r)
	if err != nil || next.Town != 40 || next.Area != 0 {
		t.Fatalf("20 级奥德赛角色传送西海岸被拒: %+v %v", next, err)
	}
	// 14 级仍按 [need level] 15 拦截。
	w.level = 14
	w.specialWarpPending = true
	if _, err = w.areaTransition(r); !errors.Is(err, world.ErrLevel) {
		t.Fatalf("14 级奥德赛角色越级进入西海岸: %v", err)
	}
}
