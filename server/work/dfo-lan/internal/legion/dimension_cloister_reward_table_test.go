package legion

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 本仓合成的翻牌两帧与**同一台服务器上跑通的那一场**逐字段对照。
//
// 权威基准：会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261010_194607_840200_next37`
// （伊斯清关，`client.log` = `exit=0x0`），同客户端同服务端。该场里：
//
//	服务端 body=7772  →  客户端 `LEGION_BASIC_CLEAR_REWARD (Size : 7776)`     过关
//	服务端 body=2405  →  客户端 `LEGION_ADDITIONAL_CLEAR_REWARD (Size : 2408)` 过关
//
// 即客户端记的 Size = 正文 + 4。次元回廊此前把正文建成 7776/2408（多 4/3 字节），
// 实机就在收到 N2252 的瞬间/几秒后崩 —— 长度必须与那一场逐字节一致。
func TestDimCloisterRewardTablesMatchWorkingSession(t *testing.T) {
	raw, err := DimCloisterBasicClearRewardRaw(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 7772 {
		t.Fatalf("N2252 正文 %d 字节, want 7772（跑通那一场的同值）", len(raw))
	}
	if got := binary.LittleEndian.Uint32(raw[1600:]); got != 0x009DC0CB {
		t.Fatalf("N2252 @1600 模板 %d（%#x）, want 10338507（0x009DC0CB）", got, got)
	}
	if got := binary.LittleEndian.Uint32(raw[1604:]); got != 20 {
		t.Fatalf("N2252 @1604 数量 %d, want 20", got)
	}
	if got := raw[7760:7762]; !bytes.Equal(got, []byte{0x41, 0x3a}) {
		t.Fatalf("N2252 @7760 token %x, want 41 3a（= N31 头）", got)
	}
	if !bytes.Equal(raw[1600+44:7760], make([]byte, 7760-1600-44)) {
		t.Fatal("N2252 第 2 条 44B 行起必须全零")
	}

	add, err := DimCloisterAdditionalClearRewardRaw()
	if err != nil {
		t.Fatal(err)
	}
	if len(add) != 2405 {
		t.Fatalf("N2253 正文 %d 字节, want 2405（跑通那一场的同值）", len(add))
	}
	official := []struct {
		template uint32
		count    byte
		flag     byte
	}{
		{0x009DC0CB, 28, 1},
		{0x009DC0CC, 12, 0},
		{0x009D94F3, 67, 0},
		{0x009D94F1, 200, 0},
	}
	for i, want := range official {
		off := 40 * i
		if add[off] != want.flag {
			t.Fatalf("N2253 第 %d 条 flag=%d, want %d", i, add[off], want.flag)
		}
		got := binary.LittleEndian.Uint32(add[off+1:])
		if got != want.template {
			t.Fatalf("N2253 第 %d 条模板 %d（%#x）, want %d（%#x）", i, got, got, want.template, want.template)
		}
		if add[off+5] != want.count {
			t.Fatalf("N2253 第 %d 条数量 %d, want %d", i, add[off+5], want.count)
		}
		if add[off+9] != 3 {
			t.Fatalf("N2253 第 %d 条 @9=%d, want 3", i, add[off+9])
		}
	}
	if !bytes.Equal(add[160:], make([]byte, len(add)-160)) {
		t.Fatal("N2253 第 5 条起必须全零")
	}
}

// 线上形状必须是**裸正文**（不是 zlib）。
//
// 早期版本发过 zlib 压缩体（照抄 s30 抓包的 64B，那一份是抓包侧就已压缩，
// 本客户端不解压），实机必崩；能跑通的那一场发的是 7772 / 2405 裸正文。
func TestDimCloisterRewardWireFramesAreRaw(t *testing.T) {
	for _, tc := range []struct {
		name string
		body func() ([]byte, error)
	}{
		{"N2252", func() ([]byte, error) { return DimCloisterBasicClearReward(0, 0) }},
		{"N2253", DimCloisterAdditionalClearReward},
	} {
		body, err := tc.body()
		if err != nil {
			t.Fatal(err)
		}
		if len(body) >= 2 && body[0] == 0x78 {
			t.Fatalf("%s 正文首字节是 0x78（zlib 头）—— 本客户端不解压，会崩", tc.name)
		}
	}
}
