package protocol

import (
	"encoding/binary"
	"fmt"
)

// 活动 662（成长胶囊教学）与 665（毕业后挑战）的帧。
//
// 帧形来源分两类，逐条注明，不要混为一谈：
//
//	A. envelope —— 事件请求/回包的本树既有口径：CMD681 的三包（u32 event / u32 sub /
//	   u32 parameter）来自已 IDA 闭环的发送方 141aa9240（见 radiant_box.go 的
//	   DecodeRadiantBoxOpen），kind-1 回包体首的 u8 状态字节见 CLIENT-MECHANICS §4。
//	   这里只把同一布局做成活动共用的通用读取，不新造解析器。
//	B. donor-live —— 旧端 662 实机验收过的**观测**布局（NOTI2638 五字节轨道帧、
//	   NOTI2265 礼盒状态表、NOTI2639 名单标记）。旧端文档明写这些布局**未经 IDA 门禁**，
//	   以实机接受为准；本文件按旧端口径原样移植，字节断言与交付测试一致。
//	   要改动它们必须先取实机或 IDA 证据，不得在此叠新猜测。
//
// 挑战（665）与礼盒（643）的回复体（B 类中证据最薄的一处）：结构按旧端调用点还原，
// 实机复测前不得当成已闭环。

// EventRequest115 是 EVENT_REQUEST(681) / EVENT_REWARD(680) 共用的事件请求三包。
type EventRequest115 struct {
	Event        uint32
	Sub          uint32
	Parameter    uint32
	HasParameter bool
}

// DecodeEventRequest115 读取事件请求。claim=true 表示领奖请求（680）：必须点名
// 参数（步骤号），否则客户端窗口拿不到要领取的那一行。两种正文宽度：665 走 A 类
// 三包（event/sub/parameter），662 走实机观测的两包（event/step），见函数体内的
// 实机帧记录。
func DecodeEventRequest115(p []byte, claim bool) (EventRequest115, error) {
	if len(p) < 8 {
		return EventRequest115{}, fmt.Errorf("event request is shorter than two words")
	}
	r := EventRequest115{Event: binary.LittleEndian.Uint32(p), Sub: binary.LittleEndian.Uint32(p[4:])}
	if len(p) >= 12 {
		r.Parameter = binary.LittleEndian.Uint32(p[8:])
		r.HasParameter = true
	} else {
		// 662 的 680/681 只有两包：实机 2026-10-04 18:04:04 第一关「Get」发的是
		// `96020000 01000000` = 事件号 + 关卡号，没有第三包（donor 的 680/681 夹具
		// 同为 8 字节）。按 A 类三包读时第二包是 sub，参数缺失 ⇒ 领奖被拒死。
		r.Parameter = r.Sub
		r.HasParameter = r.Parameter != 0
	}
	if claim && !r.HasParameter {
		return r, fmt.Errorf("event reward request carries no parameter")
	}
	return r, nil
}

// EventReply115 是事件类 kind-1 回包：状态 1 + 事件号 + 十格回执值。
// donor-live：旧端 662/665 的 680/681 回包按此形状被客户端接受（未过 IDA 门禁）。
func EventReply115(event uint32, values [10]uint32) []byte {
	p := append([]byte{1}, add32(nil, event)...)
	for _, v := range values {
		p = add32(p, v)
	}
	return p
}

// EventRefusal115 是事件类拒绝回包。wide 对应 681 的处理方按 u32 读错误码、
// 680 按 u16 读（旧端口径，同 Refusal 的 kind-1 前导状态字节 0）。
func EventRefusal115(event uint32, code uint16, wide bool) []byte {
	p := append([]byte{0}, add32(nil, event)...)
	if wide {
		return add32(p, uint32(code))
	}
	return add16(p, code)
}

// EventGiftState115 是 NOTI2265（EVENT_GIFT_USER_AVAILABILITY）的一行。
type EventGiftState115 struct {
	Gift    uint16
	Claimed bool
}

// EventGiftStates115 编码 2265 包体：u16 行数 + 每行 {u16 礼盒号, u8 已领}。
// donor-live 且由交付测试钉住字节：两行未领 = {2,0,117,0,0,118,0,0}。
func EventGiftStates115(rows []EventGiftState115) ([]byte, error) {
	seen := map[uint16]bool{}
	p := add16(nil, uint16(len(rows)))
	for _, r := range rows {
		if r.Gift == 0 || seen[r.Gift] {
			return nil, fmt.Errorf("duplicate or unnamed event gift")
		}
		seen[r.Gift] = true
		p = add16(p, r.Gift)
		if r.Claimed {
			p = append(p, 1)
		} else {
			p = append(p, 0)
		}
	}
	return p, nil
}

// DecodeEventGiftRequest115 读取 CMD643 的领奖请求：u16 礼盒号 + u8 直发标记。
// donor-live：实机捕获 8 字节帧 {gift lo, gift hi, direct, ...}，尾字节是客户端补齐。
type EventGiftRequest115 struct {
	Gift   uint16
	Direct byte
}

func DecodeEventGiftRequest115(p []byte) (EventGiftRequest115, error) {
	if len(p) < 3 {
		return EventGiftRequest115{}, fmt.Errorf("event gift request is shorter than its header")
	}
	r := EventGiftRequest115{Gift: binary.LittleEndian.Uint16(p), Direct: p[2]}
	if r.Gift == 0 {
		return r, fmt.Errorf("event gift request names no gift")
	}
	return r, nil
}

// EventGiftReply115 是 643 的 kind-1 回执：状态 1 + 礼盒号 + 结果。
func EventGiftReply115(gift uint16, result uint16) []byte {
	return add16(add16([]byte{1}, gift), result)
}

// BoostTrainingState115 是 NOTI2638（BOOST_UP_MODE_CHARAC_INFO）的语义视图。
type BoostTrainingState115 struct {
	Mode   byte // 0 = 未完成（引导轨道生效），2 = 未激活或已结束
	Step   byte
	Phase  byte // 0 引导, 1 可领, 2 已领/待任务
	Active bool
}

// BoostTrainingStatus115 编码 2638 的五字节包体 [mode, step, phase, 0, active]。
// donor-live：旧端多轮实机接受（含 09:28:31 捕获的 `00 03 00 00 01`），未过 IDA 门禁。
//
// ⚠️ **2026-10-06 删掉了一条与官服相反的守卫**：原实现禁 `mode==2 && active`，报
// "finished boost training cannot be active"。但官服的**毕业态就是** `02 0c 00 00 01`
// （mode=2 / step=12 / phase=0 / active=1）—— 见参考包
// `活动Boost与胶囊教学-20260927.zip` 的 `规格文档/2638-BOOSTUPCHARACTER.md`「已推翻」一节：
// 「第12步不是第12张训练副本：该源只有 11 个 step info，官服在最后领取后转为 02/0c/00/00/01」。
//
// 而 `boostup.next()` 越过最后一关时正好产出 `Step=12 / Phase=0 / Finished=true`，
// `boostTrainingRestore` 据此组出的就是那条官方帧 —— 被守卫挡成编码错误 ⇒
// **最后一关领奖整笔被拒**。实机 2026-10-06 03:39:44（会话 `..._033224_339881`）：
//
//	boost_event_request_refused: "finished boost training cannot be active"
//	request_hex = 96 02 00 00 0b 00 00 00   (事件 662 / 第 11 关)
//
// 该守卫零测试覆盖，编码器也只有一个调用方（`boostTrainingRestore`），
// 但那个函数被 6 处调用（含进城恢复 client_entry.go）⇒ 毕业之后每条路径都编码失败。
// 真正的不变量只有「未激活不得携带进度」（见上一段），mode/active 的取值不另设限制。
func BoostTrainingStatus115(v BoostTrainingState115) ([]byte, error) {
	if v.Mode != 0 && v.Mode != 2 {
		return nil, fmt.Errorf("unsupported boost training mode %d", v.Mode)
	}
	if !v.Active && (v.Step != 0 || v.Phase != 0) {
		return nil, fmt.Errorf("inactive boost training carries progress")
	}
	active := byte(0)
	if v.Active {
		active = 1
	}
	return []byte{v.Mode, v.Step, v.Phase, 0, active}, nil
}

// BoostRosterRow115 是 NOTI2639（BOOST_UP_MODE_ALL_CHARAC_INFO）的一行。
type BoostRosterRow115 struct {
	Slot uint32
	Mode byte
}

// 客户端读取链（IDA 闭环：handler sub_140C131D0，游标 sub_146EA09F0=1 字节 /
// sub_146EA0BA0=4 字节，顺序 u8→u32→u8→u8）：包体 = u32 行数 + 每行**定长 7 字节**
// {u8 轨道, u32 名单位次, u8 状态, u8 保留}，按 u32 次序插入 ctx+776 的树；
// 访问器 sub_140C13650 用 node+40 != 2 判定「仍在训练中」，与 Mode 的 2=已毕业同口径。
// 轨道/保留取自官服 2639 帧（cap43 全 11 帧：`01 00 00 00 | 03 00 00 00 00 01 00 | …`，
// 12 角色账号仍是这一条），详见 analysis/dumps/noti2639/。
const (
	BoostRosterRowBytes115 = 7
	boostRosterTrack115    = 3
)

// BoostRoster115 编码 2639 包体：u32 行数 + 每行 {u8 轨道, u32 名单槽位, u8 轨道状态, u8 保留}。
// 旧端 donor 布局把行宽写成 5 字节（{u32 槽位, u8 模式}）：1 行时 4+5=9 ≤ 补齐后的 16
// 字节侥幸被收下，2 行时客户端要 18 字节而包体只有 16 ⇒ 游标越界，客户端回
// CMD217(OVERFLOW) 且体内 0x0A4F=2639（2026-10-06 实机：同账号第二个角色直升后卡在赛利亚、
// 选角名单空白，都是这一条越界）。
func BoostRoster115(rows []BoostRosterRow115) ([]byte, error) {
	seen := map[uint32]bool{}
	p := add32(nil, uint32(len(rows)))
	for _, r := range rows {
		if seen[r.Slot] {
			return nil, fmt.Errorf("duplicate boost roster slot %d", r.Slot)
		}
		seen[r.Slot] = true
		if r.Mode != 0 && r.Mode != 2 {
			return nil, fmt.Errorf("unsupported boost roster mode %d", r.Mode)
		}
		p = append(p, boostRosterTrack115)
		p = add32(p, r.Slot)
		p = append(p, r.Mode, 0)
	}
	return p, nil
}

// 2722 记录块的客户端几何（IDA 闭环，见 BoostChallengeStatus115 注释）。
const (
	BoostChallengeBodyLen     = 323
	BoostChallengeRecordCount = 32
	boostChallengeRecordOff   = 3
	boostChallengeRecordSize  = 10
)

// BoostChallengeRow115 是 665 挑战面板的一行（索引 → 进度事实）。
type BoostChallengeRow115 struct {
	Unlocked      bool
	UnlockClaimed bool
	Progress      uint32
	Claims        uint32
}

// BoostChallengeState115 是 NOTI2722（BOOST_UP_SPEC_UP_CHALLENGE_INFO）的语义视图。
type BoostChallengeState115 struct {
	Enrolled    bool
	LevelMarker uint16
	Rows        map[byte]BoostChallengeRow115
}

// BoostChallengeStatus115 编码 2722 包体 = 客户端读取的定长 323 B 记录块：
//
//	u16 @0            挑战达标等级（源 boostupspecupchallenge.evt 的 GoalLevel；官服字节恒 115）
//	u8  @2            登记/开启
//	32 × @3+10*i      {u8 解锁, u8 解锁奖励已领, u32 进度, u32 已领通关奖励}
//
// 取证链（权威 IDB，逐函数落盘 analysis/dumps/noti2722-{handler,records}/）：
//   - 处理器 sub_140BAE420 用 sub_146EA0BE0(&buf,323) 取 323 B 再整块拷进 manager+328；
//     该 reader 在剩余长度不足时执行 `MEMORY[0]=0`（写空地址）⇒ 短包就是客户端崩溃，
//     实机 2026-10-06 15:50:39 我方 24 B 自造布局后 1.7 s 客户端发 CMD682 退出并落 CrashDNF2.cra。
//   - 记录槽访问器 sub_140BAF000 返回 `manager+331+10*i`（i≤0x1F），与源里
//     `[challenge info]` 最多 32 行的上限一致 ⇒ 一行一槽，索引就是源里的 [no]。
//   - 字段用途取自消费方：sub_140BAF640 读 +0（解锁）、sub_140BAF6B0 读 +1（解锁奖励已领）、
//     sub_140BAF6D0 用「+0 且 !+1」点亮可领红点、sub_140BAEFC0 读 +2 的 u32（进度）、
//     sub_140BAF0F0 算 `(+2)/(源 goal) - (+6 的 u32)`＝还能领几次、sub_140BAF660 判 `(+6) >= 源 repeat`。
//     goal/repeat 由客户端自己按槽位从事件配置容器取（manager+72），所以这里发**原始计数**，
//     不做任何缩放——再算一遍就是 §0.2 的第 5 条重复规则。
//   - 面板/弹窗门禁 sub_140BAF450 = 「事件表里有 665 且 manager+330（=@2）== 1」。
//
// 官服抓包（cap43 16 帧，定长 328 B）与此几何吻合：前 6 字节之外全零，@0=115、@2=1、
// 第 0 槽 +0/+1 依次置 1、+2 的 u32 递增。末尾多出的 5 B 客户端处理器不读，含义未闭环，
// 因此不跟着编（宁短不猜；短只少显示，缺 323 才崩）。
func BoostChallengeStatus115(s BoostChallengeState115) ([]byte, error) {
	p := make([]byte, BoostChallengeBodyLen)
	binary.LittleEndian.PutUint16(p, s.LevelMarker)
	if s.Enrolled {
		p[2] = 1
	}
	for id, r := range s.Rows {
		if int(id) >= BoostChallengeRecordCount {
			return nil, fmt.Errorf("challenge index %d out of client record range", id)
		}
		off := boostChallengeRecordOff + boostChallengeRecordSize*int(id)
		if r.Unlocked {
			p[off] = 1
		}
		if r.UnlockClaimed {
			p[off+1] = 1
		}
		binary.LittleEndian.PutUint32(p[off+2:], r.Progress)
		binary.LittleEndian.PutUint32(p[off+6:], r.Claims)
	}
	return p, nil
}

// BoostChallengeRequest115 是 665 挑战面板的领奖请求（u32 索引 + u32 动作）。
type BoostChallengeRequest115 struct {
	Index  byte
	Action uint32
}

// DecodeBoostChallengeRequest115 读 680 的挑战领奖请求：事件号 + 挑战索引 + 动作，
// 即 A 类事件请求三包（第一格必须是 665，调用方已用 isBoostChallengeRequest 分流）。
func DecodeBoostChallengeRequest115(p []byte) (BoostChallengeRequest115, error) {
	r, e := DecodeEventRequest115(p, true)
	if e != nil {
		return BoostChallengeRequest115{}, e
	}
	if r.Parameter > 32 {
		return BoostChallengeRequest115{}, fmt.Errorf("challenge index %d out of source range", r.Parameter)
	}
	return BoostChallengeRequest115{Index: byte(r.Parameter), Action: r.Sub}, nil
}

// BoostCapsuleRequest115 是胶囊在 CMD507 上的使用请求。
//
// 胶囊的 variant（普通/缓冲两种来源）**不从包里读**：它由物品模板自己在源里的
// 变体号决定（loot 侧按 c.Capsules[模板].Variant 校验），客户端那格填什么都不作数。
type BoostCapsuleRequest115 struct {
	Slot  uint16
	Space byte
}

// CapsuleAction 是胶囊的 [action type] 编号（S-0904 实机捕获）。
const CapsuleAction = 337

// DecodeBoostCapsule115 解析胶囊自己那一份 CMD507 正文，不复用通用零校验。
//
// 客户端构造器（IDA sub_14143F7B0 的动作 337 分支）按固定宽度写：
// u16 槽 | u8 容器 | u32 0 | u32 动作 | u32 参数 | u32 0 | 40 B 零 = 59 B
// （写手 sub_146D76180/sub_146D75CC0/sub_146D75CE0 分别是 u16/u8/u32；实机帧 64 B
// 是传输层补零）。第 5 个字段 p[11:15] 由发送方从物品对象 +856 取得
// （sub_1421B2820，客户端自己还要求 <=1 才发帧）。实机 2026-10-06 18:38/18:41：
// 普通胶囊 590015870 填 0、缓冲（奶系）胶囊 590015871 填 1，与两者源里
// `[boost up mode capsule]` 的第一个参数一致。
//
// 本树不吃这一格（变体按模板在源里的变体号推导），所以它非零不构成拒绝理由。
// 通用 DecodeStackableAction 的「其余字节必须为 0」是为药水/皮肤仓等动作保留的，
// 套到胶囊上会把缓冲胶囊整条直升拒死。
func DecodeBoostCapsule115(p []byte) (BoostCapsuleRequest115, error) {
	if len(p) != 59 && len(p) != 64 {
		return BoostCapsuleRequest115{}, fmt.Errorf("boost capsule request length")
	}
	slot := binary.LittleEndian.Uint16(p)
	if slot == 0 {
		return BoostCapsuleRequest115{}, fmt.Errorf("unsupported boost capsule slot")
	}
	action := binary.LittleEndian.Uint32(p[7:])
	if action != CapsuleAction {
		return BoostCapsuleRequest115{}, fmt.Errorf("stackable action %d is not the boost capsule", action)
	}
	for i, b := range p {
		// 0..2 = 槽(u16)+容器(u8)，7..14 = 动作(u32)+参数(u32)：都是字段本身。
		if i < 3 || (i >= 7 && i < 15) {
			continue
		}
		if b != 0 {
			return BoostCapsuleRequest115{}, fmt.Errorf("unsupported boost capsule fields")
		}
	}
	return BoostCapsuleRequest115{Slot: slot, Space: p[2]}, nil
}
