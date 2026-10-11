package inventory

import (
	"dfolan/internal/catalog/pvf"
	"errors"
	"fmt"
)

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

// ErrWeaponSkinNotOwned refuses a selection outside this character's saved cargo.
var ErrWeaponSkinNotOwned = errors.New("角色尚未拥有该武器外观")

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
	Product     uint32
	ProductSlot uint16
	Skin        uint32 // 被复制的武器模板，也就是幻化仓库里的皮肤 id
	Slot        uint16 // 武器所在的 list 0 槽位
	Mold        uint32 // 实际扣掉的模具模板
	MoldSlot    uint16 // 模具所在槽位
	MoldAmount  uint32 // 扣完后该格剩余数量（0 表示整格消失）
	// Duplicate 为 true 表示这件武器已经登记在幻化仓库里：什么都没有扣，调用方
	// 照旧推一次页签即可（客户端自己的重复提示 100007186 走的是同一套判定）。
	Duplicate bool
}
