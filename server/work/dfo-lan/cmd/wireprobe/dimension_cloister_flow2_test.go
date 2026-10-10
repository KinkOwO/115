package main

import (
	"encoding/hex"
	"testing"

	"dfolan/internal/legion"
)

// 次元回廊判据与解码器的回归测试。
//
// 语料全部来自抓包/实机原文：
//
//	officialCmd2043  官服 _dump_s30_c2s.txt frame #461（开始作战）
//	officialCmd2045  官服 #476（进图确认，带关卡号）
//	realCmd2080a/b   业主 2026-10-10 实机会话里点右上角 UI 的两帧（CMD2080）
const officialCmd2043 = "68f55f00000000000d000000006600000000000000000000"
const officialCmd2045 = "020000000600000002000000006600000000000000000000"

const realCmd2080a = "90aefd9401000000010000000101000000ffff0000000000"
const realCmd2080b = "000000000000000006000000000200000007000000000000"

func decodeHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestDimCloisterDispatchUsesContentIdentity 钉住分派判据是**内容身份**而不是只看频道类型：
// 客户端 clientchannelinfo 里 50（Evildom）与 84（Hall of Dimensions）两行都指向本内容，
// 实机走的是 84（2026-10-10 事件 `apocalypse_party_probe channel_type=84`）。
//
// 84 必须为真：登录期推入场账本时客户端还没建队，那一刻只有频道号能回答
// 「这条连接是不是次元回廊」（2026-10-10 实证：漏了它会发伊斯那一族的 N2254）。
func TestDimCloisterDispatchUsesContentIdentity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		session *legionSession
		want    bool
	}{
		{"Evildom Type 50", &legionSession{channelType: 50}, true},
		{"Hall of Dimensions Type 84", &legionSession{channelType: 84}, true},
		{"84 + 已建队", &legionSession{channelType: 84, dimCloisterPartyActive: true}, true},
		{"末世录", &legionSession{channelType: 119}, false},
		{"苏醒之森", &legionSession{channelType: 86}, false},
		{"伊斯", &legionSession{channelType: 81}, false},
	} {
		if got := tc.session.isDimCloister(); got != tc.want {
			t.Fatalf("%s（Type %d, partyActive=%v）: isDimCloister=%v, want %v",
				tc.name, tc.session.channelType, tc.session.dimCloisterPartyActive, got, tc.want)
		}
	}
}

// TestDimCloisterDecodeStartMatchesCapture 用抓包原文解 CMD2043：
// 内容号 0x0d、队伍字节 0x66、关卡下标 0。
func TestDimCloisterDecodeStartMatchesCapture(t *testing.T) {
	req, err := legion.DecodeDimCloisterStart(decodeHex(t, officialCmd2043))
	if err != nil {
		t.Fatal(err)
	}
	if req.Content != legion.DimCloisterContent {
		t.Fatalf("内容号 %d, want %d", req.Content, legion.DimCloisterContent)
	}
	if req.Party != legion.DimCloisterPartyType {
		t.Fatalf("队伍字节 %#x, want %#x", req.Party, legion.DimCloisterPartyType)
	}
	if req.Stage != 0 {
		t.Fatalf("关卡下标 %d, want 0", req.Stage)
	}
}

// TestDimCloisterDecodeStartStages 三帧 CMD2043 的关卡下标与三关一一对应。
func TestDimCloisterDecodeStartStages(t *testing.T) {
	base := decodeHex(t, officialCmd2043)
	for stage := 0; stage < 3; stage++ {
		body := append([]byte(nil), base...)
		body[16] = byte(stage)
		req, err := legion.DecodeDimCloisterStart(body)
		if err != nil {
			t.Fatal(err)
		}
		if req.Stage != uint32(stage) {
			t.Fatalf("帧 %d: stage=%d, want %d", stage, req.Stage, stage)
		}
	}
}

// TestDimCloisterDecodeStageConfirmMatchesCapture 用抓包原文解 CMD2045：
// 队伍字节 0x66、关卡下标 0（**这是加载副本的触发帧**）。
func TestDimCloisterDecodeStageConfirmMatchesCapture(t *testing.T) {
	party, stage, err := legion.DecodeDimCloisterStageConfirm(decodeHex(t, officialCmd2045))
	if err != nil {
		t.Fatal(err)
	}
	if party != legion.DimCloisterPartyType {
		t.Fatalf("队伍字节 %#x, want %#x", party, legion.DimCloisterPartyType)
	}
	if stage != 0 {
		t.Fatalf("关卡下标 %d, want 0", stage)
	}
	// 第 2/3 轮：同一帧只把 @16 改成 1/2。
	base := decodeHex(t, officialCmd2045)
	for want := uint32(1); want <= 2; want++ {
		body := append([]byte(nil), base...)
		body[16] = byte(want)
		_, got, err := legion.DecodeDimCloisterStageConfirm(body)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("stage=%d, want %d", got, want)
		}
	}
}

// TestDimCloisterStartAckResultCodeIsOne 钉住 CMD2043 应答的**结果码**：
// 客户端读第一个 dword（0 = 接受，252/380 = 失败文案），所以
//
//	body[0] = 1 且 body[1..4] 必须全 0  →  dword = 0x00000001
//
// 2026-10-10 实机：上一版把队伍字节写在 @1（`01 66 00 00 …`），客户端算出的结果码
// 是 0x00006601，trace 里报 `ENUM_CMDPACKET_LEGION_START ErrCode : 102`，
// 表现就是「点开始作战没有任何反应」。
func TestDimCloisterStartAckResultCodeIsOne(t *testing.T) {
	req, err := legion.DecodeDimCloisterStart(decodeHex(t, officialCmd2043))
	if err != nil {
		t.Fatal(err)
	}
	ack := legion.DimCloisterStartAck(req)
	if len(ack) != legion.DimCloisterStartAckSize {
		t.Fatalf("应答长度 %d, want %d", len(ack), legion.DimCloisterStartAckSize)
	}
	code := uint32(ack[0]) | uint32(ack[1])<<8 | uint32(ack[2])<<16 | uint32(ack[3])<<24
	if code != 1 {
		t.Fatalf("结果码 dword=%d（%x）, want 1 —— @1..@4 必须清零", code, ack[:8])
	}
	for i := 1; i < 5; i++ {
		if ack[i] != 0 {
			t.Fatalf("结果码字段 @%d = %02x, want 00（会被算进 dword）", i, ack[i])
		}
	}
	if ack[5] != req.Party {
		t.Fatalf("队伍字节 @5 = %02x, want %02x", ack[5], req.Party)
	}
}

// TestDimCloisterStageAckShape 钉住 CMD2045 应答：40 字节、成功字节 1、队伍 @5、关卡 @10。
func TestDimCloisterStageAckShape(t *testing.T) {
	ack := legion.DimCloisterStageAck(legion.DimCloisterPartyType, 2)
	if len(ack) != legion.DimCloisterEnterAckSize {
		t.Fatalf("应答长度 %d, want %d", len(ack), legion.DimCloisterEnterAckSize)
	}
	if ack[0] != 1 || ack[5] != legion.DimCloisterPartyType {
		t.Fatalf("应答形状不对: %x", ack)
	}
	stage := uint32(ack[10]) | uint32(ack[11])<<8 | uint32(ack[12])<<16 | uint32(ack[13])<<24
	if stage != 2 {
		t.Fatalf("关卡 %d, want 2", stage)
	}
}

// TestDimCloisterN23AreaNoticeUsesDungeonArea 进图前把角色切进副本区域态（area=0xff）。
func TestDimCloisterN23AreaNoticeUsesDungeonArea(t *testing.T) {
	w := &worldSession{}
	w.role.WireID = 7
	w.state.Position.Town = 160
	w.state.Position.X = 644
	w.state.Position.Y = 163
	notice, err := w.forestDungeonAreaNotice()
	if err != nil {
		t.Fatal(err)
	}
	if len(notice) < 12 {
		t.Fatalf("N23 正文 %d 字节", len(notice))
	}
	area := uint32(notice[6]) | uint32(notice[7])<<8 | uint32(notice[8])<<16
	if area != 0xff {
		t.Fatalf("area=%#x, want 0xff", area)
	}
}
