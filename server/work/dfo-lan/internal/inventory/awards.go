package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

// GrantExpireTime 是每一笔服务端发放（GM 工具 / admin CLI / 任务与副本奖励）
// 打进物品行的期限值。
//
// 依据：181 字节物品行的偏移 56 就是客户端判「已过期」的那一格（取证见
// internal/game/protocol/inventory.go 的 MaxItemPeriod 注释）。这一格为 0 时，
// 凡是脚本声明了 [expiration date] / [usable period] 的模板都会被客户端渲染成
// 「剩余期限已过」并拒绝使用（错误码 31730）。2026-09-27 玩家反馈「GM 工具发的
// 银增幅书显示过期无法使用」即此格为 0：银增幅书脚本声明 2022-11-08 到期。
//
// 发放不是购买，没有真实倒计时可写，所以直接写永不过期哨兵 —— 与商城
// （cashshop.MaxExpireTime）和礼盒开箱（internal/loot/box.go）一致。
// 写进存档而不是只在线上改：这样即使网关没开 DFO_MAX_ITEM_PERIOD，
// 已发放的物品也不会过期。
const GrantExpireTime = protocol.MaxItemPeriod

type AwardReceipt struct {
	Template, Amount uint32
	Slots            []uint16
}
type Awarder struct {
	Catalog   catalog.LootCatalog
	Rules     BagRules
	Equipment *EquipmentCatalog
}

func (a *Awarder) Grant(raw json.RawMessage, id, amount uint32) (json.RawMessage, AwardReceipt, error) {
	r := AwardReceipt{Template: id, Amount: amount}
	if a == nil || a.Catalog.Source.Checksum != a.Rules.Source {
		return nil, r, fmt.Errorf("inventory award source missing")
	}
	b, e := ReadBag(raw)
	if e != nil {
		return nil, r, e
	}
	if id == 0 || a.Catalog.Items[id].Kind == "stackable" {
		var slot uint16
		b, slot, e = b.Add(a.Catalog, a.Rules, id, amount, GrantExpireTime)
		r.Slots = []uint16{slot}
	} else {
		if a.Equipment == nil || a.Equipment.Source.Checksum != a.Rules.Source {
			return nil, r, fmt.Errorf("equipment award source missing")
		}
		kind, kindErr := a.Equipment.EquipmentKind(id)
		if kindErr != nil {
			return nil, r, kindErr
		}
		if IsPetGear(kind) {
			if amount == 0 || amount > uint32(PetGearLast-PetGearFirst+1) {
				return nil, r, fmt.Errorf("invalid pet equipment award amount")
			}
			if _, e = a.Equipment.Reward(id); e != nil {
				return nil, r, e
			}
			for n := uint32(0); n < amount; n++ {
				var slot uint16
				b, slot, e = b.AddPetGear(BagEquipment{Template: id})
				if e != nil {
					return nil, r, e
				}
				r.Slots = append(r.Slots, slot)
			}
		} else {
			b, r.Slots, e = b.AddEquipment(a.Equipment, a.Rules.EquipmentSlots, id, amount)
			// 宠物（[creature]）的期限是脚本 [usable period] 的真值，由
			// creatureRowPeriod 负责填；其余装备自己没有期限来源，一律标永不过期，
			// 免得声明过期限的装扮/装备一发下来就显示过期。
			if e == nil && kind != "[creature]" {
				b = b.stampEquipmentPeriod(r.Slots, GrantExpireTime)
			}
		}
	}
	if e != nil {
		return nil, r, e
	}
	out, e := SaveBag(raw, b)
	return out, r, e
}

// stampEquipmentPeriod 只给本次发放落到的槽位打期限，不动背包里其它装备行。
func (b Bag) stampEquipmentPeriod(slots []uint16, period uint32) Bag {
	if len(slots) == 0 || period == 0 {
		return b
	}
	want := make(map[uint16]bool, len(slots))
	for _, s := range slots {
		want[s] = true
	}
	b.Equipment = append([]BagEquipment(nil), b.Equipment...)
	for i := range b.Equipment {
		if want[b.Equipment[i].Slot] {
			b.Equipment[i].Period = period
		}
	}
	return b
}
