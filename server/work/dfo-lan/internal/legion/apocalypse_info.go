package legion

import "encoding/binary"

// 末世录 NOTI2895 LEGION_INFO 的状态值。
//
// 证据：抓包 D:\zhuabao\captures\20261008-105227（难度1/难度2 各一场全清）。
// 做法是把该会话全部 22 条 NOTI2895 对齐求差，只有下列**载荷下标**变化
// （载荷 = u16 内容号 [0:2] + 状态体，本仓库的 LegionInfoBody = 204 字节，
// 故载荷下标 = 体下标 + 2）：
//
//	@4    难度选择：起始 0xff(未选)，CMD2354 确认后取 0/1（= 客户端键，
//	      CTP 查表用 choice+1）。
//	@5    State：进频道 2，通关 CMD2046 后回到 0。
//	@9    Follow：进城镇/频道为 FFFFFFFF；刚载入某一关那一刻填该关卡号；
//	      该关的击杀确认包把它清零。
//	@13   Stage：进频道 0，攻坚房间载入后 1，之后每个关卡 +1，直到 6。
//	@29/@41/@53/@65/@77/@89  六个阶段的「已到达」标记：0xff 未到达，
//	      否则写阶段号 0..5，与 @13 同步递增。
//	@109  角色计数：CMD2355 之前 0，之后 1，结算后回 0。
//
// 抓包载荷是 208 字节（体 206），比本仓库的 204 字节体多 2 字节；逐字节比对
// 确认差异只落在尾部填充，@4..@109 的所有字段位置两边一致，因此本文件按
// 本仓库既有 LegionInfoBody=204 的体长编码（见 apocalypse_info_test.go 的
// 说明与守卫）。
//
// legion_info.go 的通用构造器不写 @4 的难度、@29.. 的阶段标记和 @109 的
// 角色计数（保持客户端构造初值 0），所以末世录进图后右上角面板的阶段与角色
// 一直是空的。本文件只新增末世录专用构造器，不改 legion_info.go，避免影响
// 维纳斯/伊斯/苏醒之森（它们的 N2655/N2255 是另一套模板）。

// ApocalypseInfoState 是末世录 LEGION_INFO 的完整状态。
type ApocalypseInfoState struct {
	LegionInfoState
	// RoleCount is the assigned-role counter at payload @109 (body @107).
	// This is the only role field the capture ever changes.
	RoleCount byte
	// StageMarks is the six-slot reach marker at payload @29+12*i (body
	// @27+12*i): 0xff = not reached, otherwise the stage number.
	StageMarks [6]byte
	// TargetMarks 是六个目标索引的「已达成」标记（体 @126+4*i / 载荷 @128+4*i）。
	// 规格 G0382：14069A750 检验它非 0 才认这一关已打；客户端据此重建已清列表。
	TargetMarks [6]uint32
	// TargetElapsedMS 是与 TargetMarks 配对的耗时（毫秒，体 @150+8*i /
	// 载荷 @152+8*i）。G0382：标记非 0 时用它冻结剩余时间。
	TargetElapsedMS [6]uint64
	// RoleShortcut 是体 @123..126（载荷 @125..128）的四个**角色快捷位**。
	//
	// `legion_info.go` 的通用构造器已经把它们预置成 `01 01 01 01`，而本文件
	// 此前**没有覆写**，所以现状是对的；这里显式留一个字段，便于以后按席位
	// 精确控制（2 号全清帧也是 `01 01 01 01`）。
	RoleShortcut [4]byte
}

// ApocalypseWaitingInfo 是刚进频道（CMD2043 应答）的等待态：State2/Outcome0/
// Stage0/FollowFFFFFFFF，无角色、阶段标记全 0xff。与抓包 @33.40s 的载荷
// 字段逐个一致。
func ApocalypseWaitingInfo() ApocalypseInfoState {
	s := ApocalypseInfoState{LegionInfoState: WaitingLegionInfoState()}
	for i := range s.StageMarks {
		s.StageMarks[i] = 0xff
	}
	return s
}

// ApocalypseClosedInfo 是「关闭态」N2895：State0，没有难度、没有阶段标记。
//
// 用途见规格 0072-结算离场.md / 2895-LEGIONINFO末世录状态.md 的 G0452：
// 「末世录未完成撤退**先 State0 清旧副本/界面**，再在本人城镇重建后恢复 State2
// 和原 Choice/Stage/目标通关标记」。客户端在 State0 收起军团窗口并清除选择标记
// （14069ABF0 的语义），随后那一帧 State2 再把面板按原进度恢复。
func ApocalypseClosedInfo() ApocalypseInfoState {
	s := ApocalypseInfoState{LegionInfoState: WaitingLegionInfoState()}
	s.State = 0
	for i := range s.StageMarks {
		s.StageMarks[i] = 0xff
	}
	return s
}

// ApocalypseCompletedInfo 是**通关之后**的 N2895 关闭态：State0 收起面板，
// 但**保留本次通关的阶段记录**。
//
// 与 ApocalypseClosedInfo 的区别（两者都用于「清界面」，但形态不同）：
//
//	ApocalypseClosedInfo   State0 + 阶段记录全 0xff + Stage0
//	                       用途：**撤退**时「清旧副本/界面」（规格 G0452 第一步）
//	ApocalypseCompletedInfo State0 + 阶段记录 [0..cleared] + Stage=cleared
//	                       用途：**通关**结算完成后的关闭帧
//
// 依据是 2 号权威抓包 `D:\zhuabao\captures\20261008-184805` 的 idx=654
// （CMD2046 之后那一帧）：
//
//	@4..8   = 01 00 00 00   State = 0，但 choice 仍是 01（本次难度）
//	@9..12  = 05 00 00 00   Outcome = 5（本字段，**不是 Following**）
//	@13..16 = 05 00 00 00   Stage = 5（本次打到第 5 关）
//	@29+12i = [0,1,2,3,4,5] 六条阶段记录**保留**
//	@128+   = [1,0,0,0,0,0] 目标标记保留
//
// 本实现此前通关后发的是**等待态 State2**（`apocalypseInfo()` 走过 run.Reset()
// 之后），客户端因此**保留**军团面板 —— 业主 2026-10-08 报「翻牌结束后右上角
// UI 还在、退出也有 UI 残留」就是这个成因。
func ApocalypseCompletedInfo(choice byte, cleared int) ApocalypseInfoState {
	s := ApocalypseInfoState{LegionInfoState: WaitingLegionInfoState()}
	s.OperationChoice = choice
	s.State = 0
	// Stage 与 Outcome 都停在**刚打完的那一间**（= 已清关数）；
	if cleared < 0 {
		cleared = 0
	}
	s.Stage = uint32(cleared)
	// Outcome（载荷 @9..12）—— 权威抓包 idx=654 该位是 05，与 Stage 一致。
	// Following 在载荷 @17 且恒为 ffffffff，不需要写。
	s.Outcome = uint32(cleared)

	// 阶段记录保留到刚打完的那一间（与清关帧同形）。
	s.StageMarks = ApocalypseMarksFor(cleared + 1)
	s.TargetMarks[0] = 1
	return s
}

// ApocalypseFinalInfo 是**触发通关视频**的终局态。
//
// 家族约定（维纳斯 `VenusFinalInfo` 的原文注释）：
//
//	「VenusFinalInfo is the terminal state that **triggers the clear movie**:
//	  family convention with the ispins N2255 "final" (**State 3, Outcome 1**)…
//	  Sent with the terminal CMD2046 ACK — the client then plays the clear movie.」
//
// 末世录此前**完全没有这一帧**，所以全清之后不进终局演出
// （业主 2026-10-08：「通关视频：全清 → 翻牌链之后，没有进入终局演出/通关视频」）。
//
// 与另两帧的分工（照维纳斯的四态）：
//
//	本函数（State3）演出**开始**
//	ApocalypseLeaveInfo（State5）演出**结束**、收起右上角面板
//	ApocalypseCompletedInfo（State0 + ChoiceFF）CMD72 回城镇、整局结束
func ApocalypseFinalInfo(choice byte, endpoint int) ApocalypseInfoState {
	s := ApocalypseInfoState{LegionInfoState: WaitingLegionInfoState()}
	s.OperationChoice = choice
	s.State = 3
	s.Outcome = 1
	if endpoint < 0 {
		endpoint = 0
	}
	s.Stage = uint32(endpoint)
	s.StageMarks = ApocalypseMarksFor(endpoint + 1)
	s.TargetMarks[0] = 1
	return s
}

// ApocalypseLeaveInfo 是**通关演出播完之后**的 N2895 leave 态。
//
// 家族约定（维纳斯 `VenusLeaveInfo` 的原文注释）：
//
//	「VenusLeaveInfo closes the operation panel once the clear movie has finished
//	  (family convention with the ispins N2255 "leave": **State 5, Outcome 1**) —
//	  the top-right panel and the relic display disappear for the finished run.」
//
// 也就是军团家族的「终局演出结束」统一用 **State5 + Outcome1**；末世录此前
// **完全没有这一帧**，所以通关演出 → 收起面板这条链路缺了一环
// （业主 2026-10-08：「通关视频没进入终局演出」「翻牌结束后右上角军团 UI 没有消失」）。
//
// 与 `ApocalypseCompletedInfo`（State0 + ChoiceFF）的分工，照维纳斯：
//
//	本函数        演出结束、收起右上角面板，**保留本次难度与阶段**
//	CompletedInfo CMD72 返回城镇时发，整局结束、choice 归 FF，NPC 不再挂「开始」
func ApocalypseLeaveInfo(choice byte, cleared int) ApocalypseInfoState {
	s := ApocalypseInfoState{LegionInfoState: WaitingLegionInfoState()}
	s.OperationChoice = choice
	s.State = 5
	s.Outcome = 1
	if cleared < 0 {
		cleared = 0
	}
	s.Stage = uint32(cleared)
	s.StageMarks = ApocalypseMarksFor(cleared + 1)
	s.TargetMarks[0] = 1
	return s
}

// ApocalypseMarksFor 返回 NOTI2895 的六个阶段到达标记。
//
// 入参 filled 是「要填满的格子数」，规则与抓包逐帧对齐：
// slot0 在进入副本流程后就是 00，之后**每到达一个新阶段多填一格**，
// 也就是 filled = max(1, 已到达的阶段号 - 1)。填不满的格子保持 0xff。
//
// 实测锚点（2026-10-08，tools/apocalypse-port/infofields.py 逐帧解出）：
//
//	filled=0 → ff ff ff ff ff ff   （进频道、选难度时）
//	filled=1 → 00 01 ff ff ff ff   （CMD2045 之后 / 攻坚房间载入后 / 第1关载入）
//	filled=3 → 00 01 02 ff ff ff   （第1关清完 / 第2关载入后，抓包 82.83s）
//	filled=4 → 00 01 02 03 ff ff
//	filled=5 → 00 01 02 03 04 ff   （第4关清完，抓包 stage=5 那一帧）
//	filled=6 → 00 01 02 03 04 05   （第5关清完，终局）
func ApocalypseMarksFor(filled int) [6]byte {
	var marks [6]byte
	n := filled
	if n > 6 {
		n = 6
	}
	for i := range marks {
		if i < n {
			marks[i] = byte(i)
		} else {
			marks[i] = 0xff
		}
	}
	return marks
}

// ApocalypseOperationAck 构造 CMD2354 的成功响应：公共成功标志 + 内容号 + 14B 操作记录。
//
// 布局严格按规格 D:\115US-001\moshilu\10-军团末世录专有\2354-LEGIONOPERATION成功响应.md：
//
//	@0    1B  公共成功标志（成功 1）
//	@1..2 2B  内容号 107（u16 LE）
//	@3    14B 操作记录：
//	          u32@0（= 正文 @3..6）  Action 1/2
//	          byte@5（= 正文 @8）    **非 0 走关闭/取消分支**
//	          u32@6（= 正文 @9..12） **選択窗倒计时截止值**（子面板+1000）
//	          其余字段沿现实现 0
//
// 截止值必须下发，否则难度选择框的倒计时恒为 0、**永不自动关闭**：
// 业主 2026-10-08 报「框内没有关闭按钮，不选难度就卡死」，而维纳斯的同类框
// 倒计时走完会自动关 —— 差别就在维纳斯 `VenusOperationAck` 带了截止值
// （`venus_flow.go`：`deadline = now + VenusSelectionSeconds`），维纳斯注释也写明
// 「客户端自己对 0 不做任何事」。
//
// ⚠️ 位移与维纳斯**不同**，不能照搬：那是 CMD2290 的布局（`p[7:11]`）。
// 本字段在 CMD2354 里是**正文 @9..12**。此前一次失败正是把 deadline 写到正文
// @7..10、close 写到 @11，整体提前 2B，压掉了真正的关闭位 byte@5（正文 @8），
// 客户端因此走取消分支、点 Open 完全没反应（实机 2026-10-08 17:32：连发 9 次
// action1）。**当时的结论「客户端不认这个字段」是错的 —— 是偏移写错了。**
//
// 另注：2 号抓包 20261008-105227 的 action1 该字段为 0，与旧实现逐字节一致；
// 但 2 号的难度框同样不会自动关闭（业主确认），所以**那一帧是"带 BUG 的参考"**，
// 不能拿它当"字段应为 0"的依据。唯一下发口径来自规格 + 维纳斯的可用先例。
//
// 时基：规格自标「未假定其时基，仍需单独校准」。此处与维纳斯同口径取
// **绝对 UNIX 秒**（`uint32(time.Now().Unix()) + 秒数`），同族客户端很可能一致，
// 但**必须以实机校准为准**（规格）：若客户端显示异常，第一嫌疑即时基。
func ApocalypseOperationAck(action uint32, deadlineSeconds uint32) []byte {
	body := successfulReply(OperationAckSize)
	binary.LittleEndian.PutUint16(body[1:], ApocalypseContent)
	binary.LittleEndian.PutUint32(body[3:], action)
	// 操作记录 @6..9 = 正文 @9..12：選択窗倒计时截止值。0 = 客户端不显示计时、
	// 也不自动关窗，所以开窗（action1）必须给非 0；确认（action2）给 0 表示
	// 「窗已在处理，不需要计时」。
	binary.LittleEndian.PutUint32(body[9:], deadlineSeconds)
	return body
}

// ApocalypseOperationClose 构造 CMD2354 的**关闭/取消**响应：与成功响应同布局，
// 但 Action 保留为 1、并把操作记录的 `byte@5`（正文 `@8`）置 1。
//
// ★ Action 必须是 **1**（开窗动作），不能是 0。
// 原生 reader 的关闭分支判据是「**Action1 + close=1**」—— 维纳斯同款可用先例：
// `venusOperationClose` 推的是 `VenusOperationAck(1, true, 0, 0)`，其注释明写
// 「原生 reader 对 Action1 close=1 转关闭分支」，并记录了「客户端对归 0 自己
// 不做任何事（2026-10-05 实测：窗口停在 0），官服由服务端在截止时刻关闭」。
//
// 实机 2026-10-08（本轮）：本函数原先把 Action 留 0，服务端 15 秒**确实发了**
// 这一帧（`events.jsonl`：`apocalypse_select_window_closed`
// `016b000000000000010000000000000000`），客户端却完全不理 ——
// 表现正是业主报的「倒计时走到 0 之后什么都没有发生」。
// 关闭位本身没写错，错的是没有和 Action1 配对。
//
// 依据同 ApocalypseOperationAck 顶部引的规范：「内部 byte@5 非 0 走**关闭/取消**
// 分支」+ 「内部 u32@0 = Action1/2」。业主 2026-10-08 要求「识别到难度选择框，
// 15 秒后没有选择难度就强制关闭」，用的就是这一对。
func ApocalypseOperationClose() []byte {
	body := successfulReply(OperationAckSize)
	binary.LittleEndian.PutUint16(body[1:], ApocalypseContent)
	// Action=1：与开窗帧同一个 action，关闭位才是它的分支。
	binary.LittleEndian.PutUint32(body[3:], 1)
	// 操作记录 @5（正文 @8）= 关闭/取消位。
	body[8] = 1
	return body
}

// ApocalypseInfo 按状态构造 NOTI2895 载荷。
func ApocalypseInfo(state ApocalypseInfoState) []byte {
	p := LegionInfo(ApocalypseContent, state.LegionInfoState)
	b := p[2:]
	b[2] = state.LegionInfoState.OperationChoice
	for i, m := range state.StageMarks {
		b[27+12*i] = m
	}
	// 目标索引标记 / 耗时：**体 @126+4*i（载荷 @128+4*i）** 与
	// **体 @150+8*i（载荷 @152+8*i）**。规格 2895-LEGIONINFO末世录状态.md：
	//
	//	「G0382已确认内部@128..151为六个目标索引u32标记/计数，
	//	  @152..199为六个目标索引u64耗时（毫秒）。14069A750检验前者非0；
	//	  140699AE0/140699FF0在非0时用后者冻结剩余时间。」
	//
	// 这两段是客户端**重建「已经打过哪几关」**的依据：只发 Stage 而不发标记时，
	// 客户端在撤退回城后仍认为一关都没打，继续时发 CMD2045(stage 0)，服务端按
	// 保存阶段拒绝 —— 实机 2026-10-08 16:24 的
	// `legion_refused id=2045 reason="apocalypse resume stage 3 does not match the saved stage 0"`
	// 就是这个成因。
	for i := 0; i < 6 && i < len(state.TargetMarks); i++ {
		binary.LittleEndian.PutUint32(b[126+4*i:], uint32(state.TargetMarks[i]))
	}
	for i := 0; i < 6 && i < len(state.TargetElapsedMS); i++ {
		binary.LittleEndian.PutUint64(b[150+8*i:], state.TargetElapsedMS[i])
	}
	// 角色计数 @107（载荷 @109）：抓包中该字节在 CMD2355 之后变为 1，
	// 其余角色字节保持 0。
	b[107] = state.RoleCount
	// 四个角色的快捷位：体 @123..126（载荷 @125..128）。通用构造器已预置
	// 01 01 01 01（与 2 号全清帧一致），只在显式给了值时覆写。
	for i := 0; i < 4 && i < len(state.RoleShortcut); i++ {
		if state.RoleShortcut[i] != 0 {
			b[123+i] = state.RoleShortcut[i]
		}
	}
	// ★ 末世录 N2895 的字段位置（2026-10-08 用 2 号权威抓包
	// D:\zhuabao\captures\20261008-184805 逐字节校准；「载荷下标 = 体下标 + 2」，
	// 本文件用 b = p[2:]）：
	//
	//	载荷 @4   choice        u8   未选 ff → 已选 00/01
	//	载荷 @5..8   **State**     u32  进行中 02 → 结算后 00   （b[3]）
	//	载荷 @9..12  **Outcome**   u32  普通 00 → 终局清关 05   （b[7]）
	//	载荷 @13..16 Stage       u32  逐关 00→01→…→05         （b[11]）
	//	载荷 @17..20 Following   u32  **整场恒定 ffffffff**     （b[15]）
	//	载荷 @29+12i 六条阶段记录（b[27]）
	//
	// ⚠️ 本文件此前把 **Outcome 与 Following 弄反了**：曾用 `FollowOverride` 去写
	// 载荷 @9，而那一格其实是 **Outcome**（规格 G0454 的终局投影字段），真正的
	// Following 在载荷 @17 且**恒为 ffffffff、从来不需要写**。
	// 权威实测（三次采样）：
	//
	//	idx=335  @5..8=02000000  @9..12=00000000  @13..16=00000000  @17..20=ffffffff
	//	idx=640  @5..8=02000000  @9..12=05000000  @13..16=05000000  @17..20=ffffffff
	//	idx=654  @5..8=00000000  @9..12=05000000  @13..16=05000000  @17..20=ffffffff
	//
	// Outcome 直接复用 `LegionInfoState.Outcome`（legion_info.go 已有字段），
	// 不需要新增；G0454 要求的「N2252 前 Outcome0、N2253 后 Outcome3」由
	// `apocalypseTerminalEndpointInfo` 设置。
	//
	// Following 保持构造初值（ffffffff），本文件不写。
	return p
}
