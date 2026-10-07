package main

import (
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
)

// A full mode0 actor rebuild resets the client's worn/avatar containers.
// Restore authoritative instances after it, including options and sockets.
func appearanceInventory(state json.RawMessage, eq *inventory.EquipmentCatalog) ([]outboundPacket, error) {
	bag, e := inventory.ReadBag(state)
	if e != nil {
		return nil, e
	}
	// 老存档时装空孔扩展：下发视图按 PVF 默认孔就地补上（不写回存档），
	// 客户端背包/时装面板才能显示孔；镶嵌路径 UseEmblems 同样会补，两端一致。
	var defaultSockets func(uint32) []byte
	if eq != nil {
		defaultSockets = eq.DefaultAvatarSockets
	}
	var plan []outboundPacket
	for _, space := range []byte{1, 7, 3} {
		rows := bag.Special[space]
		if space == 3 {
			// Coexisting clone/look avatars share a body slot; the wire
			// payload carries one row per slot.
			rows = bag.WornBaseItems()
		}
		p, e := inventory.EquipmentPayloadWithSockets(space, rows, true, defaultSockets)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"appearance_inventory_restored", 0, 13, p})
	}
	p, e := inventory.WornSpaceUpdate(state)
	if e != nil {
		return nil, e
	}
	plan = append(plan, outboundPacket{"appearance_worn_restored", 0, 14, p})
	return plan, nil
}

func appearanceRestore(service *character.Service, role database.Character) ([]outboundPacket, error) {
	plan, e := appearanceInventory(role.State, service.Equipment)
	if e != nil {
		return nil, e
	}
	addition, e := service.EntryAddition(role)
	if e != nil {
		return nil, e
	}
	skills, e := service.EntrySkills(role)
	if e != nil {
		return nil, e
	}
	variation, e := service.VariationRestore(role)
	if e != nil {
		return nil, e
	}
	plan = append([]outboundPacket{{"appearance_attributes_restored", 0, 2, addition}}, plan...)
	plan = append(plan, outboundPacket{"appearance_skills_restored", 0, 19, skills})
	plan, e = appendSkillPresetRestore(plan, service, role, "skill_preset_restored_after_appearance_skills")
	if e != nil {
		return nil, e
	}
	return append(plan, outboundPacket{"appearance_variations_restored", 1, 29, variation}), nil
}
