package inventory

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
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
//  2. **必须消耗一件真实来源**：目标要在装备库登记（`counts > 0`）**或**在背包 / 另一个穿戴槽里，
//     并且这次变换会**把那件实物消耗掉**（登记 -1 / 背包删 1 / 来源槽清空）。
//     ⚠️ 2026-10-04 实机缺陷：最初只把登记当"配方解锁"、目标是**新造**的 ⇒ 每次点击凭空多一件，
//     反复点把 11 个晶体槽全填满（用户："凭空生成晶石、越变越多"）。见 primerSourceKind。
//     窗口列出的只有已登记/已持有的条目（客户端文案 101038947「Not enough Oaths/Crystals
//     registered/owned.」），所以"没来源就跳过"不会误伤合法变换。
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
//  4. **被换下去的那件先登记、再按 `[refund primer materials]` 返还**：客户端文案 101038948
//     「Before conversion, Crystals are registered to the Armory and refunded as materials.」
//     返还表每档两行（分支 0 有料 / 分支 1 空），源里没写分支语义 ⇒ 取**有料的那一支**。
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
	row   int
	slot  uint16
	from  uint32 // 该槽原有的晶体/核心；0 = 空槽
	to    uint32
	oath  bool // 行 0（誓约核心）—— 换核心要顺带刷新 2839
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
// 正确口径：**变换是把一件实物从"军械库 / 背包"搬到目标槽**（客户端文案 101038948
// 「Before conversion, Crystals are registered to the Armory and refunded as materials.」
// 说的是"被换下去的那件会被登记并返还材料"，不是"目标可以凭空造"）。所以每一行必须
// 能找出**一件真实来源**，否则整行跳过；同一份请求里两行不能抢同一件（claims）。
//
// ⚠️ **来源只取"军械库登记"与"背包"，不取另一个穿戴槽**（2026-10-04 收紧）：
// 把已穿戴的晶体"搬到别的槽"既不是变换，而且**会变成刷材料的漏洞** ——
// 搬位会顶掉目标槽那件 ⇒ 那件按源表"登记 + 返还材料"，于是两块晶体来回搬就能
// 无限刷返还材料（每次只花金币）。所以只认"你拥有但没穿上"的实物。
// 被换下去的那件（`from`）仍然可以是穿戴中的，这正是客户端文案里
// 「Crystals in the Armory, the inventory, or equipped slots」的后半句。
type primerSourceKind uint8

const (
	primerSourceArmory primerSourceKind = iota + 1 // 军械库登记：`counts[target] - 1`
	primerSourceBag                                // 背包物品/背包装备列表：删掉 1 件
)

type primerSource struct {
	Kind primerSourceKind
}

// primerSourceClaim 记录同一份请求里已经被前面行占用的来源，防止两行共用一件实物。
type primerSourceClaim struct {
	items  map[uint16]int // Bag.Items 下标 → 已占用件数
	equips map[uint16]int // Bag.Equipment 下标 → 已占用件数
	armory map[uint32]uint32
}

func newPrimerSourceClaim() *primerSourceClaim {
	return &primerSourceClaim{
		items:  map[uint16]int{},
		equips: map[uint16]int{},
		armory: map[uint32]uint32{},
	}
}

// primerFindSource 找一件 `target` 的实物来源（顺序：背包 → 军械库登记）。
func primerFindSource(bag Bag, ledger EquipmentJournal, target uint32, claim *primerSourceClaim) (primerSource, bool) {
	if target == 0 {
		return primerSource{}, false
	}
	if claim == nil {
		claim = newPrimerSourceClaim()
	}
	for _, it := range bag.Items {
		if it.Template != target || it.Amount == 0 {
			continue
		}
		if claim.items[it.Slot] > 0 {
			continue
		}
		return primerSource{Kind: primerSourceBag}, true
	}
	for _, it := range bag.Equipment {
		if it.Template != target {
			continue
		}
		if claim.equips[it.Slot] > 0 {
			continue
		}
		return primerSource{Kind: primerSourceBag}, true
	}
	if have := ledger.Counts[target]; have > claim.armory[target] {
		return primerSource{Kind: primerSourceArmory}, true
	}
	return primerSource{}, false
}

// claimSource 记下这一行占用的来源（plan 与 prepare 各跑一遍，规则必须完全一致）。
func (c *primerSourceClaim) claimSource(src primerSource, bag Bag, target uint32) {
	switch src.Kind {
	case primerSourceBag:
		for _, it := range bag.Items {
			if it.Template == target && c.items[it.Slot] == 0 {
				c.items[it.Slot]++
				return
			}
		}
		for _, it := range bag.Equipment {
			if it.Template == target && c.equips[it.Slot] == 0 {
				c.equips[it.Slot]++
				return
			}
		}
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
		// **必须有一件真实来源**：军械库登记 / 背包 / 其它穿戴槽（见 primerSourceKind 的教训）。
		src, ok := primerFindSource(bag, ledger, target, claim)
		if !ok {
			notes = append(notes, fmt.Sprintf("行 %d → 槽 %d: 目标 %d 没有可消耗的实物（军械库登记/背包/其它槽都没有）", row, slot, target))
			result.Skipped = append(result.Skipped, target)
			return
		}
		claim.claimSource(src, bag, target)
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
		steps = append(steps, primerTransformStep{row: row, slot: slot, from: from, to: target, oath: slot == PrimerOathSlot})
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

// PreparePrimerTransform 在事务里执行计划：扣成本 → 登记并返还被换下去的那件 → 换上新件。
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
	var refundPairs []EquipmentTransformPair
	var refunds []MaterialCost
	var gold uint32
	option := 0
	var done []PrimerTransformPair
	// 本次请求真正消耗掉的来源（见 primerSourceKind）：军械库登记要减、背包要删。
	consumedBag := map[uint32]int{}
	consumedArmory := map[uint32]uint32{}
	claim := newPrimerSourceClaim()
	for _, step := range steps {
		if from, held := wornOf(live, step.slot); held && from == step.to {
			continue
		}
		// **必须先有一件真实来源**（2026-10-04 缺陷：只查登记就换，等于凭空造晶体）。
		src, ok := primerFindSource(live, liveLedger, step.to, claim)
		if !ok {
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
		// 被换下去的那一件：先算返还（按它的稀有度取 `[refund primer materials]` 有料那一支）。
		if step.from != 0 && step.from != step.to {
			items, e := s.primerRefund(step.from)
			if e != nil {
				// 读不到返还口径 ⇒ **整件跳过**，不能把玩家的晶体吃掉还不返还。
				result.Skipped = append(result.Skipped, step.to)
				continue
			}
			refundPairs = append(refundPairs, EquipmentTransformPair{
				Slot: step.slot, From: step.from, To: step.to, Group: 0,
			})
			refunds = append(refunds, items...)
		}
		// 到这里这一行才会真正执行 ⇒ 记下它占用的来源并累计消耗。
		claim.claimSource(src, live, step.to)
		switch src.Kind {
		case primerSourceBag:
			consumedBag[step.to]++
		case primerSourceArmory:
			consumedArmory[step.to]++
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
	// **先扣掉来源实物**（"变换 = 把那件实物从军械库/背包搬到目标槽"）：背包要删、登记要减。
	paid, e = removePrimerBagItems(paid, consumedBag)
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
	// 被换下去的晶体登记进装备库（与 CMD26 分解同一条判据；达上限只跳过、不拒绝变换）。
	ledgerNext := registerTransformedSources(s.Equipment, s.Journal, liveLedger, refundPairs)
	// 再减掉本次消耗掉的军械库登记（登记值就是"还有几件实物躺在军械库"）。
	ledgerNext = consumePrimerArmory(ledgerNext, consumedArmory)
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

// removePrimerBagItems 按模板删掉指定件数的背包实物（先背包物品，再背包装备列表）。
func removePrimerBagItems(bag Bag, counts map[uint32]int) (Bag, error) {
	if len(counts) == 0 {
		return bag, nil
	}
	out := bag
	out.Items = append([]BagItem(nil), bag.Items...)
	out.Equipment = append([]BagEquipment(nil), bag.Equipment...)
	for template, want := range counts {
		for want > 0 {
			found := false
			for i := range out.Items {
				if out.Items[i].Template != template || out.Items[i].Amount == 0 {
					continue
				}
				if out.Items[i].Amount > 1 {
					out.Items[i].Amount--
				} else {
					out.Items = append(out.Items[:i], out.Items[i+1:]...)
				}
				want--
				found = true
				break
			}
			if found {
				continue
			}
			for i := range out.Equipment {
				if out.Equipment[i].Template != template {
					continue
				}
				out.Equipment = append(out.Equipment[:i], out.Equipment[i+1:]...)
				want--
				found = true
				break
			}
			if !found {
				// 计划阶段找到过、事务里却没了 ⇒ 存档被并发改过，整笔回滚（宁可不动，不可凭空造）。
				return bag, fmt.Errorf("primer transform: 背包里的 %d 在事务里消失了", template)
			}
		}
	}
	return out, nil
}

// consumePrimerArmory 把本次消耗掉的军械库登记减掉（下限 0；0 值条目保留 —— 客户端用它记"见过"）。
func consumePrimerArmory(journal EquipmentJournal, counts map[uint32]uint32) EquipmentJournal {
	if len(counts) == 0 {
		return journal
	}
	out := journal
	out.Counts = make(map[uint32]uint32, len(journal.Counts))
	for k, v := range journal.Counts {
		out.Counts[k] = v
	}
	for template, used := range counts {
		have := out.Counts[template]
		if used >= have {
			out.Counts[template] = 0
			continue
		}
		out.Counts[template] = have - used
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
