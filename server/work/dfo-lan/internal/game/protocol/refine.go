package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD 430 = 锻造（Refine）。客户端 handler 为 sub_145886FF0。
//
// 请求布局（来自 2026-09-28 05:42 实机抓包）：
//
//	[0]          equipment_space     u8   装备容器（0=背包 3=已穿戴）
//	[1..2]       equipment_slot      u16  装备槽
//	[3..6]       equipment_template  u32  装备模板
//	[7..8]       material_slot       u16  材料槽（玩家身上的 Powerful Energy）
//	[9..12]      name_len            u32  装备名称长度
//	[13..13+n]   name                UTF-8 装备名称（用于客户端显示确认框）
//	[14+n..15+n]                     u16  尾部保留（实机为 0）
//
// 与 CMD80 请求结构的差别：没有 mode 字节，也没有 TicketSpace/TicketSlot/MaterialSlot
// 三件套，首字节直接是容器，材料只带一个槽。
type RefineRequest struct {
	EquipmentSpace    byte
	EquipmentSlot     uint16
	EquipmentTemplate uint32
	MaterialSlot      uint16
	Name              string
}

func DecodeRefine(p []byte) (RefineRequest, error) {
	var r RefineRequest
	if len(p) < 15 {
		return r, fmt.Errorf("锻造请求长度不足: %d", len(p))
	}
	r.EquipmentSpace = p[0]
	r.EquipmentSlot = binary.LittleEndian.Uint16(p[1:])
	r.EquipmentTemplate = binary.LittleEndian.Uint32(p[3:])
	r.MaterialSlot = binary.LittleEndian.Uint16(p[7:])
	n := binary.LittleEndian.Uint32(p[9:])
	if n > 256 || uint64(13)+uint64(n)+2 > uint64(len(p)) {
		return r, fmt.Errorf("锻造请求名称长度无效: %d", n)
	}
	end := int(n) + 13
	// 名称为 UTF-8，保留原始字节；客户端只用来渲染确认框。
	r.Name = string(p[13:end])
	return r, nil
}

// RefineReply 是 CMD 430（锻造）的成功/失败回包。
//
// 布局来自客户端真实 handler sub_145886FF0：分派器已消费 [0]=1 状态字节，
// handler 从 [1] 开始读：
//
//	u16 [1..2]   材料槽（客户端按 [3..6] 的剩余量刷新该格；0 则移除）
//	u32 [3..6]   材料剩余数量
//	u8  [7]      旧锻造等级
//	u8  [8]      结果码：0=成功 / 1=失败不变 / 2=降级 / 3=摧毁 / 4=关闭窗口
//	u8  [9]      新锻造等级
//	u8  [10]     装备容器
//	u16 [11..12] 装备槽
//
// 官方锻造规则：失败等级不变、装备不碎，所以实际只会回 0 或 1。
// 结果码语义与 CMD80 完全一致，只是客户端对 code 17 的文案不同：
// 锻造是 35076 "The equipment cannot be refined."，而不是 1652。
func RefineReply(materialSlot uint16, materialRemaining uint32, oldLevel, newLevel, result, equipSpace byte, equipSlot uint16) ([]byte, error) {
	if result > 4 {
		return nil, fmt.Errorf("锻造结果码 %d 超出 0..4", result)
	}
	if result == 0 && int(newLevel) != int(oldLevel)+1 {
		return nil, fmt.Errorf("锻造成功时新等级必须是旧等级 +1")
	}
	if result == 1 && newLevel != oldLevel {
		return nil, fmt.Errorf("锻造失败不变时新等级必须等于旧等级")
	}
	p := []byte{1}
	p = binary.LittleEndian.AppendUint16(p, materialSlot)
	p = binary.LittleEndian.AppendUint32(p, materialRemaining)
	p = append(p, oldLevel, result, newLevel, equipSpace)
	p = binary.LittleEndian.AppendUint16(p, equipSlot)
	return p, nil
}
