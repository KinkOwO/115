package protocol

import (
	"encoding/hex"
	"testing"
)

// 2026-09-28 实机抓到的 CMD272（附魔宝珠）请求明文，用来钉死请求布局。
func TestDecodeEnchantByBeadLiveSamples(t *testing.T) {
	cases := []struct {
		hex        string
		beadSpace  byte
		beadSlot   uint16
		equipSpace byte
		equipSlot  uint16
	}{
		{"004c0000180000000000000000000000", 0, 76, 0, 24},
		{"004b0003140000000000000000000000", 0, 75, 3, 20},
		{"004d00001a0000000000000000000000", 0, 77, 0, 26},
		{"00550003140000000000000000000000", 0, 85, 3, 20},
		{"00570003140000000000000000000000", 0, 87, 3, 20},
	}
	for _, c := range cases {
		raw, err := hex.DecodeString(c.hex)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DecodeEnchantByBead(raw)
		if err != nil {
			t.Fatalf("%s 解码失败: %v", c.hex, err)
		}
		if got.BeadSpace != c.beadSpace || got.BeadSlot != c.beadSlot ||
			got.EquipSpace != c.equipSpace || got.EquipSlot != c.equipSlot {
			t.Errorf("%s 解出 %+v，期望 空间%d/槽%d + 装备空间%d/槽%d",
				c.hex, got, c.beadSpace, c.beadSlot, c.equipSpace, c.equipSlot)
		}
	}
	if _, err := DecodeEnchantByBead([]byte{0, 1, 2}); err == nil {
		t.Error("长度不足的请求应被拒绝")
	}
}

// 成功回包必须是 status(1) + u8 容器 + u16 槽，共 4 字节。
func TestEnchantByBeadReplyShape(t *testing.T) {
	got := EnchantByBeadReply(3, 20)
	want := []byte{1, 3, 20, 0}
	if len(got) != len(want) {
		t.Fatalf("回包长度 %d，期望 %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("回包 = % x，期望 % x", got, want)
		}
	}
	// 大槽位走小端 u16。
	if r := EnchantByBeadReply(0, 300); r[2] != 44 || r[3] != 1 {
		t.Fatalf("u16 槽未按小端写: % x", r)
	}
}
