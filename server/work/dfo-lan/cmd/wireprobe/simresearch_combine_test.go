package main

// CMD2413（组装）请求正文的解析。
//
// 三个样本是 2026-10-12 实机捕获的**原帧**（会话日志里 `oracle_action_received`，
// `command=combine` 的 `request_hex`），逐字节写进测试 —— 以后改解析时若把行/槽位对错位，
// 这里会立刻红。

import (
	"encoding/hex"
	"testing"
)

func oracleCombineMustHex(t *testing.T, s string) []byte {
	t.Helper()
	raw, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("坏样本 %s: %v", s, err)
	}
	return raw
}

func TestSimResearchCombineRequest115RealSamples(t *testing.T) {
	cases := []struct {
		hex      string
		row      uint32
		setSlot  uint32
		partSlot uint32
	}{
		// 行 0：套装槽 1（590015755）、部位槽 15（590015769 = [shoulder]）
		{"0000010001000f000200000000000000", 0, 1, 15},
		// 行 1：套装槽 10（590015764）、部位槽 15
		{"030004000a000f000500000000000000", 1, 10, 15},
		// 行 2：套装槽 11（590015765）、部位槽 16（590015770 = [waist]）
		{"060007000b0010000800000000000000", 2, 11, 16},
	}
	for _, tc := range cases {
		got, err := simResearchCombineRequest115(oracleCombineMustHex(t, tc.hex))
		if err != nil {
			t.Fatalf("%s: 意外失败 %v", tc.hex, err)
		}
		if got.Row() != tc.row || got.SetSlot != tc.setSlot || got.PartSlot != tc.partSlot {
			t.Errorf("%s → row=%d set=%d part=%d, want row=%d set=%d part=%d",
				tc.hex, got.Row(), got.SetSlot, got.PartSlot, tc.row, tc.setSlot, tc.partSlot)
		}
	}
}

func TestSimResearchCombineRequest115RejectsBadShape(t *testing.T) {
	bad := []string{
		"0000010001000f00",               // 正文太短（只有 8 字节）
		"0100010001000f000200000000000000", // 行三元组不是 3 的倍数
		"0000020001000f000200000000000000", // 三元组不连续
		"0000010001000f000300000000000000", // 第三个数不是 s0+2
		"09000a0001000f000b00000000000000", // 行号 > 2
	}
	for _, hs := range bad {
		if got, err := simResearchCombineRequest115(oracleCombineMustHex(t, hs)); err == nil {
			t.Errorf("%s 应该被拒，却解出 %+v", hs, got)
		}
	}
}
