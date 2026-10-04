package protocol

import (
	"encoding/binary"
	"testing"
)

// 2634/2635 的载荷几何：10 字节 = u16 部位号 + u32 值A + u32 值B（小端）。
//
// 客户端用 sub_146EA0BE0(&buf, 10) **精确读 10 字节** ⇒ 长度必须是 10（短了会踩
// 客户端的长度护栏空写陷阱，见 oath_probe.go 里 8 字节 2839 的事故）。
func TestPartSetPointGeometry(t *testing.T) {
	body := PartSetPoint(0x1234, 0xAABBCCDD, 0x01020304)
	if len(body) != PartSetPointSize {
		t.Fatalf("payload = %d bytes, want %d", len(body), PartSetPointSize)
	}
	if got := binary.LittleEndian.Uint16(body[0:2]); got != 0x1234 {
		t.Fatalf("part = %#x, want 0x1234", got)
	}
	if got := binary.LittleEndian.Uint32(body[2:6]); got != 0xAABBCCDD {
		t.Fatalf("value A = %#x, want 0xAABBCCDD", got)
	}
	if got := binary.LittleEndian.Uint32(body[6:10]); got != 0x01020304 {
		t.Fatalf("value B = %#x, want 0x01020304", got)
	}
	// 两个 opcode 常量必须与 IDA 侧一致（2634 = 当前值、2635 = 达成值）。
	if PartSetPointOpcode != 2634 || PartSetPointAchievementOpcode != 2635 {
		t.Fatalf("opcodes = %d/%d, want 2634/2635", PartSetPointOpcode, PartSetPointAchievementOpcode)
	}
}
