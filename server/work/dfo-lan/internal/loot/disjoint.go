package loot

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"sort"
)

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
// 上限判据一律走 `inventory.JournalLimit`（`minimum level == 115` ∧ rarity 在集合内，
// 再按 `[equipment type]` 回落/收紧）——**不在这里另立第二处口径**：同一条规则两个答案是
// 另一棵树踩过的坑（登记在 A 处拒绝、在 B 处放行的"黑牙"）。
func journalRegistrations(
	ledger inventory.EquipmentJournal,
	bySlot map[uint16]uint32,
	deletedSlots []uint16,
	equipment inventory.EquipmentDefinitioner,
	rules *catalog.EquipmentJournalRules,
) (inventory.EquipmentJournal, []JournalRegistration, []JournalSkip, error) {
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
		limit, ok := inventory.JournalLimit(equipment, rules, template)
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

func (s *Service) Disjoint(
	ctx context.Context,
	role storage.Character,
	r protocol.DisjointItemRequest,
) (storage.Character, DisjointReceipt, bool, error) {
	var result DisjointReceipt
	fail := func(e error) (storage.Character, DisjointReceipt, bool, error) {
		return role, result, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("inventory source mismatch"))
	}
	if len(r.Items) == 0 {
		return fail(fmt.Errorf("empty disjoint request"))
	}

	slots := make([]uint16, len(r.Items))
	for i, item := range r.Items {
		slots[i] = item.Slot
	}
	// 收录判据要按接口注入（nil = 规则表没装 ⇒ 只走普通分解）。
	// 不能直接塞 `*EquipmentCatalog`：那是个带类型的 nil，进了接口就不是 nil 了。
	var defs inventory.EquipmentDefinitioner
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
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, s.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
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
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			// 收录与"扣装备 / 发材料"在**同一个 apply 回调**里 ⇒ 同生共死，不存在
			// "装备扣了、收录没写"或"收录写了、材料没发"的半状态。
			ledger, e := inventory.ReadEquipmentJournal(updated)
			if e != nil {
				return nil, nil, e
			}
			ledger, added, skipped, e := journalRegistrations(ledger, bySlot, res.DeletedSlots, defs, s.Journal)
			if e != nil {
				return nil, nil, e
			}
			if len(added) > 0 {
				raw, e := inventory.SaveEquipmentJournal(updated, ledger)
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
				Source:         s.Catalog.Source.Checksum,
				JournalAdded:   added,
				JournalSkipped: skipped,
			}
			receipt, e := json.Marshal(result)
			return updated, receipt, e
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &result); e != nil {
		return fail(e)
	}
	if result.Source != s.Catalog.Source.Checksum || len(result.DeletedSlots) != len(slots) {
		return fail(fmt.Errorf("disjoint receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, result, applied, nil
}
