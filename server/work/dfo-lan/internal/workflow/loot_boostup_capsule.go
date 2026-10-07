package workflow

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/database"
	"encoding/json"
	"fmt"
)

// UseBoostCapsule 提交胶囊升级事务（§7.2 E13）：grow/preflight 回调保留
// storage 视角，经由闭包投影到 loot.Role 后交给纯计算 PrepareBoostCapsule。
// grow 只需 ID/AccountID/Profession/State 一致，投影后行为等价。
func (s *LootService) UseBoostCapsule(ctx context.Context, role database.Character, c *boostup.Catalog, slot uint16, variant uint32,
	grow func(database.Character, byte) (database.Character, error), preflight func(database.Character, loot.BoostCapsuleReceipt) error, origin ...json.RawMessage) (database.Character, loot.BoostCapsuleReceipt, bool, error) {
	var out loot.BoostCapsuleReceipt
	if s == nil || s.Loot == nil || s.Store == nil {
		return role, out, false, fmt.Errorf("capsule store missing")
	}
	if len(origin) > 1 || len(origin) == 1 && !json.Valid(origin[0]) {
		return role, out, false, fmt.Errorf("invalid server-owned boost origin")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), "boostup-capsule", "boostup-capsule-v1",
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			state, receipt, err := s.Loot.PrepareBoostCapsule(LootRole(current), c, slot, variant, func(_ loot.Role, target byte) (loot.Role, error) {
				grown, err := grow(current, target)
				return LootRole(grown), err
			})
			if err != nil {
				return nil, nil, err
			}
			if len(origin) == 1 {
				event, err := boostup.ReadState(state)
				if err != nil {
					return nil, nil, err
				}
				event.Origin = append(json.RawMessage(nil), origin[0]...)
				state, err = boostup.WriteState(state, event)
				if err != nil {
					return nil, nil, err
				}
			}
			// 事件角色指定即带副装备礼盒（穿戴槽 22/23/25 三件）；胶囊新建角色
			// 没走过剧情 [slot expansion] 任务，开包时一并置上 USERINFO1 的三个
			// 解锁位，让装备面板挂锁与服务端穿戴校验一致。
			if state, err = inventory.UnlockEquipSlots(state, inventory.ExpandSupport|inventory.ExpandMagicStone|inventory.ExpandEarring); err != nil {
				return nil, nil, err
			}
			projected := current
			projected.State = state
			projected.WireID = role.WireID
			if preflight != nil {
				if err = preflight(projected, receipt); err != nil {
					return nil, nil, err
				}
			}
			raw, err := json.Marshal(receipt)
			return state, raw, err
		})
	if e != nil {
		return role, out, false, e
	}
	raw, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, "boostup-capsule")
	if e != nil {
		return role, out, false, e
	}
	if e = json.Unmarshal(raw, &out); e != nil {
		return role, out, false, e
	}
	if out.Slot != slot || out.Variant != variant {
		return role, out, false, fmt.Errorf("capsule request differs from committed intent")
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
