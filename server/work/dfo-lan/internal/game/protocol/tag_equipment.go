package protocol

import (
	"encoding/binary"
	"fmt"
)

// TagEquipment 将角色实例投影到145962410调用的1452C1540装备块。
// 普通登录的DetailedEquipment保持不变；队友没有随后补发NOTI13的机会。
// 字段映射取自1452C1EB3..1452C2123：原生临时181字节物品起点为rbp+1E0。
// 1452C15D3/1452C2548/1452C2EF7 使用48格已读标记，槽号覆盖0..47；
// 这是协议容器容量，不是装备玩法或资格表。36..46等扩展栏走同一行布局。
func TagEquipment(rows []DetailedWorn) ([]byte, error) {
	if len(rows) > 48 {
		return nil, fmt.Errorf("队友穿戴数量超过客户端容量")
	}
	p := []byte{byte(len(rows))}
	seen := map[uint16]bool{}
	for _, item := range rows {
		if item.Slot >= 48 || item.Template == 0 || seen[item.Slot] {
			return nil, fmt.Errorf("队友穿戴槽位或模板无效：%d", item.Slot)
		}
		seen[item.Slot] = true
		r := OrdinaryItem(item.Slot, item.Template, 0)
		if len(item.Record) != 0 {
			if len(item.Record) != len(r) || binary.LittleEndian.Uint32(item.Record[2:]) != item.Template {
				return nil, fmt.Errorf("队友装备实例与模板不一致：%d", item.Template)
			}
			copy(r[:], item.Record)
		}
		binary.LittleEndian.PutUint16(r[11:], item.Durability)
		// 已有实例以181字节记录为准，与普通背包EquipmentRow保持一致；
		// 旧存档的顶层Period可能为0，不能据此清除实例中仍有效的期限。
		period := item.Period
		if len(item.Record) != 0 {
			period = binary.LittleEndian.Uint32(r[56:60])
		}
		binary.LittleEndian.PutUint32(r[56:], ItemPeriodForWire(item.Template, period))
		// 这些区域由原生reader初始化，但不从此包读取。未接对应补充通知前
		// 拒绝有值的实例，不能静默丢失已保存的养成数据。
		// Creature EquipmentPayload writes the same instance key at +6 and
		// +24. The compact reader 1452C1682 -> v184 -> 1452C1EB7 and
		// constructor 14576D9EA preserve +6; neither reads +24. This
		// duplicate is identity, not extra saved growth. Permit only the
		// matching duplicate on the native creature/creature-skin slots.
		spans := [][2]int{{13, 14}, {22, 56}, {60, 76}, {82, 83}, {99, 103}}
		if item.Slot == 26 || item.Slot == 32 {
			key := binary.LittleEndian.Uint32(r[6:10])
			mirror := binary.LittleEndian.Uint32(r[24:28])
			if mirror != 0 && mirror != key {
				return nil, fmt.Errorf("队友宠物%d实例key不一致", item.Template)
			}
			spans = [][2]int{{13, 14}, {22, 24}, {28, 56}, {60, 76}, {82, 83}, {99, 103}}
		}
		for _, span := range spans {
			for _, value := range r[span[0]:span[1]] {
				if value != 0 {
					return nil, fmt.Errorf("队友装备%d含尚未接入补充通知的实例字段%d", item.Template, span[0])
				}
			}
		}
		avatar := item.Slot <= 11
		if len(item.AvatarOptions) > 4096 || len(item.AvatarSockets) > 4096 ||
			!avatar && (len(item.AvatarOptions) != 0 || len(item.AvatarSockets) != 0 || item.HeaderTemplateA != 0 || item.HeaderTemplateB != 0) {
			return nil, fmt.Errorf("队友装备附加数据类型或长度无效")
		}
		// 槽9的外观替换还会在头部读取模板相关块，不能套用普通装扮布局。
		if item.Slot == 9 && item.HeaderTemplateA != 0 {
			return nil, fmt.Errorf("队友光环外观替换布局尚未核实")
		}
		p = append(p, byte(item.Slot))
		p = append(p, r[2:13]...)
		p = append(p, r[110:112]...)
		p = append(p, r[148:158]...)
		p = add32(add32(p, item.HeaderTemplateA), item.HeaderTemplateB)
		p = append(p, r[14:22]...)
		if avatar {
			p = append(add32(p, uint32(len(item.AvatarOptions))), item.AvatarOptions...)
			p = append(add32(p, uint32(len(item.AvatarSockets))), item.AvatarSockets...)
		}
		if item.Slot == 26 || item.Slot == 32 {
			p = append(add32(p, 0), r[159])
		} else if r[159] != 0 {
			return nil, fmt.Errorf("非宠物队友装备包含宠物扩展")
		}
		p = append(p, 0) // 实例外的辅助属性列表；本地未保存此列表。
		p = append(p, r[56:60]...)
		p = append(p, 0) // 1451AA420的嵌套列表。
		p = append(p, r[76:82]...)
		p = append(p, 0) // 账号级附加开关，不写入物品实例。
		p = append(p, r[83:86]...)
		blob := func(start, end int) {
			p = append(add32(p, uint32(end-start)), r[start:end]...)
		}
		blob(86, 90)
		blob(90, 98)
		p = append(p, r[98])
		blob(103, 108)
		blob(108, 109)
		blob(109, 110)
		blob(112, 113)
		p = append(p, r[113:117]...)
		blob(117, 121)
		blob(121, 126)
		p = append(p, r[126])
		blob(127, 148)
		p = append(p, r[158])
		blob(160, 161)
		blob(161, 164)
		p = append(p, r[164])
		blob(165, 169)
		p = append(p, r[169:171]...)
		blob(171, 172)
		p = append(p, r[172])
		blob(173, 177)
		p = append(p, r[177:181]...)
	}
	return append(p, make([]byte, detailedWornBlockTrailerSize)...), nil
}
