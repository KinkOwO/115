package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"dfolan/internal/legion"
)

// 次元回廊「难度/关卡选择窗」的回归测试。
//
// 依据（业主 2026-10-10 的指路「也有修好的军团本啊……不要总是猜测」）：项目 opcode
// 表把这一族点名列了出来，次元回廊是**与末世录/维纳斯同构的独立一族**：
//
//	cmd  2080  MYRES_DIMENSION_CLOISTER_OPERATION_SELECT
//	cmd  2081  MYRES_DIMENSION_CLOISTER_OPERATION_CLEAR
//	noti 2314  MYRES_DIMENSION_CLOISTER_INFO
//	noti 2315  MYRES_DIMENSION_CLOISTER_OPERATION
//
// 操作窗由 **CMD 的 ACK 自己打开**（本仓 `NotiLegionOperation` 只被声明、从未发送，
// 末世录 CMD2354 / 维纳斯 CMD2290 都靠 ACK 开窗），所以应答照同族布局构造：
// 公共成功标志 + 内容号 0x0d + 14B 操作记录（Action / 关闭位 / 截止值）。

// TestDimCloisterOperationAckMatchesProvenLayout 钉住应答布局 = 低版本（110 客户端）
// 与官服 s30 #771 两条独立抓包共同印证的那 40 字节：
//
//	01 01 00 00 00 ff ff 00 00 00 00 00 <5B 票据> 00 00 00 00 00 00 00 00 00
func TestDimCloisterOperationAckMatchesProvenLayout(t *testing.T) {
	ack := legion.DimCloisterOperationAck(1, legion.DimCloisterOperationTicket)
	if len(ack) != legion.DimCloisterOperationAckSize {
		t.Fatalf("长度 %d, want %d", len(ack), legion.DimCloisterOperationAckSize)
	}
	if legion.DimCloisterOperationAckSize != 40 {
		t.Fatalf("应答长度常量 %d, want 40（低版本抓包实测）", legion.DimCloisterOperationAckSize)
	}
	if ack[0] != 1 {
		t.Fatalf("成功标志 %02x, want 01", ack[0])
	}
	if ack[1] != 1 {
		t.Fatalf("@1（action 回显）%02x, want 01", ack[1])
	}
	if ack[5] != 0xff || ack[6] != 0xff {
		t.Fatalf("@5..6 = %02x %02x, want ff ff", ack[5], ack[6])
	}
	if !bytes.Equal(ack[12:17], legion.DimCloisterOperationTicket[:]) {
		t.Fatalf("@12..16 票据 = %x, want %x", ack[12:17], legion.DimCloisterOperationTicket)
	}
	// 官服 #771 原文逐字节对照（40B）：
	// 01 01 000000 00 ffff 00000000 2b7224ff39 + 零尾
	prefix := "0101000000ffff00000000002b7224ff39"
	want := prefix + strings.Repeat("00", legion.DimCloisterOperationAckSize-len(prefix)/2)
	if got := hex.EncodeToString(ack); got != want {
		t.Fatalf("应答 = %s\n  want = %s", got, want)
	}
}

// TestDimCloisterOperationCloseSetsCloseBit 关窗应答 = 同样的 40B 形状 + 关闭位（@3=1）。
func TestDimCloisterOperationCloseSetsCloseBit(t *testing.T) {
	closeAck := legion.DimCloisterOperationClose()
	if len(closeAck) != legion.DimCloisterOperationAckSize {
		t.Fatalf("长度 %d, want %d", len(closeAck), legion.DimCloisterOperationAckSize)
	}
	if closeAck[0] != 1 || closeAck[1] != 1 {
		t.Fatalf("关窗应答头 %02x %02x, want 01 01", closeAck[0], closeAck[1])
	}
	if closeAck[3] != 1 {
		t.Fatalf("关窗位 %02x, want 01", closeAck[3])
	}
}

// TestDimCloisterOperationDecodeMatchesCapture 用业主实机那两帧 CMD2080 原文解码。
func TestDimCloisterOperationDecodeMatchesCapture(t *testing.T) {
	for _, hexBody := range []string{realCmd2080a, realCmd2080b} {
		req, err := legion.DecodeDimCloisterOperation(decodeHex(t, hexBody))
		if err != nil {
			t.Fatalf("%s: %v", hexBody, err)
		}
		if len(req.Body) != len(hexBody)/2 {
			t.Fatalf("正文长度 %d, want %d", len(req.Body), len(hexBody)/2)
		}
	}
}

// TestDimCloisterMemoryRecordsLayout 钉住记忆记录的 10 字节布局与官服三组原文。
//
// 记录区 = N2314 @63..102，四条 10B：{u16 操作号, u16 类型固定值, u16 掷出值, u16 槽位, u16 保留}。
// 字段语义来自客户端数据 `Contents/2022/DimensionCloister/Etc/MyresDimensionCloister.etc`
// 的 `[operation data set]`：官服 #758 的 (1,0,37,1) 对得上 op1 的 range 37 37，
// #910 的 (9,30,400,4) 对得上 op9 的 `[type fixed value] 30` + range 400 400。
func TestDimCloisterMemoryRecordsLayout(t *testing.T) {
	got := dimCloisterApplyMemoryRecords(make([]byte, 112), []dimCloisterMemoryRecord{
		{OpIndex: 1, Value: 37, Slot: 1},
		{OpIndex: 4, Value: 40, Slot: 2},
		{OpIndex: 7, Value: 150, Slot: 3},
	})
	want := decodeHex(t, "01000000250001000000"+"04000000280002000000"+"07000000960003000000"+"00000000000000000000")
	if !bytes.Equal(got[dimCloisterMemoryRecordFrom:dimCloisterMemoryRecordTo], want) {
		t.Fatalf("记忆记录区 = %x\nwant %x",
			got[dimCloisterMemoryRecordFrom:dimCloisterMemoryRecordTo], want)
	}
	// 官服 #910 那一组（四条，含 op9 的固定值 30）。
	third := dimCloisterApplyMemoryRecords(make([]byte, 112), dimCloisterProgressionMemories(2))
	wantThird := decodeHex(t, "01000000250001000000"+"03000000280002000000"+"06000000040003000000"+"09001e00900104000000")
	if !bytes.Equal(third[dimCloisterMemoryRecordFrom:dimCloisterMemoryRecordTo], wantThird) {
		t.Fatalf("第三组记忆记录区 = %x\nwant %x",
			third[dimCloisterMemoryRecordFrom:dimCloisterMemoryRecordTo], wantThird)
	}
	// 只给槽位 2 时，其余槽位必须是 0（不能残留官服字节）。
	only := dimCloisterApplyMemoryRecords(bytes.Repeat([]byte{0xff}, 112), []dimCloisterMemoryRecord{{OpIndex: 9, Fixed: 30, Value: 400, Slot: 2}})
	if only[dimCloisterMemoryRecordFrom] != 0 || only[dimCloisterMemoryRecordFrom+19] != 0 {
		t.Fatalf("未给槽位没有清零: %x", only[dimCloisterMemoryRecordFrom:dimCloisterMemoryRecordTo])
	}
	if binary.LittleEndian.Uint16(only[dimCloisterMemoryRecordFrom+10:]) != 9 ||
		binary.LittleEndian.Uint16(only[dimCloisterMemoryRecordFrom+14:]) != 400 {
		t.Fatalf("槽位 2 的记录 = %x, want 操作号 9 / 掷出值 400",
			only[dimCloisterMemoryRecordFrom+10:dimCloisterMemoryRecordFrom+20])
	}
}

// TestDimCloisterMemoryRecordsFromEnv 钉住诊断入口（DFO_CLOISTER_MEMORY_RECORDS）的解析：
// 合法项生效、非法项忽略、槽位越界忽略、最多四条；空值 = 走内置口径。
func TestDimCloisterMemoryRecordsFromEnv(t *testing.T) {
	t.Setenv(dimCloisterMemoryRecordsEnvVar, "")
	if got := dimCloisterMemoryRecordsFromEnv(); len(got) != 0 {
		t.Fatalf("空值应当没有覆盖, got %+v", got)
	}
	t.Setenv(dimCloisterMemoryRecordsEnvVar, "1,0,37,1; 4,0,40,2 ;bad;7,0,150,5;9,30,400,3;11,0,5,4;12,0,6,1")
	got := dimCloisterMemoryRecordsFromEnv()
	if len(got) != 4 {
		t.Fatalf("解析条数 %d, want 4（最多四条）: %+v", len(got), got)
	}
	if got[0] != (dimCloisterMemoryRecord{OpIndex: 1, Value: 37, Slot: 1}) {
		t.Fatalf("第一条 %+v", got[0])
	}
	if got[2] != (dimCloisterMemoryRecord{OpIndex: 9, Fixed: 30, Value: 400, Slot: 3}) {
		t.Fatalf("非法项没有被跳过: %+v", got)
	}
}

// TestDimCloisterMemorySlotsFollowPartySize 钉住「槽位规则」与「当前默认口径」。
//
// 槽位规则：条数 = 队伍成员数、槽位从 1 连续（官服三界的槽位数正好是 3 / 3 / 4）。
// 当前默认口径：**记录区全 0** —— 2026-10-10 第五/六轮实机证明，官服那份 3 条会崩，
// 只发 1 条（槽位 1）同样崩（`exit=0xC0000005`），根因是本客户端
// `MyresDimensionCloister.etc` 里 9 个 operation 的 `[string data]` 全空
// ⇒ 客户端拿不到格式串 ⇒ 任何非零记录都崩。补上客户端数据之前只能发全 0。
func TestDimCloisterMemorySlotsFollowPartySize(t *testing.T) {
	if dimCloisterPartyMemorySlots != 1 {
		t.Fatalf("单机队伍成员数 = %d, want 1", dimCloisterPartyMemorySlots)
	}
	// 槽位规则（补客户端数据之后要切过去的口径）。
	for cleared := 0; cleared <= 2; cleared++ {
		got := dimCloisterPartyMemoryRecords(cleared, dimCloisterPartyMemorySlots)
		if len(got) != dimCloisterPartyMemorySlots {
			t.Fatalf("已清 %d 界记忆条数 = %d, want %d", cleared, len(got), dimCloisterPartyMemorySlots)
		}
		for i, rec := range got {
			if rec.Slot != uint16(i+1) {
				t.Fatalf("已清 %d 界第 %d 条槽位 = %d, want %d", cleared, i, rec.Slot, i+1)
			}
		}
	}
	if first := dimCloisterPartyMemoryRecords(0, 1)[0]; first.OpIndex != 1 || first.Value != 37 || first.Slot != 1 {
		t.Fatalf("第一条记忆 = %+v, want op1/37/槽位1", first)
	}
	if two := dimCloisterPartyMemoryRecords(0, 2); len(two) != 2 || two[1].OpIndex != 4 {
		t.Fatalf("两队员时 = %+v, want 前两条（op1/op4）", two)
	}
	// 当前默认口径：全 0。
	for cleared := 0; cleared <= 2; cleared++ {
		if got := dimCloisterMemoryRecordsFor(cleared); len(got) != 0 {
			t.Fatalf("已清 %d 界默认记忆 = %+v, want 空（客户端数据缺口未补前不许发非零）", cleared, got)
		}
	}
	// 诊断入口仍然可以覆盖（取证用）。
	t.Setenv(dimCloisterMemoryRecordsEnvVar, "9,30,400,1;6,0,4,2")
	if got := dimCloisterMemoryRecordsFor(0); len(got) != 2 || got[1].OpIndex != 6 {
		t.Fatalf("诊断入口没有覆盖内置口径: %+v", got)
	}
}

// TestDimCloisterSelectRepliesWithCandidates 钉住处理器：CMD2080（开窗）回的是
// **官服 #770 + #771/#773 那三帧**：
//
//	本界进度态 N2314 @3=02（官服 #770）
//	两条 32B 难度候选（kind=1，opcode 就是客户端的 2080，官服 #771/#773）
//
// 不再回低版本 110 客户端那份 40B「操作窗 ACK」—— 本客户端（2.38.2.34）会把它的
// @1=0 当成「序号 0 的候选」，窗口于是直接跳到「变更记忆」态、一张卡都不显示
// （业主实机 20261010-135812 + 官方 s30 #771/#773 逐字节对照）。
// TestDimCloisterSelectRepliesWithOpenThenChoice 钉住处理器：CMD2080 走**同族 A/B 口径**。
//
//	第 1 次请求（点右上角 UI / 选记忆之书）→ 本界进度态 N2314@3=02（官服 #770）+ **A 帧（开窗）**
//	第 2 次请求（玩家选完）                  → **B 帧（已选）**，不再重推开窗态，
//	                                          也不再把两帧挤在同一批（那正是「难度框一闪而过」的成因）
//
// 形状依据：本仓伊斯族 `IspinsOperationAckA/B` 是现成可用实现，两帧 32B 逐字节同构
// （A：`01 01 000000 00 ffff 00000000 <u32 unix> 00 d0 0000 <5B token>`；
//
//	B：`01 02 000000 00 <选择值> …`）。
func TestDimCloisterSelectRepliesWithOpenThenChoice(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	w := &worldSession{}
	w.role.ID = 7
	w.role.WireID = 7
	result, err := s.dimCloisterSelect(w, decodeHex(t, realCmd2080a))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packets) != 2 {
		t.Fatalf("帧数 %d, want 2（本界进度态 + A 帧）", len(result.Packets))
	}
	if head := result.Packets[0]; head.Kind != 0 || head.ID != legion.NotiDimCloisterInfo || head.Payload[3] != 0x02 {
		t.Fatalf("开窗前导帧 = op%d/kind%d/@3=%02x, want op%d/kind0/@3=02（官服 #770）",
			head.ID, head.Kind, head.Payload[3], legion.NotiDimCloisterInfo)
	}
	open := result.Packets[1]
	if open.Kind != 1 || open.ID != legion.CmdDimCloisterOperationSelect {
		t.Fatalf("A 帧 op/kind = %d/%d, want 1/%d", open.ID, open.Kind, legion.CmdDimCloisterOperationSelect)
	}
	if len(open.Payload) != legion.DimCloisterOperationWindowAckSize {
		t.Fatalf("A 帧长度 %d, want %d", len(open.Payload), legion.DimCloisterOperationWindowAckSize)
	}
	if open.Payload[0] != 1 || open.Payload[1] != 1 || open.Payload[5] != 0xff || open.Payload[6] != 0xff {
		t.Fatalf("A 帧头 = % x, want 01 01 … ff ff（还没选）", open.Payload[:8])
	}
	if ts := binary.LittleEndian.Uint32(open.Payload[12:]); ts == 0 {
		t.Fatal("A 帧 @12..15 的时间戳是 0（同族这一格必须下发）")
	}
	openToken, choiceToken := legion.DimCloisterOperationTokens(0)
	if !bytes.Equal(open.Payload[20:25], openToken[:]) {
		t.Fatalf("A 帧 token = % x, want % x", open.Payload[20:25], openToken)
	}

	// 第二次请求 = 玩家的选择 → B 帧，且不再带开窗态。
	second, err := s.dimCloisterSelect(w, decodeHex(t, realCmd2080b))
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Packets) != 1 {
		t.Fatalf("确认帧数 %d, want 1（B 帧）", len(second.Packets))
	}
	choice := second.Packets[0]
	if choice.Kind != 1 || choice.ID != legion.CmdDimCloisterOperationSelect {
		t.Fatalf("B 帧 op/kind = %d/%d", choice.ID, choice.Kind)
	}
	if choice.Payload[0] != 1 || choice.Payload[1] != 2 || choice.Payload[5] != legion.DimCloisterOperationForcedChoice {
		t.Fatalf("B 帧 = % x, want 01 02 … %02x（业主口径：固定最高难度）",
			choice.Payload[:8], legion.DimCloisterOperationForcedChoice)
	}
	if !bytes.Equal(choice.Payload[20:25], choiceToken[:]) {
		t.Fatalf("B 帧 token = % x, want % x", choice.Payload[20:25], choiceToken)
	}
	if !s.dimCloisterWindowDeadline.IsZero() {
		t.Fatal("确认之后没有撤掉兜底关窗")
	}
}

// TestDimCloisterOperationFramesMatchFamily 钉住 A/B 形状与同族口径、逐界选择值。
func TestDimCloisterOperationFramesMatchFamily(t *testing.T) {
	token := [5]byte{1, 2, 3, 4, 5}
	open := legion.DimCloisterOperationOpenAck(1790893774, token)
	if len(open) != legion.DimCloisterOperationWindowAckSize {
		t.Fatalf("A 帧长度 %d", len(open))
	}
	if open[0] != 1 || open[1] != 1 || open[5] != 0xff || open[6] != 0xff || open[17] != 0xd0 {
		t.Fatalf("A 帧 = % x", open)
	}
	if binary.LittleEndian.Uint32(open[12:]) != 1790893774 {
		t.Fatalf("A 帧时间戳 = %d", binary.LittleEndian.Uint32(open[12:]))
	}
	if !bytes.Equal(open[20:25], token[:]) {
		t.Fatalf("A 帧 token = % x", open[20:25])
	}
	choice := legion.DimCloisterOperationChoiceAck(9, token)
	if choice[0] != 1 || choice[1] != 2 || choice[5] != 9 || choice[17] != 0xd0 {
		t.Fatalf("B 帧 = % x", choice)
	}
	if binary.LittleEndian.Uint32(choice[12:]) != 0 {
		t.Fatalf("B 帧 @12..15 应为 0（官服 #773 就是 0）: % x", choice[12:16])
	}
	// 业主 2026-10-11 定调：**固定最高难度**（9 = 选择界面最后那张卡）。
	//
	// ⚠️ 注意历史：那一档是**超越模式**（BOSS 三阶段、要走超越模式结束流程）；
	// 2026-10-10 曾因为它出现"打不死/不通关"，当时改成 7。现在业主明确要最高难度，
	// 所以断言跟随 `DimCloisterOperationForcedChoice`，不再钉死具体值。
	if legion.DimCloisterOperationForcedChoice == 0 {
		t.Fatal("本仓口径是固定难度，ForcedChoice 不应为 0")
	}
	want := legion.DimCloisterOperationForcedChoice
	if legion.DimCloisterOperationChoice(0) != want || legion.DimCloisterOperationChoice(2) != want {
		t.Fatalf("固定口径下选择值 = %d/%d, want %d/%d",
			legion.DimCloisterOperationChoice(0), legion.DimCloisterOperationChoice(2), want, want)
	}
	// 抓包原文只作对照：第 1 界的 A/B 与我们按同族口径生成的两帧，只差时间戳。
	capture := legion.DimCloisterOperationCaptureFrame(0)
	generated := legion.DimCloisterOperationOpenAck(binary.LittleEndian.Uint32(capture[0][12:]), [5]byte{0xb3, 0x81, 0x66, 0x02, 0x43})
	if !bytes.Equal(generated, capture[0]) {
		t.Fatalf("按同族口径生成的 A 帧与官服 #771 不一致:\n got % x\nwant % x", generated, capture[0])
	}
	// B 帧照样逐字节对照官服 #773（用抓包自己的 @5 值，避免被业主的固定难度口径影响）。
	if !bytes.Equal(legion.DimCloisterOperationChoiceAck(capture[1][5], [5]byte{0xa4, 0x52, 0x6e, 0xf2, 0x34}), capture[1]) {
		t.Fatalf("按同族口径生成的 B 帧与官服 #773 不一致:\n got % x\nwant % x",
			legion.DimCloisterOperationChoiceAck(capture[1][5], [5]byte{0xa4, 0x52, 0x6e, 0xf2, 0x34}), capture[1])
	}
	// 返回的必须是副本：改一份不能污染抓包常量。
	again := legion.DimCloisterOperationCaptureFrame(0)
	again[0][0] = 0x7f
	if legion.DimCloisterOperationCaptureFrame(0)[0][0] != 1 {
		t.Fatal("抓包对照帧是共享切片，改一份污染了常量")
	}
}

// TestDimCloisterOperationClearReplies 钉住 CMD2081 关窗应答。
func TestDimCloisterOperationClearReplies(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	w := &worldSession{}
	w.role.ID = 7
	result, err := s.dimCloisterOperationClear(w, decodeHex(t, realCmd2080b))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packets) != 1 || result.Packets[0].ID != legion.CmdDimCloisterOperationClear {
		t.Fatalf("关窗应答不对: %+v", result.Packets)
	}
	if result.Packets[0].Payload[3] != 1 {
		t.Fatalf("关窗位未置 1: %x", result.Packets[0].Payload)
	}
}

// TestDimCloisterWindowTimeoutQueuesClose 到期由待发队列兜底推 close（维纳斯同款）。
func TestDimCloisterWindowTimeoutQueuesClose(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	// 没排程：不入队。
	if s.dimCloisterQueueWindowClose() {
		t.Fatal("没有截止时刻却入了队")
	}
	// 已经过去的截止时刻：不入队（客户端那侧窗口早就该关了）。
	s.dimCloisterWindowDeadline = time.Now().Add(-time.Second)
	if s.dimCloisterQueueWindowClose() {
		t.Fatal("过期的截止时刻仍然入队")
	}
	// 未来截止时刻：入队一条到期事件。
	s.dimCloisterWindowDeadline = time.Now().Add(2 * time.Second)
	if !s.dimCloisterQueueWindowClose() {
		t.Fatal("有效截止时刻却没有入队")
	}
	if len(s.dimCloisterEvents) != 1 {
		t.Fatalf("队列长度 %d, want 1", len(s.dimCloisterEvents))
	}
	ev := s.dimCloisterEvents[0]
	if ev.kind != "dim_cloister_operation_window_timeout_closed" {
		t.Fatalf("事件类型 %q", ev.kind)
	}
	if len(ev.packets) != 1 || ev.packets[0].Payload[3] != 1 {
		t.Fatalf("到期关窗帧不对: %+v", ev.packets)
	}
	// 已经到期的那条必须能被 drain 出来。
	client := &gameConnection{worldState: &worldSession{}, legionState: *s}
	packets, events := client.dimCloisterEventsDue(time.Now().Add(3 * time.Second))
	if len(packets) != 1 || len(events) != 1 {
		t.Fatalf("drain = %d 帧 / %d 事件, want 1/1", len(packets), len(events))
	}
}

// TestDimCloisterStartSendsOfficialPrologue 钉住「开始作战」的三帧序：
// N2254 入场账本 → N2314 @3=01 横幅初态 → ACK2043，与官服 s30
// #752/#754/#755 逐帧同序，也与伊斯那条已验证的「先状态、后 ACK」同形。
//
// ★ 并且必须**排**一帧 @3=06（右上角 UI 点亮的那个状态）：2026-10-10 第三次实机
// 证明服务端不推它时 UI 永远不出现（等了 15~20 秒也没有）。此前点开 UI 闪退的
// 原因是**登录期的入场账本形态错了**（见 TestDimCloisterLoginLedgerOnlyForCloisterChannels），
// 与 @3=06 无关。
func TestDimCloisterStartSendsOfficialPrologue(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	w := &worldSession{}
	w.role.ID = 4242
	w.role.WireID = 4242
	result, windowFrames, err := s.dimCloisterStart(w, mustHex(t, officialCmd2043))
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Packets) != 3 {
		t.Fatalf("开始作战帧数 %d, want 3（账本 + 横幅初态 + 应答）", len(result.Packets))
	}
	if got := result.Packets[0]; got.Kind != 0 || got.ID != legion.NotiEntryCharacterInfo || len(got.Payload) != 272 {
		t.Fatalf("第 1 帧 = op%d/kind%d/%dB, want op2254/kind0/272B（官服 #752）",
			got.ID, got.Kind, len(got.Payload))
	}
	banner := result.Packets[1]
	if banner.Kind != 0 || banner.ID != legion.NotiDimCloisterInfo {
		t.Fatalf("第 2 帧 op/kind = %d/%d, want 0/%d（官服 #754）", banner.ID, banner.Kind, legion.NotiDimCloisterInfo)
	}
	if len(banner.Payload) < 4 || banner.Payload[3] != 0x01 {
		t.Fatalf("横幅初态 @3 = %02x, want 01", banner.Payload[3])
	}
	want, err := legion.DimCloisterInfoBody(legion.DimCloisterInfoHallInitial)
	if err != nil {
		t.Fatal(err)
	}
	// 官服原文 + 本角色当前的记忆组（记忆记录那 40 字节不再是抓包里那名玩家的）。
	if !bytes.Equal(banner.Payload, dimCloisterApplyMemoryRecords(want, s.dimCloisterMemoryRecords())) {
		t.Fatal("横幅初态不是官服原文 + 本角色记忆组")
	}
	ack := result.Packets[2]
	if ack.Kind != 1 || ack.ID != legion.CmdStart || len(ack.Payload) != legion.DimCloisterStartAckSize {
		t.Fatalf("第 3 帧 = op%d/kind%d/%dB, want op2043/kind1/%dB（官服 #755）",
			ack.ID, ack.Kind, len(ack.Payload), legion.DimCloisterStartAckSize)
	}

	// 倒计时结束后推 @3=06（右上角 UI）。不推它就永远没有 UI。
	if windowFrames != 1 {
		t.Fatalf("windowFrames=%d, want 1（少了它右上角 UI 不会出现）", windowFrames)
	}
	// 队列里两条：2.4s 的开窗帧 + 之后（玩家一直不点时）的兜底收尾帧。
	if len(s.dimCloisterEvents) != 2 {
		t.Fatalf("待发队列长度 %d, want 2（开窗 + 兜底收尾）", len(s.dimCloisterEvents))
	}
	ev := s.dimCloisterEvents[0]
	if ev.kind != "dim_cloister_info_opened" {
		t.Fatalf("事件类型 %q", ev.kind)
	}
	if len(ev.packets) != windowFrames {
		t.Fatalf("队列帧数 %d, 返回值 %d", len(ev.packets), windowFrames)
	}
	for _, pkt := range ev.packets {
		if pkt.Kind != 0 || pkt.ID != legion.NotiDimCloisterInfo {
			t.Fatalf("窗口帧 op/kind = %d/%d, want 0/%d", pkt.ID, pkt.Kind, legion.NotiDimCloisterInfo)
		}
		// 倒计时结束后推的必须是「开窗列表态」@3=06。
		if pkt.Payload[3] != 0x06 {
			t.Fatalf("窗口帧状态 %d, want 6（@3=06 才是右上角 UI）", pkt.Payload[3])
		}
	}
	// 兜底关窗帧必须是同族关窗 ACK（CMD2080 应答：Action1 + 关闭位 1），
	// 且排在开窗帧之后。用 @3=05 是错的：官服那份只有 place=02（副本内）形态，
	// 拿来关集结区的面板实测静默无效。
	idle := s.dimCloisterEvents[1]
	if idle.kind != "dim_cloister_info_idle_closed" {
		t.Fatalf("兜底事件类型 %q", idle.kind)
	}
	if !idle.at.After(ev.at) {
		t.Fatalf("兜底关窗帧没有排在开窗帧之后: %v vs %v", idle.at, ev.at)
	}
	if len(idle.packets) != 1 {
		t.Fatalf("兜底关窗帧数 %d, want 1", len(idle.packets))
	}
	closeAck := idle.packets[0]
	if closeAck.Kind != 1 || closeAck.ID != legion.CmdDimCloisterOperationSelect {
		t.Fatalf("兜底关窗帧 op/kind = %d/%d, want 1/%d", closeAck.ID, closeAck.Kind, legion.CmdDimCloisterOperationSelect)
	}
	if len(closeAck.Payload) != legion.DimCloisterOperationAckSize || closeAck.Payload[3] != 1 {
		t.Fatalf("兜底关窗帧没有置关闭位: %x", closeAck.Payload)
	}
}

// TestDimCloisterEventsDueDrainsInOrder 待发队列按到期顺序下发、未到期的留着。
func TestDimCloisterEventsDueDrainsInOrder(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	w := &worldSession{}
	w.role.ID = 1
	w.role.WireID = 1
	now := time.Now()
	s.dimCloisterEvents = []dimCloisterPendingEvent{
		{at: now.Add(-2 * time.Second), kind: "a", packets: []outboundPacket{{Name: "a", Kind: 0, ID: 2314, Payload: []byte{1}}}},
		{at: now.Add(time.Hour), kind: "b", packets: []outboundPacket{{Name: "b", Kind: 0, ID: 2314, Payload: []byte{2}}}},
	}
	c := &gameConnection{worldState: w, legionState: *s}
	pk, _ := c.dimCloisterEventsDue(now)
	if len(pk) != 1 || pk[0].Name != "a" {
		t.Fatalf("到期下发不对: %+v", pk)
	}
	if len(c.legionState.dimCloisterEvents) != 1 || c.legionState.dimCloisterEvents[0].kind != "b" {
		t.Fatalf("未到期项没留住: %+v", c.legionState.dimCloisterEvents)
	}
}

// TestDimCloisterAutoChoiceFollowsOfficialCadence 钉住「开窗 A 帧之后按官服节奏自动确认」。
//
// 依据：官服 s30 #771（开窗）→ #773（按值确认）相隔 3.7 秒；本客户端
// `MyresDimensionCloister.etc` 的 `[string data]` 被裁空 ⇒ 选择窗三张卡是空卡、
// 「选择记忆」按钮按不出选择值（业主 2026-10-10 实机：客户端一个请求都不发），
// 所以必须由服务端照官服节奏按下本界选择值，否则玩家进不了图。
// 客户端数据补齐、卡片能点之后，这条兜底要改成**不自动确认**。
func TestDimCloisterAutoChoiceFollowsOfficialCadence(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	w := &worldSession{}
	w.role.ID = 9
	w.role.WireID = 9
	if _, err := s.dimCloisterSelect(w, decodeHex(t, realCmd2080a)); err != nil {
		t.Fatal(err)
	}
	var choice, close *dimCloisterPendingEvent
	for i := range s.dimCloisterEvents {
		switch s.dimCloisterEvents[i].kind {
		case dimCloisterAutoChoiceKind:
			choice = &s.dimCloisterEvents[i]
		case dimCloisterTimeoutCloseKind:
			close = &s.dimCloisterEvents[i]
		}
	}
	if choice == nil || close == nil {
		t.Fatalf("开窗后待发队列 = %+v, want 自动确认 + 兜底关窗", s.dimCloisterEvents)
	}
	// 业主口径：难度选择界面**停留 3 秒**后自动按最高难度确认。
	if d := choice.at.Sub(time.Now()); d < 2*time.Second || d > 4*time.Second {
		t.Fatalf("自动确认间隔 %v, want ≈3s（业主 2026-10-10 定的停留时长）", d)
	}
	if len(choice.packets) != 1 || choice.packets[0].Payload[1] != 2 ||
		choice.packets[0].Payload[5] != legion.DimCloisterOperationForcedChoice {
		t.Fatalf("自动确认帧 = %+v, want B 帧（@1=2、@5=%02x 固定难度）",
			choice.packets, legion.DimCloisterOperationForcedChoice)
	}
	// 自动确认到期后：下发 B 帧，并且把还没到期的兜底关窗丢掉。
	c := &gameConnection{worldState: w, legionState: *s}
	pk, _ := c.dimCloisterEventsDue(time.Now().Add(dimCloisterChoiceDelay + time.Millisecond))
	if len(pk) != 1 || pk[0].Name != "dim_cloister_operation_choice_ack" {
		t.Fatalf("到期下发 = %+v, want B 帧", pk)
	}
	for _, ev := range c.legionState.dimCloisterEvents {
		if ev.kind == dimCloisterTimeoutCloseKind {
			t.Fatalf("自动确认之后兜底关窗没被丢掉: %+v", ev)
		}
	}
}

// TestDimCloisterStartDoesNotReplayForeignWindowBytes 钉住「开始作战的 N2314 一律是
// 官服原文」：官服那批窗口字节里只有 N2314 可以被回放，N9/N1539/N2315 一律不发
// （N9 的官服新版布局会把 2.38.2 读越界，见 ispins_party.go 的同类记录）。
func TestDimCloisterStartDoesNotReplayForeignWindowBytes(t *testing.T) {
	s := &legionSession{channelType: legion.DimCloisterChannelType}
	w := &worldSession{}
	w.role.ID = 4242
	w.role.WireID = 4242
	result, _, err := s.dimCloisterStart(w, mustHex(t, officialCmd2043))
	if err != nil {
		t.Fatal(err)
	}
	for _, pkt := range result.Packets {
		if pkt.Kind == 0 && (pkt.ID == 9 || pkt.ID == 1539 || pkt.ID == legion.NotiDimCloisterOperation) {
			t.Fatalf("开始作战回放了不该发的帧 op=%d", pkt.ID)
		}
	}
	for _, ev := range s.dimCloisterEvents {
		for _, pkt := range ev.packets {
			if pkt.Kind == 0 && (pkt.ID == 9 || pkt.ID == 1539 || pkt.ID == legion.NotiDimCloisterOperation) {
				t.Fatalf("窗口批次回放了不该发的帧 op=%d", pkt.ID)
			}
		}
	}
}
