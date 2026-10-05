package protocol

import (
	"encoding/binary"
	"fmt"
	"sort"
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
func BoostTrainingStatus115(v BoostTrainingState115) ([]byte, error) {
	if v.Mode != 0 && v.Mode != 2 {
		return nil, fmt.Errorf("unsupported boost training mode %d", v.Mode)
	}
	if !v.Active && (v.Step != 0 || v.Phase != 0) {
		return nil, fmt.Errorf("inactive boost training carries progress")
	}
	if v.Mode == 2 && v.Active {
		return nil, fmt.Errorf("finished boost training cannot be active")
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

// BoostRoster115 编码 2639 包体：u32 行数 + 每行 {u32 名单槽位, u8 轨道模式}。
// donor-live：空账号 = {0,0,0,0}（交付测试钉住），非空按旧端同形状；未过 IDA 门禁。
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
		p = add32(p, r.Slot)
		p = append(p, r.Mode)
	}
	return p, nil
}

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

// BoostChallengeStatus115 编码 2722 包体：登记标记 + 毕业等级标记 + u32 行数 +
// 每行 {u32 索引, u8 解锁, u8 解锁已领, u32 进度, u32 已领次数}。
// donor-live：665 是毕业后**可选**活动（默认由 -boostup-challenge 门控），旧端把此帧
// 列为未闭环；行序按索引升序，保证同一状态每次重算出同一帧。
func BoostChallengeStatus115(s BoostChallengeState115) ([]byte, error) {
	indexes := make([]byte, 0, len(s.Rows))
	for id := range s.Rows {
		indexes = append(indexes, id)
	}
	sort.Slice(indexes, func(i, j int) bool { return indexes[i] < indexes[j] })
	p := []byte{0, 0}
	if s.Enrolled {
		p[0] = 1
	}
	p = add16(p, s.LevelMarker)
	p = add32(p, uint32(len(indexes)))
	for _, id := range indexes {
		r := s.Rows[id]
		p = add32(p, uint32(id))
		p = append(p, boolByte(r.Unlocked), boolByte(r.UnlockClaimed), 0, 0)
		p = add32(p, r.Progress)
		p = add32(p, r.Claims)
	}
	return p, nil
}

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
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

// DecodeBoostCapsule115 复用本树的 CMD507 通用读取（stackable_action.go），
// 只额外要求动作号是胶囊：不新建第二套 507 解析器。
func DecodeBoostCapsule115(p []byte) (BoostCapsuleRequest115, error) {
	slot, action, e := DecodeStackableAction(p)
	if e != nil {
		return BoostCapsuleRequest115{}, e
	}
	if action != CapsuleAction {
		return BoostCapsuleRequest115{}, fmt.Errorf("stackable action %d is not the boost capsule", action)
	}
	return BoostCapsuleRequest115{Slot: slot, Space: p[2]}, nil
}
