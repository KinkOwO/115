package inventory

import (
	"dfolan/internal/catalog/pvf"
	"errors"
	"fmt"
)

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

// ReplicationActor 是幻化要核对的角色身份。Job 是职业文本（[swordman] 这类，来自
// characters 目录），空串表示目录不可用、跳过职业校验；Advancement 是转职索引。
type ReplicationActor struct {
	Job         string
	Advancement byte
	// CanUseSkill 判断本角色（职业 + 转职）用得了某项职业技能：能学会或者已经学会。
	// 它核的是 [required job skill]，见 RequiredJobSkill。nil 表示技能目录不可用，
	// 跳过这一道（与 Job == "" 同理）。
	CanUseSkill func(skill uint16) bool
}

// RequiredJobSkill 给出这件装备在 [required job skill] 里对本职业声明的技能 id。
//
// 这个字段是成对数据：先一个职业文本（type 6，如 [swordman]），紧跟该职业要求
// 的技能 id（type 0）。实机核对 2026-09-27（equipment-full 全量目录）：
//
//	光剑 401040091/401040095/28284 = [swordman]:33 [demonic swordman]:33 [at swordman]:95
//	太刀 101010912/101010438/27739 = 整段没有这个字段
//
// 33 320 件武器里只有 838 件带它，全部是鬼剑士的光剑家族（子类型 19/21/23/24/25）。
//
// 返回 0 表示这件装备没有对本职业提出技能要求（不设门槛）。**只有 [usable job] 挡不住
// 跨职业**：鬼剑士五系转职的职业文本都是 [swordman]，[usable job] 一律放行，所以狂战士
// 也能把光剑复制进幻化仓库（实机 2026-09-27）；光剑精通（Job 0 的 33）在转职上限表里
// [growtype maximum level] = [0,10,0,0,0,0]，只有剑魂那一个槽能学，这道才区分得开。
func RequiredJobSkill(fields map[string][]pvf.Token, job string) uint16 {
	subject := ""
	for _, t := range fields["[required job skill]"] {
		switch t.Type {
		case 6:
			subject = t.Text
		case 0:
			if subject == job && t.Value > 0 && t.Value <= 65535 {
				return uint16(t.Value)
			}
			subject = ""
		}
	}
	return 0
}

// ErrWeaponSkinNotUsable 表示这件外观本角色戴不上（[usable job] 或 [required job skill]
// 任一不过）。调用方用 errors.Is 认出它，好把「本职业用不了」与真正的故障分开处理。
var ErrWeaponSkinNotUsable = errors.New("幻化需要本职业能学会这件武器要求的技能")

// WeaponSkinUsable 判断一件**已经登记在幻化仓库里**的外观，本角色能不能戴上。
//
// 判定与复制路径同源（UsableByJob + RequiredJobSkill），只是时机换了：复制那道门管的是
// 「要不要收进仓库」，这道门管的是「能不能戴上」。之所以必须补这一道，是因为仓库里的
// 条目可能来自修好复制校验之前——实机 2026-09-27 狂战士仓库里就躺着光剑。删玩家存档不
// 可取也不必，让它在佩戴这一步不生效才是真正的拦截点（复制路径见 ReplicateWeaponSkin）。
//
// 放行的情况与 canLearnSkill 同一条原则——缺目录不能让本来正常的佩戴全部失败：skin 为 0
// （解除）、目录不可用、目录里查不到这件皮肤、actor.Job 为空、CanUseSkill 为 nil，都返回
// nil。解除必须永远放行，否则玩家会卡在一件戴不上的外观里出不来。
func (c *EquipmentCatalog) WeaponSkinUsable(skin uint32, actor ReplicationActor) error {
	if c == nil || skin == 0 || actor.Job == "" {
		return nil
	}
	def, err := c.definitionResolved(skin, 0)
	if err != nil {
		return nil
	}
	if err := UsableByJob(def.Fields, actor.Job, actor.Advancement); err != nil {
		return fmt.Errorf("%w: %v", ErrWeaponSkinNotUsable, err)
	}
	if skill := RequiredJobSkill(def.Fields, actor.Job); skill != 0 && actor.CanUseSkill != nil && !actor.CanUseSkill(skill) {
		return ErrWeaponSkinNotUsable
	}
	return nil
}

// 复制只收武器。客户端自己也会拦（100007142 "Only Weapon items can be
// Replicated."、100007146 "Equipped items can't be Replicated."），服务端再核一遍
// 是因为请求里只有槽位号，没有"这是武器"的标记——槽位被拖了东西就可能吃掉别的装备。
const weaponKind = "[weapon]"

// WeaponReplication 记录一次复制实际消耗了什么，供事件日志核对。
type WeaponReplication struct {
	Skin       uint32 // 被复制的武器模板，也就是幻化仓库里的皮肤 id
	Slot       uint16 // 武器所在的 list 0 槽位
	Mold       uint32 // 实际扣掉的模具模板
	MoldSlot   uint16 // 模具所在槽位
	MoldAmount uint32 // 扣完后该格剩余数量（0 表示整格消失）
	// Duplicate 为 true 表示这件武器已经登记在幻化仓库里：什么都没有扣，调用方
	// 照旧推一次页签即可（客户端自己的重复提示 100007186 走的是同一套判定）。
	Duplicate bool
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
