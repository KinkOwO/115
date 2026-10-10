package main

import (
	"bytes"
	"dfolan/internal/legion"
	"testing"
)

// ★ 清关翻牌两帧 `N2252`/`N2253` 的**线上形状**回归。
//
// 权威基准：同一台服务器、同一个客户端上**跑通的那一场**
// （会话 `..._20261010_194607_840200_next37`，伊斯清关，`client.log` = `exit=0x0`）：
//
//	服务端 body=7772  →  客户端日志 `ENUM_NOTIPACKET_LEGION_BASIC_CLEAR_REWARD (Size : 7776)`     过关
//	服务端 body=2405  →  客户端日志 `ENUM_NOTIPACKET_LEGION_ADDITIONAL_CLEAR_REWARD (Size : 2408)` 过关
//
// 即 **客户端记的 Size = 正文 + 4**。
//
// 次元回廊此前把正文建成 7776 / 2408（多 4 / 3 字节），实机就在收到 N2252 的瞬间
// （或卡几秒后）崩：客户端 N2252 处理器 `sub_1424FDC30` 会从收包游标里取走
// `0x1E5C`(7772) 字节，长度对不上就把游标顶出界 ⇒ `mov dword ptr ds:0,0`。
func TestDimCloisterClearRewardFramesMatchWorkingSession(t *testing.T) {
	raw, err := legion.DimCloisterBasicClearRewardRaw(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 7772 {
		t.Fatalf("N2252 正文 %d 字节, want 7772（跑通那一场的同值）", len(raw))
	}
	if !bytes.Equal(raw[:1600], make([]byte, 1600)) {
		t.Fatal("N2252 前 1600B 必须全零（同族布局）")
	}
	if got := legion.DimCloisterBasicClearRewardRows[0].Template; got != 10338507 {
		t.Fatalf("第一排模板 %d, want 10338507（0x009DC0CB）", got)
	}
	token := legion.DimCloisterClearToken(0)
	if !bytes.Equal(raw[7760:7762], token) {
		t.Fatalf("N2252 尾 token = %x, want %x（必须与 N31 头同源）", raw[7760:7762], token)
	}
	clear := legion.DimCloisterEnableClearDungeon()
	if !bytes.Equal(token, clear[:2]) {
		t.Fatalf("token %x != N31 头 %x", token, clear[:2])
	}

	// 线上正文 = 那 7772B 裸字节（不是 zlib、不多不少）。
	wire, err := legion.DimCloisterBasicClearReward(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wire, raw) {
		t.Fatal("N2252 线上正文必须就是那 7772B 裸正文")
	}
	if len(wire) >= 2 && wire[0] == 0x78 {
		t.Fatal("N2252 线上正文首字节 0x78（zlib）—— 本客户端不解压")
	}
	// 客户端应当记成 正文+4 = 7776（跑通那一场的实测值）。
	if got := len(wire) + 4; got != 7776 {
		t.Fatalf("N2252 客户端应记 Size=%d, want 7776", got)
	}

	addRaw, err := legion.DimCloisterAdditionalClearRewardRaw()
	if err != nil {
		t.Fatal(err)
	}
	if len(addRaw) != 2405 {
		t.Fatalf("N2253 正文 %d 字节, want 2405（跑通那一场的同值）", len(addRaw))
	}
	for i, row := range legion.DimCloisterAdditionalClearRewardRows {
		off := 40 * i
		if addRaw[off] != row.Flag || addRaw[off+9] != 3 {
			t.Fatalf("N2253 第 %d 条 flag=%d const=%d", i, addRaw[off], addRaw[off+9])
		}
		got := uint32(addRaw[off+1]) | uint32(addRaw[off+2])<<8 |
			uint32(addRaw[off+3])<<16 | uint32(addRaw[off+4])<<24
		if got != row.Template {
			t.Fatalf("N2253 第 %d 条模板 %d, want %d", i, got, row.Template)
		}
		if uint32(addRaw[off+5]) != row.Count {
			t.Fatalf("N2253 第 %d 条数量 %d, want %d", i, addRaw[off+5], row.Count)
		}
	}
	if !bytes.Equal(addRaw[200:], make([]byte, len(addRaw)-200)) {
		t.Fatal("N2253 第 5 条起必须全零")
	}
	addWire, err := legion.DimCloisterAdditionalClearReward()
	if err != nil {
		t.Fatal(err)
	}
	if len(addWire) >= 2 && addWire[0] == 0x78 {
		t.Fatal("N2253 线上正文首字节 0x78（zlib）—— 本客户端不解压")
	}
}

// TestDimCloisterClearFrameSkippedIsTheForeignList 钉住"哪些帧不许回放"。
func TestDimCloisterClearFrameSkippedIsTheForeignList(t *testing.T) {
	for _, v := range legion.DimCloisterClearSequence() {
		skipped := legion.DimCloisterClearFrameSkipped(v.Kind, v.ID)
		switch {
		case v.Kind == 0 && v.ID == 14:
			if !skipped {
				t.Fatalf("帧 %s 的 N14 必须跳过（官服物品行）", v.Name)
			}
		case v.Kind == 0 && v.ID == 2:
			if !skipped {
				t.Fatalf("帧 %s 的 N2 必须跳过（官服角色数据）", v.Name)
			}
		case v.Kind == 0 && v.ID == 2252, v.Kind == 0 && v.ID == 2253:
			if skipped {
				t.Fatalf("帧 %s 不该被整帧丢掉：它要换成本仓构造的奖励正文", v.Name)
			}
		}
	}
}
