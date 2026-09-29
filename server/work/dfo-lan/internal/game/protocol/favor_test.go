package protocol

import "testing"

// NOTI733 全量好感度列表布局锁定（逆向 handler 0x1452db190）：
// count u8 + 每条 npcID u32 + point u32 + flag u8 + colorCount u8。
// 2026-09-27 曾因误发 8 字节(npcID+point)导致首字节被当成 count、读越界
// 217 冻结；本测试防止布局再被改错。
func TestFavorPointInfoLayout(t *testing.T) {
	got := FavorPointInfo([]FavorPointInfoRecord{
		{NPCID: 100002681, Point: 1500, Flag: 0},
		{NPCID: 157, Point: 500, Flag: 0},
	})
	want := []byte{
		0x02,                   // count
		0x79, 0xeb, 0xf5, 0x05, // npcID 100002681
		0xdc, 0x05, 0x00, 0x00, // point 1500
		0x00, 0x00, // flag, colorCount
		0x9d, 0x00, 0x00, 0x00, // npcID 157
		0xf4, 0x01, 0x00, 0x00, // point 500
		0x00, 0x00,
	}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("payload = % x, want % x", got, want)
		}
	}
}

func TestFavorPointInfoEmpty(t *testing.T) {
	got := FavorPointInfo(nil)
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("empty list = % x, want 00", got)
	}
}

// count 为 u8，超过 255 条必须截断而不是溢出成 0（0 会让客户端清空 map）。
func TestFavorPointInfoClampsTo255(t *testing.T) {
	records := make([]FavorPointInfoRecord, 256)
	for i := range records {
		records[i] = FavorPointInfoRecord{NPCID: uint32(i + 1), Point: 1}
	}
	got := FavorPointInfo(records)
	if int(got[0]) != 255 || len(got) != 1+255*10 {
		t.Fatalf("count byte = %d, len = %d", got[0], len(got))
	}
}
