package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
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
	cat, err := catalog.LoadWorld("../../configs/world.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	growth, err := catalog.LoadOdysseyGrowth("../../configs/odyssey-growth-candidate.json")
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
		role := storage.Character{ConfigVersion: growth.Source, State: state}
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
