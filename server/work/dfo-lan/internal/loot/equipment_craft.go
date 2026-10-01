package loot

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// 装备库「装备生成」的执行侧（CMD2259）。
//
// 判据全部来自源与实机，没有一处猜：
//  1. **必须已登记**：请求里的模板要在装备库账本 `counts` 里（> 0）。真源是
//     「生成单个部位」窗口本身就是列出已登记条目（实机截图 2026-09-29）。
//  2. **成本来自 `[create cost]` 段**：该模板所属套装档的 `[cost]` 是**可选付法**
//     （点 1 = 登记证 + 金币；点 2 = 登记证 + 材料），任一种付得起即可。
//  3. ★ **付费要分两个仓**：三档登记证 `10361512`~`10361516` **是账号共享材料
//     （"灵魂仓库"，容器 35，见 `accountMaterialSlotByTemplate`：375..379）**，
//     **不在角色背包里** —— 这正是生成窗口那个「**取用金库材料**」勾选项的含义。
//     实机 2026-09-29 的 `have 0` 之谜就是它：`admin` 把登记证放进背包，
//     下一次背包清扫（分解时 `SweepAccountMaterials`）就把它们搬进了账号仓库，
//     而旧实现只查背包 ⇒ 永远付不起。**账号共享材料走 `AccountMaterials.Spend`，
//     其余走 `Bag.PayMaterials`，金币走背包**。
//
// 三步在 `CommitAccountMaterialEvent` 的**同一个 apply 回调**里 ⇒ 角色与账号仓库同生共死，
// 任何一步失败整批回滚（沿用 Disjoint 的既有约定）。

// CraftMaterial 是回执里的一行材料消耗。
type CraftMaterial struct {
	Template uint32 `json:"template"`
	Amount   uint32 `json:"amount"`
	// FromAccount 表示这行是从账号共享仓库（灵魂仓库 / 容器 35）里扣的。
	FromAccount bool `json:"from_account,omitempty"`
}

// EquipmentCraftReceipt 是一次「装备生成」的结果快照。
type EquipmentCraftReceipt struct {
	Template  uint32          `json:"template"`
	Slot      uint16          `json:"slot,omitempty"`
	Group     int             `json:"group"`
	Cost      int             `json:"cost_option"`
	Gold      uint32          `json:"gold,omitempty"`
	Materials []CraftMaterial `json:"materials,omitempty"`
	Source    string          `json:"source"`
}

// craftEventKey 是「装备生成」的幂等键。
//
// ★ 这个键**必须在每次成功之后都不一样**：同一件装备可以被合法地反复生成，
// 而请求正文是**逐字节相同**的（没有数量 / 序号字段）。旧键只用"模板+槽+档"，
// 于是第二次生成被当成重放、静默吞掉 —— 实机 2026-09-29 14:21 两笔全被吞，
// 表现为"点了没反应"，日志却是 `DONE` 但回执全零（回执在重放路径上取不回来）。
//
// 这里用**角色状态的哈希**当"尝试序号"：成功一次状态必变（背包多一件、金币/材料少一笔）
// ⇒ 下一次的键必然不同；而重复帧到达时状态没变 ⇒ 键相同 ⇒ **依然幂等**。
//
// ⚠️ 别退回 `reinforcement` 那一族的 `nonce + 正文哈希`：那边的正文含材料槽与数量，
// 天然每次不同；这里的正文不含，加 nonce 也只在**跨会话**时有效，同一会话内
// 同物二次生成仍会逐字节相同（照样被吞）。
func craftEventKey(template, slot uint32, group int, state json.RawMessage) string {
	return fmt.Sprintf("craft:%d:%d:%d:%x", template, slot, group, sha256.Sum256(state))
}

// CreateEquipment 执行一次装备生成。
//
// `slot` 是客户端报的槽位（留证用），真正落包位置由 `BagRules.EquipmentSlots` 决定。
// `payOption` 是玩家在窗口里点的那一支付法（请求头 `[13]`，1 起）——**必须照它扣**，
// 不许回退到别支（见 pickCraftCost 的说明）。
func (s *Service) CreateEquipment(
	ctx context.Context,
	role storage.Character,
	template uint32,
	slot uint32,
	payOption int,
) (storage.Character, EquipmentCraftReceipt, bool, error) {
	var result EquipmentCraftReceipt
	fail := func(e error) (storage.Character, EquipmentCraftReceipt, bool, error) {
		return role, result, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return fail(fmt.Errorf("equipment craft: inventory source mismatch"))
	}
	if template == 0 || template == 0xFFFFFFFF {
		return fail(fmt.Errorf("equipment craft: invalid template %d", template))
	}
	if s.CreateCost == nil {
		return fail(fmt.Errorf("equipment craft: create-cost table is not loaded"))
	}
	if s.Journal == nil {
		return fail(fmt.Errorf("equipment craft: journal rules are not loaded"))
	}
	group, ok := s.CreateCost.GroupFor(template)
	if !ok {
		return fail(fmt.Errorf("equipment craft: template %d is in no create-cost group", template))
	}

	key := craftEventKey(template, slot, group.Index, role.State)
	saved, _, applied, e := s.Store.CommitAccountMaterialEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, s.Rules.Model,
		func(current storage.Character, accountRaw json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			ledger, e := inventory.ReadEquipmentJournal(current.State)
			if e != nil {
				return nil, nil, e
			}
			if ledger.Counts[template] == 0 {
				// 未登记 ⇒ 拒绝。这是**规格**不是兜底：窗口列的只有已登记条目。
				return nil, nil, fmt.Errorf("equipment craft: template %d is not registered in the journal", template)
			}
			bag, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			account, e := inventory.ReadAccountMaterials(accountRaw)
			if e != nil {
				return nil, nil, e
			}

			option, gold, bagMats, accountMats, e := pickCraftCost(bag, account, group, payOption)
			if e != nil {
				return nil, nil, e
			}
			paid, e := bag.PayMaterials(bagMats, 1)
			if e != nil {
				return nil, nil, e
			}
			if gold > paid.Gold {
				return nil, nil, fmt.Errorf("equipment craft: need %d gold, have %d", gold, paid.Gold)
			}
			paid.Gold -= gold
			out := account
			for _, m := range accountMats {
				next, _, e := out.Spend(m.Template, m.Count)
				if e != nil {
					return nil, nil, e
				}
				out = next
			}
			paid, placed, e := paid.AddEquipment(s.Equipment, s.BagRules.EquipmentSlots, template, 1)
			if e != nil {
				return nil, nil, e
			}
			updated, e := inventory.SaveBag(current.State, paid)
			if e != nil {
				return nil, nil, e
			}
			accountNext, e := out.Save()
			if e != nil {
				return nil, nil, e
			}
			result.Template = template
			result.Group = group.Index
			result.Cost = option
			result.Gold = gold
			for _, m := range bagMats {
				result.Materials = append(result.Materials, CraftMaterial{Template: m.Template, Amount: m.Count})
			}
			for _, m := range accountMats {
				result.Materials = append(result.Materials, CraftMaterial{Template: m.Template, Amount: m.Count, FromAccount: true})
			}
			result.Source = s.Catalog.Source.Checksum
			if len(placed) > 0 {
				result.Slot = placed[0]
			}
			return updated, accountNext, nil
		})
	if e != nil {
		return fail(e)
	}
	return saved, result, applied, nil
}

// pickCraftCost 取出玩家**在窗口里指定的那一支**付法，并校验付得起。
//
// `payOption` 直接来自 CMD2259 请求头 `[13]`（`EquipmentCraftRequest.PayOption`，1 起，
// 与 `[create cost]` 里 `[cost]` 行的序号一一对应）。
//
// ★ **刻意不做「付不起就换另一支」的回退**：玩家点的是哪支就扣哪支，回退=扣错东西。
// 实机 2026-09-29 13:52 那笔（`[13]=2`，玩家选的是**巡礼之印**）就是被"挑第一支付得起的"
// 错扣成了 35,000 金币。付不起就拒绝、把差多少写清楚，由上层记日志。
//
// 返回的四项：付法序号、金币、**背包付**的材料、**账号仓库付**的材料。
// 分仓依据是 `inventory.AccountMaterialSlot`：命中说明该模板被 115 客户端固定映射到
// 账号共享容器 35（三档登记证就在其中），必须从仓库扣而不是背包。
func pickCraftCost(
	bag inventory.Bag,
	account inventory.AccountMaterials,
	group catalog.CreateCostGroup,
	payOption int,
) (int, uint32, []inventory.MaterialCost, []inventory.MaterialCost, error) {
	opt, ok := group.Option(payOption)
	if !ok {
		return 0, 0, nil, nil, fmt.Errorf("equipment craft: group %d has no cost option %d (available %v)",
			group.Index, payOption, group.Numbers())
	}
	var gold uint32
	var bagMats, accountMats []inventory.MaterialCost
	for _, p := range opt.Pairs {
		if p.Gold() {
			gold += p.Amount
			continue
		}
		if _, isAccount := inventory.AccountMaterialSlot(p.Template); isAccount {
			if have := account.Count(p.Template); have < p.Amount {
				return 0, 0, nil, nil, fmt.Errorf(
					"equipment craft: cost %d needs account item %d x%d, have %d",
					opt.Number, p.Template, p.Amount, have)
			}
			accountMats = append(accountMats, inventory.MaterialCost{Template: p.Template, Count: p.Amount})
			continue
		}
		bagMats = append(bagMats, inventory.MaterialCost{Template: p.Template, Count: p.Amount})
	}
	if gold > bag.Gold {
		return 0, 0, nil, nil, fmt.Errorf("equipment craft: cost %d needs %d gold, have %d",
			opt.Number, gold, bag.Gold)
	}
	if len(bagMats) > 0 {
		if _, e := bag.PayMaterials(bagMats, 1); e != nil {
			return 0, 0, nil, nil, fmt.Errorf("equipment craft: cost %d: %w", opt.Number, e)
		}
	}
	return opt.Number, gold, bagMats, accountMats, nil
}
