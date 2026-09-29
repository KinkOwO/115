package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD 272 = 附魔宝珠（ENUM_CMDPACKET_ENCHANT_BY_BEAD）。
// 客户端 S2C handler = 0x1452762c0（注册于 sub_1452A1F90: mov edx,0x110 + lea r8,<handler>）。
//
// 请求布局（2026-09-28 实机 8 帧全对得上，16 字节明文）：
//
//	[0]      bead_space   u8   宝珠容器（0=背包）
//	[1..2]   bead_slot    u16  宝珠槽（实测恒在 65..120 投掷品区）
//	[3]      equip_space  u8   装备容器（0=背包 3=已穿戴）
//	[4..5]   equip_slot   u16  装备槽
//	[6..15]               u8[10] 补零
//
// 样本：004c00001800… = 宝珠槽76/装备空间0/装备槽24；004b00031400… = 宝珠75/穿戴3/槽20。
type EnchantByBeadRequest struct {
	BeadSpace  byte
	BeadSlot   uint16
	EquipSpace byte
	EquipSlot  uint16
}

func DecodeEnchantByBead(p []byte) (EnchantByBeadRequest, error) {
	var r EnchantByBeadRequest
	if len(p) < 6 {
		return r, fmt.Errorf("附魔宝珠请求长度不足: %d", len(p))
	}
	r.BeadSpace = p[0]
	r.BeadSlot = binary.LittleEndian.Uint16(p[1:])
	r.EquipSpace = p[3]
	r.EquipSlot = binary.LittleEndian.Uint16(p[4:])
	return r, nil
}

// EnchantByBeadReply 是 CMD 272 的成功回包。
//
// 客户端 handler 0x1452762c0 在分派器消费 [0]=1（状态字节）后，依次读：
//
//	u8  [1]      容器
//	u16 [2..3]   槽
//
// 然后按这个 (容器, 槽) 取物品并刷新。失败回包用 Refusal(code)：handler 的
// a2==0 分支读 u16 错误码（客户端把 17 / 19 / 23 映射成不同文案）。
func EnchantByBeadReply(container byte, slot uint16) []byte {
	p := []byte{1, container}
	return binary.LittleEndian.AppendUint16(p, slot)
}
