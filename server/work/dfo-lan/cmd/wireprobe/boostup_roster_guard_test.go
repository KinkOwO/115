package main

// boostup_roster_guard_test.go —— NOTI2639（BOOST_UP_MODE_ALL_CHARAC_INFO）的客户端上限护栏。
//
// 背景（业主 2026-10-07 两条实机反馈，客户端 client_trace 原文）：
//  1. 2 行 / 明文 14 字节发出去 → 客户端 PACKET OVERFLOW → 本地把所有外发包 IGNORE
//     （EndPacket(...) : IGNORE (made after exit packet)）→ 选角界面失效、检查角色名无效。
//  2. 于是加了「超限不发」的护栏 —— 结果**选角名单整份被拒**：client_dispatch_character.go
//     要求这次快照恰好 2 个包（NOTI2639 标记 + CMD2 名单），少一个就报
//     boost roster snapshot incomplete 并丢掉名单 → 玩家重启后看不到任何角色。
//
// 所以护栏的正确形态是：**选角路径永远给一个包**（超限时降级成已实机验证过的空标记
// {0,0,0,0}），只有没有「包数契约」的动作路径（胶囊/毕业）才允许不发。
//
// 这里钉住这两条，防止以后有人把 emptyFallback 改回「一律不发」。

import (
	"dfolan/internal/game/protocol"
	"testing"
)

func TestBoostRosterGuardDowngradesInsteadOfDroppingOnSelectPath(t *testing.T) {
	// 2 行 = 明文 14 字节 > 已实测上限（9 字节 = 1 行）—— 正是现场那颗把客户端打爆的形状。
	marker, e := protocol.BoostRoster115([]protocol.BoostRosterRow115{{Slot: 1, Mode: 0}, {Slot: 2, Mode: 2}})
	if e != nil {
		t.Fatal(e)
	}
	if len(marker) <= maxVerifiedBoostRosterBytes {
		t.Fatalf("前提不成立：本用例要的是一个超限的标记，实际 %d 字节（上限 %d）",
			len(marker), maxVerifiedBoostRosterBytes)
	}

	// 选角路径（emptyFallback=true）：必须仍然给包，且是空标记 —— 绝不能返回 ok=false，
	// 否则 selectionRosterPackets 只剩名单一个包，会被判 incomplete 而丢掉整份名单。
	pkt, ok := boostRosterFrame("boost_roster_restored", marker, true)
	if !ok {
		t.Fatal("选角路径不允许「不发这一帧」：少一个包会让服务端判 boost roster snapshot incomplete，玩家看不到任何角色")
	}
	want, e := protocol.BoostRoster115(nil)
	if e != nil {
		t.Fatal(e)
	}
	if pkt.ID != 2639 || len(pkt.Payload) != len(want) || string(pkt.Payload) != string(want) {
		t.Fatalf("超限时应降级为空标记 % x，实际 id=%d payload=% x", want, pkt.ID, pkt.Payload)
	}

	// 动作路径（emptyFallback=false）：没有「包数契约」，超限就不发（宁缺勿错）。
	if _, ok := boostRosterFrame("boost_step_roster", marker, false); ok {
		t.Fatal("动作路径超限时应当不发这一帧")
	}

	// 未超限：原样发，不做任何改动（1 行 = 9 字节，已实测可用）。
	safe, e := protocol.BoostRoster115([]protocol.BoostRosterRow115{{Slot: 0, Mode: 2}})
	if e != nil {
		t.Fatal(e)
	}
	if pkt, ok := boostRosterFrame("boost_roster_restored", safe, true); !ok || string(pkt.Payload) != string(safe) {
		t.Fatalf("未超限的标记必须原样发送，实际 ok=%v payload=% x", ok, pkt.Payload)
	}
}
