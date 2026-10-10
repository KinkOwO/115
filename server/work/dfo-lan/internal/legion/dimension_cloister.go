package legion

import (
	"encoding/binary"
	"fmt"
	"time"
)

// 次元回廊（Dimension Cloister）—— 军团家族的第三个内容族。
//
// 与末世录（内容 107 / 频道 Type 119）和苏醒之森（内容 104/105 / 频道 86/87）不同，
// 次元回廊在**集结区里直接选关进图**：客户端不发 CMD2354/CMD2355，而是在
// CMD2043（开始作战）里把「第几关」放在信封后的第一个 u32 上。
//
// 官服抓包（D:\115US-001\DFO-115US-zhuabao\official_20261009-223033_live /
// session_s30）三帧 CMD2043 实证，明文体（信封 13B 之后）：
//
//	#461  00 00 00 00 | 0d 00 00 00 | 66 00 00 00 | 00 00 00 00 …   stage=0 房间=0x66
//	#504  00 00 00 00 | 0d 00 00 00 | 66 00 00 00 | 01 00 00 00 …   stage=1
//	#542  00 00 00 00 | 0d 00 00 00 | 66 00 00 00 | 02 00 00 00 …   stage=2
//
// 服务端随后下发的 N28（DUNGEON_INFO）给出三关的目标副本号：
//
//	#784  22:48:02.515  N28 = 100003195  第1界-神界    contents/2022/dimensioncloister/dungeon/01_seon
//	#858  22:49:2x      N28 = 100003190  第4界-魔界    …/04_evildom
//	#940  22:50:2x      N28 = 100003185  第5界-冥界    …/05_underworld
//
// 名字与路径来自内层 PVF 生成的副本目录；军团内容块见
// configs/legion-contents.generated.json 的 DimensionCloister（5 个 boss 图）。
const (
	// DimCloisterChannelType 是客户端 clientchannelinfo 里 Evildom 行的 Type
	// （configs/channel.local35.json：ID 50 / Evildom / Type 50）。军团家族靠频道
	// Type 选中要走的那条链，与末世录的 119 同一机制。
	DimCloisterChannelType uint32 = 50

	// DimCloisterHallChannelType 是 **实机真正走的那一行**：ID 84 /
	// Hall of Dimensions / Type 84（客户端的频道名是
	// `CHANNEL_DIMENSION_CLOISTER_LEGION`，见 2026-10-10 会话
	// `channel_town_spawn channel=84 channel_type=84`）。
	//
	// 两行都指向同一个内容号 0x0d，所以**请求分流一律用内容身份**（队伍类型 /
	// 内容号），但「这条连接是不是次元回廊」的问答（例如登录期该推哪一族的
	// 入场账本）必须认得 84，否则会把伊斯那一族的 N2254 当成次元回廊发出去。
	DimCloisterHallChannelType uint32 = 84

	// DimCloisterContent 是 CMD2043 信封 @17 上的内容/模式标记（抓包三帧都是 0x0d）。
	// 由客户端固定写出，服务端只做校验与日志，不用它分派。
	DimCloisterContent uint32 = 13

	// DimCloisterPartyType 是抓包里该频道的队伍类型字节（CMD2043 @21 / CMD2045 @12）。
	DimCloisterPartyType byte = 0x66
)

// DimCloisterStageDungeons 是三关的目标副本号，下标即客户端 CMD2043 的 stage。
//
// 官服实证：stage 0 → 100003195（神界）、stage 1 → 100003190（魔界）、
// stage 2 → 100003185（冥界）。顺序**不是**按副本号排列（客户端让玩家自己选
// 打哪几界），所以这张表必须与抓包一致，不能按 ID 排序。
var DimCloisterStageDungeons = [...]uint32{100003195, 100003190, 100003185}

// DimCloisterAllStageDungeons 是这条内容**全部 5 个界**的副本号（客户端 N2314 正文里正好 5 行：// 100003180 Moros / 100003185 Charon / 100003190 Abyss / 100003195 TheMan / 100003200 LightWoman，
// 与 `etc/105lvAbility/DimensionCloister/` 下 5 个 BOSS 的命名一一对应）。
//
// 官方口径是"每周从 5 个里出 3 个"（`MyresDimensionCloister.etc` 的
// `[weekly dungeon count] 3` + `[week count] 5` + `[weekly dungeon info]` 那张 5 周表）。
// **业主 2026-10-10 定调：本服是单机，不按周算，改成"每次建队随机抽 3 个"**，
// 所以场次用的三关由服务端每场现抽（见 legionSession.dimCloisterRollStages）。
var DimCloisterAllStageDungeons = [...]uint32{100003180, 100003185, 100003190, 100003195, 100003200}

// DimCloisterStartRequest 是 CMD2043 在次元回廊频道的解码结果。
//
// 逐字节对照抓包三帧（每帧 24B = 13B 家族信封 + 11B 正文）：
//
//	#461  68 f5 5f 00 00 00 00 00 | 0d 00 00 00 | 00 | 66 00 00 | 00 00 00 00
//	#504  68 f5 5f 00 00 00 00 00 | 0d 00 00 00 | 00 | 66 00 00 | 01 00 00 00
//	#542  68 f5 5f 00 00 00 00 00 | 0d 00 00 00 | 00 | 66 00 00 | 02 00 00 00
//	                             @8 内容          @13 队伍字节   @16 阶段 0/1/2
//
// 阶段在 @16（u32 小端），队伍标记在 @13（1 字节，抓包恒为 0x66）。
type DimCloisterStartRequest struct {
	Stage      uint32 // @16 第几关（0..2）
	Content    uint32 // @8 客户端固定写 DimCloisterContent
	Party      byte   // @13 队伍类型字节（抓包 0x66），服务端抄回应答
	BodyLength int
}

// DecodeDimCloisterStart 读次元回廊的 CMD2043。
func DecodeDimCloisterStart(p []byte) (DimCloisterStartRequest, error) {
	const want = EnvelopeSize + 11
	if len(p) < want {
		return DimCloisterStartRequest{}, fmt.Errorf(
			"次元回廊开始作战载荷 %d 字节，至少需要 %d", len(p), want)
	}
	return DimCloisterStartRequest{
		Content:    binary.LittleEndian.Uint32(p[EnvelopeSize-5:]),
		Party:      p[EnvelopeSize],
		Stage:      binary.LittleEndian.Uint32(p[EnvelopeSize+3:]),
		BodyLength: len(p),
	}, nil
}

// DimCloisterStartAckSize 是 CMD2043 应答体的长度。
//
// 官服抓包 #755 明文 16 字节（网头 16B + size=32）。家族其它内容的 StartAck
// 只有 5 字节；次元回廊的客户端会读满 16，所以这一族必须回 16。
const DimCloisterStartAckSize = 16

// DimCloisterStartAck 构造 CMD2043 的应答。
//
// ★ 客户端把 **第一个 dword 当结果码**读（`apocalypse_run.go` 的记录：
// `0=接受，252/380=两种失败文案`）。所以：
//
//	body[0] = 01   第一个 dword = 0x00000001 → 接受
//	body[1..4] = 00 00 00 00   ← **必须清零**
//	body[5] = 队伍字节
//
// 2026-10-10 实机教训：上一版把队伍字节放在 @1（`01 66 00 00 …`），客户端读到的
// 结果码是 0x00006601 = 66049，客户端 trace 里报
// `ENUM_CMDPACKET_LEGION_START ErrCode : 102`（取低 dword 的部分位），于是
// 「点开始作战没有任何反应」。官服抓包 #755 的原文校验这个口径：
//
//	01 00 00 00 00 | 3b ff f7 7e 43 | 00 00 00 00 00
//	└─ 结果码 = 1  └─ 队伍字节      └─ nonce 尾巴（无来源，写 0）
func DimCloisterStartAck(req DimCloisterStartRequest) []byte {
	body := make([]byte, DimCloisterStartAckSize)
	body[0] = 1
	body[5] = req.Party
	return body
}

// DimCloisterEnterAckSize 是 CMD2045（客户端进图确认）应答体的长度。
//
// 官服抓包 #789 明文 40 字节（网头 16B + size=40）：
//
//	01 | 00 00 00 00 | 66 00 00 00 | 00 00 00 00 | 00 00 | 2b 72 24 ff 39 …
//
// 与家族共享的 14B EnterDungeonAck 不同，所以单独给长度。
const DimCloisterEnterAckSize = 40

// DimCloisterStageAck 构造 CMD2045 的应答，按官服抓包的字节位置放字段。
//
// 抓包 #789 原文与字段位置（0-based 正文下标）：
//
//	01 00 00 00 00 66 00 00 00 00 00 00 00 2b 72 24 ff 39 …
//	             ^@5 队伍字节    ^@10 阶段
//
// 尾部 nonce 是**服务端自己的**（请求同位置是客户端时间戳），本仓没有来源，写 0。
// 客户端读的是长度与成功字节，不要把尾巴当需要复刻的常量。
func DimCloisterStageAck(party byte, stage uint32) []byte {
	body := make([]byte, DimCloisterEnterAckSize)
	body[0] = 1
	body[5] = party
	binary.LittleEndian.PutUint32(body[10:], stage)
	return body
}

// DecodeDimCloisterStageConfirm 读次元回廊 CMD2045 请求里的队伍字节与阶段。
//
// 逐字节对照抓包 #476（24B = 13B 家族信封 + 11B 正文）：
//
//	02 00 00 00 06 00 00 00 02 00 00 00 00 66 00 00 00 00 00 00 00 00 00 00
//	                                     @13 队伍 0x66  @16 阶段
//
// 与 CMD2043 同形（队伍在 @13、阶段在 @16）。
func DecodeDimCloisterStageConfirm(p []byte) (party byte, stage uint32, err error) {
	const want = EnvelopeSize + 11
	if len(p) < want {
		return 0, 0, fmt.Errorf("次元回廊进图确认载荷 %d 字节，至少需要 %d", len(p), want)
	}
	return p[EnvelopeSize], binary.LittleEndian.Uint32(p[EnvelopeSize+3:]), nil
}

// 次元回廊的操作选择族（与末世录 CMD2354 / 维纳斯 CMD2290 **完全同构**）。
//
// 依据：项目 opcode 表把这一族的全名都列了出来，次元回廊是**独立一族**，
// 不是「军团通用帧 + 官服私有字节」：
//
//	cmd  2043  ENUM_CMDPACKET_LEGION_START                            开战（军团通用）
//	cmd  2045  ENUM_CMDPACKET_LEGION_ENTER_DUNGEON                    进图（军团通用）
//	cmd  2080  ENUM_CMDPACKET_MYRES_DIMENSION_CLOISTER_OPERATION_SELECT   ← 难度/关卡选择请求
//	cmd  2081  ENUM_CMDPACKET_MYRES_DIMENSION_CLOISTER_OPERATION_CLEAR    ← 关窗
//	noti 2314  ENUM_NOTIPACKET_MYRES_DIMENSION_CLOISTER_INFO              ← 横幅/UI 权威状态
//	noti 2315  ENUM_NOTIPACKET_MYRES_DIMENSION_CLOISTER_OPERATION         ← 操作窗状态
//	noti 2316  ENUM_NOTIPACKET_MYRES_DIMENSION_OPERATION_REWARD           ← 翻牌/奖励
//
// 对照：末世录 = cmd2354 / noti2895,2896；维纳斯 = cmd2290 / noti2655,2656。
// 三者布局同族，所以次元回廊的应答**照同族布局构造**，不要照官服抓包字节回放
//（官服那份是新版布局，本机 2.38.2 读不了 —— 实测点开 UI 必 0xC0000005 闪退）。
const (
	// CmdDimCloisterOperationSelect 是难度/关卡选择请求（客户端点右上角 UI）。
	CmdDimCloisterOperationSelect uint16 = 2080
	// CmdDimCloisterOperationClear 是关窗请求。
	CmdDimCloisterOperationClear uint16 = 2081
	// NotiDimCloisterInfo 是横幅/右上角 UI 的权威状态帧。
	NotiDimCloisterInfo uint16 = 2314
	// NotiDimCloisterOperation 是操作（难度选择）窗状态帧。
	NotiDimCloisterOperation uint16 = 2315
	// NotiDimCloisterReward 是军团奖励/翻牌帧。
	NotiDimCloisterReward uint16 = 2316
	// NotiEnableClearDungeon 是「允许通关」闸门（官服 #804，32B 线上 / 16B 正文）。
	// 官服在客户端进图（CMD2045）之后 0.5 秒发它；不发就会卡在「BOSS 最后一条血锁住」。
	NotiEnableClearDungeon uint16 = 31
	// DimCloisterContentCode 是客户端读这个内容用的 u16 内容号（CMD2080/2081 记录首字段）。
	DimCloisterContentCode uint16 = 0x0d
	// DimCloisterSelectionSeconds 是难度选择窗的倒计时秒数（写进 CMD2080 应答的
	// 截止值 = now + 本值）。取 15，与末世录 ApocalypseSelectionSeconds 同口径。
	// DimCloisterSelectionSeconds 是难度选择窗的兜底存活秒数。
	//
	// 客户端那扇窗自带 25 秒倒计时（官服截图口径），服务端兜底必须晚于它，
	// 否则收尾帧会在玩家还在挑的时候插进来。取 30 秒。
	DimCloisterSelectionSeconds uint32 = 30
	// DimCloisterSelectionWindow 是同一段时长的 Duration 形态（服务端到点推 close ACK）。
	DimCloisterSelectionWindow = 30 * time.Second
)

// DimCloisterOperationAckSize 是 CMD2080（操作窗选择）应答体的长度。
//
// ★ 2026-10-10 从**低版本（110 客户端 + DFO110 离线服务端）**抓包里拿到的权威形态：
// 客户端点右上角 UI 发 CMD2080（37B）之后，服务端回的是 **40 字节**：
//
//	01 01 00 00 00 ff ff 00 00 00 00 00 <5B 票据> 00 00 00 00 00 00 00 00 00
//	└成功 └@1=01   └@5..6=ffff └@12..16 是 5 字节票据
//
// 本仓此前回的是自拼的 20 字节（`@1..2 内容号 + @3..6 Action`），**与这一族对不上**：
// 客户端拿不到它要的 40 字节窗口记录，于是在点击那一步 `exit=0xC0000005`（服务端数据
// 其余部分已与官服逐字节一致、客户端连一个包都不发，正是这个缘故）。
//
// 官服 s30 的 #771 也是同形 40 字节（`01 01 000000 00 ffff 00000000 2b7224ff39 …`），
// 两条独立抓包互相印证。
const DimCloisterOperationAckSize = 40

// DimCloisterOperationAck 构造操作窗应答（40B）。
//
//	@0      = 01 成功
//	@1      = action（客户端请求 @4 的值回显；抓包是 01）
//	@5..6   = ff ff
//	@12..16 = 5 字节票据；语义未回收，照抄抓包值
func DimCloisterOperationAck(action byte, ticket [5]byte) []byte {
	p := make([]byte, DimCloisterOperationAckSize)
	p[0] = 1
	p[1] = action
	p[5], p[6] = 0xff, 0xff
	copy(p[12:], ticket[:])
	return p
}

// DimCloisterOperationTicket 是官服 s30 #771 那帧的 5 字节票据原文
// （低版本 110 抓包同一位置是 `2f 4a c9 6a` —— 逐连接/逐次不同，语义未回收）。
var DimCloisterOperationTicket = [5]byte{0x2b, 0x72, 0x24, 0xff, 0x39}

// DimCloisterOperationClose 构造关窗响应：同族形状 + 关闭位（正文 @3）。
//
// 关窗的判据沿用同族口径（维纳斯/末世录的 reader 看「Action 段 + 关闭位」），
// 但**长度与形状必须先是这一族要的 40 字节**，否则客户端连窗口记录都拿不到。
func DimCloisterOperationClose() []byte {
	p := DimCloisterOperationAck(1, DimCloisterOperationTicket)
	p[3] = 1
	return p
}

// DecodeDimCloisterOperation 解码 CMD2080（难度/关卡选择请求）。
//
// 实机原文（业主 2026-10-10 会话，点右上角 UI 时客户端发来，37B）：
//
//	90ae fd94 01000000 01000000 0101 000000 ffff 000000 0000
//
// 低版本（110 客户端）同一动作发的是 37B：
//
//	c03aba30 01000000 01000000 00010000 00ffff 000000 0000
//
// 两条形状一致：`u32@4` = action（1 = 开窗/选择），`@12..13` = ff ff。
// 这里只解出 Action 用于日志与分支，不做门禁（开窗/确认两态都可能重发）。
type DimCloisterOperationRequest struct {
	Action uint32
	Close  bool
	Body   []byte
}

func DecodeDimCloisterOperation(p []byte) (DimCloisterOperationRequest, error) {
	if len(p) < 8 {
		return DimCloisterOperationRequest{}, fmt.Errorf("次元回廊作战选择载荷 %d 字节，至少需要 8", len(p))
	}
	req := DimCloisterOperationRequest{
		Action: binary.LittleEndian.Uint32(p[4:]),
		Body:   append([]byte(nil), p...),
	}
	if len(p) > 5 {
		req.Close = p[5] != 0
	}
	return req, nil
}
// N28 的副本号换成该关的目标号。
//
// 抓包三关的进图帧列**除 N28 首 4 字节外逐字节相同**，所以保留官服原文、只改
// 副本号，是能同时满足「不发明数据」与「三关各自正确」的最小改动。
// GetDimCloisterEntryVectorsForStage 返回第 stage 关的进图帧列（Entry 组），
// N28 的副本号换成该关的目标号。
//
// 抓包三关的进图帧列**除 N28 首 4 字节外逐字节相同**，所以保留官服原文、只改
// 副本号，是能同时满足「不发明数据」与「三关各自正确」的最小改动。
func GetDimCloisterEntryVectorsForStage(stage int) ([]DimCloisterVector, error) {
	if stage < 0 || stage >= len(DimCloisterStageDungeons) {
		return nil, fmt.Errorf("次元回廊阶段 %d 越界（官服抓包只有 0..%d）",
			stage, len(DimCloisterStageDungeons)-1)
	}
	return GetDimCloisterEntryVectorsForDungeon(DimCloisterStageDungeons[stage])
}

// GetDimCloisterEntryVectorsForDungeon 同上，但**副本号由调用方给**。
//
// 业主 2026-10-10 单机口径：每场从 5 个界里随机抽 3 个 ⇒ 副本号不能写死成官服那一组，
// 必须由会话把抽到的号传进来（否则客户端按 N28 加载的还是官服那一关）。
func GetDimCloisterEntryVectorsForDungeon(dungeonID uint32) ([]DimCloisterVector, error) {
	if dungeonID == 0 {
		return nil, fmt.Errorf("次元回廊进图帧列的副本号为 0")
	}
	// ★ 2026-10-10 业主逐屏对照后改正：**必须按副本号取"那一轮"的整列**。
	//
	// 旧实现只有一份帧列（取自官服第一轮 = 第1界），只把 N28 首 4 字节改成目标副本号 ⇒
	// 客户端"读图/横幅/右上角"跟着 N28 变成第4界，而**地图与 BOSS 仍是第1界**
	// （真正决定地图号/怪物的是 N29 等帧）。现在改用按副本号索引的三列原文。
	if column, ok := DimCloisterEntryVectorsByDungeon[dungeonID]; ok && len(column) > 0 {
		out := make([]DimCloisterVector, 0, len(column))
		for _, v := range column {
			out = append(out, DimCloisterVector{
				Name: v.Name, Kind: v.Kind, ID: v.ID,
				Body: append([]byte(nil), v.Body...), Frame: v.Frame,
			})
		}
		return out, nil
	}
	src := GetDimCloisterEntryVectors()
	out := make([]DimCloisterVector, 0, len(src))
	for _, v := range src {
		if v.Kind == 0 && v.ID == 28 && len(v.Body) >= 4 {
			body := append([]byte(nil), v.Body...)
			binary.LittleEndian.PutUint32(body[0:], dungeonID)
			v.Body = body
		}
		out = append(out, v)
	}
	return out, nil
}

// DimCloisterOfficialPartyInfo 返回官服 #759 那帧 NOTI9 的 176B 正文（诊断用）。
//
// 这是**官服新版客户端布局**（队名 @18 单字节 ASCII、容量 u32@22、队伍类型 @29=0x0d、
// 模式 u16@30）。本机 2.38.2 读它存在越界风险，所以只在 `-cloister-official-n9`
// 诊断入口下使用；正文直接取自本包已生成的官服向量 `GetDimCloisterWindowVectors()`
// 里的 WindowParty 帧（与 #759 逐字节相同），只把队名换成本场的值。
func DimCloisterOfficialPartyInfo(partyName string) ([]byte, error) {
	const nameAt = 18
	const nameMax = 4 // @18..@21，@22 起是容量 u32
	var src []byte
	for _, v := range GetDimCloisterWindowVectors() {
		if v.Kind == 0 && v.ID == 9 {
			src = v.Body
			break
		}
	}
	if len(src) == 0 {
		return nil, fmt.Errorf("次元回廊官服 N9 向量缺失")
	}
	out := append([]byte(nil), src...)
	if len(out) < nameAt+nameMax {
		return nil, fmt.Errorf("次元回廊官服 N9 正文只有 %d 字节", len(out))
	}
	for i := 0; i < nameMax; i++ {
		out[nameAt+i] = 0
	}
	for i := 0; i < len(partyName) && i < nameMax; i++ {
		out[nameAt+i] = partyName[i]
	}
	// 容量（@22，u32）本仓本来就是单人自建 4 人房，写官服那份值。
	binary.LittleEndian.PutUint32(out[22:], 4)
	return out, nil
}

// IsDimCloisterStageDungeon 报告一个副本号是不是次元回廊的关卡副本，返回其下标。
// 清关/回城路径用它把这一族与普通副本分开（与 ForestStageOfDungeonAny 同款）。
func IsDimCloisterStageDungeon(id uint32) (int, bool) {
	for i, d := range DimCloisterStageDungeons {
		if d == id {
			return i, true
		}
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// N2314（MYRES_DIMENSION_CLOISTER_INFO）权威状态帧的选取。
//
// ★ 2026-10-10 的教训（业主实机）：本仓先前只挑了两帧 N2314 发给客户端，客户端
// 收到后先正常起横幅（trace 里 `[RaidTitleDrawer::setState]0/1/2`），几秒后
// `exit=0xC0000005`。也就是说「按语义挑帧、其余省略」这条路走不通 —— 这一族是
// 权威状态帧，客户端把整段当成结构体读，省略的字段就是它下面要用的指针/下标。
//
// 所以现在的口径是**整帧照抄官服原文**（见 dimension_cloister_info.generated.go），
// 服务端只决定「该发哪一帧」，顺序照官服 s30 抓包：
//
//	#752 N2254 入场账本 → #754 N2314 @3=01 → #755 ACK2043
//	（倒计时结束）#758 N2314 @3=06 开右上角 UI
//	选定后 #770 N2314 @3=02 → 客户端 CMD2045 → 进图帧列
//	进图后 #897 N2314 @3=02（place=02，带第 1 界记录）
//	清关后 再来一次 @3=06 / @3=02，客户端就能再开一次难度窗
//	终局   #985 N2314 @3=03（放视频）→ #993 N2314 @3=05（UI 关闭）
//
// 阶段变体（dungeon0/1/2）就是官服三关各自的进图/清关帧，本仓按「本场已打第几界」
// 选一帧，不按语义重建字节。
const (
	// DimCloisterInfoHallInitial 是集结区横幅初态（@3=01）。
	DimCloisterInfoHallInitial = "hallInitial"
	// DimCloisterInfoHallOptions 是右上角 UI 打开态（@3=06，难度列表）。
	DimCloisterInfoHallOptions = "hallOptions"
	// DimCloisterInfoHallPicked 是「已选定」态（@3=02）。
	DimCloisterInfoHallPicked = "hallPicked"
	// DimCloisterInfoFinale 是终局态（@3=03，客户端据此放视频）。
	DimCloisterInfoFinale = "finale"
)

// dimCloisterDungeonInfoNames 是「第 N 界已经开打/打完」的 N2314 变体名，
// 下标 = 界号（0..2）。官服抓包正好覆盖这三界，顺序不是按副本号排。
var dimCloisterDungeonInfoNames = [...]string{"dungeon0", "dungeon1", "dungeon2"}

// DimCloisterInfoBody 返回指定的官服 N2314 原文副本。名字未知时返回错误，
// 绝不回落到别的状态 —— 发错状态就是发错数据。
func DimCloisterInfoBody(name string) ([]byte, error) {
	f, ok := dimCloisterInfoFrames[name]
	if !ok {
		return nil, fmt.Errorf("次元回廊没有名为 %q 的 N2314 状态帧", name)
	}
	return append([]byte(nil), f.Body...), nil
}

// DimCloisterDungeonInfoBody 返回第 stage 界对应的 N2314 进度帧。
func DimCloisterDungeonInfoBody(stage int) ([]byte, string, error) {
	if stage < 0 || stage >= len(dimCloisterDungeonInfoNames) {
		return nil, "", fmt.Errorf("次元回廊第 %d 界没有官服 N2314 变体（抓包只有 0..%d）",
			stage, len(dimCloisterDungeonInfoNames)-1)
	}
	name := dimCloisterDungeonInfoNames[stage]
	body, err := DimCloisterInfoBody(name)
	return body, name, err
}

// DimCloisterEntryLedgerBody 返回 N2254 入场账本副本，并把官服那份当场时间戳
// （@256..260，毫秒）换成当前时间 —— 官服九帧里只有这 5 字节和每周标记位会变，
// 时间戳照抄抓包当场的值会变成过期数据。
func DimCloisterEntryLedgerBody(nowMillis uint64) ([]byte, error) {
	const stampAt = 256
	body := append([]byte(nil), DimCloisterEntryLedger...)
	if len(body) < stampAt+5 {
		return nil, fmt.Errorf("次元回廊 N2254 模板长度 %d 不足", len(body))
	}
	body[stampAt] = byte(nowMillis)
	body[stampAt+1] = byte(nowMillis >> 8)
	body[stampAt+2] = byte(nowMillis >> 16)
	body[stampAt+3] = byte(nowMillis >> 24)
	body[stampAt+4] = byte(nowMillis >> 32)
	return body, nil
}
