package loot

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strings"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

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

func (s *Service) PlanEquipmentTransform(role Role, slots, templates []uint32, payOption int) (EquipmentTransformPlan, error) {
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

	bag, e := inventory.ReadBag(role.State)
	if e != nil {
		return EquipmentTransformPlan{Receipt: result}, (e)
	}
	ledger, e := inventory.ReadEquipmentJournal(role.State)
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
func (s *Service) PrepareEquipmentTransform(current Role, accountRaw json.RawMessage, plan EquipmentTransformPlan) (json.RawMessage, json.RawMessage, EquipmentTransformReceipt, error) {
	result, plans := plan.Receipt, plan.Steps
	live, e := inventory.ReadBag(current.State)
	if e != nil {
		return nil, nil, result, e
	}
	liveLedger, e := inventory.ReadEquipmentJournal(current.State)
	if e != nil {
		return nil, nil, result, e
	}
	account, e := inventory.ReadAccountMaterials(accountRaw)
	if e != nil {
		return nil, nil, result, e
	}

	var bagMats, accountMats []inventory.MaterialCost
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
		mat := inventory.MaterialCost{Template: soul, Count: 1}
		if _, isAccount := inventory.AccountMaterialSlot(soul); isAccount {
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
	updated, e := inventory.SaveBag(current.State, replaced)
	if e != nil {
		return nil, nil, result, e
	}
	// 方案「甲」：把换下去的源装备也登记进图鉴，否则它"换出去即消失"、再也选不回来。
	ledgerNext := registerTransformedSources(s.Equipment, s.Journal, liveLedger, done)
	if updated, e = inventory.SaveEquipmentJournal(updated, ledgerNext); e != nil {
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
func (s *Service) applyTransform(bag inventory.Bag, pairs []EquipmentTransformPair) (inventory.Bag, error) {
	out := bag
	out.Worn = append([]inventory.BagEquipment(nil), bag.Worn...)
	out.Equipment = append([]inventory.BagEquipment(nil), bag.Equipment...)
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
	gear *inventory.EquipmentCatalog,
	rules *catalog.EquipmentJournalRules,
	ledger inventory.EquipmentJournal,
	pairs []EquipmentTransformPair,
) inventory.EquipmentJournal {
	for _, p := range pairs {
		if p.From == 0 || p.From == p.To {
			continue
		}
		limit, ok := inventory.JournalLimit(gear, rules, p.From)
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
func wornOf(bag inventory.Bag, slot uint16) (uint32, bool) {
	for _, w := range bag.Worn {
		if w.Slot == slot {
			return w.Template, true
		}
	}
	return 0, false
}

// equipmentKind 读装备的 `[equipment type]` 文本（如 `[weapon]`）。
func (s *Service) equipmentKind(id uint32) string {
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
func (s *Service) kindForWornSlot(slot uint16) string {
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
func (s *Service) transformSource(bag inventory.Bag, slot uint16) (uint32, uint16, bool) {
	if t, ok := wornOf(bag, slot); ok && t != 0 {
		return t, 0, true // 0 = 源在身上
	}
	kind := s.kindForWornSlot(slot)
	if kind == "" {
		return 0, 0, false
	}
	var found inventory.BagEquipment
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
func (s *Service) costGroupFor(template uint32) (catalog.CreateCostGroup, bool) {
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
func (s *Service) equipmentGradeRarity(id uint32) (int32, int32, bool) {
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
func (s *Service) transformCost(target uint32) (uint32, uint32, error) {
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
