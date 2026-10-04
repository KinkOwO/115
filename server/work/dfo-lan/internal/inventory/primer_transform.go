package inventory

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

// 装备库「誓约 / 晶体变换」（CMD2381 ENUM_CMDPACKET_PRIMER_TRANSFORM）的执行侧。
//
// 协议几何见 internal/game/protocol/primer_transform.go（IDA + 实机帧定案）；
// 费用/返还的真源是 `etc/115lvability/equipmenttransformsystem.cos` 的
// `[need primer materials]` / `[refund primer materials]`（见 catalog.EquipmentTransformSystem）。
//
// 语义（逐条给出依据）：
//
//  1. **行 ↔ 穿戴槽**：正文 `+17` 的"行 0" = **誓约核心槽 47**，`+36` 起的 11 条记录
//     = **晶体槽 36..46**（`sub_141518820`：`id = 47 (k==0) / 35+k`；`sub_14150B940` 对
//     已穿戴晶体调 `sub_14150F150(win, slot-35, item, 3, slot, 0)` 复证）。
//     ⚠️ 记录里的 `+0/+1` 是**物品所在 (space, slot)**（`sub_1415141E0` 的 `mov [rcx],r8d` /
//     `mov [rcx+4],r9d`），实机两条非空记录都是 `(space 46, slot 0)` —— 那是军械库来源，
//     不是目标槽位；目标槽位由**行号**决定，这里就按行号落槽。
//
//  2. **必须消耗一件真实来源**：目标要在**背包装备区**里有实物行、**或**在装备库登记
//     （`counts > 0`）、**或**目标槽里本来就有东西可换（就地换）。每一行都要能找出
//     **一件真实来源**，否则整行跳过；同一份请求里两行不能抢同一件（claims）。
//     ⚠️ 2026-10-04 实机缺陷：最初只把登记当"配方解锁"、目标是**新造**的 ⇒ 每次点击凭空多一件，
//     反复点把 11 个晶体槽全填满（用户："凭空生成晶石、越变越多"）。见 primerSourceKind。
//     窗口列出的只有已登记/已持有的条目（客户端文案 101038947「Not enough Oaths/Crystals
//     registered/owned.」），所以"没来源就跳过"不会误伤合法变换。
//
//  3. **成本按目标稀有度**取 `[need primer materials]`：金币或巡礼之印 ×N，**没有灵魂项**
//     （装备变换那条链才多要一件灵魂）。
//     ⚠️ **付款方式序号没有线上字段**：`sub_14150C4D0` 对正文缓冲区的**全部 22 处写入**已穷举
//     （`analysis/tmp-primer-transform/ida/pt10_log_sec3.txt`），只有
//     `+0/+13/+17/+18/+20/+24/+32/+36/+37/+39/+43/+47/+202` 这些位置 —— 与已知语义一一对应
//     （窗口字段 / 行 0 / 记录三元组 / 选择 id / 第 4 实参），**没有任何一格是付费选择**；
//     `+13` 更是窗口对象字段（构造器恒写 1）。源里两档（group 1 金币 / group 2 巡礼之印）
//     到底由谁选，本轮没有 wire 证据。
//     这里由调用方显式传入（默认 1 = 金币），并在回执里记录实际用的序号 —— 缺口与迁移条件
//     写在 docs 里（一旦定出线上字段就改从请求读，不再由调用方猜）。
//
//  4. **被换下去的那件"回原处"；返还只与"实物是否真的离开玩家"挂钩**：客户端文案 101038948
//     「Before conversion, Crystals are registered to the Armory and refunded as materials.」
//     返还表每档两行（分支 0 有料 / 分支 1 空），源里没写分支语义 ⇒ 取**有料的那一支**。
//
//     ⚠️ 2026-10-04 用户口径（守恒，见交接文档 §0.3/§0.4 与 §3 第 1 条，第二轮定案）：
//     - 来源是**背包装备区** ⇒ 目标搬上槽、被换下的那件**原位换回那一行**（不登记、不返还）；
//     - 来源是**军械库登记** ⇒ 登记 −1，被换下的那件**登记 +1 回原处**（相抵，**不返还**）；
//     只有"背包已满、旧件回不去"才算真的消耗了一件拥有物 ⇒ 才登记 + 返还材料；
//     - **就地**换掉槽内那件 ⇒ 只消耗，不登记、不返还；
//     - 槽是空的又找不到来源 ⇒ 拒绝。
//     旧口径"被换下的那件一律登记 + 返还"在 A↔B 来回切换时每轮净增一份材料/一条登记，
//     正是用户报的"凭空生成"（日志 `each round: refunds=[10415190 ×4~5]`）。
//     判据：来回切换 N 轮后，背包 + 军械库 + 穿戴的份数总量不变、返还材料为 0。
const (
	// PrimerCrystalSlotBase 是晶体槽起始穿戴槽；36..46 共 11 个（44..46 留给太初晶体）。
	PrimerCrystalSlotBase = 36
	// PrimerCrystalSlotCount 与协议里的 11 条记录一一对应。
	PrimerCrystalSlotCount = 11
	// PrimerOathSlot 是"行 0"对应的誓约核心槽。
	PrimerOathSlot = 47
)

// PrimerTransformPair 是一件成功的「晶体/誓约变换」：槽 `Slot` 上的 `From` 换成了 `To`。
//
// `Row` 是来源行：`-1` = 行 0（誓约核心槽 47），`0..10` = 第 i 条晶体记录（槽 36+i）。
type PrimerTransformPair struct {
	Row  int    `json:"row"`
	Slot uint16 `json:"slot"`
	From uint32 `json:"from,omitempty"`
	To   uint32 `json:"to"`
}

// PrimerTransformReceipt 是一次「晶体/誓约变换」的结果快照（写进 events.jsonl）。
type PrimerTransformReceipt struct {
	Source string `json:"source"`
	// Pairs 记录真正落库的替换；Skipped 记录没能变换的模板（未登记 / 非晶体 / 付不起）。
	Pairs   []PrimerTransformPair `json:"pairs,omitempty"`
	Skipped []uint32              `json:"skipped,omitempty"`
	// Materials 是本次扣掉的成本；Refunds 是返还给玩家的材料（登记旧晶体得来）。
	Materials []CraftMaterial `json:"materials,omitempty"`
	Refunds   []CraftMaterial `json:"refunds,omitempty"`
	Gold      uint32          `json:"gold,omitempty"`
	Option    int             `json:"option,omitempty"`
}

type primerTransformStep struct {
	row  int
	slot uint16
	from uint32 // 该槽原有的晶体/核心；0 = 空槽
	to   uint32
	oath bool // 行 0（誓约核心）—— 换核心要顺带刷新 2839
	// source 是**规划时定下的那件目标实物的位置**（plan 与 prepare 必须一致；
	// 事务里位置对不上就跳过该行 —— 状态被别的请求改过时宁可不动）。
	source primerSourcePos
}

// PrimerTransformPlan 是事务外算好的变换计划。
type PrimerTransformPlan struct {
	Key     string
	Steps   []primerTransformStep
	Receipt PrimerTransformReceipt
	// PayOption 是这次用的付款方式序号（源 `[need primer materials]` 的 `[group] N`）。
	PayOption int
}

// primerTransformKey 让同一次变换（同一份请求 + 同一个前置状态）只应用一次。
//
// 与 transformKey 同构：键里带**前置状态摘要**，所以"状态已经变了还在重发的同一份请求"
// 会被识别成重放；玩家合法地再换一次（状态已不同）不会被挡。键长固定、远低于 200 字节。
func primerTransformKey(slots, templates []uint32, state json.RawMessage) string {
	h := sha256.New()
	for i := range slots {
		fmt.Fprintf(h, "%d:%d,", slots[i], templates[i])
	}
	req := h.Sum(nil)
	pre := sha256.Sum256(state)
	return fmt.Sprintf("primer-transform:%x:%x", req[:8], pre[:8])
}

// primerSourceKind 说明一件目标晶体/誓约核心是**从哪儿消耗出来的**。
//
// 2026-10-04 实机缺陷（用户："凭空生成晶石、越变越多"）：本文件最初只把军械库登记
// （`journal.Counts[target] > 0`）当"配方解锁"，换上去的目标是**新造**的 ⇒ 每次点击都
// 凭空多一件，反复点就把 11 个晶体槽全填满（实机 5 次点击后槽 36..46 全满、金币 -400000）。
// 正确口径：**变换是把一件实物搬进目标槽**。所以每一行必须能找出**一件真实来源**，
// 否则整行跳过；同一份请求里两行不能抢同一件（claims）。
//
// ★ **2026-10-04 第二轮修正（用户："图鉴里的晶石被搞乱，不单纯是增加，也可能是变高等级，
// 甚至减少，没找到规律"）**：根因是**同一个动作有三套记账**（背包来源 → 不动账本、
// 军械库来源 → 动账本 ±1、就地换 → 不动账本），玩家每点一次都要先判断目标"在哪儿"，
// 结果观感就是"图鉴自己乱变"。现在**统一为一套**（详见下方 primerFindSource 的说明）：
//
//	目标在**登记表**里 ⇒ 登记 −1；被换下的那件能登记回去（含同模板换回）或背包放得下 ⇒ 登记 +1；
//	                  两者都做不到时：源表有返还料 ⇒ 返还，没有 ⇒ **这一行整条跳过**；
//	登记表没有、**背包**里有 ⇒ 那一行换成被换下的那件（账本完全不动）；
//	两者都没有、而**槽里**有 ⇒ 就地换掉（账本完全不动）；
//	都没有且槽是空的 ⇒ 拒绝。
//
// 再叠加一条铁律：**只有实物真的离开玩家才返还材料**（出得去、回得来 = 不返还）。
type primerSourceKind uint8

const (
	// primerSourceNone：没有可用的实物来源（槽也是空的）⇒ 调用方拒绝这一行。
	primerSourceNone primerSourceKind = iota
	// primerSourceArmory：登记表里那件（`counts[target]` −1，被换下的那件登记 +1）。
	primerSourceArmory
	// primerSourceBag：背包装备区的实物行（那一行换成被换下的那件）。
	primerSourceBag
	// primerSourceInPlace：槽里那件就地换掉（账本不动）。
	primerSourceInPlace
)

// primerSourcePos 是目标实物所在的位置（可比较：plan 记下、事务里比对）。
//
//	InPlace ⇒ bagSlot 无意义（0xFFFF）：槽里那件就地换掉，账本不动
//	Armory  ⇒ bagSlot == 0：登记表里那件被搬上槽（登记 −1、被换下的那件登记 +1）
//	Bag     ⇒ bagSlot = 背包装备区那一行的槽号（那一行换成被换下的那件）
type primerSourcePos struct {
	kind    primerSourceKind
	bagSlot uint16
}

// inPlacePos 是"没有实物来源"的哨兵：槽里那件就地换掉。
var inPlacePos = primerSourcePos{kind: primerSourceInPlace, bagSlot: 0xFFFF}

// samePos 比较两个来源位置。就地哨兵只看种类（它只表示"没有实物来源"）。
func samePos(a, b primerSourcePos) bool {
	if a.kind != b.kind {
		return false
	}
	if a.kind == primerSourceBag {
		return a.bagSlot == b.bagSlot
	}
	return true
}

// primerSourceClaim 记录同一份请求里已经被前面行占用的来源，防止两行共用一件实物。
type primerSourceClaim struct {
	bagRows map[uint16]bool // 背包装备区槽号 → 已占用
	armory  map[uint32]uint32
}

func newPrimerSourceClaim() *primerSourceClaim {
	return &primerSourceClaim{
		bagRows: map[uint16]bool{},
		armory:  map[uint32]uint32{},
	}
}

// primerFindSource 找一件 `target` 的实物来源。
//
// 优先级（plan 与 prepare 必须完全一致，否则事务里会判成"来源变了"而整行跳过）：
//  1. **装备库登记**里有（`counts[target] > 0`）⇒ 从登记表里搬：登记 −1、被换下的那件登记 +1；
//  2. 登记表没有、**背包装备区**有同模板行 ⇒ 那一行换成被换下的那件（账本完全不动）；
//  3. 两者都没有、**该槽已有东西**（`occupied`）⇒ 就地换掉槽内那件（账本完全不动）；
//  4. 都没有且槽是空的 ⇒ 没有来源，调用方拒绝这一行（不许凭空生成）。
//
// 为什么登记表优先：它是**唯一能让"同一件东西现在在哪"自洽的账本**（穿戴 + 登记 = 拥有总数）。
// 旧实现把"背包来源"排在前面，于是同一件东西在背包里时账本不动、在登记表里时账本 ±1 ——
// 玩家看到的就是"图鉴自己乱变"（用户 2026-10-04 报告）。见 primerSourceKind 的说明。
func primerFindSource(bag Bag, ledger EquipmentJournal, target uint32, occupied bool, claim *primerSourceClaim) (primerSourcePos, bool) {
	if target == 0 {
		return primerSourcePos{kind: primerSourceNone}, false
	}
	if claim == nil {
		claim = newPrimerSourceClaim()
	}
	if have := ledger.Counts[target]; have > claim.armory[target] {
		return primerSourcePos{kind: primerSourceArmory}, true
	}
	for _, it := range bag.Equipment {
		if it.Template != target || claim.bagRows[it.Slot] {
			continue
		}
		return primerSourcePos{kind: primerSourceBag, bagSlot: it.Slot}, true
	}
	if occupied {
		return primerSourcePos{kind: primerSourceInPlace, bagSlot: 0xFFFF}, true
	}
	return primerSourcePos{kind: primerSourceNone}, false
}

// claimSource 记下这一行占用的来源（plan 与 prepare 各跑一遍，规则必须完全一致）。
func (c *primerSourceClaim) claimSource(pos primerSourcePos, target uint32) {
	switch pos.kind {
	case primerSourceBag:
		c.bagRows[pos.bagSlot] = true
	case primerSourceArmory:
		c.armory[target]++
	}
}

// PlanPrimerTransform 计算变换计划（纯读：不扣料、不落库）。
//
// `payOption` 由调用方给出（见文件头 §3 的缺口说明）。
func (s *ItemService) PlanPrimerTransform(role Role, r protocol.PrimerTransformRequest, payOption int) (PrimerTransformPlan, error) {
	var result PrimerTransformReceipt
	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return PrimerTransformPlan{Receipt: result}, fmt.Errorf("primer transform: inventory source mismatch")
	}
	if s.Transform == nil {
		return PrimerTransformPlan{Receipt: result}, fmt.Errorf("primer transform: transform system is not loaded")
	}
	if s.Journal == nil {
		return PrimerTransformPlan{Receipt: result}, fmt.Errorf("primer transform: journal rules are not loaded")
	}
	if s.Equipment == nil {
		return PrimerTransformPlan{Receipt: result}, fmt.Errorf("primer transform: equipment catalog is not loaded")
	}
	bag, e := ReadBag(role.State)
	if e != nil {
		return PrimerTransformPlan{Receipt: result}, e
	}
	ledger, e := ReadEquipmentJournal(role.State)
	if e != nil {
		return PrimerTransformPlan{Receipt: result}, e
	}

	var steps []primerTransformStep
	var slots, templates []uint32
	var notes []string
	claim := newPrimerSourceClaim()
	// 行 0 先算（誓约核心槽 47），再算 11 条晶体记录（槽 36..46）。
	consider := func(row int, slot uint16, entry protocol.PrimerTransformEntry) {
		target := entry.Template
		kind := s.equipmentKind(target)
		if kind != "[primer]" && kind != "[oath]" {
			notes = append(notes, fmt.Sprintf("行 %d → 槽 %d: 目标 %d 不是晶体/誓约(%q)", row, slot, target, kind))
			result.Skipped = append(result.Skipped, target)
			return
		}
		if from, held := wornOf(bag, slot); held && from == target {
			// 该槽已经是目标 ⇒ 无可变换，也**不收费**。
			notes = append(notes, fmt.Sprintf("行 %d → 槽 %d: 已经是目标 %d", row, slot, target))
			return
		}
		// 来源规则（2026-10-04 用户口径：原料总数守恒 + 被换下的那件"回原处"）：
		//   ① **登记表**里有 ⇒ 登记 −1、被换下的那件登记 +1（账本总份数不变）；
		//   ② 登记表没有、**背包**里有 ⇒ 那一行换成被换下的那件（账本完全不动）；
		//   ③ 两者都没有、而**槽里**有东西 ⇒ 就地换掉（账本完全不动）；
		//   ④ 都没有且槽是空的 ⇒ 拒绝（不许凭空生成 —— 用户最初报的缺陷）。
		occupied := false
		if _, held := wornOf(bag, slot); held {
			occupied = true
		}
		src, ok := primerFindSource(bag, ledger, target, occupied, claim)
		if !ok {
			notes = append(notes, fmt.Sprintf("行 %d → 槽 %d: 目标 %d 无实物来源且该槽为空（不允许凭空生成）", row, slot, target))
			result.Skipped = append(result.Skipped, target)
			return
		}
		claim.claimSource(src, target)
		// 太初晶体（rarity 8）只能进 44..46（与 wear.go 的穿戴校验同一条源规则）：
		// 行号与槽号是从客户端抄来的，真出现越界说明对面模型变了，宁可跳过也不要写出
		// 客户端认为非法的存档（"装备库登记不上 / 变换界面卡死"就是这么来的）。
		if kind == "[primer]" {
			if _, rarity, ok := s.equipmentGradeRarity(target); ok && rarity == 8 && slot < 44 {
				notes = append(notes, fmt.Sprintf("行 %d → 槽 %d: 太初晶体 %d 只能进 44..46", row, slot, target))
				result.Skipped = append(result.Skipped, target)
				return
			}
		}
		if _, e := s.transformPayment(catalog.TransformChainPrimer, target, payOption); e != nil {
			notes = append(notes, fmt.Sprintf("行 %d → 槽 %d: 目标 %d 算不出变换成本（%v）", row, slot, target, e))
			result.Skipped = append(result.Skipped, target)
			return
		}
		from, _ := wornOf(bag, slot)
		steps = append(steps, primerTransformStep{row: row, slot: slot, from: from, to: target,
			oath: slot == PrimerOathSlot, source: src})
		slots = append(slots, uint32(slot))
		templates = append(templates, target)
	}
	if !r.Oath.Empty() {
		consider(-1, PrimerOathSlot, r.Oath)
	}
	for i, entry := range r.Entries {
		if entry.Empty() {
			continue
		}
		slot, ok := r.CrystalSlot(i)
		if !ok {
			result.Skipped = append(result.Skipped, entry.Template)
			continue
		}
		consider(i, slot, entry)
	}
	if len(steps) == 0 {
		return PrimerTransformPlan{Receipt: result}, fmt.Errorf("primer transform: nothing transformable in %d requested rows (%s)",
			len(r.Entries)+1, strings.Join(notes, "; "))
	}
	key := primerTransformKey(slots, templates, role.State)
	return PrimerTransformPlan{Key: key, Steps: steps, Receipt: result, PayOption: payOption}, nil
}

// PreparePrimerTransform 在事务里执行计划：扣成本 → 按来源把目标搬到槽上、
// 被换下的那件回原处（背包来源回背包、军械库来源回登记）→ 只有真的消耗了拥有物才返还材料。
func (s *ItemService) PreparePrimerTransform(current Role, accountRaw json.RawMessage, plan PrimerTransformPlan) (json.RawMessage, json.RawMessage, PrimerTransformReceipt, error) {
	result, steps := plan.Receipt, plan.Steps
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
	// consumedBag 是本次要**在原槽位换成被换下那件**的背包装备行：槽号 → 新模板。
	// 守恒的关键就在这张表：目标从背包搬走、被换下的那件搬回**同一个槽**，
	// 背包的份数与穿戴槽的份数都不变。
	consumedBag := map[uint16]uint32{}
	// consumedArmory 是本次要从登记表里扣掉的模板；swapOut 是被换下、要登记回去的那一件。
	// 两者在账本上**进出各一份** ⇒ 图鉴总份数不变（见 primerSourceKind）。
	consumedArmory := map[uint32]uint32{}
	swapOut := map[uint32]uint32{}
	// lost 收集"被顶掉、又回不了背包"的旧件（源表按它们的稀有度返还材料）。
	lost := map[uint32]uint32{}
	// bagHasRoom 在循环外算一次：背包容量是**这一笔事务开始前**的实况，
	// 逐行重算会受本笔前面几行的影响（同一份请求里各行的判定必须一致）。
	bagHasRoom := primerBagCanAccept(live, s.BagRules)
	var gold uint32
	option := 0
	var done []PrimerTransformPair
	claim := newPrimerSourceClaim()
	for _, step := range steps {
		liveFrom, liveHeld := wornOf(live, step.slot)
		if liveHeld && liveFrom == step.to {
			continue
		}
		// 来源规则与 plan 完全一致（两遍必须同规则）；**位置与槽内那件也要一致**：
		// 对不上说明状态被别的请求改过 ⇒ 跳过该行（宁可不动，不可凭空造，
		// 也不可按错的旧件去登记/返还）。
		if liveFrom != step.from {
			result.Skipped = append(result.Skipped, step.to)
			continue
		}
		occupied := liveHeld
		src, ok := primerFindSource(live, liveLedger, step.to, occupied, claim)
		if !ok || !samePos(src, step.source) {
			result.Skipped = append(result.Skipped, step.to)
			continue
		}
		pay, e := s.transformPayment(catalog.TransformChainPrimer, step.to, plan.PayOption)
		if e != nil {
			result.Skipped = append(result.Skipped, step.to)
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
			result.Skipped = append(result.Skipped, step.to)
			continue
		}
		// 金币也按"逐件扣"的口径判：**用剩余金币**（live.Gold - 已累计的 gold）比，
		// 而不是最后一次性比总额 —— 否则"够换便宜那件、不够换贵那件"会整批回滚，
		// 玩家看到的还是"点了没反应"。能换的照换，换不起的那件记进回执 Skipped。
		if live.Gold < gold+pay.Gold {
			result.Skipped = append(result.Skipped, step.to)
			continue
		}
		// 到这里这一行才会真正执行 ⇒ 记下它占用的来源并累计消耗。
		claim.claimSource(src, step.to)
		switch src.kind {
		case primerSourceBag:
			// 背包那一行换成被换下的那件（空槽来源时留 0 ⇒ 下面会删掉该行）。
			replace := uint32(0)
			if step.from != 0 && step.from != step.to {
				replace = step.from
			}
			consumedBag[src.bagSlot] = replace
		case primerSourceArmory:
			// **旧件必须先能安置，才允许扣目标那件**（用户："图鉴有时会少"）。
			// 安置优先"回背包"，其次"登记回登记表"（走与分解同一条 JournalLimit 判据）；
			// 两条都做不到（背包满 + 登记达上限）⇒ 这一行整条跳过：
			// 宁可"点了没反应"，也不许扣了目标却不还旧件。
			consumedArmory[step.to]++
			if step.from != 0 && step.from != step.to {
				switch {
				case primerBagHasTemplate(live, step.from):
					// 背包里本来就有同模板的一件 ⇒ 那件顶上来即可，什么都不用登记。
				case primerCanRegister(s.Equipment, s.Journal, liveLedger, step.from):
					swapOut[step.from]++
				case bagHasRoom:
					swapOut[step.from]++
				default:
					// 旧件既登记不回去、背包又满。这时**先看源表这一档有没有返还料**：
					//  - 有 ⇒ 那件实物确实离手了，计入 lost，由调用方按源表返还；
					//  - 没有（源里是 `1 0` 空分支，如 epic/primeval）⇒ **这一行整条不做**：
					//    既回不到玩家手里、又没有补偿，就绝不能执行（宁可"点了没反应"）。
					if _, e := s.primerRefund(step.from); e != nil {
						consumedArmory[step.to]--
						result.Skipped = append(result.Skipped, step.to)
						continue
					}
					swapOut[step.from]++
					lost[step.from]++
				}
			}
		}
		accountMats = append(accountMats, pay.AccountMats...)
		bagMats = append(bagMats, pay.BagMats...)
		option = pay.Option
		gold += pay.Gold
		done = append(done, PrimerTransformPair{Row: step.row, Slot: step.slot, From: step.from, To: step.to})
	}
	if len(done) == 0 {
		return nil, nil, result, fmt.Errorf("primer transform: none of the %d steps is executable", len(steps))
	}

	paid, e := live.PayMaterials(bagMats, 1)
	if e != nil {
		return nil, nil, result, e
	}
	if gold > paid.Gold {
		return nil, nil, result, fmt.Errorf("primer transform: need %d gold, have %d", gold, paid.Gold)
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
	// **账本只在这一步动**，而且只有"目标来自登记表"的行才会动：
	// 目标登记 −1、被换下的那件登记 +1 ⇒ 总份数恒定（见 primerSourceKind 的说明）。
	ledgerNext, lostNow, e := applyPrimerJournal(s.Equipment, s.Journal, liveLedger, consumedArmory, swapOut, lost)
	if e != nil {
		return nil, nil, result, e
	}
	// 现场核对：**登记表的份数变化必须恰好等于"消耗 − 登记回去"**。
	// 差额只允许来自"目标被搬到了身上"（那些件不再登记在册），不允许凭空多/少。
	// 真实存档里也可能是旧版本留下的不一致（用户 2026-10-04 报"图鉴被搞乱"），
	// 所以这里不拒绝，只把差异喊出来 —— 一旦对不上，日志里能直接看到是哪几个模板
	// （AGENTS §0.3：不靠猜）。**稳定复现这一行就说明还有没被覆盖的记账路径**。
	if delta := int64(journalTotal(liveLedger)) - int64(journalTotal(ledgerNext)); delta != 0 {
		moved := int64(0)
		for template, used := range consumedArmory {
			moved += int64(used)
			moved -= int64(swapOut[template])
			moved -= int64(lost[template])
		}
		if delta != moved {
			log.Printf("primer transform: 图鉴份数变化异常 %d→%d（差 %d，预期 %d）consumed=%v registered=%v lost=%v（请连同本条一起上报）",
				journalTotal(liveLedger), journalTotal(ledgerNext), delta, moved, consumedArmory, swapOut, lost)
		}
	}
	// **有实物真的离手了吗？** 只有一种情形：目标来自登记表（登记被消耗），而被顶掉的那件
	// **连背包都放不下** —— 那件实物回不来。这时按源表返还材料。
	//
	// ⚠️ 但**源表那一档没有返还料时（例如 `epic`/`primeval` 的 `1 0` 空分支）绝不能让它凭空消失**：
	// 把它登记回登记表（只受上限约束）——玩家在登记表里还看得见那件，也不会被白吃。
	// 对照（都不返还）：空槽来源（只是搬到身上）、旧件回了登记表或背包（进出相抵 / 原位换回）、
	// 就地换（根本没动任何容器）。旧实现每轮无条件返还 ——
	// 于是 A↔B 来回切换就能无限刷材料（用户 2026-10-04 报告）。
	var refunds []MaterialCost
	for template, count := range lostNow {
		if count == 0 {
			continue
		}
		items, e := s.primerRefund(template)
		if e != nil || len(items) == 0 {
			// 调用方（plan/prepare）已保证"没有返还料的档不会被记成离手" ⇒ 走到这里说明
			// 口径被破坏。宁可整笔报错，也不要悄悄吃掉玩家一件实物。
			return nil, nil, result, fmt.Errorf("primer transform: %d 被记为离手却在源表里没有返还料", template)
		}
		for _, m := range items {
			refunds = append(refunds, MaterialCost{Template: m.Template, Count: m.Count * count})
		}
	}
	// 返还材料：账号共享材料走账号仓，其余进背包（与分解产物同一套分仓判据）。
	for _, m := range refunds {
		if _, isAccount := AccountMaterialSlot(m.Template); isAccount {
			nxt, _, e := out.Add(m.Template, m.Count)
			if e != nil {
				return nil, nil, result, e
			}
			out = nxt
			continue
		}
		next, _, e := paid.Add(s.Catalog, s.BagRules, m.Template, m.Count)
		if e != nil {
			return nil, nil, result, e
		}
		paid = next
	}
	// 再扣掉来源实物（"变换 = 把那件实物从军械库/背包搬到目标槽"）：
	// 背包那一行**换成被换下的那件**（空槽则整行删掉），军械库登记按份数减。
	paid, e = consumePrimerBagSources(paid, consumedBag)
	if e != nil {
		return nil, nil, result, e
	}
	equipped, e := s.equipPrimerCrystals(paid, done)
	if e != nil {
		return nil, nil, result, e
	}
	updated, e := SaveBag(current.State, equipped)
	if e != nil {
		return nil, nil, result, e
	}
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
	for _, m := range refunds {
		_, isAccount := AccountMaterialSlot(m.Template)
		result.Refunds = append(result.Refunds, CraftMaterial{Template: m.Template, Amount: m.Count, FromAccount: isAccount})
	}
	return updated, accountNext, result, nil
}

// primerRefund 给出「换下去这件」按源表返还的材料。
//
// 源 `[refund primer materials]` 每档两行：分支 0 有料、分支 1 空（源里没有分支语义标签）。
// 这里取**有料的那一支**（0），读不到就报错 —— 调用方据此跳过整件，绝不静默吞掉。
func (s *ItemService) primerRefund(from uint32) ([]MaterialCost, error) {
	if s.Transform == nil {
		return nil, fmt.Errorf("变换成本表未装载")
	}
	_, rarity, ok := s.equipmentGradeRarity(from)
	if !ok {
		return nil, fmt.Errorf("读不到 %d 的稀有度", from)
	}
	name, ok := catalog.TransformRarityName(rarity)
	if !ok {
		return nil, fmt.Errorf("稀有度 %d 在变换表里没有档位", rarity)
	}
	items, ok := s.Transform.RefundFor(catalog.TransformChainPrimer, name, 0)
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("变换表里 %s 没有返还材料", name)
	}
	out := make([]MaterialCost, 0, len(items))
	for _, m := range items {
		out = append(out, MaterialCost{Template: m.Template, Count: m.Count})
	}
	return out, nil
}

// consumePrimerBagSources 把"目标从那件搬走"的背包装备行**在原槽位换成被换下的那件**。
//
// 语义（用户口径 §0.4「被换下的那件回原处」）：目标从背包那一行被搬进穿戴槽，
// 空出来的**同一个槽位**立刻由被换下的那件顶上 ⇒ 背包份数不变、穿戴槽份数不变，
// 只是两件实物换了位置。`replace == 0`（槽原来是空的、没有东西可换下）则整行删掉。
//
// 为什么不是"删掉目标 + 另找空位塞进被换下那件"：那样会用到别的槽位、也可能因背包满
// 而失败；而"原槽位替换"天然成功、且与"回原处"口径逐字一致。
func consumePrimerBagSources(bag Bag, replace map[uint16]uint32) (Bag, error) {
	if len(replace) == 0 {
		return bag, nil
	}
	out := bag
	out.Equipment = make([]BagEquipment, 0, len(bag.Equipment))
	replaced := 0
	for _, row := range bag.Equipment {
		want, ok := replace[row.Slot]
		if !ok {
			out.Equipment = append(out.Equipment, row)
			continue
		}
		replaced++
		if want == 0 {
			continue // 空槽来源：没有东西换下，整行消耗掉
		}
		next := row
		next.Template = want
		// ⚠️ `Record` 是**实例**行（模板号在 `[2:6)`、打造/调适在别处），它属于被搬走的
		// 那一件；这里换的是"背包里放着哪一件实物"，所以整体清掉。留着会直接被
		// `ValidateRecord` 判成 `equipment instance template mismatch`（2026-09-30 的教训：
		// 那一错会让**整个角色读不出来**）。被换下的那件此刻在穿戴槽里，它的行不受影响。
		// 若要让它带着"这一行原来的调适/附魔"回到背包，需要另开一单取证客户端语义。
		next.Record = nil
		out.Equipment = append(out.Equipment, next)
	}
	// 事务内复核：计划里占用的来源行必须真的都还在（并发改动 ⇒ 整笔回滚，宁可不动）。
	if replaced != len(replace) {
		return bag, fmt.Errorf("primer transform: 背包装备来源行在事务里少了（计划 %d 行，实际 %d 行）",
			len(replace), replaced)
	}
	return out, nil
}

// journalTotal 数一遍账本里的总份数（守恒核对用）。
func journalTotal(j EquipmentJournal) uint32 {
	var total uint32
	for _, n := range j.Counts {
		total += n
	}
	return total
}

// primerBagHasTemplate 报告背包装备区里有没有该模板的实物行（有 ⇒ 那件可以"顶上"）。
func primerBagHasTemplate(bag Bag, template uint32) bool {
	for _, row := range bag.Equipment {
		if row.Template == template {
			return true
		}
	}
	return false
}

// primerCanRegister 报告"这件换下来的旧件能不能登记回装备库"
// （走与 CMD26 分解**同一条** `JournalLimit` 判据：资格 + 上限）。
func primerCanRegister(gear *EquipmentCatalog, rules *catalog.EquipmentJournalRules, ledger EquipmentJournal, template uint32) bool {
	limit, ok := JournalLimit(gear, rules, template)
	if !ok {
		return false
	}
	return ledger.Counts[template] < limit
}

// primerBagCanAccept 报告背包装备区还能不能放下"被换下的那件"。
//
// 判据与 `Bag.AddEquipment` 同一套（0 与 1 不是装备槽；已被 Items/Equipment 占用的槽号不能复用）。
// 规则表没给区间时无法证明"放不下" ⇒ 一律按"放得下"处理：**不轻易判成实物被吃掉**，
// 免得在没有证据的情况下给玩家凭空发返还材料。
func primerBagCanAccept(bag Bag, rules BagRules) bool {
	slots := rules.EquipmentSlots
	if slots[0] == 0 || slots[0] > slots[1] {
		return true
	}
	occupied := map[uint16]bool{}
	for _, row := range bag.Items {
		occupied[row.Slot] = true
	}
	for _, row := range bag.Equipment {
		occupied[row.Slot] = true
	}
	for n := uint32(slots[0]); n <= uint32(slots[1]); n++ {
		if !occupied[uint16(n)] {
			return true
		}
	}
	return false
}

// applyPrimerJournal 把一次变换对**装备库登记**的影响一次算清，并保证三条纪律：
//
//	① `consume`（目标，从登记表里取）各减指定份数，下限 0，且 **0 值条目保留**
//	   （客户端用"已登记但 0 份"与"从未登记"区分状态）；
//	② `register`（被换下那件要回家）**先减后加，不拿旧值去撞上限**：
//	   先做消费再判上限，就能正确处理"誓约核心在册 1 份、上限 1、又被换回登记表"
//	   这种同模板换回（旧实现先判上限 ⇒ 登记被拒 ⇒ **只扣不还**，图鉴就少了）；
//	③ 真的放不下（`blocked`，即登记达上限、背包也满）的那几件记进 `lost`，
//	   由调用方按源表返还材料 —— **这是唯一的返还路径**（用户口径：只有实物真的离手才返还）。
//
// 返回值：新账本、**真的离手的那些件**（调用方据此返还材料；空表示没有实物离手）、错误。
func applyPrimerJournal(
	gear *EquipmentCatalog,
	rules *catalog.EquipmentJournalRules,
	ledger EquipmentJournal,
	consume, register, lost map[uint32]uint32,
) (EquipmentJournal, map[uint32]uint32, error) {
	if len(consume) == 0 && len(register) == 0 {
		return ledger, nil, nil
	}
	out := ledger.clone()
	if out.Counts == nil {
		out.Counts = map[uint32]uint32{}
	}
	// ① 先消费（此时用**未加过的**旧值算下限，语义与 consumePrimerArmory 一致）。
	for template, used := range consume {
		have := out.Counts[template]
		if used >= have {
			out.Counts[template] = 0
			continue
		}
		out.Counts[template] = have - used
	}
	// ② 再登记（消费之后才判上限）。
	// ⚠️ `lostNow` 必须在登记循环**之前**定下来：循环会把 `lost` 里已经落账的部分减掉，
	// 循环之后再取就会被清空（这一步踩过一次）。
	lostNow := make(map[uint32]uint32, len(lost))
	for template, count := range lost {
		if count > 0 {
			lostNow[template] = count
		}
	}
	for template, amount := range register {
		if template == 0 || amount == 0 {
			continue
		}
		limit, ok := JournalLimit(gear, rules, template)
		if !ok {
			continue
		}
		room := uint32(0)
		if cap := out.Counts[template]; cap < limit {
			room = limit - cap
		}
		add := amount
		if add > room {
			add = room
		}
		if add > 0 {
			next, _, e := out.Add(template, add, limit)
			if e != nil {
				return ledger, nil, e
			}
			out = next
		}
		if rest := amount - add; rest > 0 {
			// 剩下的登记不进去：这些件的着落由调用方的 `lost` 决定（背包放得下就不会走到这）。
			if lost[template] < rest {
				// 调用方没把它记为"离手" ⇒ 说明它其实能回背包。这里不静默吞掉，
				// 而是明确报错，让上层看见口径不一致（宁可不动，不可写出解释不了的账本）。
				return ledger, nil, fmt.Errorf(
					"primer transform: 模板 %d 登记不进去（上限 %d、在册 %d）却又没有被记为离手",
					template, limit, out.Counts[template])
			}
			lost[template] -= rest
		}
	}
	return out, lostNow, nil
}

// primerRefundsFor 给出"这些被换下的件"按源表该返还的材料。
//
// ⚠️ 调用方必须**先证明这些件真的没能回到玩家手里**（登记表被消耗到 0、且背包放不下）——
// 本函数只负责按源表算料，不做守恒判断。源里每一档两行（`0 1 10415190 N` 有料 /
// `1 0` 空），这里取**有料那一支**；取不到（那一档本来就不返还）只记日志、不报错。
func (s *ItemService) primerRefundsFor(swapOut map[uint32]uint32) []MaterialCost {
	var out []MaterialCost
	for template, count := range swapOut {
		if template == 0 || count == 0 {
			continue
		}
		items, e := s.primerRefund(template)
		if e != nil {
			// 源表这一档没有返还料（分支 1 空）⇒ 没有可返还的，不算错。
			log.Printf("primer transform: %d 这一档没有返还材料，跳过（%v）", template, e)
			continue
		}
		for _, m := range items {
			out = append(out, MaterialCost{Template: m.Template, Count: m.Count * count})
		}
	}
	return out
}

// equipPrimerCrystals 把目标晶体/誓约核心写进各自的穿戴槽。
//
// 与 applyTransform 同一纪律：只改 `Template` / `Durability`，其余字段（`Record` 里的
// 调适/打造）原样保留；**但 `Record[2:6]` 也存着模板号**，`ValidateRecord` 会断言它等于
// `Template`（不等则整个角色读不出来 —— 2026-09-30 那次的教训），所以必须同步重写。
// 空槽新建一条（无 `Record` 也合法：`EquipmentRow` 只在长度匹配时套用，空行不校验）。
func (s *ItemService) equipPrimerCrystals(bag Bag, pairs []PrimerTransformPair) (Bag, error) {
	out := bag
	out.Worn = append([]BagEquipment(nil), bag.Worn...)
	idx := map[uint16]int{}
	for i, w := range out.Worn {
		idx[w.Slot] = i
	}
	for _, p := range pairs {
		d, e := s.Equipment.Reward(p.To)
		if e != nil {
			return bag, e
		}
		if i, ok := idx[p.Slot]; ok {
			out.Worn[i].Template = p.To
			out.Worn[i].Durability = d
			if len(out.Worn[i].Record) == protocol.CurrentItemRecordSize {
				rec := append([]byte(nil), out.Worn[i].Record...)
				binary.LittleEndian.PutUint32(rec[2:], p.To)
				out.Worn[i].Record = rec
			}
			continue
		}
		out.Worn = append(out.Worn, BagEquipment{Slot: p.Slot, Template: p.To, Durability: d})
		idx[p.Slot] = len(out.Worn) - 1
	}
	return out, nil
}
