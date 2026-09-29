package protocol

import (
	"encoding/binary"
	"fmt"
)

// 装备技能栏 / 冷却提醒 / 自定义按键 的持久化线格式。
//
// 事实来源：外部文档 `equipment-skill-persistence-20260929.md`（作者侧**已实机确认**：
// 重登后三处设置都保留）。本仓此前**完全没有**这一族处理（既没解析 2254/2256/2257，
// 也没在登录时发 2609）⇒ 客户端只能保住本会话内的配置。
//
// 请求（C2S 2254 / 2256，**均为 96 字节解密正文**）：
//
//	[0:13]  13 字节 0（固定前缀）
//	[13]    计数：2254 = 40（装备技能槽位数）、2256 = 10（自定义指令组数）
//	[14:94] 80 字节快照：40 × i16（技能槽位） 或 10 × 8 字节（指令组）
//	[94:96] 2 字节 0（零填充）
//
// 应答（S2C 2609）：两组 80 字节快照直接相连（共 160 字节），**追加 8 字节零填充**
// = 168 字节。客户端一次消费前 160 字节。
const (
	// EquipmentSkillRequestSize 是 C2S2254/2256 的解密正文长度。
	EquipmentSkillRequestSize = 96
	// EquipmentSkillPrefixSize 是正文里固定为 0 的前缀长度。
	EquipmentSkillPrefixSize = 13
	// EquipmentSkillSnapshotSize 是单项快照长度（两种请求都是 80）。
	EquipmentSkillSnapshotSize = 80
	// EquipmentSkillCount 是 2254 的计数（40 个 i16 技能槽位）。
	EquipmentSkillCount byte = 40
	// EquipmentCommandCount 是 2256 的计数（10 个 8 字节指令组）。
	EquipmentCommandCount byte = 10
	// EquipmentSkillInfoSize 是 S2C2609 的正文长度（80+80+8）。
	EquipmentSkillInfoSize = 168
)

// DecodeEquipmentSkillSnapshot 解析 C2S2254/2256 的正文，返回其中的 80 字节快照。
//
// wantCount 是这一步期望的计数（40 或 10）—— 它把两种请求分开：形状完全相同，
// 只有计数不同。前缀、计数与零填充都严格校验：客户端发来的任何偏离都说明我们
// 认错了包，宁可拒绝也不要落一份错位的快照进库。
func DecodeEquipmentSkillSnapshot(body []byte, wantCount byte) ([]byte, error) {
	if len(body) != EquipmentSkillRequestSize {
		return nil, fmt.Errorf("equipment skill request must be %d bytes", EquipmentSkillRequestSize)
	}
	for i := 0; i < EquipmentSkillPrefixSize; i++ {
		if body[i] != 0 {
			return nil, fmt.Errorf("equipment skill request prefix must be zero")
		}
	}
	if body[EquipmentSkillPrefixSize] != wantCount {
		return nil, fmt.Errorf("equipment skill snapshot count mismatch")
	}
	at := EquipmentSkillPrefixSize + 1
	out := make([]byte, EquipmentSkillSnapshotSize)
	copy(out, body[at:at+EquipmentSkillSnapshotSize])
	for _, b := range body[at+EquipmentSkillSnapshotSize:] {
		if b != 0 {
			return nil, fmt.Errorf("equipment skill request padding must be zero")
		}
	}
	return out, nil
}

// EquipmentSkillInfo 构造 S2C2609：两组快照相连 + 8 字节零填充。
//
// 两组缺一不可（客户端一次读 160 字节），所以任一组为空时用等长零块补齐 ——
// 这与"该角色还没设过"是同一语义（客户端拿到全零等于回到默认）。
func EquipmentSkillInfo(skills, commands []byte) ([]byte, error) {
	if len(skills) != 0 && len(skills) != EquipmentSkillSnapshotSize {
		return nil, fmt.Errorf("equipment skill snapshot must be %d bytes", EquipmentSkillSnapshotSize)
	}
	if len(commands) != 0 && len(commands) != EquipmentSkillSnapshotSize {
		return nil, fmt.Errorf("equipment command snapshot must be %d bytes", EquipmentSkillSnapshotSize)
	}
	out := make([]byte, EquipmentSkillInfoSize)
	copy(out, skills)
	copy(out[EquipmentSkillSnapshotSize:], commands)
	return out, nil
}

// EquipmentSkillSlots 把 80 字节快照按 40 个 i16 解出来（只读用途：日志/测试）。
func EquipmentSkillSlots(snapshot []byte) ([]int16, error) {
	if len(snapshot) != EquipmentSkillSnapshotSize {
		return nil, fmt.Errorf("equipment skill snapshot must be %d bytes", EquipmentSkillSnapshotSize)
	}
	out := make([]int16, EquipmentSkillCount)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(snapshot[i*2:]))
	}
	return out, nil
}
