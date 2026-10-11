package protocol

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// 战令（阿拉德通行证 / Arad Pass）协议：0x08A1 + 0x08A2，两者都是**纯 S2C**。
//
// 结构全部来自 `D:\wd\20261002-211855` 抓包的实证（18 份 0x08A1、21 份 0x08A2）：
//
// 0x08A1（正文 760B，线上帧 776B）
//
//	0x00 u32   0
//	0x04 u8    等级（1..50，源 `[pass level info]` 只到 50）
//	0x05 u24   当前**累计**经验
//	0x08 u32   满级经验 ×256（抓包 = 50000×256 = 12,800,000）
//	0x0C u32   当前累计经验 ×256（= 0x05 的同一个数，定点表示）
//	0x10 u32   (满级经验 − 当前经验) ×256
//	0x14 u32   购票赠送经验 ×256（抓包 = 5000×256 = 1,280,000）
//	0x18 u8    0
//	0x19 u8    1
//	0x1A u8    任务列表1条数 = 16（源 `[normal mission]` 16 条）
//	0x1B 16×7B 任务行：首字节 01..10，其余 0（任务进度；抓包已见个别行计数器变化）
//	0x8B 98×00 保留
//	0xED u8    任务列表2条数 = 12（源 `[season mission]` 12 条）
//	0xEE 12×7B 任务行：首字节 01..0C，其余 0
//	0x142 128×00 保留
//	0x1C2 u8   状态标志：登录首帧 0，其后 18 份里 17 份为 1
//	0x1C3 325×00 保留
//
// 交叉校验（18/18 份样本自洽）：0x05 的 u24 == 0x0C/256；0x0C/256 + 0x10/256 == 0x08/256；
// 且 0x08/256 == 源 `[pass level info]` 的 50000、0x14/256 == 源 `[purchase pass exp]` 的 5000。
const (
	// AradPassEventInfoOpcode 是账号级战令状态（776B，登录下发 18 次）。
	AradPassEventInfoOpcode uint16 = 2209
	// AradPassCharacInfoOpcode 是角色槽级战令状态（1120B，登录下发 21 次）。
	AradPassCharacInfoOpcode uint16 = 2210

	// aradPassEventInfoSize 是 0x08A1 的解密正文长度（线上帧多 16B 帧头）。
	aradPassEventInfoSize = 760
	// aradPassCharacSlots 是 0x08A2 的角色槽数（21 份样本恒 20 条记录）。
	aradPassCharacSlots = 20
	// aradPassCharacSlotSize 是单条角色槽记录的字节数（空槽时 charID = 0xFFFFFFFF）。
	aradPassCharacSlotSize = 55
	// aradPassCharacHeaderSize 是 0x08A2 的记录区之前的头部字节数。
	aradPassCharacHeaderSize = 4

	// aradPassNormalMissions / aradPassSeasonMissions 是两条任务列表的条数，
	// 与源 `newaradpassevent.evt` 的 `[normal mission]`(16) / `[season mission]`(12) 一致。
	aradPassNormalMissions = 16
	aradPassSeasonMissions = 12

	// 0x08A1 内部偏移。
	aradPassOffsetNormalCount = 0x1A
	aradPassOffsetNormalRows  = 0x1B
	aradPassOffsetSeasonCount = 0xED
	aradPassOffsetSeasonRows  = 0xEE
	// aradPassOffsetClaimedNormal / Royal 是两条轨**各自**的逐档已领标记起始偏移。
	//
	// 实机证据（2026-10-11）：普通轨 `0x1C2`（Lv1/3/5/7 与 Lv5/7/9 共 8 格吻合）；
	// 皇家轨 `0x28A`（窗口计数法二分 + "连尾部都亮"的判据定案）。
	// **两段不相邻** —— 早先按"普通 25 字节之后紧跟皇家"写（0x1DB/0x1F4）都不亮。
	aradPassOffsetClaimedNormal = 0x1C2
	aradPassOffsetClaimedRoyal  = 0x28A
	// aradPassOffsetRoyalDailyDay 是「Royal Pass 每日登录礼包」**已领到的日**（u16，正文 0x1C0..0x1C1）。
	//
	// ★★ 2026-10-11 实机 + IDA 双向定案：这是每日礼包**唯一**的"已领"位。
	//
	//	客户端发 `680(516,3)` 前的守卫是 `sub_1422A07B0(passObj) == 0`，
	//	其实现 `sub_1422A0940` 读 `*(u8*)(passObj + 0x990)`：
	//
	//		表值(word) + 正文[0x2EE] <= 正文[0x1C0]  ⇒  返回 1（已领）
	//		                                        ⇒  客户端**连包都不发**
	//
	//	0x990 - 0x7D0 = 0x1C0（正文基址 = 对象 +0x7D0），所以它就是正文偏移 0x1C0。
	//	`word` 由 `sub_147619FF0(manager, &w, 1)` 从**活动表**查出（键 1），刻度未知。
	//
	//	同一个谓词也驱动面板那行的绿勾：把它写成 0xFF 时面板**当场**变"已领取"
	//	（业主 2026-10-11 截图确认）。
	//
	// ⇒ 服务端用**饱和值**表达"今天已领"（两字节都写 0xFF）：只要
	//	`word + 正文[0x2EE] <= 0xFFFF` 恒成立就够，**不需要**知道官方那个日序号的刻度。
	//	跨日写 0，门禁自动放开 —— 与 `state.RoyalDailyKey` 同一把钥匙（09:00 UTC）。
	aradPassOffsetRoyalDailyDay = 0x1C0
	// AradPassRoyalDailyClaimed 是"今天已领"时写进上面那格的饱和值（正文 0x1C0 的 u16）。
	AradPassRoyalDailyClaimed = 0xFFFF
	// aradPassClaimedPerTrack 是每轨的档数（25）；位图/数组长度 = 2 × 该值。
	aradPassClaimedPerTrack = 25
	// aradPassClaimedSlots 是标记数组长度：25 普通 + 25 皇家 = 50 档。
	aradPassClaimedSlots = 2 * aradPassClaimedPerTrack

	// aradPassExpScale 是经验字段的定点倍数（抓包实测 50000 → 12,800,000）。
	aradPassExpScale = 256

	// AradPassBodyPurchased 是 0x08A1 正文前 4 字节在"已购皇家"时的取值。
	//
	// ★ 实机双向验证（2026-10-11）：写 2 → ROYAL 行解锁；留 0 → 上锁。
	// 0x08A2 的正文头（AradPassCharacHeader）也要求 2 —— 两层同一语义。
	AradPassBodyPurchased uint32 = 2

	// aradPassRoyalDefaultOffset/Value 是"已购皇家"默认写入的位置与取值。
	// 可用 DFO_ARAD_PASS_ROYAL_BYTE=<偏移>,<值> 覆盖，方便逐个试候选而不重编。
	aradPassRoyalDefaultOffset = 0x18
	aradPassRoyalDefaultValue  = 1
)

// aradPassRoyalByte 返回"已购皇家"要写的位置与取值。
//
// ⚠ 这是**待验证的候选**：抓包里那位玩家从未购买通行证，所以"已购"该写哪儿没有样本。
// 实机若不解锁，改环境变量 `DFO_ARAD_PASS_ROYAL_BYTE=<偏移>,<值>` 试下一个候选即可
// （偏移用十进制或 0x 前缀都行），不用重新编译。
func aradPassRoyalByte() (int, byte) {
	raw := strings.TrimSpace(os.Getenv("DFO_ARAD_PASS_ROYAL_BYTE"))
	if raw == "" {
		return aradPassRoyalDefaultOffset, aradPassRoyalDefaultValue
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 2 {
		return aradPassRoyalDefaultOffset, aradPassRoyalDefaultValue
	}
	off, e1 := strconv.ParseInt(strings.TrimSpace(parts[0]), 0, 32)
	val, e2 := strconv.ParseInt(strings.TrimSpace(parts[1]), 0, 32)
	if e1 != nil || e2 != nil || off <= 0 || off >= aradPassEventInfoSize || val < 0 || val > 255 {
		return aradPassRoyalDefaultOffset, aradPassRoyalDefaultValue
	}
	return int(off), byte(val)
}

// AradPassMissionRow 是任务列表里**一行**的三个计数。
//
// ★ 行宽 7 字节；**字段语义由判别探针实机定案**（2026-10-11）：
// 探针把三个候选位置写成互不相同的哨兵（`@+1=41`、`@+3=42`、`@+5=43`），
// 面板一次读出：
//
//	面板列         显示         ⇒ 字段
//	`n / m`        `41 / 100`   `@+1` = 进度（客户端会按脚本目标截断：41 → `20/20`、`5/5`）
//	`k time(s)`    `43 time…`   `@+5` = 完成次数
//	`Total t`      `420`        `@+3` = 累计经验（非 0 也会让状态列变 Completed）
//
// 反汇编侧的旁证（`sub_1422A1110` 的"变化检测"只读 `u16@+3` 与 `u16@+5`）：
// `@+5` 变了 ⇒ 有一次新完成（UI 需要重画）；`@+3` 被带进 UI 记录当累计值。
//
// ⚠ 我们**第一版把"进度"写在 `@+3`**，所以任务页永远显示 `0 / m`；改到 `@+1` 才显示。
type AradPassMissionRow struct {
	// Progress 写 @+1：面板 `n / m` 里的 n（已推进到几）。
	Progress uint16
	// TotalExp 写 @+3：面板 `Total` 列的累计经验，**单位是十分之一经验**（客户端显示时 ×10）。
	//
	// ★ 两个实测锚点（2026-10-11）：
	//	`@+3 = 42`（探针哨兵）⇒ 面板 `Total 420`
	//	`@+3 = 300`（= 1×300）⇒ 面板 `Total 3000`
	//   ⇒ 显示值 = `@+3 × 10`。所以"通关一次 300 经验"要写 **30**。
	// （0x08A1 的经验字段本来就是定点口径：头部累计经验是 ×256，这里是 ×0.1。）
	// 非 0 时状态列还会显示 Completed。
	TotalExp uint16
	// Times 写 @+5：面板 `k time(s)` 里的 k（做满过几轮）。也是客户端的"变化"判据。
	Times uint16
}

// AradPassProgress 是 0x08A1 里**可实证驱动**的字段；其余字节按抓包模板补零。
type AradPassProgress struct {
	// Level 是当前等级（1 起）。
	Level uint32
	// Exp 是当前**累计**经验（不是本级内经验）。等级由 `[pass level info]` 的阈值表决定。
	Exp uint32
	// MaxExp 是满级累计经验（源 `[pass level info]` 最后一级 = 50000）。
	MaxExp uint32
	// PurchaseExp 是购票赠送经验（源 `[purchase pass exp]` = 5000）。
	PurchaseExp uint32
	// Royal 表示已购皇家通行证。
	//
	// ⚠ **仍未定字节**：抓包里那位玩家从没买过通行证，18 份 0x08A1 里没有"已购"的样本。
	// 当前按最强候选落到 `0x18`，可用 `DFO_ARAD_PASS_ROYAL_BYTE=<偏移>,<值>` 逐个试而不重编。
	// 2026-10-11 实机：角色状态 `royal=true`、购票经验 5000 也显示出来了（面板 Lv9 / 5000 EXP），
	// 但 **ROYAL 轨仍显示上锁** ⇒ `0x18` 不是（至少不只是）"已购"位。
	Royal bool
	// Purchased 表示**该账号已购皇家通行证** ⇒ 正文前 4 字节写 `AradPassBodyPurchased`（= 2）。
	//
	// ★★ 2026-10-11 实机定位：这是 **ROYAL 轨解锁的开关**。
	// 取证过程：先按 `516`（战令对象 id）扫 exe 找到面板刷新器 `sub_14235DAF0`，
	// 其"已购"判定 `sub_14229FF90` 的第一句是 `*(u32*)(passObj+0xAC8) != 2 → return 0`
	// （0xAC8 = **2210** 正文起点）；把 2210 头改成 2 仍锁。
	// 再用缓冲探针 `DFO_ARAD_PASS_BODY_POKE=0:2` 把 **2209**（0x7D0）正文的 u32 头也写成 2
	// ⇒ **ROYAL 行的锁当场打开**（面板左上徽章也变成 Royal）。
	// 结论：2209 头与 2210 头**同一语义的两个层级**（账号级 / 每角色），都必须是 2；
	// 官服抓包那台"未购买"的账号上 2209 头正是 0 —— 与"0 = 未购买"一致。
	Purchased bool
	// ClaimedSlots 是**逐档已领标记**：索引 i 对应"该轨第 i+1 档"，**非 0 = 已领**。
	//
	// 索引空间 50 档：`0..24` = 普通轨 slot 1..25，`25..49` = 皇家轨 slot 1..25。
	// 编码时**按轨拆到正文里两段不同的基址**（见 aradPassOffsetClaimedNormal / Royal）——
	// 调用方不需要知道这个布局。
	//
	// ★★ 为什么是"每档 1 字节"而不是位图（2026-10-11 实机）：
	//
	//	发出 0x1C2 = 01 00 …  ⇒ 面板**只有第 1 格**变成 Complete
	//	发出 0x1C2 = ff 01 …  ⇒ 面板**只有第 1、2 格**变成 Complete，第 3、4 格仍是 Get
	//
	//   —— 若是位图，`0xff` 该点亮 8 格。而且第二个被点亮的格子是 **Free 第 2 格**，
	//   不是 Royal 第 1 格 ⇒ 数组按**每轨序号**排、每档 1 字节。
	//
	// ★★ 两条轨的基址**不相邻**（2026-10-11 实机定案）：
	//   普通轨在 `0x1C2`（8 格实机证据），皇家轨在 **`0x28A`**。
	//   定位过程：先用缓冲探针把候选区间二分到 `0x28A..0x29C`，
	//   再在 `0x28A` 打一个 25 字节窗口 —— "全部 Complete"（连第 22..25 档都亮）
	//   只可能发生在**数组起点恰好等于窗口起点**的情形 ⇒ B = 0x28A。
	ClaimedSlots []byte
	// RoyalDailyDay 是「Royal Pass 每日登录礼包」的已领标记（正文 0x1C0 的 u16）。
	//
	// 0 = 今天还能领；非 0（服务端固定写 AradPassRoyalDailyClaimed = 0xFFFF）= 今天已领。
	// 取证见 aradPassOffsetRoyalDailyDay 的说明。
	RoyalDailyDay uint16
	// NormalMissions / SeasonMissions 是两条任务列表的逐行计数，长度应分别 ≤ 16 / 12
	// （缺的按 0 补齐；序号一律写成 1..N —— 客户端遇到 `row[0]==0` 就停止收行）。
	NormalMissions []AradPassMissionRow
	SeasonMissions []AradPassMissionRow
}

// AradPassEventInfo 编码 0x08A1。
func AradPassEventInfo(p AradPassProgress) ([]byte, error) {
	if p.Level == 0 || p.Level > 255 {
		return nil, fmt.Errorf("arad pass: invalid level %d", p.Level)
	}
	if p.MaxExp == 0 {
		return nil, fmt.Errorf("arad pass: max exp is required")
	}
	if p.Exp > p.MaxExp {
		return nil, fmt.Errorf("arad pass: exp %d exceeds max %d", p.Exp, p.MaxExp)
	}
	body := make([]byte, aradPassEventInfoSize)
	// 0x00..0x03：已购皇家 = 2；未购买留 0（与官服抓包一致）。
	if p.Purchased {
		binary.LittleEndian.PutUint32(body, AradPassBodyPurchased)
	}
	body[4] = byte(p.Level)
	putU24(body[5:], p.Exp)
	binary.LittleEndian.PutUint32(body[8:], p.MaxExp*aradPassExpScale)
	binary.LittleEndian.PutUint32(body[12:], p.Exp*aradPassExpScale)
	binary.LittleEndian.PutUint32(body[16:], (p.MaxExp-p.Exp)*aradPassExpScale)
	binary.LittleEndian.PutUint32(body[20:], p.PurchaseExp*aradPassExpScale)
	body[24], body[25] = 0, 1
	// 已购皇家：默认落在 0x18（见 AradPassProgress.Royal 的说明）。
	if p.Royal {
		off, val := aradPassRoyalByte()
		body[off] = val
	}
	// 两条任务列表：条数 + 每行「序号 + 6 字节计数」。
	//
	// 序号字节**必须非 0**：客户端的收行循环是 `if (*(BYTE*)row == 0) break;`
	// （见 AradPassMissionRow 的注释），写 0 会让后面所有行被丢掉。
	body[aradPassOffsetNormalCount] = aradPassNormalMissions
	for i := 0; i < aradPassNormalMissions; i++ {
		putAradPassMissionRow(body[aradPassOffsetNormalRows+7*i:], byte(i+1), missionRowAt(p.NormalMissions, i))
	}
	body[aradPassOffsetSeasonCount] = aradPassSeasonMissions
	for i := 0; i < aradPassSeasonMissions; i++ {
		putAradPassMissionRow(body[aradPassOffsetSeasonRows+7*i:], byte(i+1), missionRowAt(p.SeasonMissions, i))
	}
	// 每日礼包"今天已领"（正文 0x1C0 的 u16；见 aradPassOffsetRoyalDailyDay）。
	//
	// ★ 必须写在**两条任务行之后**、且与已领档数组互不重叠：0x1C0 紧挨着普通轨数组
	// 基址 0x1C2，中间只隔 0x1C1 一个字节。
	if p.RoyalDailyDay != 0 {
		binary.LittleEndian.PutUint16(body[aradPassOffsetRoyalDailyDay:], p.RoyalDailyDay)
	}
	// 已领标记：每档 1 字节（非 0 = 已领），**按轨写到两段不同的基址**。
	normal, royal := aradPassClaimedOffset(), aradPassRoyalClaimedOffset()
	for i := 0; i < len(p.ClaimedSlots) && i < aradPassClaimedSlots; i++ {
		off := normal + i
		if i >= aradPassClaimedPerTrack {
			off = royal + (i - aradPassClaimedPerTrack)
		}
		if off >= 0 && off < len(body) {
			body[off] = p.ClaimedSlots[i]
		}
	}
	return body, nil
}

// aradPassClaimedOffset / aradPassRoyalClaimedOffset 返回两条轨的标记起始偏移。
//
// 缺省 0x1C2 / 0x28A（均为实机验证值）；`DFO_ARAD_PASS_CLAIMED_OFFSET` 与
// `DFO_ARAD_PASS_CLAIMED_ROYAL_OFFSET` 可分别挪位，用来不重编试别的布局。
func aradPassClaimedOffset() int {
	return aradPassClaimedOffsetFromEnv("DFO_ARAD_PASS_CLAIMED_OFFSET", aradPassOffsetClaimedNormal)
}

func aradPassRoyalClaimedOffset() int {
	return aradPassClaimedOffsetFromEnv("DFO_ARAD_PASS_CLAIMED_ROYAL_OFFSET", aradPassOffsetClaimedRoyal)
}

func aradPassClaimedOffsetFromEnv(env string, def int) int {
	raw := strings.TrimSpace(os.Getenv(env))
	if raw == "" {
		return def
	}
	v, e := strconv.ParseInt(raw, 0, 32)
	if e != nil || v < 0 || v+aradPassClaimedPerTrack > aradPassEventInfoSize {
		return def
	}
	return int(v)
}

// missionRowAt 取第 i 行的计数；越界返回零值（缺的行按"未推进"下发）。
func missionRowAt(rows []AradPassMissionRow, i int) AradPassMissionRow {
	if i < 0 || i >= len(rows) {
		return AradPassMissionRow{}
	}
	return rows[i]
}

// putAradPassMissionRow 写一行 7 字节：`u8 序号 | u16 进度 | u16 累计经验 | u16 次数`。
func putAradPassMissionRow(dst []byte, index byte, row AradPassMissionRow) {
	if len(dst) < 7 {
		return
	}
	dst[0] = index
	binary.LittleEndian.PutUint16(dst[1:], row.Progress)
	binary.LittleEndian.PutUint16(dst[3:], row.TotalExp)
	binary.LittleEndian.PutUint16(dst[5:], row.Times)
}

// AradPassCharSlot 是 0x08A2 里的一条角色槽（55 字节）。
//
// 字段划分来自客户端解析函数 `sub_1422A1770` 的反汇编（`ida_probe_aradpass_2210.py`）：
//
//	u8  @+0  记录键低位
//	u32 @+1  记录键高位（客户端把它与 @+0 组成红黑树键）
//	u32 @+5  **charID**（0xFFFFFFFF = 空槽）
//	u32 @+9  种类：1 / 2 / 3
//	u8  @+15 标志
//	u32 @+17 参与"非空"判据
//
// ★ 客户端判"这一条记录算不算存在"的口径是
//
//	if ( *v6 != 0 || *(u32*)(v6+1) != 0 || *(u32*)(v6+17) != 0 )
//
// —— 也就是 **@+0 / @+1 / @+17 全 0 的记录会被整条跳过**（即使 charID 是有效的）。
// 最初移植时照抓包"21 份样本全是空槽"把 20 槽全空，等于客户端手上一条本角色的记录都没有；
// 而 `charID` 解析不出来时客户端还会**自己发 CMD681 `(516, 7, u8@0, u32@1)`**
// 向服务端要这一条 —— 说明它确实需要一份**本角色**的通行证记录。
//
// 其余 46 字节的语义没有样本也没有消费者证据 ⇒ 一律留 0，不发明字段。
type AradPassCharSlot struct {
	// CharID 是该槽的角色号；0 或 0xFFFFFFFF 表示空槽。
	CharID uint32
	// Lead 写 @+0：记录键的**低位**（与 Key 组成红黑树键）。
	//
	// 候选语义是"角色登记序号"：源脚本 `[royal registable count] **1 10**` 与面板上的
	// `Royal Pass Character(n/10)` 都指向 1..10 —— **0 很可能被当成无效**，
	// 所以缺省取 1（上一轮写 0，面板计数仍是 0）。
	Lead byte
	// Key 写 @+1：客户端的记录键高位。**必须非 0**，否则整条记录被客户端跳过。
	Key uint32
	// Kind 写 @+9：**位掩码**（客户端按位判定，见下）。
	//
	// 客户端的两条判据：
	//
	//	(*(u32*)(v6+9) - 2) <= 1        ⇒ bit1 置位（值 2 或 3）
	//	((*(u32*)(v6+9) - 1) & 0xFFFFFFFD) == 0  ⇒ bit0 置位（值 1 或 3）
	//
	// 置位后分别进两条列表（bit1 那条还要求 Flag != 0），末尾各调一次"单元格刷新"。
	// ⇒ 取 **3（两位都置）**：这个角色同时是"普通通行证角色"与"皇家通行证角色"，
	// 面板 Royal Pass 页签的 `Royal Pass Character(n/10)` 就是数这条列表
	// （源脚本 `[royal registable count] 1 10`）。
	Kind uint32
	// Flag 写 @+15：bit1 那条列表要求它非 0（读作"这条记录有效/已启用"）。
	Flag byte
	// State 写 @+16 —— ★★ **"这个角色的皇家通行证是否生效"就是这一格**。
	//
	// 依据：面板刷新函数 `sub_14235DAF0` 里
	//
	//	v29 = sub_145693930(管理器, 516);                       // 取战令对象
	//	if (v29 == 0 || sub_14229FF90(v29) == 0) { 未购买分支 }  // 关掉 UI 2727/2728 并早退
	//
	// 而 `sub_14229FF90(obj)` 的返回值**就是**匹配到的记录的第 16 字节
	// （`v17 = v16[16]`；匹配条件 = 列表A 里那条记录的 charID 解析出的角色名与当前角色相同）。
	// ⇒ 这一格非 0 才等于"已购买/已激活"。
	State byte
	// Extra 写 @+17：与 Lead/Key 一起参与"这条记录算不算存在"的判据，语义未定。
	Extra uint32
}

// AradPassCharacHeader 是 0x08A2 正文前 4 字节（4B 头）的取值。
//
// ★★ **它必须等于 2**，否则客户端认定"没买"。依据同上：`sub_14229FF90` 的第一句就是
//
//	if ( *(_DWORD *)(a1 + 2760) != 2 ) return 0;   // 2760 = 0xAC8 = 2210 正文起点
//
// 我们此前一直写 0 ⇒ 这个判定永远返回 0 ⇒ 面板认为未购买 ⇒ ROYAL 轨上锁、
// `Royal Pass Character(0/10)` 恒为 0。可用 `DFO_ARAD_PASS_CHARAC_HEADER` 覆盖。
const AradPassCharacHeader uint32 = 2

// AradPassCharacInfo 编码 0x08A2（4B 头 + 20×55B 槽记录 = 1104B 正文）。
//
// header 写正文前 4 字节：**必须传 `AradPassCharacHeader`（= 2）**，
// 否则 `sub_14229FF90` 的第一句就返回 0、面板认定"没买"（见 AradPassCharacHeader）。
func AradPassCharacInfo(header uint32, slots []AradPassCharSlot) ([]byte, error) {
	if len(slots) > aradPassCharacSlots {
		return nil, fmt.Errorf("arad pass: %d character slots exceed %d", len(slots), aradPassCharacSlots)
	}
	body := make([]byte, aradPassCharacHeaderSize+aradPassCharacSlots*aradPassCharacSlotSize)
	binary.LittleEndian.PutUint32(body, header)
	for i := 0; i < aradPassCharacSlots; i++ {
		rec := body[aradPassCharacHeaderSize+i*aradPassCharacSlotSize:]
		charID := uint32(0xFFFFFFFF)
		if i < len(slots) && slots[i].CharID != 0 && slots[i].CharID != 0xFFFFFFFF {
			charID = slots[i].CharID
		}
		binary.LittleEndian.PutUint32(rec[5:], charID)
		if charID == 0xFFFFFFFF || i >= len(slots) {
			continue // 空槽：其余 51 字节留 0
		}
		rec[0] = slots[i].Lead
		binary.LittleEndian.PutUint32(rec[1:], slots[i].Key)
		binary.LittleEndian.PutUint32(rec[9:], slots[i].Kind)
		rec[15] = slots[i].Flag
		rec[16] = slots[i].State
		binary.LittleEndian.PutUint32(rec[17:], slots[i].Extra)
	}
	return body, nil
}

// AradPassClaim 是战令领奖请求（CMD680 / 0x02A8）。
//
// 抓包实证（2026 抓包 21:34:51.600，scheduleId 516）：
//
//	04 02 00 00 | 00 00 00 00 | 01 00 00 00 | 00 00 00 00
//	<赛季 516>    <轨道 0>      <奖励槽 1>    <保留 0>
//
// ⚠ 只有这一个样本，所以「轨道」与「奖励槽」的语义是**推断**：轨道 0 = 普通、2 = 皇家；
// 奖励槽对应源 `[reward table]` 的 slot（1 起）。调用方对它做范围校验，不要直接采信。
type AradPassClaim struct {
	Event, Track, Slot, Reserved uint32
}

// EventRequestTrack115 读事件请求（CMD680/681）的**第 2 个 u32**（"轨道/动作"位）。
//
// 8B 与 16B 两种形态都有这个字段（16B 才在后面多一个奖励槽）；帧不足 8B 返回 false。
func EventRequestTrack115(p []byte) (uint32, bool) {
	if len(p) < 8 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(p[4:]), true
}

// AradPassRoyalDailyTrack 是 CMD680 里「轨道」位的取值 **3** —— 它不代表某条奖励轨，
// 而是「**领取 Royal Pass 每日登录礼包**」这个动作。
//
// ★ 实机（2026-10-11）：Royal Pass 页签点 `Get (1)`（Royal Pass Daily Login rewards），
// 客户端发 `516, 3`（**8 字节**：只有活动号 + 这个 3，没有奖励槽字段）。
// 此前我们把 8B 形态一律当"进城自动帧"忽略 ⇒ 业主点几次都领不下来。
//
// 面板原文：`Once a day / Your Max Royal Pass Character Selection count (1) determines
// your rewards. (Reset daily at 9:00 UTC.)` ⇒ **每天 09:00 UTC 换日**（注意这条线
// 是 09:00，与任务行的 06:00 不同 —— 两者都是客户端自己写的规则）。
const AradPassRoyalDailyTrack uint32 = 3

// DecodeAradPassClaim 解析战令领奖请求（8B 或 16B；8B 时只有赛季与轨道）。
func DecodeAradPassClaim(p []byte) (AradPassClaim, error) {
	var r AradPassClaim
	if len(p) != 8 && len(p) != 16 {
		return r, fmt.Errorf("arad pass: invalid claim length %d", len(p))
	}
	r.Event = binary.LittleEndian.Uint32(p)
	r.Track = binary.LittleEndian.Uint32(p[4:])
	if len(p) >= 16 {
		r.Slot = binary.LittleEndian.Uint32(p[8:])
		r.Reserved = binary.LittleEndian.Uint32(p[12:])
	}
	if r.Event == 0 {
		return r, fmt.Errorf("arad pass: claim event id required")
	}
	return r, nil
}

// putU24 写小端 24 位整数（0x08A1 的经验字段）。
func putU24(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
}
