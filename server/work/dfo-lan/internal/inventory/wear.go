package inventory

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type WearRules struct {
	Source  string            `json:"source"`
	Slots   map[string]uint16 `json:"slots"`
	Special bool              `json:"special,omitempty"`
}

func LoadWearRules(path, source string) (WearRules, error) {
	var r WearRules
	b, e := os.ReadFile(path)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	// 2026-10-01（next145 续）：与 gamedata/pvf_catalogs/characters_runtime 三处哈希门禁
	// 同一口径 —— 文件的 `source` 留空 = 不钉来源，**自动派生**为当次内层实际哈希；非空仍强校验。
	// 原因：这份 wear 规则表是**按语义手写的槽位映射**（不是 PVF 导出目录），它不随客户端
	// 三件套换代而变；把它钉在某个历史内层哈希上，会让"内层重建成功"再次变成"启动失败"。
	//
	// 两件事必须一起做，否则会从"启动失败"变成"运行时失败"：
	//  1) 门禁放开（下面这行）；
	//  2) **回填** r.Source = source —— 因为 WearRules.Source 同时是运行时不变量：
	//     wear.go 的 MoveOrdinary、knight_deck.go、creation_equipment.go 都拿它跟
	//     role.ConfigVersion 比对。只留空不回填，装备穿戴会在运行时全被拒。
	//
	// 之所以必须放开：preparePVFShields 对 LoadWearRules 的调用**不受 checksBaselines 保护**
	// （pvf_catalogs.go:318），是直读启动的必由之路；前一轮（next142）只改了另外三处门禁，
	// 所以在这里撞墙（报 "wear rules source mismatch"）。
	if r.Source != "" && r.Source != source {
		return r, fmt.Errorf("wear rules source mismatch")
	}
	if r.Source == "" {
		if len(source) != 64 {
			return r, fmt.Errorf("wear rules source mismatch")
		}
		r.Source = source
	}
	if len(r.Slots) == 0 {
		return r, fmt.Errorf("wear rules source mismatch")
	}
	for _, slot := range r.Slots {
		if !EquipmentBodySlot(slot) || (!r.Special && (slot < 12 || slot > 25)) {
			return r, fmt.Errorf("unsupported ordinary equipment slot")
		}
	}
	return r, nil
}

type WearService struct {
	Store       *storage.Store
	Catalog     *EquipmentCatalog
	Professions catalog.Characters
	BagRules    BagRules
	Rules       WearRules
	Shields     *KnightShields
}

func (s *WearService) EggHatchTarget(template uint32) uint32 {
	if s != nil && s.Catalog != nil {
		if d, err := s.Catalog.Definition(template); err == nil {
			kind := d.Fields["[equipment type]"]
			subType := d.Fields["[sub type]"]
			output := d.Fields["[output index]"]
			if len(kind) > 0 && kind[0].Text == "[creature]" && len(subType) > 0 && subType[0].Value == 1 && len(output) > 0 && output[0].Value > 0 {
				return uint32(output[0].Value)
			}
		}
	}
	return EggHatchOutputs[template]
}

func (s *WearService) wearable(role storage.Character, item BagEquipment, slot uint16) error {
	d, err := s.Catalog.Definition(item.Template)
	if err != nil {
		return err
	}
	// Wearing existing instances must not inherit quest/drop eligibility.
	if e := item.ValidateRecord(); e != nil {
		return e
	}
	kind := d.Fields["[equipment type]"]
	if len(kind) == 0 || kind[0].Type != 6 {
		return fmt.Errorf("missing equipment type")
	}
	var state struct {
		Level       byte `json:"level"`
		Advancement byte `json:"advancement"`
	}
	if e := json.Unmarshal(role.State, &state); e != nil {
		return e
	}
	if (kind[0].Text == "[oath]" || kind[0].Text == "[primer]") && state.Level < 115 {
		return fmt.Errorf("oath equipment requires level 115")
	}
	if kind[0].Text == "[primer]" {
		if slot < 36 || slot > 46 {
			return fmt.Errorf("oath crystal does not fit destination slot")
		}
		rarity := d.Fields["[rarity]"]
		if len(rarity) != 1 || rarity[0].Type != 0 {
			return fmt.Errorf("oath crystal rarity unavailable")
		}
		if rarity[0].Value == 8 && slot < 44 {
			return fmt.Errorf("primeval oath crystal requires slot 44..46")
		}
	}
	job, ok := s.Professions.Professions[role.Profession]
	if !ok {
		return fmt.Errorf("equipment profession unavailable")
	}
	expected, ok := s.Rules.Slots[kind[0].Text]
	talismanSlot := s.Rules.Special && kind[0].Text == "[talisman]" && slot >= 33 && slot <= 35
	primerSlot := s.Rules.Special && kind[0].Text == "[primer]" && slot >= 36 && slot <= 46
	// 融合石使用客户端 CMD19 指定的八栏，不按 [amalgamation part] 换算普通部位。
	// 36..43 为候选范围；44..46 保留给太初晶体，47 为誓约核心。
	// 取证及当前实机验收边界见 docs/protocol/amalgamation-stone-wear-20260930.md。
	amalgamationSlot := s.Rules.Special && kind[0].Text == "[amalgamation stone]" && slot >= 36 && slot <= 43
	// 2026-09-25 实机 CMD19：光剑 28240 请求穿戴槽 24。
	// 源 dualweapon.skl 限定女鬼剑转职 4；光剑源 [sub type] 为 5。
	// 仅增加副手例外，之后仍执行武器自身的职业、转职及等级校验。
	subType := d.Fields["[sub type]"]
	offhandLightsabre := slot == 24 && kind[0].Text == "[weapon]" &&
		job.Job == "[at swordman]" && state.Advancement == 4 &&
		len(subType) == 1 && subType[0].Type == 0 && subType[0].Value == 5
	// 宠物幻化栏（槽 32）：115 客户端把**宠物本体**（[creature]，实机 63003/63008）
	// 直接拖进幻化栏，而不是 [creature skin]；配置里 [creature] 只映射到 26，于是
	// 2026-09-26 实机五次 CMD19（list7 -> list3 槽 32）全被
	// "equipment does not fit destination slot" 拒掉，客户端弹「目标栏位已满」。
	// 只在扩展券把这一栏打开之后（USERINFO1 解锁字节 bit5）放行：没开栏时
	// 客户端也不该往这里放。
	creatureSkin := s.Rules.Special && kind[0].Text == "[creature]" &&
		slot == CreatureSkinSlot && s.creatureSkinUnlocked(role)
	// 光环幻化栏同型：客户端拖进槽 11 的是 [aurora avatar]（光环本体），而规则表里
	// 该类型只映射到槽 9，于是 expected(9) != slot(11) 会把幻化整个拒掉。放行同样要求
	// 券已开栏（bit3）：客户端 UI 的挂锁读同一位。不放行其它装扮类型，避免把上衣或
	// 武器装扮塞进幻化栏。
	auraSkin := s.Rules.Special && kind[0].Text == "[aurora avatar]" &&
		slot == AuraSkinSlot && s.auraSkinUnlocked(role)
	// 先判断受限例外；表外类型也可以命中例外，但仍须通过后续源规则校验。
	if !talismanSlot && !primerSlot && !amalgamationSlot && !offhandLightsabre && !creatureSkin && !auraSkin &&
		(!ok || expected != slot) {
		return fmt.Errorf("equipment does not fit destination slot")
	}
	if kind[0].Text == "[creature]" {
		subType := d.Fields["[sub type]"]
		if len(subType) > 0 && subType[0].Value == 1 {
			return fmt.Errorf("creature must be hatched before equipping")
		}
	}
	if durability := d.Fields["[durability]"]; len(durability) > 0 {
		if len(durability) != 1 || durability[0].Type != 0 || durability[0].Value < 0 || durability[0].Value > 65535 {
			return fmt.Errorf("invalid equipment durability")
		}
	}
	level := state.Level
	if s.Store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		hasConqueror, _ := s.Store.HasActivePremium(ctx, role.AccountID, storage.PremiumConqueror, time.Now())
		cancel()
		if hasConqueror {
			if int(level)+10 <= 255 {
				level += 10
			} else {
				level = 255
			}
		}
	}
	return WearableBy(d.Fields, kind[0].Text, job.Job, state.Advancement, level)
}

// creatureSkinUnlocked 读存档里 USERINFO1 解锁字节的宠物幻化栏位（bit5）。
// 读不到存档时按未开启处理：宁可不放行，也不要把宠物塞进客户端没打开的栏。
func (s *WearService) creatureSkinUnlocked(role storage.Character) bool {
	b, e := ReadBag(role.State)
	if e != nil {
		return false
	}
	return b.ExpandEquipFlags&ExpandCreatureSkin != 0
}

// auraSkinUnlocked 读存档里 USERINFO1 解锁字节的光环幻化栏位（bit3）。与
// creatureSkinUnlocked 同一道理：客户端 UI 的挂锁读同一位，未开栏时服务端也不该
// 放行，否则界面还锁着、东西却进去了，客户端/服务端状态就不一致了。
func (s *WearService) auraSkinUnlocked(role storage.Character) bool {
	b, e := ReadBag(role.State)
	if e != nil {
		return false
	}
	return b.ExpandEquipFlags&ExpandAuraSkin != 0
}

func (s *WearService) itemGroup(item *BagEquipment, flagGroup byte) byte {
	if item == nil {
		return flagGroup
	}
	d, err := s.Catalog.Definition(item.Template)
	if err != nil {
		return flagGroup
	}
	if !d.IsAvatar() {
		return 0
	}
	if d.IsCloneAvatar() {
		return 0
	}
	return 1
}

// MoveOrdinary validates both directions before swapping one physical item.
// Equipped items retain identity and durability; no reward or copy is created.
func (s *WearService) MoveOrdinary(role storage.Character, r protocol.ItemMoveRequest) (json.RawMessage, error) {
	if IsKnightShieldMove(r) {
		return s.moveKnightShield(role, r)
	}
	if s == nil || s.Catalog == nil || s.Catalog.Source.SaveIdentity() != role.ConfigVersion || s.Professions.Source.SaveIdentity() != role.ConfigVersion {
		return nil, fmt.Errorf("wear service source mismatch")
	}
	validSpace := func(v byte) bool { return v == 0 || v == 3 || (s.Rules.Special && (v == 1 || v == 7)) }
	if !validSpace(r.SourceList) || !validSpace(r.DestinationList) || r.Count > 1 || r.Extra != 0 || r.Selection != 0xffffffff || r.Flags[0] != 0 || r.Flags[1] != 0 || r.Flags[2] > 1 {
		return nil, fmt.Errorf("unsupported ordinary equipment move")
	}
	if r.SourceList == r.DestinationList && r.SourceSlot == r.DestinationSlot {
		return nil, fmt.Errorf("identical equipment locations")
	}
	b, e := ReadBag(role.State)
	if e != nil {
		return nil, e
	}
	// Migrate legacy worn appearance avatars that were saved with Group 0
	for idx := range b.Worn {
		if b.Worn[idx].Slot <= 11 && b.Worn[idx].Group == 0 {
			if d, err := s.Catalog.Definition(b.Worn[idx].Template); err == nil && d.IsAvatar() && !d.IsCloneAvatar() {
				b.Worn[idx].Group = 1
			}
		}
	}
	find := func(list byte, slot uint16, group byte) (*BagEquipment, error) {
		rows := b.Worn
		if list == 0 {
			// 快捷栏（quick_slots [0,8]）里的装备行存在 b.Equipment 里，槽位与
			// 装备区共用同一张表。0/1 是金币/点券显示格（Rows() 恒占槽 0/1，
			// ReadBag 也拒绝这两格放物品），所以装备只能进 2..8。
			bagSlot := slot >= s.BagRules.EquipmentSlots[0] && slot <= s.BagRules.EquipmentSlots[1]
			quickSlot := s.BagRules.Quick(slot) && slot >= 2
			if !bagSlot && !quickSlot {
				return nil, fmt.Errorf("slot outside equipment bag or quick belt")
			}
			rows = b.Equipment
			for _, v := range b.Items {
				if v.Slot == slot {
					return nil, fmt.Errorf("slot contains stackable item")
				}
			}
		} else if list == 1 || list == 7 {
			rows = b.Special[list]
		} else if !EquipmentBodySlot(slot) || (!s.Rules.Special && (slot < 12 || slot > 25)) {
			return nil, fmt.Errorf("slot outside body equipment")
		}
		for _, v := range rows {
			if v.Slot == slot && (list != 3 || v.Group == group) {
				copy := v
				return &copy, nil
			}
		}
		return nil, nil
	}
	var srcGroup, dstGroup byte
	if r.SourceList == 3 {
		srcGroup = r.Flags[2]
		if r.SourceItem != 0 {
			for _, w := range b.Worn {
				if w.Slot == r.SourceSlot && w.Template == r.SourceItem {
					srcGroup = w.Group
					break
				}
			}
		}
	}
	a, e := find(r.SourceList, r.SourceSlot, srcGroup)
	if e != nil {
		return nil, e
	}
	if r.DestinationList == 3 {
		dstGroup = s.itemGroup(a, r.Flags[2])
		// Unequip direction (live 2026-09-22): the bag side is empty
		// (a == nil) and flags carry no group signal, so the worn item must
		// be located by identity - DestinationItem names it exactly.
		if a == nil && r.DestinationItem != 0 && r.DestinationSlot <= 11 {
			for _, w := range b.Worn {
				if w.Slot == r.DestinationSlot && w.Template == r.DestinationItem {
					dstGroup = w.Group
					break
				}
			}
		}
	}
	z, e := find(r.DestinationList, r.DestinationSlot, dstGroup)
	if e != nil {
		return nil, e
	}
	if a == nil && z == nil {
		return nil, fmt.Errorf("both equipment locations are empty")
	}
	// Avatar coexistence (live 2026-09-22): when the client equips one avatar
	// group over a slot whose OTHER group is occupied, DestinationItem names
	// the displayed item (the other group's piece), not a same-group replace
	// target. Treat that as a group-local insert instead of stale identity.
	staleDestination := r.DestinationItem != 0 && (z == nil || r.DestinationItem != z.Template)
	if staleDestination && r.DestinationList == 3 && r.DestinationSlot <= 11 && a != nil {
		for _, w := range b.Worn {
			if w.Slot == r.DestinationSlot && w.Group != dstGroup && w.Template == r.DestinationItem {
				staleDestination = false
				break
			}
		}
	}
	if (a == nil && r.SourceItem != 0) || (a != nil && r.SourceItem != 0 && r.SourceItem != a.Template) || staleDestination {
		return nil, fmt.Errorf("stale equipment identity")
	}
	// 快捷栏装备约束（2026-09-29）：快捷栏同一时间只能放一件装备。只有「从背包往
	// 空快捷槽新增装备」会抬升数量，拖到已被装备占用的快捷槽（换装）与快捷槽之间
	// 互拖（栏内调位）都不改变数量，照常放行；0/1 是金币/点券格不算快捷栏，堆叠物
	// 由堆叠路径负责、不在此列。仅作用于列表 0（快捷栏只存在于背包列表）。
	if z != nil && r.SourceList == 0 && r.DestinationList == 0 &&
		s.BagRules.Quick(r.SourceSlot) && r.SourceSlot >= 2 && !s.BagRules.Quick(r.DestinationSlot) {
		for _, v := range b.Equipment {
			if v.Slot != r.SourceSlot && s.BagRules.Quick(v.Slot) && v.Slot >= 2 {
				return nil, fmt.Errorf("quick belt already holds an equipment")
			}
		}
	}
	if a != nil && r.DestinationList == 3 {
		if r.DestinationSlot == 26 {
			if hatched := s.EggHatchTarget(a.Template); hatched != 0 {
				a.Template = hatched
			}
		}
		if e = s.wearable(role, *a, r.DestinationSlot); e != nil {
			return nil, e
		}
	}
	if z != nil && r.SourceList == 3 {
		if e = s.wearable(role, *z, r.SourceSlot); e != nil {
			return nil, e
		}
	}
	for _, move := range []struct {
		item *BagEquipment
		list byte
	}{{a, r.DestinationList}, {z, r.SourceList}} {
		if move.item == nil || move.list == 3 {
			continue
		}
		d, err := s.Catalog.Definition(move.item.Template)
		if err != nil {
			return nil, err
		}
		kind := d.Fields["[equipment type]"]
		if len(kind) == 0 {
			return nil, fmt.Errorf("missing equipment kind")
		}
		expected := EquipmentBagSpace(kind[0].Text)
		if move.list != expected {
			return nil, fmt.Errorf("equipment inventory family mismatch")
		}
	}
	replace := func(list byte, slot uint16, group byte, item *BagEquipment) {
		rows := &b.Worn
		if list == 0 {
			rows = &b.Equipment
		}
		var special []BagEquipment
		if list == 1 || list == 7 {
			special = b.Special[list]
			rows = &special
		}
		kept := make([]BagEquipment, 0, len(*rows)+1)
		for _, v := range *rows {
			if list == 3 {
				if !(v.Slot == slot && v.Group == group) {
					kept = append(kept, v)
				}
			} else {
				if v.Slot != slot {
					kept = append(kept, v)
				}
			}
		}
		if item != nil {
			v := *item
			v.Slot = slot
			if list == 3 {
				v.Group = group
			} else {
				v.Group = 0
			}
			if slot == 26 && list == 3 {
				var rec [protocol.CurrentItemRecordSize]byte
				if len(v.Record) == protocol.CurrentItemRecordSize {
					copy(rec[:], v.Record)
				}
				binary.LittleEndian.PutUint16(rec[0:], 26)
				binary.LittleEndian.PutUint32(rec[2:], v.Template)
				binary.LittleEndian.PutUint32(rec[6:], 1)
				binary.LittleEndian.PutUint32(rec[24:], 1)
				v.Record = rec[:]
			}
			if list == 7 {
				var rec [protocol.CurrentItemRecordSize]byte
				if len(v.Record) == protocol.CurrentItemRecordSize {
					copy(rec[:], v.Record)
				}
				binary.LittleEndian.PutUint16(rec[0:], slot)
				binary.LittleEndian.PutUint32(rec[2:], v.Template)
				var key uint32
				if slot < 140 {
					key = uint32(slot + 2)
					if len(v.Record) == protocol.CurrentItemRecordSize {
						if existingKey := binary.LittleEndian.Uint32(v.Record[6:10]); existingKey != 0 && existingKey != 1 {
							key = existingKey
						}
					}
				}
				binary.LittleEndian.PutUint32(rec[6:], key)
				binary.LittleEndian.PutUint32(rec[24:], key)
				v.Record = rec[:]
			}
			kept = append(kept, v)
		}
		*rows = kept
		if list == 1 || list == 7 {
			if b.Special == nil {
				b.Special = map[byte][]BagEquipment{}
			}
			b.Special[list] = kept
		}
	}
	replace(r.SourceList, r.SourceSlot, srcGroup, z)
	replace(r.DestinationList, r.DestinationSlot, dstGroup, a)
	if _, e = EquipmentPayload(3, b.WornBaseItems(), false); e != nil {
		return nil, e
	}
	return SaveBag(role.State, b)
}

func (s *WearService) Move(ctx context.Context, role storage.Character, key string, r protocol.ItemMoveRequest) (storage.Character, bool, error) {
	if s == nil || s.Store == nil {
		return role, false, fmt.Errorf("wear storage unavailable")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "ordinary-equipment-move-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		raw, e := s.MoveOrdinary(current, r)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(r)
		return raw, receipt, e
	})
	saved.WireID = role.WireID
	return saved, applied, e
}

// HasWornWeapon checks the native weapon slot in the persisted worn set.
func HasWornWeapon(state json.RawMessage) bool {
	bag, err := ReadBag(state)
	if err != nil {
		return false
	}
	for _, item := range bag.Worn {
		if item.Slot == 12 && item.Template != 0 {
			return true
		}
	}
	return false
}

// WornSpaceUpdate rebuilds the local actor's equipped visuals after entry.
// Restored for this handoff from the documented39 NOTI14 path; not a claim
// that the original39 source has been recovered byte for byte.
func WornSpaceUpdate(state json.RawMessage) ([]byte, error) {
	b, e := ReadBag(state)
	if e != nil {
		return nil, e
	}
	base := b.WornBaseItems()
	if len(base) == 0 {
		return nil, nil
	}
	return EquipmentPayload(3, base, false)
}

// NonAvatarWornSpaceUpdate restores the equipment rows that a mode-1 Clone
// reattach cannot represent. The native mode-1 reader clears absent slots,
// including weapons, armour and oath items. The Clone slots and creature body
// are rebuilt by the detailed packet and must not be touched here.
func NonAvatarWornSpaceUpdate(state json.RawMessage) ([]byte, error) {
	b, err := ReadBag(state)
	if err != nil {
		return nil, err
	}
	var rows []BagEquipment
	for _, item := range b.WornBaseItems() {
		if item.Slot > 11 && item.Slot != 26 {
			rows = append(rows, item)
		}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return EquipmentPayload(3, rows, false)
}

func WornPayload(state json.RawMessage) ([]byte, error) {
	b, e := ReadBag(state)
	if e != nil {
		return nil, e
	}
	return EquipmentPayload(3, b.WornBaseItems(), true)
}
