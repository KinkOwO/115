package main

// boostup_roster_guard_test.go —— NOTI2639（BOOST_UP_MODE_ALL_CHARAC_INFO）的畸形输入护栏。
//
// 历史（2026-10-07）：这一帧曾因**行宽写错**（5 字节/行，客户端按 7 字节/行读）让客户端越界读，
// 发出 CMD217 后本地把所有外发包 IGNORE（`EndPacket(...) : IGNORE (made after exit packet)`）
// ⇒ 选角界面失效、检查角色名无效。当时把现象误读成「太长」，加了 9 字节上限；**上限低于编码的
// 最小合法长度 11 字节**，反而把正确帧也降级掉。行宽已改对（见
// docs/protocol/noti2639-row-width-authoritative-20261007.md），上限改为本仓自定的
// 「32 槽 × 7 字节 + 4」防御值。
//
// 这里仍然钉住两条形态约束，防止以后有人把 emptyFallback 改回「一律不发」：
//  1. **选角路径永远给一个包**（异常时降级成已实机验证过的空标记 {0,0,0,0}）——
//     client_dispatch_character.go 要求这次快照恰好 2 个包（NOTI2639 标记 + CMD2 名单），
//     少一个就报 `boost roster snapshot incomplete` 并把名单整份丢弃（玩家重启后看不到任何角色）。
//  2. 没有「包数契约」的动作路径（胶囊/毕业）允许不发。

import (
	"dfolan/internal/game/protocol"
	"testing"
)

func TestBoostRosterGuardDowngradesInsteadOfDroppingOnSelectPath(t *testing.T) {
	// 造一个**真正超过自定防御上限**的畸形标记：上限按 32 槽计（4+7×32=228），
	// 这里给 40 行（4+7×40=284）⇒ 只有超限分支会处理它。
	rows := make([]protocol.BoostRosterRow115, 0, 40)
	for i := 0; i < 40; i++ {
		rows = append(rows, protocol.BoostRosterRow115{Slot: uint32(i), Mode: 0})
	}
	marker, e := protocol.BoostRoster115(rows)
	if e != nil {
		t.Fatal(e)
	}
	if len(marker) <= maxBoostRosterMarkerBytes {
		t.Fatalf("前提不成立：本用例要的是一个超限的标记，实际 %d 字节（上限 %d）",
			len(marker), maxBoostRosterMarkerBytes)
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

	// 未超限：原样发，不做任何改动（1 行 = 4+7 = 11 字节，实机已实测可用）。
	safe, e := protocol.BoostRoster115([]protocol.BoostRosterRow115{{Slot: 0, Mode: 2}})
	if e != nil {
		t.Fatal(e)
	}
	if pkt, ok := boostRosterFrame("boost_roster_restored", safe, true); !ok || string(pkt.Payload) != string(safe) {
		t.Fatalf("未超限的标记必须原样发送，实际 ok=%v payload=% x", ok, pkt.Payload)
	}
}
