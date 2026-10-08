package main

import (
	"bytes"
	"testing"

	"dfolan/internal/game/protocol"
)

// TestDungeonEntryKindFollowsTheSeamlessFlag 钉住「入口类型」的来源：
// CMD72 选项 5（SettlementExitSeamless）置位的无缝续刷 ⇒ NOTI28 body[30] = 5，
// 其余一切进本途径 ⇒ 0。
//
// 为什么值得单独钉：这一字节是官服用来区分「新副本」与「继续挑战」的稳定信号之一
//（analysis/tasks/next178 §3）；如果它在某条入口上漏掉或张冠李戴，玩家看到的就是
//「再次挑战时 buff 被重上」，而抓包里不会有任何异常。
func TestDungeonEntryKindFollowsTheSeamlessFlag(t *testing.T) {
	plain := &worldSession{}
	if got := plain.dungeonEntryKind(); got != 0 {
		t.Fatalf("普通进本的入口类型 = %d, want 0", got)
	}
	seamless := &worldSession{seamlessRetry: true}
	if got := seamless.dungeonEntryKind(); got != 5 {
		t.Fatalf("无缝续刷的入口类型 = %d, want 5", got)
	}
	if seamless.dungeonEntryKind() != protocol.SettlementExitSeamless {
		t.Fatal("入口类型必须直接取 SettlementExitSeamless，别另立常量")
	}
	// nil 会话不该 panic（进图计划在极早期就会读它）。
	var none *worldSession
	if got := none.dungeonEntryKind(); got != 0 {
		t.Fatalf("nil 会话的入口类型 = %d, want 0", got)
	}
}

// TestDungeonRelayFlagFollowsTheSeamlessFlag 钉住 NOTI27 头字节 `relay` 的来源：
// 无缝续刷 ⇒ 1，其余 ⇒ 0（同一份 seamlessRetry，见 dungeonRelayFlag 的注释）。
func TestDungeonRelayFlagFollowsTheSeamlessFlag(t *testing.T) {
	if got := (&worldSession{}).dungeonRelayFlag(); got != 0 {
		t.Fatalf("普通进本的 relay = %d, want 0", got)
	}
	if got := (&worldSession{seamlessRetry: true}).dungeonRelayFlag(); got != 1 {
		t.Fatalf("无缝续刷的 relay = %d, want 1", got)
	}
	var none *worldSession
	if got := none.dungeonRelayFlag(); got != 0 {
		t.Fatalf("nil 会话的 relay = %d, want 0", got)
	}
}

// TestDungeonSelectionHeadFollowsTheOfficialRechallengeShape 钉住「继续挑战」的头帧形状：
//
//	普通进本 = 门应答 NOTI15 + NOTI27（relay 0）—— 逐字节与以前相同；
//	无缝续刷 = **只发 NOTI27**（relay 1）。
//
// 后者的依据是官服抓包（next178 §14）：那一轮服务端的帧列里**既没有 15 也没有 16**，
// 客户端那一轮也没发 CMD15/CMD16；本仓此前把 15/27/16 当「合成握手」主动发出去，
// 客户端因此走 `change module : MAIN_GAME -> SELECT_DUNGEON` 的「新副本」路径。
func TestDungeonSelectionHeadFollowsTheOfficialRechallengeShape(t *testing.T) {
	plain := dungeonSelectionHeadFor(false)
	relay := dungeonSelectionHeadFor(true)

	if len(plain) != 2 || plain[0].ID != 15 || plain[1].ID != 27 {
		t.Fatalf("普通进本的头应是 15+27，得到 %+v", plain)
	}
	if plain[0].Payload[0] != 1 {
		t.Fatal("门应答载荷变了")
	}
	if plain[1].Payload[1] != 0 {
		t.Fatalf("普通进本的 relay 字节 = %#x, want 0", plain[1].Payload[1])
	}

	if len(relay) != 1 {
		t.Fatalf("无缝续刷的头应**只有** NOTI27，得到 %d 帧", len(relay))
	}
	if relay[0].ID != 27 {
		t.Fatalf("无缝续刷的第一帧应是 NOTI27，得到 %d", relay[0].ID)
	}
	if relay[0].Payload[1] != 1 {
		t.Fatalf("继续挑战的 relay 字节 = %#x, want 1", relay[0].Payload[1])
	}

	// dungeonSelectionHead() 必须仍是「普通进本」那一份（城镇选图那条路一直用它）。
	if !bytes.Equal(dungeonSelectionHead()[0].Payload, plain[0].Payload) ||
		!bytes.Equal(dungeonSelectionHead()[1].Payload, plain[1].Payload) {
		t.Fatal("dungeonSelectionHead() 不再是普通进本形态")
	}
}

// assertEntryTail 检查进图计划的尾部：… → 28 → 29 → 475。
//
// 475（NOTI475 CHARACTER_BUFF_DUNGEON，角色 buff·副本）是 2026-10-08 按官服补的：
// 官方把它放在 START_MAP 之后（analysis/tasks/next178 §14/§17），所以尾部形状是有意变的。
func assertEntryTail(t *testing.T, plan []outboundPacket) {
	t.Helper()
	n := len(plan)
	if n < 3 {
		t.Fatalf("进图计划太短：%d 帧", n)
	}
	if plan[n-3].ID != 28 || plan[n-2].ID != 29 || plan[n-1].ID != 475 {
		t.Fatalf("进图计划尾部应为 28→29→475，实际 %d→%d→%d",
			plan[n-3].ID, plan[n-2].ID, plan[n-1].ID)
	}
}
