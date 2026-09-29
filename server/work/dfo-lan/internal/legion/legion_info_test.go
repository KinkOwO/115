package legion

import (
	"encoding/binary"
	"testing"
)

// NOTI2895 的形状：u16 content + 204 字节，偏移与客户端初始化器 sub_140698280 一致。
func TestLegionInfoLayout(t *testing.T) {
	p := LegionInfo(ApocalypseContent, DefaultLegionInfoState())
	if len(p) != 2+LegionInfoBody || LegionInfoBody != 204 {
		t.Fatalf("legion info length = %d", len(p))
	}
	if got := binary.LittleEndian.Uint16(p[0:]); got != 107 {
		t.Fatalf("content = %d, want 107", got)
	}
	b := p[2:]
	if got := binary.LittleEndian.Uint16(b[0:]); got != 0xFFFF {
		t.Fatalf("@0 = %#x, want 0xFFFF", got)
	}
	if b[2] != 0xFF {
		t.Fatalf("@2 = %#x, want 0xFF", b[2])
	}
	if got := binary.LittleEndian.Uint32(b[3:]); got != 14 {
		t.Fatalf("@3 = %d, want 14", got)
	}
	if got := binary.LittleEndian.Uint32(b[7:]); got != 4 {
		t.Fatalf("@7 = %d, want 4", got)
	}
	if got := binary.LittleEndian.Uint64(b[11:]); got != ^uint64(0) {
		t.Fatalf("@11 = %#x, want all ones", got)
	}
	if got := binary.LittleEndian.Uint32(b[23:]); got != 0xFFFFFFFF {
		t.Fatalf("@23 = %#x", got)
	}
	if got := binary.LittleEndian.Uint32(b[99:]); got != 0xFFFFFFFF {
		t.Fatalf("@99 = %#x", got)
	}
	if got := binary.LittleEndian.Uint32(b[123:]); got != 0x01010101 {
		t.Fatalf("@123 = %#x, want four ones", got)
	}
	// 未列出的偏移必须是 0：6×12 记录区、16 字节块、尾 u32。
	for _, span := range [][2]int{{107, 16}, {128, 16}, {144, 8}, {152, 16}, {168, 16}, {184, 16}, {200, 4}} {
		for i := span[0]; i < span[0]+span[1]; i++ {
			if b[i] != 0 {
				t.Fatalf("offset %d = %#x, want 0", i, b[i])
			}
		}
	}
	// 显式传值必须落在那三个偏移上。
	for i := 0; i < 6; i++ {
		off := 27 + i*12
		if b[off] != 255 || binary.LittleEndian.Uint64(b[off+4:]) != ^uint64(0) {
			t.Fatal("invalid unset phase destination/time")
		}
	}
	other := LegionInfo(ApocalypseContent, LegionInfoState{State: 2, Outcome: 0, Stage: 7})
	if binary.LittleEndian.Uint32(other[2+3:]) != 2 || binary.LittleEndian.Uint32(other[2+7:]) != 0 || binary.LittleEndian.Uint64(other[2+11:]) != 7 {
		t.Fatalf("explicit state not written: %x", other[2:2+19])
	}
}
