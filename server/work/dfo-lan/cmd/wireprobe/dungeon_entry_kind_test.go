package main

import (
	"dfolan/internal/game/protocol"
	"testing"
)

// TestDungeonEntryKindFollowsTheSeamlessFlag 钉住「入口类型」的来源：
// CMD72 选项 5（SettlementExitSeamless）置位的无缝续刷 ⇒ NOTI28 body[30] = 5，
// 其余一切进本途径 ⇒ 0。
//
// 为什么值得单独钉：这一字节是官服用来区分「新副本」与「继续挑战」的唯一信号
//（analysis/tasks/next178 §3）；如果它在某条入口上漏掉或张冠李戴，玩家看到的就是
// 「再次挑战时 buff 被重上」，而抓包里不会有任何异常。
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
