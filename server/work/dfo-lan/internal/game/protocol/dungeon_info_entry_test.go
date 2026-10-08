package protocol

import (
	"bytes"
	"testing"
)

// TestDungeonInfoPutsTheEntryKindAtByte30 钉住 NOTI28 的入口类型位移。
//
// 官服取证（analysis/tasks/next178 §3，2026-10-08 抓包）：同一个副本冷进场的 body[30]
// 是 0x00，两次「继续挑战」（CMD72 选项 5）都是 0x05，而两次之间其余字段逐帧相同 ——
// 这一字节就是「这是继续、不是新副本」的信号。
//
// 本仓客户端在 0x1452ada9e **单独**读这一个字节（testdata/native_dungeon_info_cursor.json
// 的 offset=30/size=1），所以它落在同一个位移上：错一格就是另一个字段。
func TestDungeonInfoPutsTheEntryKindAtByte30(t *testing.T) {
	base := DungeonInfoState{ID: 100005014, Difficulty: 1, Maze: 2, Boss: [2]byte{3, 4}}
	plain := DungeonInfo(base)
	if len(plain) != 41 {
		t.Fatalf("payload = %d bytes, want 41（native_dungeon_info_cursor.json 的 consumed）", len(plain))
	}
	if plain[30] != 0 {
		t.Fatalf("普通进本的 body[30] = %#x, want 0", plain[30])
	}

	// 常数本身也要钉：官服抓包里那两个「继续挑战」写的就是 0x05。
	if SettlementExitSeamless != 5 {
		t.Fatalf("SettlementExitSeamless = %d, want 5", SettlementExitSeamless)
	}
	seamless := base
	seamless.Entry = SettlementExitSeamless
	got := DungeonInfo(seamless)
	if got[30] != SettlementExitSeamless {
		t.Fatalf("无缝续刷的 body[30] = %#x, want %#x（= CMD72 的选项号 5）",
			got[30], SettlementExitSeamless)
	}
	if bytes.Equal(got, plain) {
		t.Fatal("入口类型没有进载荷")
	}
	// 只能有一个字节变：其它位移都被客户端逐个读，顺手改动就是动别的字段。
	for i := range plain {
		if plain[i] != got[i] && i != 30 {
			t.Fatalf("offset %d 也被改了：%#x -> %#x", i, plain[i], got[i])
		}
	}

	// 其它入口值原样写进去（0 与 5 之外不许被夹带成别的数）。
	for _, kind := range []byte{1, 2, 5} {
		s := base
		s.Entry = kind
		if p := DungeonInfo(s); p[30] != kind {
			t.Fatalf("entry %d 写成了 %#x", kind, p[30])
		}
	}
}
