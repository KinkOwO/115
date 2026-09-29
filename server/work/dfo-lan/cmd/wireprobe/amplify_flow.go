package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

// CMD 205 = 使用增幅书（红字书）给装备打次元属性。
//
// 实机抓包（三次）显示请求是「装备槽 + 装备模板 + 书槽 + 书模板 + 属性类型」；
// 成功后就地刷新书那一行与装备那一行 —— 不重建整包，避免打断窗口引用的对象
// （强化路径已经吃过一次亏，见 reinforcement_flow.go 的注释）。
func (w *worldSession) applyAmplifyGrimoire(service *inventory.WearService, p, raw []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, fmt.Errorf("打红字需要已选角色且位于城镇")
	}
	r, err := protocol.DecodeAmplifyOption(p)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := fmt.Sprintf("amplify-grimoire:%x", sha256.Sum256(raw))
	saved, out, err := service.ApplyAmplifyGrimoire(ctx, w.role, key, r)
	if err != nil {
		return nil, err
	}
	w.role = saved
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	bookRow := bagRowOrEmpty(bag, out.BookSlot)
	rows := [][protocol.CurrentItemRecordSize]byte{bookRow}
	if out.EquipmentSpace == 0 {
		gearRow := bagRowOrEmpty(bag, out.EquipmentSlot)
		rows = append(rows, gearRow)
	}
	body, err := protocol.InventoryUpdate(rows)
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{{"amplify_grimoire_result", 1, 205, protocol.AmplifyOptionReply(out.EquipmentSpace, out.EquipmentSlot, out.BookSlot, out.BookRemaining, out.AmplifyType, out.AmplifyValue)}}
	plan = append(plan, outboundPacket{"amplify_grimoire_inventory", 0, 14, body})
	if out.EquipmentSpace == 3 {
		if wornBody, e := inventory.WornSpaceUpdate(saved.State); e == nil && len(wornBody) > 0 {
			plan = append(plan, outboundPacket{"amplify_grimoire_worn", 0, 14, wornBody})
		}
	}
	event(map[string]any{"kind": "amplify_grimoire_committed", "character_id": saved.ID,
		"book": out.BookTemplate, "book_slot": out.BookSlot, "book_remaining": out.BookRemaining,
		"equipment": out.Equipment.Template, "space": out.EquipmentSpace, "slot": out.EquipmentSlot,
		"amplify_type": out.AmplifyType, "amplify_type_name": out.AmplifyTypeName, "amplify_value": out.AmplifyValue,
		"re_amplified": out.ReAmplified, "golden": out.Golden, "pure": out.Pure,
		"prev_reinforce_level": out.PrevReinforceLevel, "amplify_level": out.AmplifyLevel})
	return w.appendFameUpdate(plan, event), nil
}

// amplifyGrimoireRefusal 是 CMD205（打红字）被拒时发给客户端的错误体。
//
// ⚠️ 不要照抄 CMD80/CMD430 的错误码表：205 的客户端 handler 是另一个函数
// （sub_145282510），它的「错误码 → dstr 文案」映射**还没有实机取证**。
// 这里只用 4（客户端文案 = dstr 1658「No items are available.」），语义最贴近
// 205 现有的拒绝原因（不是增幅书 / 书不在背包 / 目标不在装备槽 / 属性类型越界）。
// 等实机抓到 205 的报错文案再按需细分。
//
// 关键点是**必须有回包**：不回包时客户端会一直停在等待态，
// 表现和「这条命令没实现」一模一样。
func amplifyGrimoireRefusal() []byte { return protocol.Refusal(4) }
