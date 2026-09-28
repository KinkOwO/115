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

// 14529B2F0 的固定券分支接受 1..15（少数指定模板另有例外）。
// 不把测试券的 +20 伪装成普通逐级强化或随机强化来绕过客户端检查。
func FixedReinforcementSupported(level byte) bool { return level >= 1 && level <= 15 }

// CMD80 成功体按 14529B2F0 的全部 reader 构造，包括末尾材料与外观字段。
// result=0 表示强化成功，result=1 表示概率失败且等级不变；均消耗一张券。
func ReinforcementTicketReply(r ReinforcementRequest, remaining uint32, old, level, result byte) ([]byte, error) {
	if r.Mode != 0 || (r.EquipmentSpace != 0 && r.EquipmentSpace != 3) ||
		r.TicketSpace != 0 || result > 1 ||
		(result == 0 && !FixedReinforcementSupported(level)) || (result == 1 && old != level) {
		return nil, fmt.Errorf("强化券结果不符合客户端固定券规则")
	}
	p := []byte{1, r.Mode}
	p = binary.LittleEndian.AppendUint16(p, r.TicketSlot)
	p = binary.LittleEndian.AppendUint32(p, remaining)
	p = binary.LittleEndian.AppendUint16(p, 0xffff) // 没有保护券。
	p = append(p, 1, old, result, level, r.EquipmentSpace)
	p = binary.LittleEndian.AppendUint16(p, r.EquipmentSlot)
	p = binary.LittleEndian.AppendUint16(p, 0xffff) // 固定券不消耗另一个材料槽。
	p = binary.LittleEndian.AppendUint32(p, 0)
	// 与当前服务端 EquippedAppearance 的 WeaponA/B/WeaponTail 默认值一致。
	p = append(p, make([]byte, 12)...)
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
	p := []byte{1, r.Mode}
	p = binary.LittleEndian.AppendUint16(p, r.TicketSlot)
	p = binary.LittleEndian.AppendUint32(p, remaining)
	p = binary.LittleEndian.AppendUint16(p, 0xffff)
	p = append(p, 1, old, result, level, r.EquipmentSpace)
	p = binary.LittleEndian.AppendUint16(p, r.EquipmentSlot)
	p = binary.LittleEndian.AppendUint16(p, 0xffff)
	p = binary.LittleEndian.AppendUint32(p, 0) // 语义未核实，与券路径保持一致填 0。
	p = append(p, make([]byte, 12)...)
	return p, nil
}

// GoldReinforcementMaxLevel 是装备实例行偏移 10 低五位能表达的最高强化等级。
const GoldReinforcementMaxLevel = 31

// GoldReinforcementResultCap 是客户端 CMD80 reader 实机接受的结果等级上限（成功分支）。
const GoldReinforcementResultCap = 15
