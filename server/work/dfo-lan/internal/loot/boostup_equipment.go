package loot

import (
	"dfolan/internal/boostup"
	"dfolan/internal/inventory"
	"fmt"
)

// BoostWornFacts 读取角色**身上穿戴**装备的事实（部位/模板/[equipment type]）。
// 穿戴证明的事务编排（含套装积分与权益视图判据）在
// workflow.LootService.ReconcileBoostEquipment（§7.2 E13）。
//
// 这里只采集事实，不再重复判定「该部位能不能进这个槽」：那条规则的唯一真源是
// inventory.WearService 的穿戴路径（CMD19，含誓约/护石/太初/副手/幻化等例外），
// 能出现在 bag.Worn 里的装备已经在那里被判过一次。活动侧再抄一份部位-槽位表
// 就是第二个判据源（§0.2），而且会随穿戴例外一起失真。
// 只保留一条不依赖槽位表的身份自检：装扮与非装扮分居 0..11 与 12+ 两段。
func (s *Service) BoostWornFacts(role Role) ([]boostup.WornFact, error) {
	if s.Equipment == nil {
		return nil, fmt.Errorf("boost equipment source missing")
	}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return nil, err
	}
	var facts []boostup.WornFact
	for _, item := range bag.Worn {
		if err = item.ValidateRecord(); err != nil {
			return nil, err
		}
		def, err := s.Equipment.Definition(item.Template)
		if err != nil {
			return nil, err
		}
		kind := def.Fields["[equipment type]"]
		if len(kind) == 0 || kind[0].Type != 6 {
			return nil, fmt.Errorf("boost worn source type missing")
		}
		if avatar := def.IsAvatar(); avatar != (item.Slot <= 11) {
			return nil, fmt.Errorf("boost worn identity mismatch")
		}
		facts = append(facts, boostup.WornFact{Slot: item.Slot, Template: item.Template, Kind: kind[0].Text})
	}
	return facts, nil
}

// BoostWornEnchantCards 给出每个穿戴槽位上的附魔卡（0 = 未附魔）。
// 附魔卡由 inventory 唯一读写（offset 14），活动侧只取事实。
func (s *Service) BoostWornEnchantCards(role Role) (map[uint16]uint32, error) {
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return nil, err
	}
	cards := map[uint16]uint32{}
	for _, item := range bag.Worn {
		cards[item.Slot] = inventory.EnchantCardOf(item)
	}
	return cards, nil
}
