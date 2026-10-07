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
	if a == nil {
		return nil, r, fmt.Errorf("inventory award source missing")
	}
	if id != 0 && a.Catalog.Items[id].Kind == "stackable" && a.Catalog.HasRuntimeDetails() {
		if _, err := a.Catalog.ItemScript(id); err != nil {
			return nil, r, err
		}
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
		// [MOD-CAPABILITY-20261006] 装备分支的每一处失败都**带上模板号**：规则脚本的池子
		// 是一串手抄的模板号，旧口径只回一句 "equipment definition missing"，
		// 现场根本判不出是哪一个号错了（2026-10-06 实测踩到）。
		if a.Equipment == nil {
			return nil, r, fmt.Errorf("模板 %d：装备目录未装配（equipment award source missing）", id)
		}
		kind, kindErr := a.Equipment.EquipmentKind(id)
		if kindErr != nil {
			return nil, r, fmt.Errorf("模板 %d 既不是可发放的堆叠物，也取不到装备定义（equipment definition missing）：%w", id, kindErr)
		}
		if IsPetGear(kind) {
			if amount == 0 || amount > uint32(PetGearLast-PetGearFirst+1) {
				return nil, r, fmt.Errorf("模板 %d：宠物装备发放数量非法（%d）", id, amount)
			}
			if _, e = a.Equipment.Reward(id); e != nil {
				return nil, r, fmt.Errorf("模板 %d 取不到奖励耐久：%w", id, e)
			}
			for n := uint32(0); n < amount; n++ {
				var slot uint16
				b, slot, e = b.AddPetGear(BagEquipment{Template: id})
				if e != nil {
					return nil, r, fmt.Errorf("模板 %d 宠物装备入包失败：%w", id, e)
				}
				r.Slots = append(r.Slots, slot)
			}
		} else {
			var slotErr error
			b, r.Slots, slotErr = b.AddEquipment(a.Equipment, a.Rules.EquipmentSlots, id, amount)
			if slotErr != nil {
				return nil, r, fmt.Errorf("模板 %d 装备入包失败：%w", id, slotErr)
			}
			// 宠物（[creature]）的期限是脚本 [usable period] 的真值，由
			// creatureRowPeriod 负责填；其余装备自己没有期限来源，一律标永不过期，
			// 免得声明过期限的装扮/装备一发下来就显示过期。
			if kind != "[creature]" {
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

// GrantPet 发一只**宠物本体**（放到宠物容器 list 7 的 0..139）。
//
// 为什么单开一条而不走 Grant：Grant 的装备分支按规则表把非宠物装备的都塞进**普通装备栏**
// （Rules.EquipmentSlots），`[creature]` 本体落进去就变成"宠物出现在装备栏里、F6 列表里没有"。
// 2026-10-06 之前 mod 脚本只能这么发，所以这条能力是补上真正的落位。
//
// 门禁（都在这里挡，报错带模板号）：
//   - 模板必须取得到定义且 `[equipment type]` == "[creature]"；
//   - **宠物蛋不能当本体发**（蛋要先孵化，模板号在 EggHatchOutputs 里）；
//   - 数量只能是 1（本体是"一只一行"，不堆叠）。
func (a *Awarder) GrantPet(raw json.RawMessage, template, count uint32) (json.RawMessage, AwardReceipt, error) {
	r := AwardReceipt{Template: template, Amount: count}
	if a == nil {
		return nil, r, fmt.Errorf("inventory award source missing")
	}
	if a.Equipment == nil {
		return nil, r, fmt.Errorf("模板 %d：装备目录未装配（equipment award source missing）", template)
	}
	kind, err := a.Equipment.EquipmentKind(template)
	if err != nil {
		return nil, r, fmt.Errorf("模板 %d 取不到装备定义（equipment definition missing）：%w", template, err)
	}
	if !IsCreature(kind) {
		return nil, r, fmt.Errorf("模板 %d 不是宠物本体（[equipment type] = %s）", template, kind)
	}
	if IsCreatureEgg(template) {
		return nil, r, fmt.Errorf("模板 %d 是宠物蛋，先孵化再发（蛋不能当本体入栏）", template)
	}
	if count != 1 {
		return nil, r, fmt.Errorf("模板 %d：宠物本体一次只能发 1 只（现在是 %d）", template, count)
	}
	b, err := ReadBag(raw)
	if err != nil {
		return nil, r, err
	}
	next, slot, err := b.AddPetCreature(template)
	if err != nil {
		return nil, r, err
	}
	out, err := SaveBag(raw, next)
	if err != nil {
		return nil, r, err
	}
	r.Slots = []uint16{slot}
	return out, r, nil
}

// GrantPetItem 发一件**宠物用品**（饲料 / 改名卡这类，落到宠物容器 list 7 的 376..431）。
//
// 复用 Bag.Add 的既有分流（bag.go 里 IsPetConsumable 的堆叠物会自动进 addPetStack），
// 所以堆叠合并、堆叠上限、56 格上限都只有一份实现；这里只负责**把模板性质校验清楚**并
// 断言落位落在用品区间（否则说明目录口径变了，宁可报错也不要悄悄写进普通背包）。
func (a *Awarder) GrantPetItem(raw json.RawMessage, template, count uint32) (json.RawMessage, AwardReceipt, error) {
	r := AwardReceipt{Template: template, Amount: count}
	if a == nil {
		return nil, r, fmt.Errorf("inventory award source missing")
	}
	if count == 0 {
		return nil, r, fmt.Errorf("模板 %d：宠物用品数量必须大于 0", template)
	}
	item, known := a.Catalog.Items[template]
	if !known || item.Kind != "stackable" {
		return nil, r, fmt.Errorf("模板 %d 不是堆叠物，不能当宠物用品发", template)
	}
	if !IsPetConsumable(item.StackableType) {
		return nil, r, fmt.Errorf("模板 %d 不是宠物用品（stackable type = %s）", template, item.StackableType)
	}
	b, err := ReadBag(raw)
	if err != nil {
		return nil, r, err
	}
	next, slot, err := b.Add(a.Catalog, a.Rules, template, count, GrantExpireTime)
	if err != nil {
		return nil, r, err
	}
	if slot < PetConsumableFirst || slot > PetConsumableLast {
		return nil, r, fmt.Errorf("模板 %d 落到了 %d 号格，不在宠物用品区间 %d..%d",
			template, slot, PetConsumableFirst, PetConsumableLast)
	}
	out, err := SaveBag(raw, next)
	if err != nil {
		return nil, r, err
	}
	r.Slots = []uint16{slot}
	return out, r, nil
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
