package inventory

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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
type EquipmentCraftPlan struct {
	Key   string
	Group catalog.CreateCostGroup
}

func (s *ItemService) PlanEquipmentCraft(role Role, template, slot uint32, payOption int) (EquipmentCraftPlan, error) {
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return EquipmentCraftPlan{}, (fmt.Errorf("equipment craft: inventory source mismatch"))
	}
	if template == 0 || template == 0xFFFFFFFF {
		return EquipmentCraftPlan{}, (fmt.Errorf("equipment craft: invalid template %d", template))
	}
	if s.CreateCost == nil {
		return EquipmentCraftPlan{}, (fmt.Errorf("equipment craft: create-cost table is not loaded"))
	}
	if s.Journal == nil {
		return EquipmentCraftPlan{}, (fmt.Errorf("equipment craft: journal rules are not loaded"))
	}
	group, ok := s.CreateCost.GroupFor(template)
	if !ok {
		return EquipmentCraftPlan{}, (fmt.Errorf("equipment craft: template %d is in no create-cost group", template))
	}

	key := craftEventKey(template, slot, group.Index, role.State)
	return EquipmentCraftPlan{Key: key, Group: group}, nil
}
func (s *ItemService) PrepareEquipmentCraft(current Role, accountRaw json.RawMessage, template uint32, payOption int, plan EquipmentCraftPlan) (json.RawMessage, json.RawMessage, EquipmentCraftReceipt, error) {
	var result EquipmentCraftReceipt
	group := plan.Group
	ledger, e := ReadEquipmentJournal(current.State)
	if e != nil {
		return nil, nil, result, e
	}
	if ledger.Counts[template] == 0 {
		// 未登记 ⇒ 拒绝。这是**规格**不是兜底：窗口列的只有已登记条目。
		return nil, nil, result, fmt.Errorf("equipment craft: template %d is not registered in the journal", template)
	}
	bag, e := ReadBag(current.State)
	if e != nil {
		return nil, nil, result, e
	}
	account, e := ReadAccountMaterials(accountRaw)
	if e != nil {
		return nil, nil, result, e
	}

	option, gold, bagMats, accountMats, e := pickCraftCost(bag, account, group, payOption)
	if e != nil {
		return nil, nil, result, e
	}
	paid, e := bag.PayMaterials(bagMats, 1)
	if e != nil {
		return nil, nil, result, e
	}
	if gold > paid.Gold {
		return nil, nil, result, fmt.Errorf("equipment craft: need %d gold, have %d", gold, paid.Gold)
	}
	paid.Gold -= gold
	out := account
	for _, m := range accountMats {
		next, _, e := out.Spend(m.Template, m.Count)
		if e != nil {
			return nil, nil, result, e
		}
		out = next
	}
	paid, placed, e := paid.AddEquipment(s.Equipment, s.BagRules.EquipmentSlots, template, 1)
	if e != nil {
		return nil, nil, result, e
	}
	updated, e := SaveBag(current.State, paid)
	if e != nil {
		return nil, nil, result, e
	}
	accountNext, e := out.Save()
	if e != nil {
		return nil, nil, result, e
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
	result.Source = s.Catalog.Source.SaveIdentity()
	if len(placed) > 0 {
		result.Slot = placed[0]
	}
	return updated, accountNext, result, nil
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
// 分仓依据是 `AccountMaterialSlot`：命中说明该模板被 115 客户端固定映射到
// 账号共享容器 35（三档登记证就在其中），必须从仓库扣而不是背包。
func pickCraftCost(
	bag Bag,
	account AccountMaterials,
	group catalog.CreateCostGroup,
	payOption int,
) (int, uint32, []MaterialCost, []MaterialCost, error) {
	opt, ok := group.Option(payOption)
	if !ok {
		return 0, 0, nil, nil, fmt.Errorf("equipment craft: group %d has no cost option %d (available %v)",
			group.Index, payOption, group.Numbers())
	}
	var gold uint32
	var bagMats, accountMats []MaterialCost
	for _, p := range opt.Pairs {
		if p.Gold() {
			gold += p.Amount
			continue
		}
		if _, isAccount := AccountMaterialSlot(p.Template); isAccount {
			if have := account.Count(p.Template); have < p.Amount {
				return 0, 0, nil, nil, fmt.Errorf(
					"equipment craft: cost %d needs account item %d x%d, have %d",
					opt.Number, p.Template, p.Amount, have)
			}
			accountMats = append(accountMats, MaterialCost{Template: p.Template, Count: p.Amount})
			continue
		}
		bagMats = append(bagMats, MaterialCost{Template: p.Template, Count: p.Amount})
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

// EquipmentTransformPair 是一件成功的「装备变换」：部位 `slot` 上的 `from` 换成了 `to`。
//
// `FromBag` / `BagSlot` 记录**源在背包**的情形（客户端允许把背包里的装备放进界面「变换前」槽）：
// 此时目标会被**穿上**到 `slot`，而该部位原来的那件退回 `FromBagSlot`。
type EquipmentTransformPair struct {
	Slot    uint16 `json:"slot"`
	From    uint32 `json:"from"`
	To      uint32 `json:"to"`
	Group   int    `json:"group"`
	FromBag bool   `json:"from_bag,omitempty"`
	BagSlot uint16 `json:"bag_slot,omitempty"`
}

// EquipmentTransformReceipt 是一次「装备变换」的结果快照（写进 events.jsonl）。
type EquipmentTransformReceipt struct {
	Source    string                   `json:"source"`
	Pairs     []EquipmentTransformPair `json:"pairs,omitempty"`
	Materials []CraftMaterial          `json:"materials,omitempty"`
	Gold      uint32                   `json:"gold,omitempty"`
	Option    int                      `json:"option,omitempty"`
	// Skipped 记录请求里没能变换的模板（未登记 / 找不到档位 / 身上没有该件 / 付不起）。
	Skipped []uint32 `json:"skipped,omitempty"`
}

// TransformEquipment 实现「装备变换」（CMD2259, action=1）。
//
// 语义（2026-09-30 实机取证确定，见 docs/更新文档.md 的取证节）：
// 客户端把「玩家在图鉴里选中的**已收录**目标装备」+「它们各自的部位槽位」一起发上来，
// 服务端要**把身上穿的这些部位换成目标**：
//
//  1. 目标必须**已在装备库登记**（`Journal.Counts[target] != 0`）。这是**规格**不是兜底：
//     图鉴窗口列的只有已登记条目（与 CreateEquipment 同一条规矩）。
//  2. 成本按目标的**档位**（`[grade]` + `[rarity]`）从 `[create cost]` 里取；
//     付法序号就是请求头 `[13]`（`PayOption`），**照它扣，不回退**（见 pickCraftCost）。
//  3. 逐件扣料，然后把 `worn[slot]` 的模板换成目标 —— 见 applyTransform 说明保留/清空哪些字段。
//
// 不满足条件的**单件**只跳过、不整体拒绝：客户端一次会把整屏（实测 11 件）都报上来，
// 其中本来就只有一部分是"能换的"。一件都换不了时返回 error，**上层只记日志、绝不回包**
// —— 客户端在 2259 上没有失败分支（两次实测收到 Error 都 `exit=0xC0000005`）。

type transformStep struct {
	slot    uint16
	from    uint32
	to      uint32
	bagSlot uint16 // 0 = 源在身上；非 0 = 源在背包装备区的这个槽
}

type EquipmentTransformPlan struct {
	Key     string
	Steps   []transformStep
	Receipt EquipmentTransformReceipt
}

func (s *ItemService) PlanEquipmentTransform(role Role, slots, templates []uint32, payOption int) (EquipmentTransformPlan, error) {
	var result EquipmentTransformReceipt
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return EquipmentTransformPlan{Receipt: result}, (fmt.Errorf("equipment transform: inventory source mismatch"))
	}
	if s.CreateCost == nil {
		return EquipmentTransformPlan{Receipt: result}, (fmt.Errorf("equipment transform: create-cost table is not loaded"))
	}
	if s.Journal == nil {
		return EquipmentTransformPlan{Receipt: result}, (fmt.Errorf("equipment transform: journal rules are not loaded"))
	}
	if s.Equipment == nil {
		return EquipmentTransformPlan{Receipt: result}, (fmt.Errorf("equipment transform: equipment catalog is not loaded"))
	}
	if len(slots) == 0 || len(templates) == 0 {
		return EquipmentTransformPlan{Receipt: result}, (fmt.Errorf("equipment transform: empty request"))
	}

	bag, e := ReadBag(role.State)
	if e != nil {
		return EquipmentTransformPlan{Receipt: result}, (e)
	}
	ledger, e := ReadEquipmentJournal(role.State)
	if e != nil {
		return EquipmentTransformPlan{Receipt: result}, (e)
	}
	var plans []transformStep
	n := len(slots)
	if len(templates) < n {
		n = len(templates)
	}
	var notes []string
	for i := 0; i < n; i++ {
		target := templates[i]
		slot := uint16(slots[i])
		if target == 0 || target == 0xFFFFFFFF {
			continue
		}
		from, bagSlot, held := s.transformSource(bag, slot)
		if !held || from == 0 {
			notes = append(notes, fmt.Sprintf("slot %d: 身上和背包里都没有该部位的装备(部位类型=%q)", slot, s.kindForWornSlot(slot)))
			result.Skipped = append(result.Skipped, target)
			continue
		}
		if from == target {
			notes = append(notes, fmt.Sprintf("slot %d: 已经是目标 %d，无可变换", slot, target))
			continue
		}
		if ledger.Counts[target] == 0 {
			notes = append(notes, fmt.Sprintf("slot %d: 目标 %d 未在装备库登记(counts=%d)", slot, target, ledger.Counts[target]))
			result.Skipped = append(result.Skipped, target)
			continue
		}
		if _, _, e := s.transformCost(target); e != nil {
			notes = append(notes, fmt.Sprintf("slot %d: 目标 %d 算不出变换成本（%v）", slot, target, e))
			result.Skipped = append(result.Skipped, target)
			continue
		}
		plans = append(plans, transformStep{slot: slot, from: from, to: target, bagSlot: bagSlot})
	}
	if len(plans) == 0 {
		// 把逐件判定写进错误串：2026-09-30 03:06 那次日志只说了 "nothing transformable"，
		// 而四个条件看起来都满足 —— 继续靠猜会浪费一轮。现在一次就能看清卡在哪条。
		return EquipmentTransformPlan{Receipt: result}, (fmt.Errorf("equipment transform: nothing transformable in %d requested slots (%s)",
			len(templates), strings.Join(notes, "; ")))
	}

	key := transformKey(slots, templates, role.State)
	return EquipmentTransformPlan{Key: key, Steps: plans, Receipt: result}, nil
}
func (s *ItemService) PrepareEquipmentTransform(current Role, accountRaw json.RawMessage, plan EquipmentTransformPlan) (json.RawMessage, json.RawMessage, EquipmentTransformReceipt, error) {
	result, plans := plan.Receipt, plan.Steps
	live, e := ReadBag(current.State)
	if e != nil {
		return nil, nil, result, e
	}
	liveLedger, e := ReadEquipmentJournal(current.State)
	if e != nil {
		return nil, nil, result, e
	}
	account, e := ReadAccountMaterials(accountRaw)
	if e != nil {
		return nil, nil, result, e
	}

	var bagMats, accountMats []MaterialCost
	var gold uint32
	option := 0
	var done []EquipmentTransformPair
	next := live
	for _, p := range plans {
		// 事务内重校验：状态可能已被别的请求改过。
		if liveLedger.Counts[p.to] == 0 {
			result.Skipped = append(result.Skipped, p.to)
			continue
		}
		if from, bs, held := s.transformSource(next, p.slot); !held || from != p.from || bs != p.bagSlot {
			result.Skipped = append(result.Skipped, p.to)
			continue
		}
		// 成本：**目标稀有度对应的灵魂 ×1 + 固定金币**（客户端「变换确认」界面的口径，
		// 不走 [create cost] —— 那张表没有太初档，武器永远匹配不到）。
		soul, g, e := s.transformCost(p.to)
		if e != nil {
			result.Skipped = append(result.Skipped, p.to)
			continue
		}
		mat := MaterialCost{Template: soul, Count: 1}
		if _, isAccount := AccountMaterialSlot(soul); isAccount {
			if have := account.Count(soul); have < mat.Count {
				result.Skipped = append(result.Skipped, p.to)
				continue
			}
			accountMats = append(accountMats, mat)
		} else {
			bagMats = append(bagMats, mat)
		}
		option = 1
		gold += g
		done = append(done, EquipmentTransformPair{
			Slot: p.slot, From: p.from, To: p.to, Group: 0,
			FromBag: p.bagSlot != 0, BagSlot: p.bagSlot,
		})
	}
	if len(done) == 0 {
		return nil, nil, result, fmt.Errorf("equipment transform: none of the %d pairs is affordable", len(plans))
	}
	if gold > next.Gold {
		return nil, nil, result, fmt.Errorf("equipment transform: need %d gold, have %d", gold, next.Gold)
	}
	paid, e := next.PayMaterials(bagMats, 1)
	if e != nil {
		return nil, nil, result, e
	}
	paid.Gold -= gold
	out := account
	for _, m := range accountMats {
		nxt, _, e := out.Spend(m.Template, m.Count)
		if e != nil {
			return nil, nil, result, e
		}
		out = nxt
	}
	replaced, e := s.applyTransform(paid, done)
	if e != nil {
		return nil, nil, result, e
	}
	updated, e := SaveBag(current.State, replaced)
	if e != nil {
		return nil, nil, result, e
	}
	// 方案「甲」：把换下去的源装备也登记进图鉴，否则它"换出去即消失"、再也选不回来。
	ledgerNext := registerTransformedSources(s.Equipment, s.Journal, liveLedger, done)
	if updated, e = SaveEquipmentJournal(updated, ledgerNext); e != nil {
		return nil, nil, result, e
	}
	accountNext, e := out.Save()
	if e != nil {
		return nil, nil, result, e
	}
	result.Source = s.Catalog.Source.SaveIdentity()
	result.Option = option
	result.Gold = gold
	result.Pairs = done
	for _, m := range bagMats {
		result.Materials = append(result.Materials, CraftMaterial{Template: m.Template, Amount: m.Count})
	}
	for _, m := range accountMats {
		result.Materials = append(result.Materials, CraftMaterial{Template: m.Template, Amount: m.Count, FromAccount: true})
	}
	return updated, accountNext, result, nil
}

// applyTransform 把 worn 里这些槽的模板换成目标。
//
// ★ **只改 `Template` 与 `Durability`，其余字段（`Record` / `AvatarOptions` / `AvatarSockets` /
// `Refine` / `Period` …）原样保留** —— 这正是「装备变换」的核心语义：**打造效果跟着走**。
//
// 打造效果就在那条 181 字节的 `Record` 里（见 `internal/inventory/amplify.go` 的偏移表）：
//
//	offset 10 : 等级字节 bit0-4（强化与增幅**共用这一格**：`offset 19` 非 0 时客户端渲染成
//	            「增幅 +N」，否则渲染成「强化 +N」）、bit5-7 = 再封装次数
//	offset 19 : 次元属性类型（红字）        offset 20 : 次元属性数值
//	附魔：`enchantCard(row)` 同样从 row 读；锻造另有服务端字段 `Refine`（见 BagEquipment 注释）。
//
// ⚠️ **2026-09-30 修正**：早先版本把 `Record` / `AvatarOptions` / `AvatarSockets` **清空**了，
// 那等于把玩家的**强化 +N、增幅、附魔全部抹掉** —— `Record` 是这些状态的**唯一载体**
// （`BagEquipment` 里除 `Refine` 外没有对应的服务端字段，`amplifyLevel(row)` / `enchantCard(row)`
// 都是直接从行里读）。行内的模板号由 `EquipmentRowInSpace` 每次序列化时重写
// （`r[2:6] = Template`），所以换模板**根本不需要**动 `Record`。
func (s *ItemService) applyTransform(bag Bag, pairs []EquipmentTransformPair) (Bag, error) {
	out := bag
	out.Worn = append([]BagEquipment(nil), bag.Worn...)
	out.Equipment = append([]BagEquipment(nil), bag.Equipment...)
	idx := map[uint16]int{}
	for i, w := range out.Worn {
		idx[w.Slot] = i
	}
	for _, p := range pairs {
		d, e := s.Equipment.Reward(p.To)
		if e != nil {
			return bag, e
		}
		if p.FromBag {
			// 源在背包装备区：把目标**穿上**到 `p.Slot`；该部位原来那件（若有）退回源所在的背包格。
			// 这等价于"带着打造把这件换上去"，对应客户端「变换前」槽里放的是背包装备的情形。
			srcIdx := -1
			for i := range out.Equipment {
				if out.Equipment[i].Slot == p.BagSlot {
					srcIdx = i
					break
				}
			}
			if srcIdx < 0 {
				return bag, fmt.Errorf("equipment transform: bag slot %d disappeared", p.BagSlot)
			}
			arm := out.Equipment[srcIdx]
			arm.Template, arm.Durability, arm.Slot = p.To, d, p.Slot
			if len(arm.Record) == protocol.CurrentItemRecordSize {
				rec := append([]byte(nil), arm.Record...)
				binary.LittleEndian.PutUint32(rec[2:], p.To)
				arm.Record = rec
			}
			if wi, ok := idx[p.Slot]; ok {
				old := out.Worn[wi]
				old.Slot = p.BagSlot
				out.Equipment[srcIdx] = old
				out.Worn[wi] = arm
			} else {
				out.Equipment = append(out.Equipment[:srcIdx], out.Equipment[srcIdx+1:]...)
				out.Worn = append(out.Worn, arm)
				idx[p.Slot] = len(out.Worn) - 1
			}
			continue
		}
		i, ok := idx[p.Slot]
		if !ok {
			return bag, fmt.Errorf("equipment transform: worn slot %d disappeared", p.Slot)
		}
		out.Worn[i].Template = p.To
		out.Worn[i].Durability = d
		// ★ `Record`（181 字节实例行）的 **offset 2..6 也存着模板号**，而
		// `BagEquipment.ValidateRecord` 会断言 `Record[2:6] == Template`
		// （`equipment instance template mismatch`，internal/inventory/equipment_record.go:16）。
		//
		// ⚠️ 2026-09-30 02:10 实测教训：只改 `Template`、留着 `Record` 里旧模板号 ⇒ **整个角色
		// 读不出来** —— 服务端每次 `ReadBag` 都校验失败，客户端选角界面**一个角色都不显示**、
		// 切换角色黑屏。所以**必须同步**；而且要先**拷贝**再写，否则会就地改到调用方那份 Bag
		// （`Record` 是切片，与入参共享底层数组）。
		if len(out.Worn[i].Record) == protocol.CurrentItemRecordSize {
			rec := append([]byte(nil), out.Worn[i].Record...)
			binary.LittleEndian.PutUint32(rec[2:], p.To)
			out.Worn[i].Record = rec
		}
	}
	return out, nil
}

// registerTransformedSources 把**换下去的源装备**登记进装备库（装备图鉴）。
//
// 为什么必须有这一步（2026-09-30 实机取证 + 用户确认，方案「甲」）：
// 变换只把 `worn` 那一条**改写**成目标 —— 源装备的实体就此消失。而图鉴（`Journal.Counts`）
// 是"能不能被选为变换目标"的**唯一池子**，它的入口**只有分解**。于是：
//
//	2026-09-30 01:31 那次 11 件变换：A 套只有 3 件（100151106/100101165/100323418）在 counts 里，
//	⇒ 玩家第二次只能把**那 3 件**换回来，另外 8 件在图鉴里根本不存在 ⇒ "换装前的套装消失了"。
//
// 登记走与分解（CMD26）**同一条路**：`JournalLimit` 判资格 → `ledger.Add`。
// 达上限时**不报错**（与分解一致：只登记、绝不因此拒绝变换）。
func registerTransformedSources(
	gear *EquipmentCatalog,
	rules *catalog.EquipmentJournalRules,
	ledger EquipmentJournal,
	pairs []EquipmentTransformPair,
) EquipmentJournal {
	for _, p := range pairs {
		if p.From == 0 || p.From == p.To {
			continue
		}
		limit, ok := JournalLimit(gear, rules, p.From)
		if !ok {
			continue
		}
		next, _, e := ledger.Add(p.From, 1, limit)
		if e != nil {
			continue
		}
		ledger = next
	}
	return ledger
}

// wornOf 读身上某个槽当前穿的模板。
func wornOf(bag Bag, slot uint16) (uint32, bool) {
	for _, w := range bag.Worn {
		if w.Slot == slot {
			return w.Template, true
		}
	}
	return 0, false
}

// equipmentKind 读装备的 `[equipment type]` 文本（如 `[weapon]`）。
func (s *ItemService) equipmentKind(id uint32) string {
	d, e := s.Equipment.Definition(id)
	if e != nil {
		return ""
	}
	toks := d.Fields["[equipment type]"]
	if len(toks) == 0 {
		return ""
	}
	return toks[0].Text
}

// kindForWornSlot 反查"这个穿戴槽位对应哪个装备类型"（12 → `[weapon]`、14 → `[coat]`…）。
func (s *ItemService) kindForWornSlot(slot uint16) string {
	for kind, at := range s.WearRules.Slots {
		if at == slot {
			return kind
		}
	}
	return ""
}

// transformSource 找「变换前」那一件：**先看身上穿的，再看背包装备区里同部位的**。
//
// 实机 2026-09-30 03:24 的教训：客户端允许把**背包里**的装备放进界面「变换前」槽
// （图鉴提示也写着「转换之前，装备将在军械库中注册并作为灵魂退款」），而请求里**只有部位码**。
// 旧实现只查 `worn`，于是那件被当成"身上没有装备"直接跳过 —— 客户端只看到换装动画、
// 装备却没变（`TRANSFORM-REFUSED: … (slot 12: 身上没有装备(held=false from=0))`）。
//
// 背包里同部位有**多件**时返回 false（无法确定是哪一件，宁可不动）。
func (s *ItemService) transformSource(bag Bag, slot uint16) (uint32, uint16, bool) {
	if t, ok := wornOf(bag, slot); ok && t != 0 {
		return t, 0, true // 0 = 源在身上
	}
	kind := s.kindForWornSlot(slot)
	if kind == "" {
		return 0, 0, false
	}
	var found BagEquipment
	seen := false
	for _, e := range bag.Equipment {
		if e.Template == 0 || s.equipmentKind(e.Template) != kind {
			continue
		}
		if seen {
			return 0, 0, false // 同部位多件 ⇒ 无法确定
		}
		found, seen = e, true
	}
	if !seen {
		return 0, 0, false
	}
	return found.Template, found.Slot, true
}

// costGroupFor 把「变换目标模板」映射到 `[create cost]` 的档位组。
//
// 实测 2026-09-30：请求里的目标模板**一个都不在**任何组的 `[item index]` 里 ——
// 组是按 `[grade]` + `[rarity]` 分档的（组 1/6/8=(119,3)、组 2/7=(120,6)、组 3/5/9=(121,4)），
// 组内 items 只是"该档的代表"。所以先精确查（保持既有行为），查不到再按**档位**找，
// 取**列表里第一个**同档的组（组序稳定 ⇒ 结果可复现）。
func (s *ItemService) costGroupFor(template uint32) (catalog.CreateCostGroup, bool) {
	if g, ok := s.CreateCost.GroupFor(template); ok {
		return g, true
	}
	grade, rarity, ok := s.equipmentGradeRarity(template)
	if !ok {
		return catalog.CreateCostGroup{}, false
	}
	for _, g := range s.CreateCost.Groups {
		for _, t := range g.Items {
			gg, rr, ok := s.equipmentGradeRarity(t)
			if ok && gg == grade && rr == rarity {
				return g, true
			}
		}
	}
	// 兜底：按 **`[rarity]` 最接近** 找一个档位。
	//
	// [ALIGN-20260930-WEAPON] 武器 `(122,8 [weapon])` 与誓约 `(116,4/8 [oath]|[primer])` 的档位
	// 在 `[create cost]` 里**根本不存在**（那张表只有 (119,3) / (120,6) / (121,4) 三档，且只覆盖
	// 11 个防具首饰部位 + `[amalgamation stone]`）。但用户提供的官方规则明确说
	// 「装备库的**武器**页签里能选择所有分解过的武器做变换」「拿到太初武器自选后直接分解，
	// 再在装备库—武器中通过装备变换把 +12 等打造继承过去」⇒ 武器必须能变换。
	//
	// 口径：取**rarity ≥ 目标 rarity 里最小的档**；没有更高的就取**最高的档**
	// ⇒ rarity 8 → (120,6) 那一档（组 2）。
	// ⚠️ **这个兜底口径是本地定的**（源表无依据），拿到更明确的规则随时可换。
	type cand struct {
		g catalog.CreateCostGroup
		r int32
	}
	var cands []cand
	for _, g := range s.CreateCost.Groups {
		for _, t := range g.Items {
			if _, rr, ok := s.equipmentGradeRarity(t); ok {
				cands = append(cands, cand{g, rr})
				break // 每组取第一个 item 的 rarity 作代表
			}
		}
	}
	if len(cands) == 0 {
		return catalog.CreateCostGroup{}, false
	}
	best := -1
	for i, c := range cands {
		if c.r < rarity {
			continue
		}
		if best < 0 || c.r < cands[best].r {
			best = i
		}
	}
	if best < 0 {
		best = 0
		for i, c := range cands {
			if c.r > cands[best].r {
				best = i
			}
		}
	}
	return cands[best].g, true
}

// equipmentGradeRarity 读装备的 `[grade]` / `[rarity]`（档位）。
func (s *ItemService) equipmentGradeRarity(id uint32) (int32, int32, bool) {
	d, e := s.Equipment.Definition(id)
	if e != nil {
		return 0, 0, false
	}
	num := func(k string) (int32, bool) {
		ts := d.Fields[k]
		if len(ts) == 0 || ts[0].Type != 0 {
			return 0, false
		}
		return ts[0].Value, true
	}
	g, ok1 := num("[grade]")
	r, ok2 := num("[rarity]")
	return g, r, ok1 && ok2
}

// walletSoulByRarity 是「装备变换」要扣的灵魂：**按目标的稀有度一对一**。
//
// 依据是客户端「变换确认」界面的口径（用户实机截图）——目标 `117010280`（rarity 8 = 太初）
// 那一栏写着「**所需灵魂 1 太初(s)**」。而源里的 `[create cost]` 只按 `(grade,rarity)` 分了三档
// （`10361513`/`10361514`/`10361515`，对应 (119,3)/(120,6)/(121,4)），**根本没有"太初灵魂"这一档**
// —— 武器（rarity 8）在那边永远匹配不到，客户端也就一直卡。所以变换的成本**不走 `[create cost]`**，
// 一律按这张表取。
//
// 五个模板号全部在**账号材料槽**里（375..379，见 accountMaterialSlotByTemplate），所以从这里扣。
var walletSoulByRarity = map[int32]uint32{
	2: 10361512, // 稀有灵魂
	3: 10361513, // 神器灵魂
	4: 10361514, // 传说灵魂
	6: 10361515, // 史诗灵魂
	8: 10361516, // 太初灵魂
}

// transformGoldCost 是一次装备变换的金币成本（客户端界面同样显示的金币栏）。
const transformGoldCost = 50000

// soulFor 按稀有度取对应的灵魂模板（客户端界面只列 2/3/4/6/8 五档）。
func soulFor(rarity int32) (uint32, bool) {
	tpl, ok := walletSoulByRarity[rarity]
	return tpl, ok
}

// transformCost 给出一次变换的成本：**目标稀有度对应的灵魂 ×1** + 固定金币。
//
// ⚠️ 金币量（50,000）取自客户端界面的金币栏；如果官方另有按稀有度递进的表，改这一个常量即可。
func (s *ItemService) transformCost(target uint32) (uint32, uint32, error) {
	_, rarity, ok := s.equipmentGradeRarity(target)
	if !ok {
		return 0, 0, fmt.Errorf("读不到目标 %d 的稀有度", target)
	}
	soul, ok := soulFor(rarity)
	if !ok {
		return 0, 0, fmt.Errorf("稀有度 %d 没有对应的灵魂（客户端界面只列 2/3/4/6/8）", rarity)
	}
	return soul, transformGoldCost, nil
}

// transformKey 让同一次变换（同一份请求 + 同一个前置状态）只应用一次。
//
// ⚠️ 键必须 **≤200 字节**：`storage.CommitAccountMaterialEvent` 会拒绝超长的 event_key，
// 报的就是 `invalid account material event`。把 11 个槽 + 11 个模板**原样拼进去**是 219 字节
// —— 2026-09-30 01:16 实机就是这么被整批拒掉的（CreateEquipment 只带 1 个模板，所以没暴露）。
// 所以这里把两侧输入各自**哈希成 8 字节**再拼，长度固定 43。
//
// 语义与 craftEventKey 同构：键里带**前置状态摘要**，所以"状态已经变了还在重发的同一份请求"
// 会被识别成重复；而玩家合法地再换一次（状态已不同）不会被挡。
func transformKey(slots []uint32, templates []uint32, state json.RawMessage) string {
	h := sha256.New()
	for _, s := range slots {
		fmt.Fprintf(h, "%d,", s)
	}
	h.Write([]byte{'|'})
	for _, t := range templates {
		fmt.Fprintf(h, "%d,", t)
	}
	req := h.Sum(nil)
	pre := sha256.Sum256(state)
	return fmt.Sprintf("transform:%x:%x", req[:8], pre[:8])
}

// JournalRegistration 是一次成功的收录：模板 → 收录后的**绝对**份数。
//
// 只有事实，没有"库内实例"：官方客户端的 CMD26「装备库添加」**就是一次分解**，
// 装备本体被消耗，不存在可还原的实例属性，也没有按条目的容量/锁定/占用模型。
// （另一棵树曾为此造过一整套 `armory/store.go`，后来整份删掉 —— 别重开这条路。）
type JournalRegistration struct {
	Template uint32 `json:"template"`
	Limit    uint32 `json:"limit"`
	After    uint32 `json:"after"`
}

// JournalSkip 说明某一件**本该登记却没登记**的原因。
//
// 存在的意义就是"不静默放行"：排查"分解了但图鉴里没有"时，必须能区分
// 「不在收录范围」/「已到上限」/「背包快照与删除结果对不上」这三件事。
type JournalSkip struct {
	Slot     uint16 `json:"slot"`
	Template uint32 `json:"template,omitempty"`
	Reason   string `json:"reason"`
}

// disjointEventKey 让"同一批分解"只应用一次。
//
// ⚠️ 键里必须带**整批内容**：旧版只取 `Items[0]`，于是"第一件相同、其余不同"的两批分解会
// 撞同一个 event_key —— 第二批被 `CommitCharacterEvent` 当成重放、取回上一批的 receipt，
// 接着 `len(result.DeletedSlots) != len(slots)` 触发 `disjoint receipt conflict`，**整批被拒**
// （装备没删、图鉴没登记）。实测 2026-09-30 02:36:48 / 02:41:48 各一次。
//
// 对 (slot, template) 排序后再哈希 ⇒ 同一批的不同排列也是同一个键，内容变则键变。
func disjointEventKey(toolSlot uint16, items []protocol.DisjointItemEntry) string {
	type pair struct {
		slot uint16
		tpl  uint32
	}
	ps := make([]pair, 0, len(items))
	for _, it := range items {
		ps = append(ps, pair{it.Slot, it.Template})
	}
	sort.Slice(ps, func(i, j int) bool {
		if ps[i].slot != ps[j].slot {
			return ps[i].slot < ps[j].slot
		}
		return ps[i].tpl < ps[j].tpl
	})
	h := sha256.New()
	fmt.Fprintf(h, "tool:%d|n:%d|", toolSlot, len(ps))
	for _, p := range ps {
		fmt.Fprintf(h, "%d:%d,", p.slot, p.tpl)
	}
	return fmt.Sprintf("disjoint:%x", h.Sum(nil)[:12])
}

type DisjointReceipt struct {
	DeletedSlots []uint16                       `json:"deleted_slots"`
	ToolSlot     uint16                         `json:"tool_slot"`
	Rewards      []protocol.DisjointRewardEntry `json:"rewards"`
	Source       string                         `json:"source"`
	// JournalAdded 是本次分解**同时写进装备库账本**的收录增量（与扣装备、发材料同一事务）。
	// 空表示这次没有任何一件需要登记 —— 调用方据此决定要不要补发 2610。
	JournalAdded []JournalRegistration `json:"journal_added,omitempty"`
	// JournalSkipped 记录未收录的原因，空表示全部登记成功或整条特性没开。
	JournalSkipped []JournalSkip `json:"journal_skipped,omitempty"`
}

// journalRegistrations 计算本次分解要写进装备库账本的收录增量（纯函数，不落库）。
//
// 规格 `CMD/0026-DISJOINTITEM`：客户端的 CMD26「分解」**同时就是「装备库添加」**，
// 所以登记挂在分解上，而不是另有一条登记命令。三条纪律：
//
//  1. **模板一律取服务端背包里那一行**（`bySlot` 由 `Bag.Disjoint` 之前的背包快照构建）。
//     `Bag.Disjoint` 只按槽位取行、**不校验请求里客户端上报的模板**，所以请求值不能当依据。
//  2. **达上限只跳过收录，不拒绝分解** —— 分解本身照常扣装备、发材料，回执里记下原因。
//  3. **规则表未装载 ⇒ 不收录也不报错**（与其它可选表一致）。此时连 skip 都不记：
//     整条特性是关的，不是"这一件被跳过"。
//
// 上限判据一律走 `JournalLimit`（`minimum level == 115` ∧ rarity 在集合内，
// 再按 `[equipment type]` 回落/收紧）——**不在这里另立第二处口径**：同一条规则两个答案是
// 另一棵树踩过的坑（登记在 A 处拒绝、在 B 处放行的"黑牙"）。
func journalRegistrations(
	ledger EquipmentJournal,
	bySlot map[uint16]uint32,
	deletedSlots []uint16,
	equipment EquipmentDefinitioner,
	rules *catalog.EquipmentJournalRules,
) (EquipmentJournal, []JournalRegistration, []JournalSkip, error) {
	if rules == nil || equipment == nil {
		return ledger, nil, nil, nil
	}
	var added []JournalRegistration
	var skipped []JournalSkip
	for _, slot := range deletedSlots {
		template := bySlot[slot]
		if template == 0 {
			// 理论上不可达（Bag.Disjoint 刚把这一行删掉）。真出现说明背包快照与删除结果
			// 对不上 —— 必须看得见，不能静默略过。
			skipped = append(skipped, JournalSkip{Slot: slot, Reason: "no bag row"})
			continue
		}
		limit, ok := JournalLimit(equipment, rules, template)
		if !ok {
			// 不在收录范围（等级不是 115 / rarity 不在 {2,3,4,6,8} / 禁拆）：只走普通分解。
			skipped = append(skipped, JournalSkip{Slot: slot, Template: template, Reason: "not registrable"})
			continue
		}
		if ledger.Counts[template] >= limit {
			// 达上限（普通 99，`[oath]` 这类按类型收紧的 1）：**只跳过收录**。
			skipped = append(skipped, JournalSkip{Slot: slot, Template: template, Reason: "cap reached"})
			continue
		}
		next, after, e := ledger.Add(template, 1, limit)
		if e != nil {
			// 上限已在上面判过 ⇒ 走到这里说明前提被破坏。宁可整批回滚，也不要写出一份
			// 自己都解释不了的账本（半提交在界面上看不出来，却已经吃掉了玩家的装备）。
			return ledger, nil, nil, e
		}
		ledger = next
		added = append(added, JournalRegistration{Template: template, Limit: limit, After: after})
	}
	return ledger, added, skipped, nil
}

type DisjointPlan struct {
	Key         string
	Slots       []uint16
	Definitions EquipmentDefinitioner
}

func (s *ItemService) PlanDisjoint(role Role, r protocol.DisjointItemRequest) (DisjointPlan, error) {
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return DisjointPlan{}, (fmt.Errorf("inventory source mismatch"))
	}
	if len(r.Items) == 0 {
		return DisjointPlan{}, (fmt.Errorf("empty disjoint request"))
	}

	slots := make([]uint16, len(r.Items))
	for i, item := range r.Items {
		slots[i] = item.Slot
	}
	// 收录判据要按接口注入（nil = 规则表没装 ⇒ 只走普通分解）。
	// 不能直接塞 `*EquipmentCatalog`：那是个带类型的 nil，进了接口就不是 nil 了。
	var defs EquipmentDefinitioner
	if s.Equipment != nil {
		defs = s.Equipment
	}

	// [ALIGN-20260930-DISJOINT-KEY] 幂等键必须覆盖**整批**请求。
	//
	// 原来只用 `Items[0]`（`disjoint:<tool>:<第一件slot>:<第一件template>`）—— 只要两次分解的
	// **第一件相同**就会撞同一个 event_key：第二次被 `CommitCharacterEvent` 当作重放，直接取回
	// 上一次的 receipt ⇒ `len(result.DeletedSlots) != len(slots)` ⇒ `disjoint receipt conflict`
	// （见下面那个校验）⇒ **整批被拒**（装备没删、图鉴没登记）。
	// 实测 2026-09-30 02:36:48 / 02:41:48 各一次 —— 用户看到的就是"分解了却没入库"。
	key := disjointEventKey(r.ToolSlot, r.Items)
	return DisjointPlan{Key: key, Slots: slots, Definitions: defs}, nil
}
func (s *ItemService) PrepareDisjoint(current Role, r protocol.DisjointItemRequest, plan DisjointPlan) (json.RawMessage, json.RawMessage, error) {
	var result DisjointReceipt
	slots, defs := plan.Slots, plan.Definitions
	b, e := ReadBag(current.State)
	if e != nil {
		return nil, nil, e
	}
	// 登记用的是**背包里那一行**的模板（见 journalRegistrations 的纪律 1）。
	// 删除之后这一行就没了，所以必须在 Disjoint 之前先按槽位记下来。
	bySlot := make(map[uint16]uint32, len(b.Equipment))
	for _, row := range b.Equipment {
		bySlot[row.Slot] = row.Template
	}
	b, res, e := b.Disjoint(s.Catalog, s.BagRules, s.Equipment, slots, r.ToolSlot)
	if e != nil {
		return nil, nil, e
	}
	updated, e := SaveBag(current.State, b)
	if e != nil {
		return nil, nil, e
	}
	// 收录与"扣装备 / 发材料"在**同一个 apply 回调**里 ⇒ 同生共死，不存在
	// "装备扣了、收录没写"或"收录写了、材料没发"的半状态。
	ledger, e := ReadEquipmentJournal(updated)
	if e != nil {
		return nil, nil, e
	}
	ledger, added, skipped, e := journalRegistrations(ledger, bySlot, res.DeletedSlots, defs, s.Journal)
	if e != nil {
		return nil, nil, e
	}
	if len(added) > 0 {
		raw, e := SaveEquipmentJournal(updated, ledger)
		if e != nil {
			return nil, nil, e
		}
		updated = raw
	}
	var rewards []protocol.DisjointRewardEntry
	for _, rw := range res.Rewards {
		rewards = append(rewards, protocol.DisjointRewardEntry{
			Slot:     rw.Slot,
			Template: rw.Template,
			Count:    rw.Count,
		})
	}
	result = DisjointReceipt{
		DeletedSlots:   res.DeletedSlots,
		ToolSlot:       res.ToolSlot,
		Rewards:        rewards,
		Source:         s.Catalog.Source.SaveIdentity(),
		JournalAdded:   added,
		JournalSkipped: skipped,
	}
	receipt, e := json.Marshal(result)
	return updated, receipt, e
}

// ItemService owns inventory item transformations that span several bag rules.
// Persistence and event replay remain the responsibility of workflow.
type ItemService struct {
	// Model preserves the existing character/account event model.
	Model          string
	Catalog        catalog.LootCatalog
	BagRules       BagRules
	Equipment      *EquipmentCatalog
	AvatarDisjoint *AvatarDisjointRules
	AvatarSockets  *AvatarSocketRules
	EmblemInlay    *EmblemInlayRules
	EmblemCompound *EmblemCompoundRules
	// WearRules maps a requested body slot to equipment still in the bag.
	// A nil map retains the existing equipped-only lookup.
	WearRules WearRules
	// A nil Journal skips registration; a nil CreateCost leaves crafting
	// at its existing window-only step without changing saved items.
	Journal    *catalog.EquipmentJournalRules
	CreateCost *catalog.EquipmentCreateCost
}
