package inventory

// Historical one-step implementation retained only as a regression fixture.
// Production uses the native PVF recipe and per-instance consumable contract.
import "fmt"

// 莱纳斯模具：复制（幻化）一件武器外观时要吃掉一枚。
//
// 依据是客户端自己的确认框文案：100007147 "Replicating consumes the selected
// weapon and materials.\nContinue?"、100007180 "The weapon and materials selected
// for the Replication will disappear."，缺料时的提示 100007183 "Not enough Linus'
// Steel Molds or Molds."。两种模具都算数，按玩家选中的那种扣，缺货才退回另一种。
const (
	LinusMold      uint32 = 10308836 // Linus' Mold，190 点券
	LinusSteelMold uint32 = 10308357 // Linus' Steel Mold，690 点券
)

// MoldForMode 把 CMD1592 里的 mode 映射到玩家在幻化窗口里选中的那枚模具。
//
// mode 是客户端窗口内的选择位：发送器 0x14407dca0 读
// (*(u8*)(window+0x7F0) != 0) ? 1 : 2，实机抓包里只有 1 和 2 两个取值，而窗口里恰好
// 只有这两种模具；包体里再无任何别的字段能表达材料，所以它就是材料选择。此前不看
// mode 一律先扣普通模具，玩家选精钢模具也照样扣掉普通那枚（实机 2026-09-27）。
//
// 极性只写在这一处：实机两种材料各选一次核对，如果与实扣结果相反，把下面两个
// 返回值对调即可。
func MoldForMode(mode uint32) uint32 {
	if mode == 2 {
		return LinusSteelMold
	}
	return LinusMold
}

// otherMold 给出另一种莱纳斯模具；非法输入归一到普通模具。
func otherMold(preferred uint32) uint32 {
	if preferred == LinusMold {
		return LinusSteelMold
	}
	return LinusMold
}

// ReplicateWeaponSkin 落地一次武器复制：吃掉 list 0 slot 上的武器本体和一枚模具，
// 并把武器模板当作皮肤 id 追加进幻化仓库（Bag.WeaponSkins）。
//
// 全部校验都在改动之前做完。槽位不存在、那一格不是武器、这件武器本职业用不了、
// 两种模具都没有，都直接返回 error 且背包原样不动——宁可什么都不发生，也不能吃
// 装备却不给皮肤。
//
// preferred 是玩家在窗口里选中的那枚模具模板（见 MoldForMode）：先按它扣，缺货才
// 退回另一种。actor 用来核对这件武器是不是本职业能用的：先 [usable job]，再
// [required job skill]（见 RequiredJobSkill）。客户端自己的确认框只拦"不是武器"和
// "已穿戴"，不拦职业，复制出来的皮肤进了仓库本职业根本用不上。
//
// catalog 用来核对武器类型与职业要求，可为 nil（此时仅要求那一格属于装备袋，装备袋
// 里不会出现可叠加物）。
func (b Bag) ReplicateWeaponSkin(slot uint16, catalog *EquipmentCatalog, preferred uint32, actor ReplicationActor) (Bag, WeaponReplication, error) {
	index := -1
	for i, item := range b.Equipment {
		if item.Slot == slot {
			index = i
			break
		}
	}
	if index < 0 {
		return b, WeaponReplication{}, fmt.Errorf("幻化槽位 %d 上没有装备", slot)
	}
	skin := b.Equipment[index].Template
	if catalog != nil {
		// 走 definitionResolved 而不是 EquipmentKind：薄壳武器自身没有 [usable job]，
		// 不追 [import script] 就会把本职业能用的武器判成不能用。
		def, err := catalog.definitionResolved(skin, 0)
		if err != nil {
			return b, WeaponReplication{}, fmt.Errorf("幻化槽位 %d 上的 %d 不在装备目录里", slot, skin)
		}
		kind := def.Fields["[equipment type]"]
		if len(kind) == 0 || kind[0].Text != weaponKind {
			return b, WeaponReplication{}, fmt.Errorf("幻化槽位 %d 上的 %d 不是武器", slot, skin)
		}
		if actor.Job != "" {
			if err := UsableByJob(def.Fields, actor.Job, actor.Advancement); err != nil {
				return b, WeaponReplication{}, err
			}
			// [usable job] 只认职业文本，而鬼剑士五系转职的职业文本都是 [swordman]，
			// 狂战士照样过；真正区分职业的是 [required job skill]（光剑要光剑精通）。
			if skill := RequiredJobSkill(def.Fields, actor.Job); skill != 0 && actor.CanUseSkill != nil && !actor.CanUseSkill(skill) {
				return b, WeaponReplication{}, fmt.Errorf("幻化需要本职业能学会这件武器要求的技能")
			}
		}
	}
	for _, id := range b.WeaponSkinStorage() {
		if id == skin {
			return b, WeaponReplication{Skin: skin, Slot: slot, Duplicate: true}, nil
		}
	}
	if preferred != LinusSteelMold {
		preferred = LinusMold
	}
	moldIndex, mold := -1, uint32(0)
	for _, want := range [2]uint32{preferred, otherMold(preferred)} {
		for i, item := range b.Items {
			if item.Template == want {
				moldIndex, mold = i, want
				break
			}
		}
		if moldIndex >= 0 {
			break
		}
	}
	if moldIndex < 0 {
		return b, WeaponReplication{}, fmt.Errorf("幻化需要一枚莱纳斯模具或莱纳斯精钢模具")
	}
	out := b
	out.Equipment = append([]BagEquipment(nil), b.Equipment...)
	out.Equipment = append(out.Equipment[:index], out.Equipment[index+1:]...)
	out.Items = append([]BagItem(nil), b.Items...)
	left := out.Items[moldIndex].Amount - 1
	if left == 0 {
		out.Items = append(out.Items[:moldIndex], out.Items[moldIndex+1:]...)
	} else {
		out.Items[moldIndex].Amount = left
	}
	out.WeaponSkins = append(append([]uint32(nil), b.WeaponSkins...), skin)
	return out, WeaponReplication{
		Skin: skin, Slot: slot,
		Mold: mold, MoldSlot: b.Items[moldIndex].Slot, MoldAmount: left,
	}, nil
}
