package legion

import "encoding/binary"

// 次元回廊 CMD2080 应答 = **同族（伊斯/维纳斯/末世录）的 A/B 两帧 32B**。
//
// 依据（业主 2026-10-10 指路「伊斯/苏醒之森/维纳斯/末世录都有难度选择框，可以参考」）：
// 本仓这三族都是现成可用实现，形状逐字节对得上：
//
//	IspinsOperationAckA  01 01 000000 00 ffff 00000000 <LE unix 秒> 00 f0 0000 <5B token> 00*7
//	IspinsOperationAckB  01 02 000000 00 <aux 选择值> …        00 f0 0000 <5B token> 00*7
//
// 官服 s30 抓包里次元回廊那两帧（#771/#773）就是这一对，只差 @17 是 d0、token 每界不同：
//
//	#771 01 01 000000 00 ffff 00000000 00 ce fe c8 6a 00 d0 0000 b3 81 66 02 43 00*7   ← A：开窗
//	#773 01 02 000000 00 07   00000000 00 00000000 00 d0 0000 a4 52 6e f2 34 00*7      ← B：选择值 7
//
// 所以语义不是「两条候选项」，而是**A=开窗（还没选，@5..6=ffff）、B=已选（@5=选择值）**；
// 两帧在官服里相隔 3.7 秒（#771 22:47:28.354 → #773 22:47:32.096），
// 也就是「服务端先开窗、随后按下默认值确认」——这也解释了业主实机看到的
// 「难度框一闪而过、自动选了难度」（本仓原来把两帧挤在同一批里发）。
//
// 三界的选择值（抓包 @5，与 .dgn 的难度/已清界数一致）：第 1/2 界 = 7，第 3 界 = 9。

// DimCloisterOperationWindowAckSize 是 CMD2080 应答正文长度（A/B 同为 32B）。
const DimCloisterOperationWindowAckSize = 32

// DimCloisterOperationOpenAck 构造 A 帧：开窗（尚未选择）。
//
// unixSeconds 是 @12..15 的 LE unix 秒，口径与 IspinsOperationAckA/VenusOperationAck 一致
// （同族里这一格就是时间戳，客户端据它显示倒计时）。
func DimCloisterOperationOpenAck(unixSeconds uint32, token [5]byte) []byte {
	p := make([]byte, DimCloisterOperationWindowAckSize)
	p[0], p[1] = 1, 1
	p[5], p[6] = 0xff, 0xff
	binary.LittleEndian.PutUint32(p[12:], unixSeconds)
	p[17] = 0xd0
	copy(p[20:], token[:])
	return p
}

// DimCloisterOperationChoiceAck 构造 B 帧：已选（choice = 难度值，落在 @5）。
func DimCloisterOperationChoiceAck(choice byte, token [5]byte) []byte {
	p := make([]byte, DimCloisterOperationWindowAckSize)
	p[0], p[1] = 1, 2
	p[5] = choice
	p[17] = 0xd0
	copy(p[20:], token[:])
	return p
}

// dimCloisterOperationTokens 是抓包里每界 A/B 两帧的 @20..24 五字节。
// 语义未回收（同族里是会话/校验串），按已清界数照抄官服值。
var dimCloisterOperationTokens = [][2][5]byte{
	{{0xb3, 0x81, 0x66, 0x02, 0x43}, {0xa4, 0x52, 0x6e, 0xf2, 0x34}}, // 第 1 界
	{{0x0a, 0xbe, 0x53, 0xfc, 0x36}, {0xa4, 0x52, 0x6e, 0xf2, 0x34}}, // 第 2 界
	{{0x44, 0x38, 0x78, 0x7e, 0x3c}, {0xe4, 0xfb, 0x64, 0xdb, 0x34}}, // 第 3 界
}

func dimCloisterOperationGroup(cleared int) int {
	if cleared < 0 {
		return 0
	}
	if cleared >= len(dimCloisterOperationTokens) {
		return len(dimCloisterOperationTokens) - 1
	}
	return cleared
}

// DimCloisterOperationTokens 返回第 cleared 界的 A/B 两帧 token（越界夹到最后一界）。
func DimCloisterOperationTokens(cleared int) (open, choice [5]byte) {
	g := dimCloisterOperationGroup(cleared)
	return dimCloisterOperationTokens[g][0], dimCloisterOperationTokens[g][1]
}

// DimCloisterOperationForcedChoice 是**业主要的口径开关**：非 0 时所有界都固定用这个难度值。
//
// ★ 2026-10-11 业主定调：**调成最高难度** ⇒ 固定 **9**（官服那 9 档里的最高档，
// 也就是选择界面最后那张卡）。
//
// ⚠️ 历史教训（2026-10-10，业主实机）：最后那张卡是**超越模式**——选它 BOSS 是**三阶段**，
// 且要走超越模式的结束流程；而**正常难度是两阶段、打死即通关**。
// 当时把它设成 9，出现的就是「BOSS 剩一条血打不死 / 消失却不通关」。
// 本次业主明确要求最高难度，所以设 9；若再出现"打不死/不通关"，
// 就把它改回 7（正常难度），那是这一档的固有行为，不是别的 bug。
//
// 官服 s30 抓包：第 1/2 轮玩家选的是 7，第 3 轮是 9。
// 想恢复"逐界官服实测值"（第1/2界 7、第3界 9）就设回 0。
const DimCloisterOperationForcedChoice byte = 9

// DimCloisterOperationChoice 返回第 cleared 界的难度选择值（抓包 @5）。
func DimCloisterOperationChoice(cleared int) byte {
	if DimCloisterOperationForcedChoice != 0 {
		return DimCloisterOperationForcedChoice
	}
	g := dimCloisterOperationGroup(cleared)
	if g == 2 {
		return 9
	}
	return 7
}

// DimCloisterOperationOpenFrame 返回第 cleared 界的 A 帧（开窗，带当前时间戳）。
func DimCloisterOperationOpenFrame(cleared int, unixSeconds uint32) []byte {
	open, _ := DimCloisterOperationTokens(cleared)
	return DimCloisterOperationOpenAck(unixSeconds, open)
}

// DimCloisterOperationChoiceFrame 返回第 cleared 界的 B 帧（已选，选择值见 DimCloisterOperationChoice）。
func DimCloisterOperationChoiceFrame(cleared int) []byte {
	_, choice := DimCloisterOperationTokens(cleared)
	return DimCloisterOperationChoiceAck(DimCloisterOperationChoice(cleared), choice)
}

// DimCloisterEnableClearDungeon 是官服 s30 #804 `N31 ENABLE_CLEAR_DUNGEON` 的 16B 正文
// （逐字节照抄：`413a0000 6f13b397 3b000000 00000000`）。
//
// 它是"允许通关"的闸门：官服在客户端进图（CMD2045 #476，22:48:29.995）之后
// 0.5 秒（22:48:30.544）发它；本仓原先的进图帧列漏了这一帧，实机表现就是
// 业主报的「BOSS 转阶段后锁住 1 条血、打上去没有伤害数字」。
func DimCloisterEnableClearDungeon() []byte {
	return []byte{0x41, 0x3a, 0x00, 0x00, 0x6f, 0x13, 0xb3, 0x97, 0x3b, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

// DimCloisterOperationCaptureFrames 是官服 s30 那三组原文（A、B），仅作取证/对照用：
//
//	第 1 界 #771/#773、第 2 界 #846/#847、第 3 界 #927/#928。
var DimCloisterOperationCaptureFrames = [][2][32]byte{
	{
		{0x01, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0xce, 0xfe, 0xc8, 0x6a,
			0x00, 0xd0, 0x00, 0x00, 0xb3, 0x81, 0x66, 0x02, 0x43, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x01, 0x02, 0x00, 0x00, 0x00, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0xd0, 0x00, 0x00, 0xa4, 0x52, 0x6e, 0xf2, 0x34, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	},
	{
		{0x01, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x04, 0xff, 0xc8, 0x6a,
			0x00, 0xd0, 0x00, 0x00, 0x0a, 0xbe, 0x53, 0xfc, 0x36, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x01, 0x02, 0x00, 0x00, 0x00, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0xd0, 0x00, 0x00, 0xa4, 0x52, 0x6e, 0xf2, 0x34, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	},
	{
		{0x01, 0x01, 0x00, 0x00, 0x00, 0xff, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46, 0xff, 0xc8, 0x6a,
			0x00, 0xd0, 0x00, 0x00, 0x44, 0x38, 0x78, 0x7e, 0x3c, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		{0x01, 0x02, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0xd0, 0x00, 0x00, 0xe4, 0xfb, 0x64, 0xdb, 0x34, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	},
}

// DimCloisterOperationCaptureFrame 返回第 cleared 界的官服原文（A、B 各一份副本）。
func DimCloisterOperationCaptureFrame(cleared int) [2][]byte {
	g := dimCloisterOperationGroup(cleared)
	return [2][]byte{
		append([]byte(nil), DimCloisterOperationCaptureFrames[g][0][:]...),
		append([]byte(nil), DimCloisterOperationCaptureFrames[g][1][:]...),
	}
}
