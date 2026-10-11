package inventory

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
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
	// VaultGold 是本次从**账号金库**调取的金币（背包不够时才会 > 0）。
	VaultGold uint32 `json:"vault_gold,omitempty"`
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
	// Unpriced 表示**源里没有这件的 `[create cost]` 条目**（`[item index]` 精确查找未命中，
	// Group 是按品级/稀有度兜出来的近似档）。此时教学期免单，见 PrepareEquipmentCraft。
	Unpriced bool
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
	group, ok := s.costGroupFor(template)
	if !ok {
		return EquipmentCraftPlan{}, (fmt.Errorf("equipment craft: template %d is in no create-cost group", template))
	}

	_, exact := s.CreateCost.GroupFor(template)
	key := craftEventKey(template, slot, group.Index, role.State)
	return EquipmentCraftPlan{Key: key, Group: group, Unpriced: !exact}, nil
}

// payTransformGold 从「背包金币 + 账号金库」里扣掉 `need` 金币，
// 返回新的背包、剩余金库金币、以及**本次从金库调取了多少**。
//
// 口径（2026-10-04 业主确认的官方语义「取用金库材料」）：**背包优先，不足部分从账号金库同步调取**。
// 背景：实机角色背包 0 金币、金库 8 亿，变换/生成被 "need 470000 gold, have 0" 拒绝 —— 金库金币
// 此前完全不参与。`vaultGold` 由调用方从 `storage.AccountVaultState.Gold` 传入并回写。
//
// 余额不足时返回错误（**不改任何状态**，让上层按"这一件换不起"跳过或整笔回滚）。
func payTransformGold(bag Bag, vaultGold, need uint32) (Bag, uint32, uint32, error) {
	if need == 0 {
		return bag, vaultGold, 0, nil
	}
	fromBag := need
	if fromBag > bag.Gold {
		fromBag = bag.Gold
	}
	rest := need - fromBag
	if rest > vaultGold {
		return bag, vaultGold, 0, fmt.Errorf("need %d gold, have %d in bag + %d in vault",
			need, bag.Gold, vaultGold)
	}
	out := bag
	out.Gold = bag.Gold - fromBag
	return out, vaultGold - rest, rest, nil
}

// tutorialFreeCraft 判定一次「生成」是否走**教学免单**：源里查不到这件的
// `[create cost]` 条目（`plan.Unpriced`）**且**玩家仍在 662 训练轨道
// （`bag.TutorialMode()`）。
//
// 两个条件缺一不可：出关后同一件仍走正常（兜底）定价，免单不得外溢到常规玩法。
// 依据见 PrepareEquipmentCraft 里的说明与 next155 文档。
func tutorialFreeCraft(plan EquipmentCraftPlan, bag Bag) bool {
	return plan.Unpriced && bag.TutorialMode()
}

func (s *ItemService) PrepareEquipmentCraft(current Role, accountRaw json.RawMessage, vaultGold uint32, template uint32, payOption int, plan EquipmentCraftPlan) (json.RawMessage, json.RawMessage, uint32, EquipmentCraftReceipt, error) {
	var result EquipmentCraftReceipt
	group := plan.Group
	ledger, e := ReadEquipmentJournal(current.State)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	if ledger.Counts[template] == 0 {
		// 未登记 ⇒ 拒绝。这是**规格**不是兜底：窗口列的只有已登记条目。
		return nil, nil, vaultGold, result, fmt.Errorf("equipment craft: template %d is not registered in the journal", template)
	}
	bag, e := ReadBag(current.State)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	account, e := ReadAccountMaterials(accountRaw)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}

	// ★ 教学免单（业主明确要求，2026-10-04）：662 训练轨道内、源里查不到成本条目的件，
	// 按 **0 材料 / 0 金币** 执行 —— 客户端界面就是这样显示的（实测第 10 关 100051285
	// 「只有一个定价无法切换、金币免费」）。
	//
	// 为什么必须这样做：第 10 关的教学目标就是让玩家**走一遍装备变换**（`[guide actions]
	// open equipment journal`），源里那件没有 `[create cost]` 条目 ⇒ 需求本来就是 0。
	// 此前按品级兜一档收 35,000 金币 ⇒ 背包/金库都是 0 的学员号被拒死，而 2259 拒绝链
	// 又因客户端会崩（`[ALIGN-20260930-CRAFT-NO-REFUSAL]`）不能回包 ⇒ 界面「点了没反应」，
	// 教学直接卡在第 10 关。
	//
	// ⚠️ 这是**刻意偏离原生源**（源里没有这条定价），只作用于训练轨道（`bag.TutorialMode()`），
	// 出关后同一件仍走正常定价。依据与边界见 CHANGELOG 与
	// `analysis/tasks/next155-662第十关卡点-装备库2259口径分歧.md`。
	var option int
	var gold uint32
	var bagMats, accountMats []MaterialCost
	if tutorialFreeCraft(plan, bag) {
		option = payOption
	} else {
		option, gold, bagMats, accountMats, e = pickCraftCost(bag, vaultGold, account, group, payOption)
		if e != nil {
			return nil, nil, vaultGold, result, e
		}
	}
	paid, e := bag.PayMaterials(bagMats, 1)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	paid, vaultNext, fromVault, e := payTransformGold(paid, vaultGold, gold)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	out := account
	for _, m := range accountMats {
		next, _, e := out.Spend(m.Template, m.Count)
		if e != nil {
			return nil, nil, vaultGold, result, e
		}
		out = next
	}
	placed, slots, e := paid.AddEquipment(s.Equipment, s.BagRules.EquipmentSlots, template, 1)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	updated, e := SaveBag(current.State, placed)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	accountNext, e := out.Save()
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	result.Template = template
	result.Group = group.Index
	result.Cost = option
	result.Gold = gold
	result.VaultGold = fromVault
	for _, m := range bagMats {
		result.Materials = append(result.Materials, CraftMaterial{Template: m.Template, Amount: m.Count})
	}
	for _, m := range accountMats {
		result.Materials = append(result.Materials, CraftMaterial{Template: m.Template, Amount: m.Count, FromAccount: true})
	}
	result.Source = s.Catalog.Source.SaveIdentity()
	if len(slots) > 0 {
		result.Slot = slots[0]
	}
	return updated, accountNext, vaultNext, result, nil
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
// ★ **金币口径必须与 `payTransformGold` 一致**：背包优先，不足部分从账号金库调取。
// 2026-10-04 实机（662 第十关，角色背包 0 金币）踩到：这里原先只比 `bag.Gold`，
// 于是"金库里有 8 亿"也会被拒在 `needs 35000 gold, have 0` —— 而紧随其后的
// `payTransformGold(paid, vaultGold, gold)` 本来就会去金库取，等于这道前置检查
// 把同一笔成本判了两次、还判错一次。`vaultGold` 由调用方从
// `storage.AccountVaultState.Gold` 传入（与变换线同一个来源）。
//
// 返回的四项：付法序号、金币、**背包付**的材料、**账号仓库付**的材料。
// 分仓依据是 `AccountMaterialSlot`：命中说明该模板被 115 客户端固定映射到
// 账号共享容器 35（三档登记证就在其中），必须从仓库扣而不是背包。
func pickCraftCost(
	bag Bag,
	vaultGold uint32,
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
	// 背包优先、不足从账号金库调取：与 payTransformGold 同一条口径（见顶部注释）。
	if total := uint64(bag.Gold) + uint64(vaultGold); uint64(gold) > total {
		return 0, 0, nil, nil, fmt.Errorf(
			"equipment craft: cost %d needs %d gold, have %d in bag + %d in vault",
			opt.Number, gold, bag.Gold, vaultGold)
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
	// VaultGold 是本次从**账号金库**调取的金币（背包不够时才会 > 0）。
	VaultGold uint32 `json:"vault_gold,omitempty"`
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
//  2. 成本按目标的**稀有度**从源 `etc/115lvability/equipmenttransformsystem.cos` 的
//     `[need materials]` 里取（**不是** `[create cost]` —— 那张表按**件**定价，是生成用的；
//     两张表口径不同，正是第 10 关免单要按件谓词判的原因，见 PrepareEquipmentTransform）；
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
	// unpriced 表示**源里没有这件的 `[create cost]` 条目**（`[item index]` 精确查找未命中）
	// —— 与生成路径 `EquipmentCraftPlan.Unpriced` **同一个按件谓词**。
	// 变换表 `[need materials]` 是按**稀有度**分档的，所以只看那张表会把"源里根本没定价的
	// 教学件"按它所属稀有度收钱；教学轨道内要按这个按件谓词免单，见
	// PrepareEquipmentTransform 的「教学免单」分支。
	unpriced bool
}

type EquipmentTransformPlan struct {
	Key     string
	Steps   []transformStep
	Receipt EquipmentTransformReceipt
	// PayOption 是请求头 [13] 的付款方式序号。它必须随计划一起递交事务回调 ——
	// 成本按玩家点的那一支扣（源里 `[need materials]` 每档两支），不回退。
	PayOption int
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
		// 账本守卫：旧件必须能登记回图鉴（在收录范围内且未达上限），否则这一条**整条不做**。
		// 与事务里的判断同一个函数（`equipmentSwapAllowed`），保证"能规划的"＝"能落库的"。
		if !equipmentSwapAllowed(s.Equipment, s.Journal, ledger, from, target) {
			notes = append(notes, fmt.Sprintf("slot %d: 目标 %d 的旧件 %d 回不了图鉴（不在收录范围或已达上限）", slot, target, from))
			result.Skipped = append(result.Skipped, target)
			continue
		}
		if _, e := s.transformPayment(catalog.TransformChainEquipment, target, payOption); e != nil {
			notes = append(notes, fmt.Sprintf("slot %d: 目标 %d 算不出变换成本（%v）", slot, target, e))
			result.Skipped = append(result.Skipped, target)
			continue
		}
		// 按件记下"源里有没有这件的成本条目"（与生成路径同谓词）。
		// `GroupFor` 返回 (近似档, 是否精确命中)；精确未命中 ⇒ 源里没有这件 ⇒ 教学期免单。
		_, exact := s.CreateCost.GroupFor(target)
		plans = append(plans, transformStep{slot: slot, from: from, to: target, bagSlot: bagSlot, unpriced: !exact})
	}
	if len(plans) == 0 {
		// 把逐件判定写进错误串：2026-09-30 03:06 那次日志只说了 "nothing transformable"，
		// 而四个条件看起来都满足 —— 继续靠猜会浪费一轮。现在一次就能看清卡在哪条。
		return EquipmentTransformPlan{Receipt: result}, (fmt.Errorf("equipment transform: nothing transformable in %d requested slots (%s)",
			len(templates), strings.Join(notes, "; ")))
	}

	key := transformKey(slots, templates, role.State)
	return EquipmentTransformPlan{Key: key, Steps: plans, Receipt: result, PayOption: payOption}, nil
}
func (s *ItemService) PrepareEquipmentTransform(current Role, accountRaw json.RawMessage, vaultGold uint32, plan EquipmentTransformPlan) (json.RawMessage, json.RawMessage, uint32, EquipmentTransformReceipt, error) {
	result, plans := plan.Receipt, plan.Steps
	live, e := ReadBag(current.State)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	liveLedger, e := ReadEquipmentJournal(current.State)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	account, e := ReadAccountMaterials(accountRaw)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}

	// 事务内重校验：状态可能已被别的请求改过。**账本守卫（`equipmentSwapAllowed`）与
	// 扣账必须分开**：守卫不通过的条目要在**任何写入之前**剔除（否则会出现"装备换了、
	// 图鉴却没记账"的半状态）。
	var bagMats, accountMats []MaterialCost
	var gold uint32
	option := 0
	next := live
	var applicable []transformStep
	for _, p := range plans {
		if liveLedger.Counts[p.to] == 0 {
			result.Skipped = append(result.Skipped, p.to)
			continue
		}
		if from, bs, held := s.transformSource(next, p.slot); !held || from != p.from || bs != p.bagSlot {
			result.Skipped = append(result.Skipped, p.to)
			continue
		}
		if !equipmentSwapAllowed(s.Equipment, s.Journal, liveLedger, p.from, p.to) {
			// 旧件回不到图鉴（不在收录范围 / 已达上限）⇒ 这一条整条不做：
			// 宁可"点了没反应"，也不许扣了目标却不还旧件（那就是"图鉴变少"）。
			result.Skipped = append(result.Skipped, p.to)
			continue
		}
		applicable = append(applicable, p)
	}
	var done []EquipmentTransformPair
	for _, p := range applicable {
		// 成本：源 `[need materials]` 里该稀有度的、玩家点的那一支付法（请求头 [13]）。
		// ⚠️ 旧实现是「灵魂 ×1 + 固定 50000 金币」，只有 primeval 一档与源相同 ——
		// rare..epic 一直多扣金币（源是 25000/30000/35000/40000）。现在整表直读。
		var pay transformPayment
		pay, e = s.transformPayment(catalog.TransformChainEquipment, p.to, plan.PayOption)
		// ★ 教学免单：662 训练轨道内，**源里没有这件成本条目**的目标按 0 材料 / 0 金币执行。
		//
		// 为什么必须用按件谓词（`p.unpriced`）而不是"算不出成本"：变换表 `[need materials]`
		// 是**按稀有度**分档的。第 10 关教学件 `100051285` 在 `[create cost]` 里根本没有条目
		// （PVF 投影 `configs/equipment-create-cost.generated.json` 全表查无此件），
		// 却因稀有度是 legendary 而命中「登记证×1 + 35,000 金币 / 融合石×7」——
		// 实机 2026-10-06 03:15:20 就是这么被拒的：
		//
		//	equipment craft open: panel=36 context=0x551afe0 action=0 pay_option=1 slots=[14] templates=[100051285]
		//	equipment craft TRANSFORM-REFUSED: requested=1: need 35000 gold, have 6973 in bag + 0 in vault
		//
		// 客户端对同一件显示的却是 **FREE**（实机截图：材料行 FREE、金币不扣）⇒ L0 口径就是 0。
		// 且 2259 没有拒绝分支（回 Error 客户端 `exit=0xC0000005`）⇒ 拒绝等于"点了没反应"，
		// 界面只做本地乐观更新、退出重读就打回 —— 教学卡在第 10 关。
		//
		// ⚠️ 与生成路径一致，这是**刻意偏离原生源**；只作用于训练轨道（`live.TutorialMode()`），
		// 出关后同一件仍按源扣。原来的"算不出成本也免单"作为兜底保留（表未装载等情况）。
		tutorialFree := live.TutorialMode() && (p.unpriced || e != nil)
		switch {
		case tutorialFree:
			pay = transformPayment{Option: plan.PayOption}
		case e != nil:
			result.Skipped = append(result.Skipped, p.to)
			continue
		}
		affordable := true
		for _, m := range pay.AccountMats {
			if account.Count(m.Template) < m.Count {
				affordable = false
				break
			}
		}
		if !affordable {
			result.Skipped = append(result.Skipped, p.to)
			continue
		}
		accountMats = append(accountMats, pay.AccountMats...)
		bagMats = append(bagMats, pay.BagMats...)
		option = pay.Option
		gold += pay.Gold
		done = append(done, EquipmentTransformPair{
			Slot: p.slot, From: p.from, To: p.to, Group: 0,
			FromBag: p.bagSlot != 0, BagSlot: p.bagSlot,
		})
	}
	if len(done) == 0 {
		return nil, nil, vaultGold, result, fmt.Errorf("equipment transform: none of the %d pairs is affordable", len(plans))
	}
	paid, e := next.PayMaterials(bagMats, 1)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	// 金币：背包优先，不足部分从**账号金库**同步调取（业主确认的官方「取用金库材料」语义）。
	paid, vaultNext, fromVault, e := payTransformGold(paid, vaultGold, gold)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	result.VaultGold = fromVault
	out := account
	for _, m := range accountMats {
		nxt, _, e := out.Spend(m.Template, m.Count)
		if e != nil {
			return nil, nil, vaultGold, result, e
		}
		out = nxt
	}
	replaced, e := s.applyTransform(paid, done)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	updated, e := SaveBag(current.State, replaced)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	// **账本口径与 CMD2381（晶体/誓约变换）对齐**：目标那件从登记表 −1，被换下的源装备 +1
	// ⇒ 图鉴**总份数不变**。旧实现只做 +1、不做 −1，于是每换一次装备图鉴就净 +1
	// （2026-10-04 用户报告"图鉴里的东西越来越多/乱变"，两条变换链是同一个病根）。
	// 守卫已在上面剔除过不满足的条目，这里不再产生新的 Skipped。
	ledgerNext, e := applyEquipmentTransformJournal(s.Equipment, s.Journal, liveLedger, done)
	if e != nil {
		return nil, nil, vaultGold, result, e
	}
	if journalTotal(ledgerNext) != journalTotal(liveLedger) {
		// 正常路径下份数总量恒定（进 = 出）；不平就喊出来，便于实机一眼看到。
		log.Printf("装备变换: 图鉴份数变化异常 %d→%d pairs=%+v（请连同本条一起上报）",
			journalTotal(liveLedger), journalTotal(ledgerNext), done)
	}
	if updated, e = SaveEquipmentJournal(updated, ledgerNext); e != nil {
		return nil, nil, vaultGold, result, e
	}
	accountNext, e := out.Save()
	if e != nil {
		return nil, nil, vaultGold, result, e
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
	return updated, accountNext, vaultNext, result, nil
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

// equipmentSwapAllowed 回答"这一条装备变换能不能记账"：
//
//	目标 `to` 必须在册（`counts > 0`）—— 这是"能不能被选为变换目标"的规格；
//	旧件 `from` 必须能**登记回图鉴**（在收录范围内、且未达上限）—— 否则就是"扣了目标却不还旧件"。
//
// `from` 为空或与 `to` 相同 ⇒ 无旧件可还，视为允许（只把目标搬到身上）。
// 计划与事务两遍都调它，保证"能规划出来的"与"能落库的"完全一致。
func equipmentSwapAllowed(
	gear *EquipmentCatalog,
	rules *catalog.EquipmentJournalRules,
	ledger EquipmentJournal,
	from, to uint32,
) bool {
	if to == 0 || ledger.Counts[to] == 0 {
		return false
	}
	if from == 0 || from == to {
		return true
	}
	limit, ok := JournalLimit(gear, rules, from)
	if !ok {
		return false
	}
	return ledger.Counts[from] < limit
}

// applyEquipmentTransformJournal 把一次「装备变换」（CMD2259）对图鉴账本的影响算清。
//
// 口径与 CMD2381 完全一致（见 primer_transform.go 的 primerSourceKind）：**目标来自登记表
// ⇒ 登记 −1，旧件登记 +1（总份数不变）**。旧实现只做 +1 ⇒ 每换一次装备图鉴净 +1。
//
// ⚠️ 计划与事务都必须**先用 `equipmentSwapAllowed` 过滤**，这里只处理已经通过守卫的条目；
// 万一仍有条目过不了，直接返回错误（宁可整笔回滚，也不要写出"装备换了、图鉴没记账"的半状态）。
func applyEquipmentTransformJournal(
	gear *EquipmentCatalog,
	rules *catalog.EquipmentJournalRules,
	ledger EquipmentJournal,
	pairs []EquipmentTransformPair,
) (EquipmentJournal, error) {
	out := ledger.clone()
	if out.Counts == nil {
		out.Counts = map[uint32]uint32{}
	}
	for _, p := range pairs {
		if p.From == 0 || p.From == p.To {
			continue
		}
		if !equipmentSwapAllowed(gear, rules, out, p.From, p.To) {
			return ledger, fmt.Errorf(
				"equipment transform: 目标 %d 或旧件 %d 过不了图鉴守卫（不许只扣不还）", p.To, p.From)
		}
		limit, _ := JournalLimit(gear, rules, p.From)
		// 先减目标、再加旧件（同模板换回时这样才能通过上限）。
		have := out.Counts[p.To]
		if have <= 1 {
			out.Counts[p.To] = 0 // 0 值条目保留："已登记但 0 份"与"从未登记"是两种状态
		} else {
			out.Counts[p.To] = have - 1
		}
		next, _, e := out.Add(p.From, 1, limit)
		if e != nil {
			return ledger, e
		}
		out = next
	}
	return out, nil
}

// registerTransformedSources 把**换下去的源装备**登记进装备库（装备图鉴）。
//
// ⚠️ **只登记、不扣目标** —— 这是低层原语，供 `applyEquipmentTransformJournal` 之外的历史
// 调用点与测试使用。运行期请走 `applyEquipmentTransformJournal`（它同时做 −1 与 +1，
// 保证"换一次装备图鉴净 +1"的旧缺陷不再复发）。
//
// 为什么必须有登记这一步（2026-09-30 实机取证 + 用户确认，方案「甲」）：
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

// costGroupFor 把「目标模板」映射到 `[create cost]` 的档位组（**生成**走这条）。
//
//  1. 精确命中 `[item index]`（保持既有行为）；
//  2. 查不到再按**档位**（`[grade]` + `[rarity]`）找 —— 组内 items 只是"该档的代表"
//     （实测 2026-09-30：请求里的目标模板**一个都不在**任何组的 `[item index]` 里）。
//     同档多组时优先取**部位类型相同**的那组：`(121,4)` 同时有防具组（灵魂+金币）与
//     融合石组（只有金币/巡礼之印），扣法不同、不能混。
//
// ★ **刻意不做「rarity 就近」兜底**（2026-10-04 撤掉 [ALIGN-20260930-WEAPON] 的就近档）：
// 就近档会让 (122,8) 武器按 (120,6) 档扣「10361514 + 35,000」，而实机客户端界面写的是
// 「1 太初(s) + 50,000 金币」⇒ **照就近档扣就是扣错东西**。档位在表里不存在时返回 false，
// 由调用方明确拒绝并把档位写进拒因；变换链的兜底在
// `transformPayment`（源表 `[need materials]`）里，不在这里混用第二张表。
func (s *ItemService) costGroupFor(template uint32) (catalog.CreateCostGroup, bool) {
	if s.CreateCost == nil {
		return catalog.CreateCostGroup{}, false
	}
	if g, ok := s.CreateCost.GroupFor(template); ok {
		return g, true
	}
	grade, rarity, ok := s.equipmentGradeRarity(template)
	if !ok {
		return catalog.CreateCostGroup{}, false
	}
	kind := s.equipmentKind(template)
	var first, sameKind catalog.CreateCostGroup
	haveFirst, haveKind := false, false
	for _, g := range s.CreateCost.Groups {
		for _, t := range g.Items {
			gg, rr, ok := s.equipmentGradeRarity(t)
			if !ok || gg != grade || rr != rarity {
				continue
			}
			if !haveFirst {
				first, haveFirst = g, true
			}
			// 组内 11 件覆盖 11 个部位，所以同部位那一件未必是组里第一个 ⇒ 必须扫全组。
			if !haveKind && kind != "" && s.equipmentKind(t) == kind {
				sameKind, haveKind = g, true
			}
		}
		if haveKind {
			break
		}
	}
	if haveKind {
		return sameKind, true
	}
	return first, haveFirst
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

// transformPayment 给出一次变换要付什么：**按目标的稀有度**查源表，再按玩家点的付法取支。
//
// 源（唯一内容真源 = 内层 PVF）：`etc/115lvability/equipmenttransformsystem.cos`
//
//	装备变换（CMD2259 action=1）→ `[need materials]`：金币（25000..50000，按稀有度）
//	                              **外加一件灵魂** 10361512..10361516
//	晶体/誓约变换（CMD2381）    → `[need primer materials]`：只有金币或巡礼之印，
//	                              没有灵魂项
//
// ⚠️ 旧实现把这条规则硬编码成「`walletSoulByRarity` + 常量 50000 金币」，
// 只有 primeval 一档恰好等于源值 ⇒ rare..epic 一直**多扣**金币。现在整表直读。
//
// 付款方式序号 `payOption` 直接来自请求头（2259 的 u8@13 / 2381 的 u32@13，都是 1 起），
// **刻意不做「付不起就换另一支」的回退**：玩家点的是哪支就扣哪支（先例见 pickCraftCost）。
//
// 材料按 `AccountMaterialSlot` 分两仓：命中 = 账号共享材料（灵魂仓库），其余走背包。
type transformPayment struct {
	Option      int
	Gold        uint32
	BagMats     []MaterialCost
	AccountMats []MaterialCost
}

func (s *ItemService) transformPayment(chain catalog.TransformChain, target uint32, payOption int) (transformPayment, error) {
	var out transformPayment
	if s.Transform == nil {
		return out, fmt.Errorf("变换成本表未装载（%s）", catalog.EquipmentTransformSystemPath)
	}
	_, rarity, ok := s.equipmentGradeRarity(target)
	if !ok {
		return out, fmt.Errorf("读不到目标 %d 的稀有度", target)
	}
	// 源用稀有度**名字**做键（rare/unique/legendary/epic/primeval），码值是 2/3/6/4/8。
	name, ok := catalog.TransformRarityName(rarity)
	if !ok {
		return out, fmt.Errorf("稀有度 %d 在变换表里没有档位（源只列 rare/unique/legendary/epic/primeval）", rarity)
	}
	opt, ok := s.Transform.Payment(chain, name, payOption)
	if !ok {
		return out, fmt.Errorf("变换表里 %s 没有付款方式 %d", name, payOption)
	}
	out.Option, out.Gold = opt.Number, opt.Gold
	for _, m := range opt.Items {
		mat := MaterialCost{Template: m.Template, Count: m.Count}
		if _, isAccount := AccountMaterialSlot(m.Template); isAccount {
			out.AccountMats = append(out.AccountMats, mat)
			continue
		}
		out.BagMats = append(out.BagMats, mat)
	}
	return out, nil
}

// TransformCostLine 是「装备变换」里**单件**的成本预览。
type TransformCostLine struct {
	Slot        uint32         `json:"slot"`
	Template    uint32         `json:"template"`
	Rarity      int32          `json:"rarity"`
	RarityName  string         `json:"rarity_name,omitempty"`
	Gold        uint32         `json:"gold"`
	AccountMats []MaterialCost `json:"account_mats,omitempty"`
	BagMats     []MaterialCost `json:"bag_mats,omitempty"`
	// Unpriced 表示源 `[create cost]` 里**没有这件**（`[item index]` 精确查找未命中），
	// 与生成路径 `EquipmentCraftPlan.Unpriced` 同一个按件谓词。教学轨道内据此免单。
	Unpriced bool `json:"unpriced,omitempty"`
	// Problem 记录算不出成本的原因（读不到稀有度 / 变换表没这一档 / 没这支付法）。
	Problem string `json:"problem,omitempty"`
}

// InspectTransformCost 只回答「这次变换要付什么」，**不读存档之外的东西、不扣料、不改状态**。
//
// 存在的理由（2026-10-09）：CMD2259 的应答只有「窗口 + 方法」两个字节，**没有失败与禁用
// 语义**，变换按钮能不能点是客户端拿自己的成本×库存判的。所以当出现「客户端没禁用、
// 服务端却以材料不足拒绝」时，服务端**无法**通知客户端，只能先把**自己算的成本**原样摊开
// 到日志里，拿去和界面上显示的数字对账，才能判定是按件还是按套、以及魂的种类对不对。
//
// ★ 纯诊断：不调 PlanEquipmentTransform、不做账本守卫、不碰事务 —— 预览的结果**不等于**
// 真正执行时会扣多少（真正执行还会逐件跳过未登记/旧件回不了图鉴的条目）。
func (s *ItemService) InspectTransformCost(slots, templates []uint32, payOption int) []TransformCostLine {
	n := len(slots)
	if len(templates) < n {
		n = len(templates)
	}
	out := make([]TransformCostLine, 0, n)
	for i := 0; i < n; i++ {
		line := TransformCostLine{Slot: slots[i], Template: templates[i]}
		target := templates[i]
		if target == 0 || target == 0xFFFFFFFF {
			continue
		}
		_, rarity, ok := s.equipmentGradeRarity(target)
		if ok {
			line.Rarity = rarity
			if name, known := catalog.TransformRarityName(rarity); known {
				line.RarityName = name
			}
		}
		if s.CreateCost != nil {
			_, exact := s.CreateCost.GroupFor(target)
			line.Unpriced = !exact
		}
		pay, e := s.transformPayment(catalog.TransformChainEquipment, target, payOption)
		if e != nil {
			line.Problem = e.Error()
			out = append(out, line)
			continue
		}
		line.Gold, line.AccountMats, line.BagMats = pay.Gold, pay.AccountMats, pay.BagMats
		out = append(out, line)
	}
	return out
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
	Boxes          *BoxCatalog
	// WearRules maps a requested body slot to equipment still in the bag.
	// A nil map retains the existing equipped-only lookup.
	WearRules WearRules
	// A nil Journal skips registration; a nil CreateCost leaves crafting
	// at its existing window-only step without changing saved items.
	Journal    *catalog.EquipmentJournalRules
	CreateCost *catalog.EquipmentCreateCost
	// Transform 是三条变换链的费用/返还表（装备 2259 + 晶体 2381 共用）。
	// nil = 算不出成本 ⇒ 变换拒绝执行，绝不静默改成免费。
	Transform *catalog.EquipmentTransformSystem
	// Points 是逐件「套装/誓约积分」表（setpointinfo.cos / oathpointinfo.cos +
	// equipmentgrouping.etc 的能力组映射）。nil = 算不出积分 ⇒ 不推 NOTI2634
	// （客户端保持原值），绝不发 0 冒充。
	Points *catalog.PointRules
}

// PartSetIndexes 返回每个模板的套装号（`.equ` 的 `[part set index]`），
// 套装积分按它归属；没有该字段的模板记 -1。
//
// 源里**根本没有这件模板**时同样记 -1（并交由调用方决定是否出声）：套装积分是逐件相加的，
// 一件查不到只该让这一件 0 分，不该把整个角色的积分数抹成"算不出"。读源**出错**
// （源已关闭、文件损坏）仍然如实返回错误，不吞。
//
// 为什么用 `Definition`（而不是 `definitionResolved` 的 import 链）：名望侧
// `character.fame` 读的是 `[part set index]` 的**自身**值（`fameInt(d, …)` 直接取字段，
// 不跟 `[import script]`）。套装积分是"一件与另一件算不算同一套"的判据，两侧口径必须
// 一致，所以这里同样只认自身字段。
func (c *EquipmentCatalog) PartSetIndexes(ids []uint32) (map[uint32]int32, error) {
	out := make(map[uint32]int32, len(ids))
	if c == nil {
		for _, id := range ids {
			out[id] = -1
		}
		return out, nil
	}
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, done := out[id]; done {
			continue
		}
		d, err := c.Definition(id)
		if err != nil {
			// 源里没有这件模板、或读不出来 ⇒ 这一件不归属任何套装。
			// 聚合是逐件相加的，所以这里不给整批报错。
			out[id] = -1
			continue
		}
		index := int32(-1)
		if cells := d.Fields["[part set index]"]; len(cells) > 0 {
			index = cells[0].Value
		}
		out[id] = index
	}
	return out, nil
}
