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

// TestDungeonSelectionHeadRelayFollowsTheMode 钉住「继续挑战」那一对握手：
// 门应答（NOTI15）不变，NOTI27 换成 relay=1 的形态；普通进本仍是 relay=0。
func TestDungeonSelectionHeadRelayFollowsTheMode(t *testing.T) {
	plain := dungeonSelectionHeadFor(false)
	relay := dungeonSelectionHeadFor(true)
	if len(plain) != 2 || len(relay) != 2 {
		t.Fatalf("握手帧数 %d/%d, want 2/2", len(plain), len(relay))
	}
	if plain[0].ID != 15 || relay[0].ID != 15 || plain[0].Payload[0] != 1 || relay[0].Payload[0] != 1 {
		t.Fatal("门应答（NOTI15）不该被入口形态影响")
	}
	if plain[1].ID != 27 || relay[1].ID != 27 {
		t.Fatalf("第二帧应是 NOTI27，得到 %d/%d", plain[1].ID, relay[1].ID)
	}
	if plain[1].Payload[1] != 0 {
		t.Fatalf("普通进本的 relay 字节 = %#x, want 0", plain[1].Payload[1])
	}
	if relay[1].Payload[1] != 1 {
		t.Fatalf("继续挑战的 relay 字节 = %#x, want 1", relay[1].Payload[1])
	}
	// dungeonSelectionHead() 必须仍是「普通进本」那一份（城镇选图那条路一直用它）。
	if !bytes.Equal(dungeonSelectionHead()[1].Payload, plain[1].Payload) {
		t.Fatal("dungeonSelectionHead() 不再是普通进本形态")
	}
}
