package main

import (
	"bytes"
	"encoding/binary"
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
	want := binary.LittleEndian.Uint16(eventInfoTable[:2]) + 3
	got := binary.LittleEndian.Uint16(townEventInfoTable[:2])
	if got != want {
		t.Fatalf("记录数 %d, want %d", got, want)
	}
	// 启动日志 `NOTI108 rows=%d bytes=%d` 直接读这两个数，实机可比对：
	// 修复后应为 rows=22 bytes=1386；若仍是 rows=3 bytes=248 说明拿的是未合并表。
	t.Logf("合并表 rows=%d bytes=%d（原表 rows=%d bytes=%d）",
		got, len(townEventInfoTable),
		binary.LittleEndian.Uint16(eventInfoTable[:2]), len(eventInfoTable))
	for _, id := range []uint16{10017, 10018, 662} {
		var buf [2]byte
		binary.LittleEndian.PutUint16(buf[:], id)
		if !bytes.Contains(townEventInfoTable, buf[:]) {
			t.Fatalf("合并表里找不到活动 id %d", id)
		}
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
