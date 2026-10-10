package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"testing"

	"context"
)

// 结算面板的「再次挑战」是 CMD72 option=0，dstr 479 原文 "Restart the dungeon."，
// 语义是重开刚刚结算的那张图（option=2 才是 dstr 481 "Return to town."）。而进图
// 帧组只有在客户端已经处于「选图」状态时才被接受，所以「重开」路由的第一段必须
// 是 ACK15 + NOTI27，与 CMD2062「清关 → 下一个剧情关卡门」同形。
//
// 实机对照（上游贡献包 2026-09-22 / 2026-09-24）：
//   - 结算面板上直接收 16+28+29 → 客户端 0xC0000005，约 1.7 秒后退出；
//   - 结算面板上先收 15+27，再收 16+…+29 → 正常进入同一张图，不闪退、不用手选。
func TestSettlementRetryOpensSelectionBeforeEntry(t *testing.T) {
	w := &worldSession{role: database.Character{ID: 7, WireID: 10}}
	s := &dungeon.Session{
		Definition: catalog.DungeonDefinition{ID: 100004946},
		Maze:       catalog.DungeonMaze{Index: 0, Start: [2]byte{0, 0}, Boss: [2]byte{1, 1}},
		Room:       catalog.DungeonRoom{Map: 100016164},
	}
	sel := protocol.DungeonSelection{ID: 100004946, Party: 65535}
	entry, e := w.dungeonEntryPlan(context.Background(), "dungeon_select_ack", 16, sel, s)
	if e != nil {
		t.Fatal(e)
	}
	plan := append(dungeonSelectionHead(), entry...)
	if len(plan) < 3 {
		t.Fatalf("plan too short: %d packets", len(plan))
	}
	if plan[0].ID != 15 || plan[0].Kind != 1 || plan[0].Name != "dungeon_gate_ack" {
		t.Fatalf("first packet is not the gate acknowledgement ACK15: %+v", plan[0])
	}
	if plan[1].ID != 27 || plan[1].Kind != 0 || plan[1].Name != "dungeon_selection_sent" {
		t.Fatalf("second packet is not NOTI27: %+v", plan[1])
	}
	if plan[2].ID != 16 || plan[2].Kind != 1 {
		t.Fatalf("third packet is not the selection acknowledgement ACK16: %+v", plan[2])
	}
	assertEntryTail(t, plan)
	// 回城 / 场景切换帧一旦混进进图帧组，客户端就会在切换与进图之间撞车。
	for _, p := range plan {
		if p.Name == "town_actor_state" || p.Name == "dungeon_return_area" || p.Name == "dungeon_return_users" {
			t.Fatalf("entry plan carries a return-to-town frame: %+v", p)
		}
	}
}

// 结算是 CMD72 的唯一入口，会话已经释放时「再次挑战」必须明确拒绝，而不是臆造一
// 张图；这条分支在任何目录 / 数据库查询之前返回。
func TestSettlementRetryRequiresActiveRun(t *testing.T) {
	w := &worldSession{
		role:     database.Character{ID: 7, WireID: 10},
		dungeons: &catalog.DungeonCatalog{Dungeons: map[uint32]catalog.DungeonDefinition{}},
	}
	pending, route, e := w.restartDungeon()
	if e == nil {
		t.Fatal("retry without an active dungeon was accepted")
	}
	if pending != nil || len(route) != 0 {
		t.Fatalf("refused retry still produced a session or plan: %+v %+v", pending, route)
	}
}

// [ISPINS-ARENA-BOSS] 伊斯大陆会话的 CMD46（141B 通用结算请求，每阶段 boss
// 死亡后客户端必发，官服 s4 c2s 帧 338/393/442/489）必须整包吞掉：官服对它
// 无任何专门应答，generic 结算族（N34/N37/N26/N261/N19/N2758/N29/N21）在官服
// 伊斯结算里一个都没有。2026-10-03 四测：这些包发出后客户端 1.2s 内 op=682
// 崩溃退出 —— 吞掉 = 静默无应答；非伊斯会话保持 generic 路径不变。
func TestIspinsSwallowsGenericPlayResult(t *testing.T) {
	w := &worldSession{
		role:           database.Character{ID: 7, WireID: 10},
		ispins:         &ispinsRun{},
		activeDungeon:  &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 100002987}},
		completionSent: true,
	}
	plan, e := w.dungeonResult(nil)
	if e != nil || len(plan) != 0 {
		t.Fatalf("ispins CMD46 must be swallowed silently: plan=%v err=%v", plan, e)
	}
	w.ispins = nil
	if plan, e = w.dungeonResult(nil); e == nil || len(plan) != 0 {
		t.Fatalf("non-ispins CMD46 must keep the generic settlement path: plan=%v err=%v", plan, e)
	}
}
