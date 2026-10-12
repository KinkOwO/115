package protocol

import (
	"encoding/binary"
	"testing"
)

// TestExtraDropFrameLengths 钉长度：两个长度都来自 IDA 里 handler 的读流调用，
// 改一个字节客户端就 OVERFLOW（本仓吃过 1294 少 1 字节、紧跟的帧被吞的亏）。
func TestExtraDropFrameLengths(t *testing.T) {
	info := ExtraDropEventInfo115(0x11223344, 0x55667788)
	if len(info) != ExtraDropEventInfoBytes || ExtraDropEventInfoBytes != 8 {
		t.Fatalf("2880 载荷 %d 字节，期望 8", len(info))
	}
	if got := binary.LittleEndian.Uint32(info[0:4]); got != 0x11223344 {
		t.Fatalf("2880 first = %#x", got)
	}
	if got := binary.LittleEndian.Uint32(info[4:8]); got != 0x55667788 {
		t.Fatalf("2880 second = %#x", got)
	}

	got := ExtraDropGetReward115(0x0102030405060708, 0xAB)
	if len(got) != ExtraDropGetRewardBytes || ExtraDropGetRewardBytes != 9 {
		t.Fatalf("2881 载荷 %d 字节，期望 9", len(got))
	}
	if v := binary.LittleEndian.Uint64(got[0:8]); v != 0x0102030405060708 {
		t.Fatalf("2881 value = %#x", v)
	}
	if got[8] != 0xAB {
		t.Fatalf("2881 flag = %#x", got[8])
	}
}
