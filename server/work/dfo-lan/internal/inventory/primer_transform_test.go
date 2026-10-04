package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 装备库「誓约 / 晶体变换」（CMD2381）的执行侧用例。
//
// 真源链：`etc/115lvability/equipmenttransformsystem.cos` → catalog.EquipmentTransformSystem
// → ItemService.transformPayment / primerRefund → 本文件。费用表用与真实源同形的合成表
// （真实源由 catalog 包的真实源用例钉住），所以这里既验证「按源扣」，也验证「源变则结果变」。

const primerTransformPay = 1 // 源 `[need primer materials]` 的 `[group] 1` = 金币

func primerTransformService(t *testing.T) (*ItemService, catalog.LootCatalog) {
	t.Helper()
	cat, e := catalog.LoadLoot(testfixture.LootLevel150Path(t))
	if e != nil {
		t.Fatalf("load loot catalog: %v", e)
	}
	gear, e := transformEquipmentCatalog(t)
	if e != nil {
		t.Fatalf("load equipment catalog: %v", e)
	}
	rules, e := catalog.LoadEquipmentJournalRules("../../configs/equipment-journal.generated.json", cat.Source.Checksum)
	if e != nil {
		t.Fatalf("load journal rules: %v", e)
	}
	return &ItemService{
		Catalog:   cat,
		Equipment: gear,
		Journal:   &rules,
		Transform: transformSystemFixture(t),
		BagRules:  BagRules{MissingStackLimit: 2147483647},
	}, cat
}

// primerTransformState 造一份带穿戴晶体与装备库账本的角色状态。
func primerTransformState(t *testing.T, bag Bag, ledger EquipmentJournal) json.RawMessage {
	t.Helper()
	state, e := SaveBag(json.RawMessage(`{"level":115,"advancement":0}`), bag)
	if e != nil {
		t.Fatalf("save bag: %v", e)
	}
	if state, e = SaveEquipmentJournal(state, ledger); e != nil {
		t.Fatalf("save journal: %v", e)
	}
	return state
}

func primerTransformBag(gold uint32, worn ...BagEquipment) Bag {
	return Bag{Version: "ordinary-bag-v1", Gold: gold, Worn: worn}
}

func primerTransformRole(cat catalog.LootCatalog, state json.RawMessage) Role {
	return Role{ConfigVersion: cat.Source.SaveIdentity(), State: state}
}

func primerAccountRaw(t *testing.T) json.RawMessage {
	t.Helper()
	raw, e := NewAccountMaterials().Save()
	if e != nil {
		t.Fatal(e)
	}
	return raw
}

// 计划：行 0 → 誓约核心槽 47；记录位次 i → 晶体槽 36+i。
// 未登记 / 非晶体 / 已经是目标的记录被跳过（**不收费**）。
func TestPrimerTransformPlanMapsRowsToSlots(t *testing.T) {
	s, cat := primerTransformService(t)
	// 穿戴槽 36 已经是一件稀有晶体（rarity 2）。
	state := primerTransformState(t,
		primerTransformBag(1_000_000, BagEquipment{Slot: 36, Template: 100401592}),
		EquipmentJournal{Counts: map[uint32]uint32{100401592: 1, 100401597: 1}})
	role := primerTransformRole(cat, state)

	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = 100401592 // 与槽 36 现有件相同 ⇒ 无可变换
	r.Entries[1].Template = 100401597 // 槽 37
	r.Entries[2].Template = 100051399 // 不是晶体
	r.Entries[3].Template = 999999    // 未在装备库登记

	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("steps = %+v, want exactly one (slot 37)", plan.Steps)
	}
	step := plan.Steps[0]
	if step.row != 1 || step.slot != 37 || step.to != 100401597 || step.from != 0 || step.oath {
		t.Fatalf("step = %+v, want {row:1 slot:37 to:100401597 from:0 oath:false}", step)
	}
	if len(plan.Receipt.Skipped) != 2 {
		t.Fatalf("skipped = %v, want the non-primer and the unregistered template", plan.Receipt.Skipped)
	}
	if plan.Key == "" {
		t.Fatal("plan key must not be empty")
	}
	// 第一条记录对应槽 36（位次从 0 起）。
	first := protocol.PrimerTransformRequest{}
	first.Entries[0].Template = 100401597
	plan, e = s.PlanPrimerTransform(role, first, primerTransformPay)
	if e != nil || len(plan.Steps) != 1 || plan.Steps[0].slot != 36 {
		t.Fatalf("plan = %+v err=%v, want slot 36", plan.Steps, e)
	}
}

// 行 0（正文 +17）是**誓约核心槽 47**：非空时按同一条链变换（行号 -1）。
func TestPrimerTransformOathRowTargetsSlot47(t *testing.T) {
	s, cat := primerTransformService(t)
	state := primerTransformState(t,
		primerTransformBag(1_000_000),
		EquipmentJournal{Counts: map[uint32]uint32{100610079: 1}}) // 一件誓约核心（[oath]）
	role := primerTransformRole(cat, state)

	r := protocol.PrimerTransformRequest{}
	r.Oath.Template = 100610079
	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("steps = %+v, want exactly one (oath core)", plan.Steps)
	}
	step := plan.Steps[0]
	if step.row != -1 || step.slot != PrimerOathSlot || !step.oath || step.to != 100610079 {
		t.Fatalf("step = %+v, want row -1 / slot 47 / oath", step)
	}
	// 空行 + 空记录 ⇒ 明确报错（上层据此只记日志、照常回窗口应答）。
	if _, e := s.PlanPrimerTransform(role, protocol.PrimerTransformRequest{}, primerTransformPay); e == nil {
		t.Fatal("empty request must be refused")
	}
}

// **誓约核心**（行 0 → 槽 47）的完整执行：同样必须消耗军械库那件实物（2026-10-04 缺口的回归）。
// 誓约变换是这条链的主用途，plan 之外也要有落库口径的用例。
func TestPrimerTransformOathCoreConsumesArmoryRegistration(t *testing.T) {
	s, cat := primerTransformService(t)
	state := primerTransformState(t,
		primerTransformBag(1_000_000),
		EquipmentJournal{Counts: map[uint32]uint32{100610079: 1}})
	role := primerTransformRole(cat, state)
	r := protocol.PrimerTransformRequest{}
	r.Oath.Template = 100610079
	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	updated, accountNext, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if len(receipt.Pairs) != 1 || receipt.Pairs[0].Slot != PrimerOathSlot || receipt.Pairs[0].To != 100610079 {
		t.Fatalf("pairs = %+v, want one pair into slot %d", receipt.Pairs, PrimerOathSlot)
	}
	if receipt.Gold == 0 {
		t.Fatal("oath conversion must charge the source-driven cost")
	}
	if len(receipt.Refunds) != 0 {
		t.Fatalf("refunds = %+v, want none on an empty core slot", receipt.Refunds)
	}
	next, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	if got, ok := wornOf(next, PrimerOathSlot); !ok || got != 100610079 {
		t.Fatalf("worn slot %d = %d ok=%v, want %d", PrimerOathSlot, got, ok, 100610079)
	}
	if next.Gold != 1_000_000-receipt.Gold {
		t.Fatalf("gold = %d, want %d", next.Gold, 1_000_000-receipt.Gold)
	}
	ledger, e := ReadEquipmentJournal(updated)
	if e != nil {
		t.Fatal(e)
	}
	// 实物守恒：核心是从军械库那 1 件搬上来的 ⇒ 登记必须降到 0，否则下次还能再变一个。
	if ledger.Counts[100610079] != 0 {
		t.Fatalf("oath registration must be consumed to 0: %v", ledger.Counts)
	}
	if mats, e := ReadAccountMaterials(accountNext); e != nil {
		t.Fatal(e)
	} else if mats.Count(10415190) != 0 {
		t.Fatalf("unexpected refund: %v", mats)
	}
}

// 执行（空槽 + 军械库来源）：按**目标稀有度**从源表扣成本、把军械库那件搬上装备栏、
// 把被换下/搬走的那件登记回族谱并返还材料。
//
// ⚠️ 2026-10-04 守恒口径：这一条只对**空槽 + 军械库登记**成立（那时才真正消耗了
// "拥有的实物"）。槽里已有东西时一律就地换，不登记、不返还（见下面的用例）。
func TestPrimerTransformExecutesSourceCostAndRefunds(t *testing.T) {
	s, cat := primerTransformService(t)
	state := primerTransformState(t,
		primerTransformBag(100_000),
		EquipmentJournal{Counts: map[uint32]uint32{100401597: 1}})
	role := primerTransformRole(cat, state)

	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = 100401597 // 槽 36：rarity 6 = legendary ⇒ 35000 金币

	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].source.kind != primerSourceArmory {
		t.Fatalf("plan steps = %+v, want one armory-sourced step", plan.Steps)
	}
	updated, accountNext, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if receipt.Gold != 35000 || receipt.Option != 1 {
		t.Fatalf("receipt gold/option = %d/%d, want 35000/1", receipt.Gold, receipt.Option)
	}
	if len(receipt.Pairs) != 1 || receipt.Pairs[0].Slot != 36 || receipt.Pairs[0].Row != 0 ||
		receipt.Pairs[0].From != 0 || receipt.Pairs[0].To != 100401597 {
		t.Fatalf("pairs = %+v", receipt.Pairs)
	}
	// 晶体链没有灵魂项（源 `[need primer materials]` 只有金币/巡礼之印）⇒ 成本材料为空。
	if len(receipt.Materials) != 0 {
		t.Fatalf("materials = %+v, want none for the primer chain", receipt.Materials)
	}
	// 槽原来是空的 ⇒ 没有"被换下的那件"，登记 -1 到 0；成本只有金币，所以也不该有返还。
	if len(receipt.Refunds) != 0 {
		t.Fatalf("refunds = %+v, want none when the slot was empty", receipt.Refunds)
	}
	// 落库核对：金币扣了、槽里换了、军械库那件被消耗。
	next, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	if next.Gold != 65_000 {
		t.Fatalf("gold = %d, want 65000", next.Gold)
	}
	got, ok := wornOf(next, 36)
	if !ok || got != 100401597 {
		t.Fatalf("worn slot 36 = %d ok=%v, want 100401597", got, ok)
	}
	nextLedger, e := ReadEquipmentJournal(updated)
	if e != nil {
		t.Fatal(e)
	}
	// 实物守恒：目标是从军械库那 1 件搬上来的 ⇒ 登记必须降到 0，否则下次还能再变一个。
	if nextLedger.Counts[100401597] != 0 {
		t.Fatalf("armory registration must be consumed to 0: %v", nextLedger.Counts)
	}
	nextAccount, e := ReadAccountMaterials(accountNext)
	if e != nil {
		t.Fatal(e)
	}
	if nextAccount.Count(10415190) != 0 {
		t.Fatalf("unexpected refund: %v", nextAccount)
	}
}

// **军械库来源的守恒**：两槽都空、目标分别登记在军械库里 ⇒ 两件都被搬进穿戴槽、
// 登记各自减 1，且**没有"被换下的那件"**（槽原来是空的）⇒ 不登记、不返还。
//
// 这条是"变换把实物搬家"的正面用例：不加来源消耗的话，同一件登记能反复变出无数件
// （用户最初报的"凭空生成晶体、越变越多"）。
func TestPrimerTransformConsumesArmoryRegistrations(t *testing.T) {
	s, cat := primerTransformService(t)
	state := primerTransformState(t,
		primerTransformBag(1_000_000),
		EquipmentJournal{Counts: map[uint32]uint32{100401592: 1, 100401597: 1}})
	role := primerTransformRole(cat, state)
	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = 100401597 // 槽 36（legendary ⇒ 35000）
	r.Entries[1].Template = 100401592 // 槽 37（rare ⇒ 25000）
	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	for _, step := range plan.Steps {
		if step.source.kind != primerSourceArmory {
			t.Fatalf("step %+v: want an armory-sourced fill", step)
		}
	}
	updated, accountNext, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if len(receipt.Pairs) != 2 {
		t.Fatalf("pairs = %+v, want two armory-sourced fills", receipt.Pairs)
	}
	if receipt.Gold != 35000+25000 {
		t.Fatalf("gold = %d, want 60000 (legendary 35000 + rare 25000)", receipt.Gold)
	}
	// 两槽原来都是空的 ⇒ 没有被换下的那件 ⇒ 不返还材料（守恒：没有实物被消耗掉）。
	if len(receipt.Refunds) != 0 {
		t.Fatalf("refunds = %+v, want none on empty slots", receipt.Refunds)
	}
	ledger, e := ReadEquipmentJournal(updated)
	if e != nil {
		t.Fatal(e)
	}
	if ledger.Counts[100401597] != 0 || ledger.Counts[100401592] != 0 {
		t.Fatalf("both armory registrations must be consumed: %v", ledger.Counts)
	}
	bag, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	for slot, want := range map[uint16]uint32{36: 100401597, 37: 100401592} {
		if got, ok := wornOf(bag, slot); !ok || got != want {
			t.Fatalf("worn slot %d = %d ok=%v, want %d", slot, got, ok, want)
		}
	}
	nextAccount, e := ReadAccountMaterials(accountNext)
	if e != nil {
		t.Fatal(e)
	}
	if nextAccount.Count(10415190) != 0 {
		t.Fatalf("unexpected refund: %v", nextAccount)
	}
}

// **军械库来源 + 槽里已有旧件**（用户口径 §0.4「被换下的那件回原处」）：
//
//	目标（legendary）来自**军械库登记**、槽 36 上戴着 rare
//	⇒ 目标搬进槽 36；被换下的 rare **回登记表**（回原处，目标那件本来就登记在表里），
//	  图鉴总份数一份不多、一份不少；没有任何东西离开玩家 ⇒ **不返还材料**。
//
// 这条正是"来回切换凭空生成装备/材料"的回归点：旧实现除了把 rare 登记 +1，
// 还会再返还一份灵魂 —— 来回 A↔B 就能无限刷（用户实机日志 `each round: refunds=[…]`）。
func TestPrimerTransformArmorySourceKeepsJournalAndRefundsNothing(t *testing.T) {
	s, cat := primerTransformService(t)
	const (
		rare      = uint32(100401592)
		legendary = uint32(100401597)
	)
	state := primerTransformState(t,
		primerTransformBag(1_000_000, BagEquipment{Slot: 36, Template: rare}),
		EquipmentJournal{Counts: map[uint32]uint32{rare: 1, legendary: 1}})
	role := primerTransformRole(cat, state)
	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = legendary // 槽 36 ← 军械库那件（槽上的 rare 被换下）
	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].source.kind != primerSourceArmory {
		t.Fatalf("plan steps = %+v, want one armory-sourced step", plan.Steps)
	}
	updated, accountNext, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	// 没有任何东西离开玩家 ⇒ 不返还材料（守恒判据）。
	if len(receipt.Refunds) != 0 {
		t.Fatalf("refunds = %+v, want none (被换下的那件回了登记表)", receipt.Refunds)
	}
	bag, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	if got, ok := wornOf(bag, 36); !ok || got != legendary {
		t.Fatalf("worn 36 = %d ok=%v, want %d", got, ok, legendary)
	}
	ledger, e := ReadEquipmentJournal(updated)
	if e != nil {
		t.Fatal(e)
	}
	// 目标 legendary -1 → 0；被换下的 rare 回登记（原 1 ⇒ 2）。总份数不变（2）。
	if ledger.Counts[legendary] != 0 {
		t.Fatalf("target registration must be consumed: %v", ledger.Counts)
	}
	if ledger.Counts[rare] != 2 {
		l1, ok1 := JournalLimit(s.Equipment, s.Journal, rare)
		l2, ok2 := JournalLimit(s.Equipment, s.Journal, legendary)
		t.Fatalf("swapped-out crystal must go back to the armory: %v (registrable rare=%d/%v legendary=%d/%v pairs=%+v)",
			ledger.Counts, l1, ok1, l2, ok2, receipt.Pairs)
	}
	if got := armoryTotal(ledger); got != 2 {
		t.Fatalf("armory total = %d, want 2 (守恒)", got)
	}
	nextAccount, e := ReadAccountMaterials(accountNext)
	if e != nil {
		t.Fatal(e)
	}
	if nextAccount.Count(10415190) != 0 {
		t.Fatalf("unexpected refund materials: %v", nextAccount)
	}
}

// 空槽上做变换：只扣成本、把目标从军械库搬上装备栏，不登记旧件也不返还材料。
func TestPrimerTransformOnEmptySlotEquipsWithoutRefund(t *testing.T) {
	s, cat := primerTransformService(t)
	state := primerTransformState(t,
		primerTransformBag(100_000),
		EquipmentJournal{Counts: map[uint32]uint32{100401597: 1}})
	role := primerTransformRole(cat, state)
	r := protocol.PrimerTransformRequest{}
	r.Entries[1].Template = 100401597 // 槽 37
	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	updated, accountNext, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if len(receipt.Refunds) != 0 {
		t.Fatalf("refunds = %+v, want none on an empty slot", receipt.Refunds)
	}
	if receipt.Gold != 35000 {
		t.Fatalf("gold = %d, want 35000", receipt.Gold)
	}
	next, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	if got, ok := wornOf(next, 37); !ok || got != 100401597 {
		t.Fatalf("worn slot 37 = %d ok=%v", got, ok)
	}
	ledger, e := ReadEquipmentJournal(updated)
	if e != nil {
		t.Fatal(e)
	}
	// **实物守恒**：目标是从军械库那 1 件里搬过来的 ⇒ 登记必须降到 0（2026-10-04 缺陷的回归点）。
	if ledger.Counts[100401597] != 0 {
		t.Fatalf("target registration must be consumed to 0: %v", ledger.Counts)
	}
	nextAccount, e := ReadAccountMaterials(accountNext)
	if e != nil {
		t.Fatal(e)
	}
	if nextAccount.Count(10415190) != 0 {
		t.Fatalf("unexpected refund: %v", nextAccount)
	}
}

// **重复请求不再凭空造晶体**（用户实机缺陷的正面回归）：同一件目标只有 1 件实物时，
// 第二行必须跳过；把同一份请求连发三次，装备栏晶体总数不能增长。
func TestPrimerTransformNeverConjuresExtraCrystals(t *testing.T) {
	s, cat := primerTransformService(t)
	state := primerTransformState(t,
		primerTransformBag(1_000_000),
		EquipmentJournal{Counts: map[uint32]uint32{100401597: 1}})
	role := primerTransformRole(cat, state)
	// 两行都想要同一件：只有 1 件实物 ⇒ 只能成功一行。
	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = 100401597
	r.Entries[1].Template = 100401597
	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("plan steps = %+v, want exactly one (only one real unit exists)", plan.Steps)
	}
	updated, _, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if len(receipt.Pairs) != 1 {
		t.Fatalf("pairs = %+v, want 1", receipt.Pairs)
	}
	crystals := func(raw json.RawMessage) int {
		bag, e := ReadBag(raw)
		if e != nil {
			t.Fatal(e)
		}
		n := 0
		for _, w := range bag.Worn {
			if w.Slot >= PrimerCrystalSlotBase && w.Slot <= PrimerCrystalSlotBase+PrimerCrystalSlotCount-1 && w.Template != 0 {
				n++
			}
		}
		return n
	}
	if got := crystals(updated); got != 1 {
		t.Fatalf("crystal count = %d, want 1", got)
	}
	// 再连发两轮同样的请求：允许"把晶体从槽 36 挪到槽 37"这类合法搬运，但**总数必须守恒**
	// （目标登记已经为 0，任何一轮都不许再造出第二件 —— 这就是用户实机那个缺陷的判据）。
	role.State = updated
	for i := 0; i < 2; i++ {
		again, e := s.PlanPrimerTransform(role, r, primerTransformPay)
		if e != nil {
			// 没有可消耗的实物时明确拒绝也算合格（不再凭空造）。
			if got := crystals(role.State); got != 1 {
				t.Fatalf("round %d: crystal count = %d, want 1", i+1, got)
			}
			continue
		}
		nextState, _, _, _, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, again)
		if e != nil {
			t.Fatalf("round %d: prepare: %v", i+1, e)
		}
		role.State = nextState
		if got := crystals(role.State); got > 1 {
			t.Fatalf("round %d: crystal count = %d, must never exceed the 1 real unit", i+1, got)
		}
	}
	if got := crystals(role.State); got != 1 {
		t.Fatalf("final crystal count = %d, want 1", got)
	}
	ledger, e := ReadEquipmentJournal(role.State)
	if e != nil {
		t.Fatal(e)
	}
	if ledger.Counts[100401597] != 0 {
		t.Fatalf("armory must not re-register free targets: %v", ledger.Counts)
	}
}

// **已穿戴的晶体不能当来源**（2026-10-04 收紧）：否则"把 A 搬到 B 的槽"会顶掉 B、
// 让 B 按源表登记并返还材料，两块晶体来回搬就能无限刷返还材料。
func TestPrimerTransformDoesNotUseWornCrystalAsSource(t *testing.T) {
	s, cat := primerTransformService(t)
	// 身上戴着 100401597（槽 36），军械库登记为 0、背包也没有。
	state := primerTransformState(t,
		primerTransformBag(100_000, BagEquipment{Slot: 36, Template: 100401597}),
		EquipmentJournal{Counts: map[uint32]uint32{}})
	role := primerTransformRole(cat, state)
	r := protocol.PrimerTransformRequest{}
	r.Entries[1].Template = 100401597 // 槽 37 ← 身上那件
	if _, e := s.PlanPrimerTransform(role, r, primerTransformPay); e == nil {
		t.Fatal("身上那件不能当变换来源，必须拒绝")
	}
	// 对照：同样一件只要登记在军械库（或背包）里就应当可用。
	state = primerTransformState(t,
		primerTransformBag(100_000),
		EquipmentJournal{Counts: map[uint32]uint32{100401597: 1}})
	role = primerTransformRole(cat, state)
	if _, e := s.PlanPrimerTransform(role, r, primerTransformPay); e != nil {
		t.Fatalf("军械库登记的那件应当可用: %v", e)
	}
}

// **目标没有副本、但该槽位里有东西可换 ⇒ 允许变换**（2026-10-04 用户实机缺陷：
// "变幻目标为迷雾誓约时誓约的变换没有生效"，日志 `skipped=[100610094]`）。
//
// 此时用**槽内那件**当来源（被换下去的那件走既有流程登记回图鉴 +1）。空槽仍然拒绝 ——
// 那正是用户最初报的"凭空生成晶体、越变越多"。
func TestPrimerTransformReplacesOccupiedSlotWithoutOwnedCopy(t *testing.T) {
	s, cat := primerTransformService(t)
	// 槽 36 戴着 100401592（rare：源表里有它的 `[refund primer materials]` 返还行）；
	// 目标是 100401597（legendary）——背包没有、军械库登记 0。
	state := primerTransformState(t,
		primerTransformBag(1_000_000, BagEquipment{Slot: 36, Template: 100401592}),
		EquipmentJournal{Counts: map[uint32]uint32{}})
	role := primerTransformRole(cat, state)
	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = 100401597 // 槽 36 ← 目标（无副本）

	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("槽内有旧件时应当允许变换（不再要求另有副本）: %v", e)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("plan steps = %+v, want 1", plan.Steps)
	}
	updated, _, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if len(receipt.Pairs) != 1 {
		t.Fatalf("pairs = %+v, want 1", receipt.Pairs)
	}
	bag, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	if worn, ok := wornOf(bag, 36); !ok || worn != 100401597 {
		t.Fatalf("slot 36 = %d ok=%v, want the new target 100401597", worn, ok)
	}
	// ⚠️ 无副本、就地换掉槽内那件时，那件**被消耗**：既不登记回图鉴、也不返还材料。
	// 否则"反复变换"每轮都会新增一条登记与一份材料 = 用户报的"凭空生成"（2026-10-04 回归）。
	if len(receipt.Materials) != 0 || len(receipt.Refunds) != 0 {
		t.Fatalf("in-place conversion must not refund materials: materials=%+v refunds=%+v",
			receipt.Materials, receipt.Refunds)
	}
	ledger, e := ReadEquipmentJournal(updated)
	if e != nil {
		t.Fatal(e)
	}
	if len(ledger.Counts) != 0 {
		t.Fatalf("in-place conversion must not register the consumed item: %v", ledger.Counts)
	}
	// 连做第二轮：图鉴与材料仍然不许增长（用户报的"反复变换 → 凭空生成"回归判据）。
	role.State = updated
	again, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e == nil {
		next, _, _, receipt2, e2 := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, again)
		if e2 != nil {
			t.Fatalf("round 2 prepare: %v", e2)
		}
		if len(receipt2.Materials) != 0 || len(receipt2.Refunds) != 0 {
			t.Fatalf("round 2 refunded materials: %+v / %+v", receipt2.Materials, receipt2.Refunds)
		}
		ledger2, e3 := ReadEquipmentJournal(next)
		if e3 != nil {
			t.Fatal(e3)
		}
		if len(ledger2.Counts) != 0 {
			t.Fatalf("round 2 registered the consumed item: %v", ledger2.Counts)
		}
	}
}

// 金币按**逐件**判：只够换便宜那件时，贵的跳过、便宜照换（不能整批回滚成"点了没反应"）。
func TestPrimerTransformSkipsOnlyThePricierUnfundedRow(t *testing.T) {
	s, cat := primerTransformService(t)
	// 30000 金币：够 rare(25000)，不够 legendary(35000)。
	state := primerTransformState(t,
		primerTransformBag(30_000),
		EquipmentJournal{Counts: map[uint32]uint32{100401592: 1, 100401597: 1}})
	role := primerTransformRole(cat, state)
	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = 100401597 // 槽 36：legendary ⇒ 35000（不够）
	r.Entries[1].Template = 100401592 // 槽 37：rare ⇒ 25000（够）

	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	if len(plan.Steps) != 2 {
		t.Fatalf("plan steps = %+v, want both rows planned (affordability is a transaction-time concern)", plan.Steps)
	}
	updated, _, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if len(receipt.Pairs) != 1 || receipt.Pairs[0].Slot != 37 || receipt.Pairs[0].To != 100401592 {
		t.Fatalf("pairs = %+v, want only the affordable row (slot 37)", receipt.Pairs)
	}
	if len(receipt.Skipped) != 1 || receipt.Skipped[0] != 100401597 {
		t.Fatalf("skipped = %v, want [100401597]", receipt.Skipped)
	}
	if receipt.Gold != 25000 {
		t.Fatalf("gold = %d, want 25000", receipt.Gold)
	}
	next, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	if next.Gold != 5_000 {
		t.Fatalf("remaining gold = %d, want 5000", next.Gold)
	}
	if got, ok := wornOf(next, 37); !ok || got != 100401592 {
		t.Fatalf("worn slot 37 = %d ok=%v, want 100401592", got, ok)
	}
	if _, ok := wornOf(next, 36); ok {
		t.Fatal("unaffordable row must not be written")
	}
}

// **源变则结果变**：换一张金币不同的同形表，扣费必须跟着源走（防止实现里又出现常量）。
func TestPrimerTransformCostFollowsSourceTable(t *testing.T) {
	s, cat := primerTransformService(t)
	variant, e := catalog.ParseEquipmentTransformSystem(
		replaceTransformLegendaryGold(transformSystemFixtureText, 12345))
	if e != nil {
		t.Fatalf("parse variant table: %v", e)
	}
	s.Transform = &variant

	state := primerTransformState(t,
		primerTransformBag(100_000),
		EquipmentJournal{Counts: map[uint32]uint32{100401597: 1}})
	role := primerTransformRole(cat, state)
	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = 100401597

	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	_, _, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if receipt.Gold != 12345 {
		t.Fatalf("gold = %d, want the variant table's 12345 (cost must follow the source)", receipt.Gold)
	}
}

// 缺成本表 / 缺账本表一律拒绝执行，绝不静默按 0 成本放行。
func TestPrimerTransformRefusesWithoutTables(t *testing.T) {
	s, cat := primerTransformService(t)
	state := primerTransformState(t,
		primerTransformBag(100_000),
		EquipmentJournal{Counts: map[uint32]uint32{100401597: 1}})
	role := primerTransformRole(cat, state)
	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = 100401597

	noTable := *s
	noTable.Transform = nil
	if _, e := noTable.PlanPrimerTransform(role, r, primerTransformPay); e == nil {
		t.Fatal("missing cost table must be refused")
	}
	noJournal := *s
	noJournal.Journal = nil
	if _, e := noJournal.PlanPrimerTransform(role, r, primerTransformPay); e == nil {
		t.Fatal("missing journal rules must be refused")
	}
	// 表外付款方式（3）同样拒绝（源里每档只有 group 1/2）。
	if _, e := s.PlanPrimerTransform(role, r, 3); e == nil {
		t.Fatal("unknown pay option must be refused")
	}
	// 全空请求：没有可变换项 ⇒ 明确报错（上层据此只记日志、照常回窗口应答）。
	if _, e := s.PlanPrimerTransform(role, protocol.PrimerTransformRequest{}, primerTransformPay); e == nil {
		t.Fatal("empty request must be refused")
	}
}

// **原料总数守恒**（用户口径 §0.3/§0.4，2026-10-04；用户原话"原料总数守恒是必须遵守的"）：
//
//	槽 36 空着、背包里有一件 A ⇒ 变换成 A：A 从背包搬上槽（总数不变、图鉴不动）；
//	下一轮把目标换成 B（B 在**军械库登记**里）⇒ 目标从登记 -1，槽里的 A **回登记 +1**
//	（两者相抵，图鉴也不动）；再一轮换成 A（A 现在在登记里）⇒ 同样相抵。
//
// 判据（每轮都要成立）：
//   - 「背包 + 穿戴」的实物**总数**不变（位置可以变，数量不许变）；
//   - 「背包 + 穿戴」+「军械库登记」的**总份数**不变（军械库那件只是搬到身上）；
//   - **返还材料必须为 0**（旧实现每轮返还 3~5 个微光灵魂 —— 那正是用户报的"凭空生成"）。
func TestPrimerTransformKeepsCarriedCountsConstant(t *testing.T) {
	s, cat := primerTransformService(t)
	const (
		rare      = uint32(100401592)
		legendary = uint32(100401597)
		bagSlot   = uint16(50) // > 47 ⇒ 不是穿戴槽，落在背包装备区
	)
	bag := Bag{Version: "ordinary-bag-v1", Gold: 10_000_000,
		Equipment: []BagEquipment{{Slot: bagSlot, Template: legendary}},
	}
	state := primerTransformState(t, bag, EquipmentJournal{Counts: map[uint32]uint32{}})
	role := primerTransformRole(cat, state)

	// 第 1 轮的目标在背包里；之后两轮的目标都在军械库登记里（挨个变成被换下的那件）。
	targets := []uint32{legendary, rare, legendary}
	var totalGold uint32
	var refundSources int
	for round, target := range targets {
		r := protocol.PrimerTransformRequest{}
		r.Entries[0].Template = target
		plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
		if e != nil {
			t.Fatalf("round %d: plan: %v", round+1, e)
		}
		next, _, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
		if e != nil {
			t.Fatalf("round %d: prepare: %v", round+1, e)
		}
		role.State = next
		totalGold += receipt.Gold
		refundSources += len(receipt.Refunds)
		live, e := ReadBag(next)
		if e != nil {
			t.Fatal(e)
		}
		// 判据 1：目标确实搬到了槽 36（否则"没生效"会伪装成守恒）。
		if got, ok := wornOf(live, 36); !ok || got != target {
			t.Fatalf("round %d: worn 36 = %d ok=%v, want %d", round+1, got, ok, target)
		}
		// 判据 2：背包 + 穿戴的实物总数不变（出发时是 1 件）。
		if got, want := carriedTotal(live), 1; got != want {
			t.Fatalf("round %d (→%d): carried total = %d, want %d", round+1, target, got, want)
		}
		// 判据 3：军械库登记里至多 1 份（目标从登记扣、旧件回登记 ⇒ 相抵）。
		ledger, e := ReadEquipmentJournal(next)
		if e != nil {
			t.Fatal(e)
		}
		if got := armoryTotal(ledger); got > 1 {
			t.Fatalf("round %d (→%d): armory total = %d, must stay ≤ 1: %v", round+1, target, got, ledger.Counts)
		}
	}
	// 成本照付：金币按目标稀有度从源表算（legendary 35000、rare 25000）。
	if want := uint32(2*35000 + 25000); totalGold != want {
		t.Fatalf("total gold = %d, want %d", totalGold, want)
	}
	// **不得返还材料**：这些路径里没有任何实物离开玩家。
	if refundSources != 0 {
		t.Fatalf("refunds = %d rows, want 0（只有旧件真的回不到玩家手里时才返还）", refundSources)
	}
}

// carriedTotal 数「背包实物 + 穿戴」的总件数（守恒判据就是拿它逐轮比对）。
func carriedTotal(bag Bag) int {
	total := 0
	for _, row := range bag.Equipment {
		if row.Template != 0 {
			total++
		}
	}
	for _, row := range bag.Worn {
		if row.Template != 0 {
			total++
		}
	}
	return total
}

// armoryTotal 数装备库登记里的总份数。
func armoryTotal(j EquipmentJournal) uint32 {
	var total uint32
	for _, n := range j.Counts {
		total += n
	}
	return total
}

// **账本进出相抵时不返还**（用户口径 §0.3/§0.4）：目标（rare）来自登记表、槽里戴着
// legendary、背包放得下 ⇒ 目标登记 −1（1→0）、legendary 登记 +1（1→2）⇒
// **总份数不变**，那件 legendary 仍然在玩家手里 ⇒ **不返还任何材料**。
//
// 这是"每轮无条件返还 ⇒ 来回切换无限刷"的正面回归点。
func TestPrimerTransformKeepsJournalAndRefundsNothingWhenItBalances(t *testing.T) {
	s, cat := primerTransformService(t)
	const (
		rare      = uint32(100401592)
		legendary = uint32(100401597)
	)
	bag := Bag{Version: "ordinary-bag-v1", Gold: 1_000_000,
		Worn: []BagEquipment{{Slot: 36, Template: legendary}},
	}
	state := primerTransformState(t, bag,
		EquipmentJournal{Counts: map[uint32]uint32{rare: 1, legendary: 1}})
	role := primerTransformRole(cat, state)

	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = rare // 槽 36 ← 登记表里那件 rare（legendary 被换下）
	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].source.kind != primerSourceArmory {
		t.Fatalf("plan steps = %+v, want an armory-sourced step", plan.Steps)
	}
	updated, accountNext, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if len(receipt.Refunds) != 0 {
		t.Fatalf("refunds = %+v, want none (旧件回了登记表，实物没离手)", receipt.Refunds)
	}
	nextAccount, e := ReadAccountMaterials(accountNext)
	if e != nil {
		t.Fatal(e)
	}
	if nextAccount.Count(10415190) != 0 {
		t.Fatalf("unexpected refund materials: %v", nextAccount)
	}
	ledger, e := ReadEquipmentJournal(updated)
	if e != nil {
		t.Fatal(e)
	}
	// rare 被搬走（1→0），legendary 被换下登记回来（1→2）⇒ 总份数仍然是 2（守恒）。
	if ledger.Counts[rare] != 0 {
		t.Fatalf("consumed target must drop to 0: %v", ledger.Counts)
	}
	if ledger.Counts[legendary] != 2 {
		t.Fatalf("swapped-out crystal must go back to the journal: %v", ledger.Counts)
	}
	if got := armoryTotal(ledger); got != 2 {
		t.Fatalf("journal total = %d, want 2 (守恒)", got)
	}
	live, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	if got, ok := wornOf(live, 36); !ok || got != rare {
		t.Fatalf("worn 36 = %d ok=%v, want %d", got, ok, rare)
	}
}

// **安置也要按"份数"算**：背包里还有同模板的一件时，旧件并不需要真的塞进某个空槽
// （那件只是换了件一模一样的），所以不登记、不返还、也不动账本。
//
// 这条与下面那条"旧件真的离手"是一对：**离手**只发生在
// "登记达上限 + 背包已经放不下"同时成立时（注册上限对 `[oath]` 这类是 1，
// 对普通晶体是 99 —— 实战里只有背包满才触发得到）。
func TestPrimerTransformPrefersARegisteredCopyOverASlot(t *testing.T) {
	s, cat := primerTransformService(t)
	const (
		rare      = uint32(100401592)
		legendary = uint32(100401597)
		filler    = uint32(100401633) // 填充物：与目标/旧件都不同模板
	)
	slots := [2]uint16{9, 12}
	s.BagRules = BagRules{EquipmentSlots: slots, MissingStackLimit: 2147483647}
	full := make([]BagEquipment, 0, 4)
	for n := slots[0]; n <= slots[1]; n++ {
		full = append(full, BagEquipment{Slot: n, Template: filler})
	}
	// 背包里再放一件 rare（与槽里那件同模板）⇒ 背包满 + 同模板在背包里。
	bag := Bag{Version: "ordinary-bag-v1", Gold: 1_000_000,
		Worn:      []BagEquipment{{Slot: 36, Template: rare}},
		Equipment: append(full, BagEquipment{Slot: slots[1] + 1, Template: rare}),
	}
	s.BagRules = BagRules{EquipmentSlots: [2]uint16{slots[0], slots[1] + 1}, MissingStackLimit: 2147483647}
	state := primerTransformState(t, bag, EquipmentJournal{Counts: map[uint32]uint32{legendary: 1}})
	role := primerTransformRole(cat, state)

	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = legendary
	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	updated, accountNext, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if len(receipt.Refunds) != 0 {
		t.Fatalf("refunds = %+v, want none (背包里还有一件同模板的 ⇒ 没离手)", receipt.Refunds)
	}
	nextAccount, e := ReadAccountMaterials(accountNext)
	if e != nil {
		t.Fatal(e)
	}
	if nextAccount.Count(10415190) != 0 {
		t.Fatalf("unexpected refund materials: %v", nextAccount)
	}
	ledger, e := ReadEquipmentJournal(updated)
	if e != nil {
		t.Fatal(e)
	}
	if ledger.Counts[legendary] != 0 {
		t.Fatalf("target registration must be consumed: %v", ledger.Counts)
	}
	live, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	if got, ok := wornOf(live, 36); !ok || got != legendary {
		t.Fatalf("worn 36 = %d ok=%v, want %d", got, ok, legendary)
	}
}

// **只有"登记不回去、背包也塞不进"时才算实物离手**：这时才按源表返还材料
// （`[refund primer materials]` rare 分支 0 = 10415190 ×1）。
//
// 构造：旧件是 rare 晶体、合成规则把它这一档的登记上限压到 1 且已在册 1 份
// ⇒ 登记被上限挡住；背包塞满 ⇒ 也放不进背包。于是那件 rare 真的离手 ⇒ 返还 1 个微光灵魂。
// **这是唯一的返还路径**（用户口径：只有实物真的离开玩家才返还）。
func TestPrimerTransformRefundsOnlyWhenTheOldItemCannotBePlaced(t *testing.T) {
	s, cat := primerTransformService(t)
	const (
		rare      = uint32(100401592) // 被换下的那件（rarity 2 ⇒ 返还 1）
		legendary = uint32(100401597) // 目标（登记表里 1 份）
		filler    = uint32(100401633) // 背包填充物：与目标/旧件都不同模板
	)
	_, rareRarity, ok := s.equipmentGradeRarity(rare)
	if !ok {
		t.Fatalf("读不到 %d 的稀有度", rare)
	}
	// 合成一份"这一档晶体上限 1"的账本规则（口径与真实源 `[oath] … 1` 同形）。
	rules := catalog.EquipmentJournalRules{
		Maximum:       99,
		MaximumByType: []catalog.JournalTypeLimit{{Kind: "[primer]", Rarity: uint32(rareRarity), Maximum: 1}},
	}
	s.Journal = &rules
	slots := [2]uint16{9, 12}
	s.BagRules = BagRules{EquipmentSlots: slots, MissingStackLimit: 2147483647}
	full := make([]BagEquipment, 0, 4)
	for n := slots[0]; n <= slots[1]; n++ {
		full = append(full, BagEquipment{Slot: n, Template: filler})
	}
	bag := Bag{Version: "ordinary-bag-v1", Gold: 1_000_000,
		Worn:      []BagEquipment{{Slot: 36, Template: rare}},
		Equipment: full,
	}
	state := primerTransformState(t, bag,
		EquipmentJournal{Counts: map[uint32]uint32{legendary: 1, rare: 1}})
	role := primerTransformRole(cat, state)

	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = legendary
	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].source.kind != primerSourceArmory || plan.Steps[0].from != rare {
		t.Fatalf("plan steps = %+v, want one armory step swapping %d out", plan.Steps, rare)
	}
	updated, accountNext, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if len(receipt.Refunds) != 1 || receipt.Refunds[0].Template != 10415190 || receipt.Refunds[0].Amount != 1 {
		t.Fatalf("refunds = %+v (pairs=%+v skipped=%v gold=%d), want 10415190 x1 (旧件安置不进去 ⇒ 真的离手)",
			receipt.Refunds, receipt.Pairs, receipt.Skipped, receipt.Gold)
	}
	nextAccount, e := ReadAccountMaterials(accountNext)
	if e != nil {
		t.Fatal(e)
	}
	if nextAccount.Count(10415190) != 1 {
		t.Fatalf("refund must land in the account material vault: %v", nextAccount)
	}
	ledger, e := ReadEquipmentJournal(updated)
	if e != nil {
		t.Fatal(e)
	}
	// 目标 legendary 1→0；rare 安置不进去 ⇒ 回到它在册的 1 份（不增长）。
	if ledger.Counts[legendary] != 0 {
		t.Fatalf("target registration must be consumed: %v", ledger.Counts)
	}
	if ledger.Counts[rare] != 1 {
		t.Fatalf("the unplaceable crystal must not add a registration: %v", ledger.Counts)
	}
	live, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	if got, ok := wornOf(live, 36); !ok || got != legendary {
		t.Fatalf("worn 36 = %d ok=%v, want %d", got, ok, legendary)
	}
}

// **"回不去又没有补偿"的行整条不做**（用户："图鉴有时会少"的根因）：
// 誓约核心的登记上限是 1，当它已在册 1 份、**背包又满**时：
//
//	既登记不回去、也放不进背包 —— 而合成源的 `primeval` 档是 `1 0` **空返还分支**，
//	于是没有任何补偿可给 ⇒ 这一行**必须整条跳过**：目标登记不许扣、穿戴不许改。
//
// 反面对照见 TestPrimerTransformRefundsOnlyWhenTheOldItemCannotBePlaced
// （`rare` 档有返还料 ⇒ 允许执行并返还）。
func TestPrimerTransformSkipsSwapThatCannotBePaidBack(t *testing.T) {
	s, cat := primerTransformService(t)
	const (
		oath      = uint32(100610079) // 誓约核心：登记上限 1，已在册 1 份
		legendary = uint32(100401597) // 目标：登记表里 1 份
		rare      = uint32(100401592)
	)
	if limit, ok := JournalLimit(s.Equipment, s.Journal, oath); !ok || limit != 1 {
		t.Fatalf("前提失效：誓约核心 %d 的登记上限 = %d/%v，want 1/true", oath, limit, ok)
	}
	if _, e := s.primerRefund(oath); e == nil {
		t.Fatalf("前提失效：%d 这一档在合成源里居然有返还料，本用例不再有意义", oath)
	}
	// 背包塞满（9..12），槽 36 戴着 oath ⇒ 旧件既登记不回去（上限 1）、也放不进背包。
	slots := [2]uint16{9, 12}
	s.BagRules = BagRules{EquipmentSlots: slots, MissingStackLimit: 2147483647}
	full := make([]BagEquipment, 0, 4)
	for n := slots[0]; n <= slots[1]; n++ {
		full = append(full, BagEquipment{Slot: n, Template: rare})
	}
	bag := Bag{Version: "ordinary-bag-v1", Gold: 1_000_000,
		Worn:      []BagEquipment{{Slot: 36, Template: oath}},
		Equipment: full,
	}
	state := primerTransformState(t, bag, EquipmentJournal{Counts: map[uint32]uint32{oath: 1, legendary: 1}})
	role := primerTransformRole(cat, state)

	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = legendary // 槽 36 ← 登记表里那件 legendary
	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	updated, _, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
	// 整行不可执行 ⇒ 允许两种收口（整笔报错 / 记 Skipped），但**存档必须一字不动**。
	if e != nil {
		t.Logf("prepare 拒绝整笔（可接受）: %v", e)
		updated = state
	} else {
		if len(receipt.Pairs) != 0 {
			t.Fatalf("pairs = %+v, want none (旧件既回不去又没补偿 ⇒ 这行不许执行)", receipt.Pairs)
		}
		if len(receipt.Skipped) != 1 || receipt.Skipped[0] != legendary {
			t.Fatalf("skipped = %v, want [%d]", receipt.Skipped, legendary)
		}
	}
	ledger, e := ReadEquipmentJournal(updated)
	if e != nil {
		t.Fatal(e)
	}
	if ledger.Counts[legendary] != 1 {
		t.Fatalf("target registration must stay intact: %v", ledger.Counts)
	}
	if ledger.Counts[oath] != 1 {
		t.Fatalf("oath registration must stay at its cap: %v", ledger.Counts)
	}
	liveBag, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	if got, ok := wornOf(liveBag, 36); !ok || got != oath {
		t.Fatalf("worn 36 = %d ok=%v, want %d (没执行就不该改穿戴)", got, ok, oath)
	}
}

// **同一个模板来回换时账本不许漂**（用户报的"图鉴自己乱变"的正面判据）：
// 登记表里 A 1 份、槽里戴着 B ⇒ A→B→A 来回各两轮，每一轮之后
// 「穿戴 + 登记表」的总份数都必须恒等于 2，且登记表里只允许出现 A、B 两个模板。
func TestPrimerTransformJournalDoesNotDriftAcrossRounds(t *testing.T) {
	s, cat := primerTransformService(t)
	const (
		rare      = uint32(100401592)
		legendary = uint32(100401597)
	)
	bag := Bag{Version: "ordinary-bag-v1", Gold: 10_000_000,
		Worn: []BagEquipment{{Slot: 36, Template: legendary}},
	}
	state := primerTransformState(t, bag, EquipmentJournal{Counts: map[uint32]uint32{rare: 1}})
	role := primerTransformRole(cat, state)

	for round, target := range []uint32{rare, legendary, rare, legendary} {
		r := protocol.PrimerTransformRequest{}
		r.Entries[0].Template = target
		plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
		if e != nil {
			t.Fatalf("round %d: plan: %v", round+1, e)
		}
		next, _, _, _, e := s.PreparePrimerTransform(role, primerAccountRaw(t), 0, plan)
		if e != nil {
			t.Fatalf("round %d: prepare: %v", round+1, e)
		}
		role.State = next
		live, e := ReadBag(next)
		if e != nil {
			t.Fatal(e)
		}
		ledger, e := ReadEquipmentJournal(next)
		if e != nil {
			t.Fatal(e)
		}
		// 总份数 = 穿戴件数 + 登记份数，必须恒为 2（一件在手上、一件在登记表里）。
		if got := carriedTotal(live) + int(armoryTotal(ledger)); got != 2 {
			t.Fatalf("round %d (→%d): worn+journal total = %d, want 2 (穿戴 %v / 登记 %v)",
				round+1, target, got, wornFingerprint(live), ledger.Counts)
		}
		// 登记表里只允许出现参与这次来回的两件模板，且每件 ≤ 2 份（不许长出第三件）。
		for template, count := range ledger.Counts {
			if template != rare && template != legendary {
				t.Fatalf("round %d: unexpected journal template %d (count %d): %v",
					round+1, template, count, ledger.Counts)
			}
			if count > 2 {
				t.Fatalf("round %d: journal count for %d = %d, want ≤2: %v",
					round+1, template, count, ledger.Counts)
			}
		}
		if got, ok := wornOf(live, 36); !ok || got != target {
			t.Fatalf("round %d: worn 36 = %d ok=%v, want %d", round+1, got, ok, target)
		}
	}
}

// **找不到实物来源时绝不允许凭空生成**：目标指向空槽、军械库与背包都没有那件 ⇒
// 整帧拒绝，且不写出任何晶体（用户最初报的"越变越多"的正面判据）。
func TestPrimerTransformNeverFillsEmptySlotsWithoutSource(t *testing.T) {
	s, cat := primerTransformService(t)
	state := primerTransformState(t,
		primerTransformBag(1_000_000),
		EquipmentJournal{Counts: map[uint32]uint32{}})
	role := primerTransformRole(cat, state)
	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = 100401597
	r.Entries[3].Template = 100401597
	if _, e := s.PlanPrimerTransform(role, r, primerTransformPay); e == nil {
		t.Fatal("没有实物来源时空槽必须拒绝，绝不能凭空生成")
	}
	next, e := ReadBag(state)
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := wornOf(next, 36); ok {
		t.Fatal("refused request must not have written a crystal")
	}
}

// bagEquipTemplate 读背包装备区某个槽的模板（0 = 该槽没有行）。
func bagEquipTemplate(bag Bag, slot uint16) uint32 {
	for _, row := range bag.Equipment {
		if row.Slot == slot {
			return row.Template
		}
	}
	return 0
}

// equipmentFingerprint / wornFingerprint 给背包与穿戴行做可比较的指纹（槽:模板，按槽排序）。
func equipmentFingerprint(bag Bag) []string {
	out := make([]string, 0, len(bag.Equipment))
	for _, row := range bag.Equipment {
		out = append(out, fmt.Sprintf("%#x:%d", row.Slot, row.Template))
	}
	sort.Strings(out)
	return out
}

func wornFingerprint(bag Bag) []string {
	out := make([]string, 0, len(bag.Worn))
	for _, row := range bag.Worn {
		out = append(out, fmt.Sprintf("%#x:%d", row.Slot, row.Template))
	}
	sort.Strings(out)
	return out
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// replaceTransformLegendaryGold 把合成源表里**晶体链** legendary 档的金币换成另一个值
// （用来证明实现真的在读源表，而不是又把数值写成常量）。
func replaceTransformLegendaryGold(text string, gold uint32) string {
	section := strings.Index(text, "[need primer materials]")
	if section < 0 {
		return text
	}
	head, tail := text[:section], text[section:]
	marker := "`legendary`"
	i := strings.Index(tail, marker)
	if i < 0 {
		return text
	}
	j := strings.Index(tail[i:], "0 35000")
	if j < 0 {
		return text
	}
	start := i + j
	return head + tail[:start] + "0 " + strconv.FormatUint(uint64(gold), 10) + tail[start+len("0 35000"):]
}
