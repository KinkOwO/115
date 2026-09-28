package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD80：当前客户端 145AEB720 的发送顺序；20260925 实机向量：
// 00030c00c2c20506004100ffff0e000000c3f0c1fabda3b0cdc2b3c4b7bfcbffffffff00。
// 名称只是显示信息，不能据此决定目标装备、强化等级或费用。
type ReinforcementRequest struct {
	Mode, EquipmentSpace     byte
	EquipmentSlot            uint16
	EquipmentTemplate        uint32
	TicketSpace              byte
	TicketSlot, MaterialSlot uint16
	NPC, ProtectionSlot      uint16
	Multiple                 byte
}

func DecodeReinforcement(p []byte) (ReinforcementRequest, error) {
	var r ReinforcementRequest
	if len(p) < 22 {
		return r, fmt.Errorf("强化请求长度不足")
	}
	n := binary.LittleEndian.Uint32(p[13:17])
	if n > 512 || uint64(n)+22 > uint64(len(p)) {
		return r, fmt.Errorf("强化请求名称长度无效")
	}
	// CMD80 使用序号 10 的四字节对齐编码。正文长度不是四的倍数时，
	// 解密结果会保留末尾零填充；不能把填充误判为名称长度错误。
	// 同时保留原生发送器尚未补齐的直接向量，拒绝非零或多余尾部。
	end := int(n) + 22
	if len(p) != end && len(p) != (end+3)/4*4 {
		return r, fmt.Errorf("强化请求尾部长度无效")
	}
	if err := padding(p[end:], 4); err != nil {
		return r, err
	}
	r.Mode, r.EquipmentSpace = p[0], p[1]
	r.EquipmentSlot = binary.LittleEndian.Uint16(p[2:4])
	r.EquipmentTemplate = binary.LittleEndian.Uint32(p[4:8])
	r.TicketSpace = p[8]
	r.TicketSlot = binary.LittleEndian.Uint16(p[9:11])
	r.MaterialSlot = binary.LittleEndian.Uint16(p[11:13])
	tail := p[17+int(n):]
	r.NPC, r.ProtectionSlot = binary.LittleEndian.Uint16(tail), binary.LittleEndian.Uint16(tail[2:])
	r.Multiple = tail[4]
	return r, nil
}

// FixedReinforcementSupported ...
func FixedReinforcementSupported(level byte) bool { return level >= 1 && level <= 15 }

// ★★ CMD80 回包 [1] 的取值规则（改过两次，这里是最终版，别再改回去）：
//
// 客户端唯一处理 CMD80 回包的函数 sub_14529B2F0 把 [1] 读进 v152，结构是：
//
//	if (v152 == 10)      { v52 = _RTDynamicCast(item, &off_14DCB4FF0, &off_14DCB71E8); ... }
//	else if (v152 == 11) { v52 = _RTDynamicCast(item, &off_14DCB4FF0, &off_14DCB71E8); ... }
//	else                 { v52 = _RTDynamicCast(item, &off_14DCB4FF0, &off_14DCB64C8); ... }
//
// 进 10/11 分支之后还要过一道门禁：`if (v52 || v149 == 3)`。
// v149 是结果码，3 = 摧毁。所以：
//
//	result == 3（摧毁）→ v149==3 成立，无论 dynamic_cast 是否成功都能进分支 ✅
//	result != 3        → 只能靠 _RTDynamicCast 到 off_14DCB71E8 成功才进得去 ❌
//
// 这台单机服务端的装备对象转不到 off_14DCB71E8，于是非摧毁结果一旦写 10/11
// 就进不去分支，直接落到分支链之后的公共落点弹 dstr 1658「No items are available.」
// —— 这就是「强化和增幅都被修坏了」的根因（2026-09-28）。
//
// 因此最终规则是：
//
//	非摧毁（result 0/1/2）：[1] 原样回显请求里的 mode（0=强化 / 1=增幅）
//	                        → 走兜底 else 分支，转型目标是宽松得多的 off_14DCB64C8，
//	                          这是长期实机验证可用的老路径。
//	摧毁（result == 3）  ：[1] 必须写成 10（强化）/ 11（增幅）
//	                        → 只有这里才需要硬闯 10/11 分支，因为摧毁对话框和
//	                          36 字节尾部布局都长在分支里面；不写就永远等不到
//	                          结束信号，窗口刷不出来，游戏卡死（伴随 CMD217）。
//
// 10 = 强化（分支里 56 字节结果对象 sub_1412DCDE0，注册 NOTI 2478）
// 11 = 增幅（分支里 80 字节结果对象 sub_1410E8CD0，注册 NOTI 2581）
const (
	cmd80KindReinforce byte = 10
	cmd80KindAmplify   byte = 11
)

// cmd80UpgradeKind 决定回包 [1] 到底写什么。见上面常量块里的完整推导。
// 只有摧毁（result==3）才升级成 10/11，其余一律回显请求 mode。
func cmd80UpgradeKind(mode, result byte) byte {
	if result == 3 {
		if mode == 1 {
			return cmd80KindAmplify
		}
		return cmd80KindReinforce
	}
	return mode
}

// cmd80Tail 拼 CMD80 回包 [17] 之后的尾部。两个布局不同，别混用：
//
//	非摧毁（整包 35 字节）: [17..18]=槽位 0xffff  [19..22]=0  [23]=0  [24]=0  [25..34]=10 字节
//	摧毁  （整包 36 字节）: [17]=额外条目数       再接上面那 18 字节
//
// 摧毁分支（result==3）会先读一个 u8 当「额外条目数」，再按 (u16 槽, u32, u32)
// 循环读那么多条。如果把非摧毁布局的 0xffff 留在 [17]，客户端会当 255 条去读，
// 一个 35 字节的包要读 2500 字节 —— 直接读爆，这就是 CMD217 的来源。
func cmd80Tail(p []byte, destroyed bool) []byte {
	if destroyed {
		p = append(p, 0) // 额外条目数：没有
	}
	p = binary.LittleEndian.AppendUint16(p, 0xffff) // 槽位：无
	p = binary.LittleEndian.AppendUint32(p, 0)
	p = append(p, 0, 0)
	p = append(p, make([]byte, 10)...)
	return p
}

// CMD80 成功体按 14529B2F0 的全部 reader 构造，包括末尾材料与外观字段。
// result=0 表示强化成功，result=1 表示概率失败且等级不变；均消耗一张券。
func ReinforcementTicketReply(r ReinforcementRequest, remaining uint32, old, level, result byte) ([]byte, error) {
	if r.Mode != 0 || (r.EquipmentSpace != 0 && r.EquipmentSpace != 3) ||
		r.TicketSpace != 0 || result > 1 ||
		(result == 0 && !FixedReinforcementSupported(level)) || (result == 1 && old != level) {
		return nil, fmt.Errorf("强化券结果不符合客户端固定券规则")
	}
	// 固定券永远不会摧毁（result 只可能是 0/1），所以这里直接回显 mode。
	p := []byte{1, cmd80UpgradeKind(r.Mode, result)}
	p = binary.LittleEndian.AppendUint16(p, r.TicketSlot)
	p = binary.LittleEndian.AppendUint32(p, remaining)
	p = binary.LittleEndian.AppendUint16(p, 0xffff) // 没有保护券。
	p = append(p, 1, old, result, level, r.EquipmentSpace)
	p = binary.LittleEndian.AppendUint16(p, r.EquipmentSlot)
	p = cmd80Tail(p, false)
	// 与当前服务端 EquippedAppearance 的 WeaponA/B/WeaponTail 默认值一致。
	return p, nil
}

// CMD80 金币强化（材料 + 金币）的成功体。与固定券共用 14529B2F0 的 reader 布局，
// 槽位与剩余量换成被消耗的材料；差别只有两点：
//
//  1. 等级上限按客户端实机上限 15（超过会出现 ADD_HACKTYPE_CNT 与窗口锁死）；
//  2. 失败分支与固定券一样要求 old == level —— 实机把 8→7 的降级写进回包同样被判定异常，
//     真实掉级改由随后的装备行下发（客户端按行更新显示）。
func ReinforcementGoldReply(r ReinforcementRequest, remaining uint32, old, level, result byte) ([]byte, error) {
	if r.Mode != 0 || (r.EquipmentSpace != 0 && r.EquipmentSpace != 3) ||
		r.TicketSpace != 0 || result > 1 || level > GoldReinforcementMaxLevel || old > GoldReinforcementMaxLevel ||
		(result == 0 && level > GoldReinforcementResultCap) ||
		(result == 1 && old != level) {
		return nil, fmt.Errorf("金币强化结果不符合客户端规则")
	}
	// 金币强化同样不会摧毁（result > 1 已被上面拒绝），回显 mode 即可。
	p := []byte{1, cmd80UpgradeKind(r.Mode, result)}
	p = binary.LittleEndian.AppendUint16(p, r.TicketSlot)
	p = binary.LittleEndian.AppendUint32(p, remaining)
	p = binary.LittleEndian.AppendUint16(p, 0xffff)
	p = append(p, 1, old, result, level, r.EquipmentSpace)
	p = binary.LittleEndian.AppendUint16(p, r.EquipmentSlot)
	p = cmd80Tail(p, false)
	return p, nil
}

// AmplifyUpgradeReply 是 CMD80 **mode=1（增幅）** 的结果体。
//
// 增幅不是独立 opcode，而是 CMD80 的 mode=1（2026-09-28 实机抓包确认：44 字节明文，
// 与强化共用 ReinforcementRequest，唯一差别是 [0]=1）。客户端处理 CMD80 回包的
// handler 只有一个（sub_14529B2F0），所以增幅回包**沿用与强化完全相同的布局**，
// 只把 [1] 的升级种类写成 11；[11]/[13] 换成增幅的旧/新等级。
//
//	[0]=1 状态字节（分派器消费）
//	[1]=增幅种类：非摧毁回显 mode=1；result==3（摧毁）时才写成 11
//	    —— 见 cmd80UpgradeKind 的推导，无脑写 11 会让非摧毁结果弹 1658。
//	[2..3]=增幅材料槽 [4..7]=材料剩余 [8..9]=0xffff
//	[10]=1 [11]=旧增幅等级 [12]=结果(0成功/1失败不变/2降级/3摧毁) [13]=新增幅等级 [14]=装备空间
//	[15..16]=装备槽，之后接 cmd80Tail：非摧毁 35 字节，摧毁多一个条目数字节 → 36 字节
func AmplifyUpgradeReply(r ReinforcementRequest, remaining uint32, oldLevel, newLevel, result byte) ([]byte, error) {
	if r.Mode != 1 {
		return nil, fmt.Errorf("增幅回包的 mode 必须是 1，收到 %d", r.Mode)
	}
	// 结果码必须与等级变化自洽：客户端 handler（sub_14529B2F0）对 result==1 要求
	// old == new、对 result==2 要求 old > new；不一致会被判定异常并锁死增幅窗口
	// （强化那边 ReinforcementGoldReply 已有同样约束）。
	switch result {
	case 0:
		if int(newLevel) != int(oldLevel)+1 {
			return nil, fmt.Errorf("增幅成功回包的新等级必须是旧等级 +1")
		}
	case 1:
		if newLevel != oldLevel {
			return nil, fmt.Errorf("增幅失败（等级不变）回包要求新旧等级相同")
		}
	case 2:
		if int(oldLevel) <= int(newLevel) {
			return nil, fmt.Errorf("增幅失败（降级）回包要求旧等级 > 新等级")
		}
	case 3:
		// 摧毁：等级已无意义，不做约束。
	default:
		return nil, fmt.Errorf("未知的增幅结果码 %d", result)
	}
	p := []byte{1, cmd80UpgradeKind(r.Mode, result)}
	p = binary.LittleEndian.AppendUint16(p, r.TicketSlot)
	p = binary.LittleEndian.AppendUint32(p, remaining)
	p = binary.LittleEndian.AppendUint16(p, 0xffff)
	p = append(p, 1, oldLevel, result, newLevel, r.EquipmentSpace)
	p = binary.LittleEndian.AppendUint16(p, r.EquipmentSlot)
	// result==3（摧毁）必须走 36 字节布局：客户端先读 [17] 当条目数再循环。
	p = cmd80Tail(p, result == 3)
	return p, nil
}

// GoldReinforcementMaxLevel 是装备实例行偏移 10 低五位能表达的最高强化等级。
const GoldReinforcementMaxLevel = 31

// GoldReinforcementResultCap 是客户端 CMD80 reader 实机接受的结果等级上限（成功分支）。
const GoldReinforcementResultCap = 15
