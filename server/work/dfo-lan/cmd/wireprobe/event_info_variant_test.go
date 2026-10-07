package main

import (
	"bytes"
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// TestTownEventInfoTableCarriesBoostRows 钉住 662 入口修复的形状：
// 进城那条 108 必须是「频道门 + 活动行」的合并表，而不是只含频道门的原表。
// 反例（未合并）由 DFO_EVENT_INFO_VARIANT=plain 复现，这里断言默认值。
func TestTownEventInfoTableCarriesBoostRows(t *testing.T) {
	if bytes.Equal(townEventInfoTable, eventInfoTable) {
		t.Fatal("默认表体未合并活动行 —— 进城那条 108 会抹掉选角发的 662/10017/10018")
	}
	if len(townEventInfoTable) <= len(eventInfoTable) {
		t.Fatalf("合并表未变长: %d vs %d", len(townEventInfoTable), len(eventInfoTable))
	}
	if townEventInfoTable[len(townEventInfoTable)-1] != 0 {
		t.Fatal("合并表丢了 0x00 尾")
	}
	want := binary.LittleEndian.Uint16(eventInfoTable[:2]) + 4
	got := binary.LittleEndian.Uint16(townEventInfoTable[:2])
	if got != want {
		t.Fatalf("记录数 %d, want %d", got, want)
	}
	// 启动日志 `NOTI108 rows=%d bytes=%d` 直接读这两个数，实机可比对：
	// 频道门 19 + 10017/10018/662 + 毕业后的 665 = 23 行；
	// 若仍是 rows=3 bytes=248 说明拿的是未合并表。
	t.Logf("合并表 rows=%d bytes=%d（原表 rows=%d bytes=%d）",
		got, len(townEventInfoTable),
		binary.LittleEndian.Uint16(eventInfoTable[:2]), len(eventInfoTable))
	for _, id := range []uint16{10017, 10018, 662, 665} {
		var buf [2]byte
		binary.LittleEndian.PutUint16(buf[:], id)
		if !bytes.Contains(townEventInfoTable, buf[:]) {
			t.Fatalf("合并表里找不到活动 id %d", id)
		}
	}
}

// TestTownEventInfoTableCountFollowsRows 钉住表头 count 的来源：它必须等于正文里
// 真实的行数，而不是写死的 3。挑战行（665）打开时多一条 ⇒ count 也必须 +1，
// 否则客户端按 count 读取会把最后一行丢掉，665 入口照样不出现。
func TestTownEventInfoTableCountFollowsRows(t *testing.T) {
	base := binary.LittleEndian.Uint16(eventInfoTable[:2])
	for _, tc := range []struct {
		challenge bool
		rows      uint16
	}{
		{false, 3},
		{true, 4},
	} {
		merged, ok := buildTownEventInfoTable(tc.challenge)
		if !ok {
			t.Fatalf("challenge=%v 合并失败", tc.challenge)
		}
		if got := binary.LittleEndian.Uint16(merged[:2]); got != base+tc.rows {
			t.Fatalf("challenge=%v count=%d, want %d", tc.challenge, got, base+tc.rows)
		}
		if last := merged[len(merged)-1]; last != 0 {
			t.Fatalf("challenge=%v 丢了 0x00 尾", tc.challenge)
		}
	}
	plain, ok1 := buildTownEventInfoTable(false)
	with, ok2 := buildTownEventInfoTable(true)
	if !ok1 || !ok2 {
		t.Fatal("合并失败")
	}
	bodyPlain, bodyWith := plain[2:len(plain)-1], with[2:len(with)-1]
	if !bytes.HasPrefix(bodyWith, bodyPlain) || len(bodyWith) <= len(bodyPlain) {
		t.Fatal("665 行不是追加在最后一条活动行之后")
	}
	extra := bodyWith[len(bodyPlain):]
	var id [2]byte
	binary.LittleEndian.PutUint16(id[:], 665)
	if !bytes.HasPrefix(extra, id[:]) {
		t.Fatalf("多出来的行不是 665：%x", extra)
	}
}

// TestEventInfoTableWithBoostRowsKeepsOfficialRows 合并是**纯追加**：
// 原 19 行必须逐字节原样保留，否则军团/攻坚的频道门会被这次改动碰坏。
func TestEventInfoTableWithBoostRowsKeepsOfficialRows(t *testing.T) {
	merged, ok := buildTownEventInfoTable(false)
	if !ok {
		t.Fatal("合并失败")
	}
	official := eventInfoTable[2 : len(eventInfoTable)-1]
	if !bytes.Contains(merged, official) {
		t.Fatal("官方 19 行没有逐字节保留")
	}
	if len(eventInfoTableLegacy) != 54 {
		t.Fatalf("legacy 表长 %d, want 54", len(eventInfoTableLegacy))
	}
}

// 官服抓包（同一套 115 客户端）里 662/665 这两条 EVENT_INFO 记录的原始字节，
// 逐字取自 internal/legion/event_info_official.plain 的 @0x117c（662）与 @0x16ad（665），
// 各 65 B。这两条是本机能拿到的**唯一**关于 665 行应该长什么样的实测证据
// （旧端把 665 门控着从未实机发过）。
var officialBoostRecords = map[uint16]string{
	662: "96020102041f000000536b79206f6620612054686f7573616e64205365617320426f6f73742055700000000000000000802b716a8f17fc6a000000000000000000",
	665: "99020102041f000000536b79206f6620612054686f7573616e64205365617320426f6f73742055700000000000000000802b716a8f17fc6a000000000000000000",
}

// TestBoostRecordsMatchOfficialCapture 钉住 662/665 两行与官服**逐字节相同**：
// 唯一的实质差异原本是日期（旧值 start=0 / end=2033，官服是活动期
// 1785801600 → 1794905999，即两份 .evt 里都写着的
// `[event period] 2026-08-04 09:00:00 → 2026-11-17 09:00:01` 那一段）。
// 2026-10-06「665 城里没有入口」的取证里，这是我方能查到的最后一条与官服不符的字段，
// 所以把它钉死：以后谁改日期、改 flag、改标题，测试直接红。
func TestBoostRecordsMatchOfficialCapture(t *testing.T) {
	rows, err := protocol.BoostOpeningEvents115(boostup.EventStart, boostup.EventEnd, true)
	if err != nil {
		t.Fatalf("活动行编码失败: %v", err)
	}
	body := rows[2 : len(rows)-1]
	if binary.LittleEndian.Uint16(rows[:2]) != 4 {
		t.Fatalf("活动行数 %d, want 4（10017/10018/662/665）", binary.LittleEndian.Uint16(rows[:2]))
	}
	seen := map[uint16][]byte{}
	for o := 0; o < len(body); {
		start := o
		id := binary.LittleEndian.Uint16(body[o:])
		o += 2 + 3
		str := func() string {
			n := int(binary.LittleEndian.Uint32(body[o:]))
			o += 4
			s := string(body[o : o+n])
			o += n
			return s
		}
		name := str()
		str()
		str()
		o += 8
		str()
		str()
		o++ // flag
		seen[id] = body[start:o]
		if id == 662 || id == 665 {
			if name != "Sky of a Thousand Seas Boost Up" {
				t.Fatalf("id=%d 标题 %q 与官服不符", id, name)
			}
		}
	}
	for id, want := range officialBoostRecords {
		got, ok := seen[id]
		if !ok {
			t.Fatalf("表里没有 id=%d 的行", id)
		}
		if hex.EncodeToString(got) != want {
			t.Fatalf("id=%d 与官服记录不一致:\n got=%x\nwant=%s", id, got, want)
		}
	}
}
