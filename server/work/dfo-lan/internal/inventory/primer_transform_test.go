package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"encoding/json"
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
	updated, accountNext, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), plan)
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

// 执行：按**目标稀有度**从源表扣成本、换上新晶体、把被换下去的那件登记并返还材料。
func TestPrimerTransformExecutesSourceCostAndRefunds(t *testing.T) {
	s, cat := primerTransformService(t)
	state := primerTransformState(t,
		primerTransformBag(100_000, BagEquipment{Slot: 36, Template: 100401592}),
		EquipmentJournal{Counts: map[uint32]uint32{100401592: 1, 100401597: 1}})
	role := primerTransformRole(cat, state)

	r := protocol.PrimerTransformRequest{}
	r.Entries[0].Template = 100401597 // 槽 36：rarity 6 = legendary ⇒ 35000 金币

	plan, e := s.PlanPrimerTransform(role, r, primerTransformPay)
	if e != nil {
		t.Fatalf("plan: %v", e)
	}
	updated, accountNext, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), plan)
	if e != nil {
		t.Fatalf("prepare: %v", e)
	}
	if receipt.Gold != 35000 || receipt.Option != 1 {
		t.Fatalf("receipt gold/option = %d/%d, want 35000/1", receipt.Gold, receipt.Option)
	}
	if len(receipt.Pairs) != 1 || receipt.Pairs[0].Slot != 36 || receipt.Pairs[0].Row != 0 ||
		receipt.Pairs[0].From != 100401592 || receipt.Pairs[0].To != 100401597 {
		t.Fatalf("pairs = %+v", receipt.Pairs)
	}
	// 晶体链没有灵魂项（源 `[need primer materials]` 只有金币/巡礼之印）⇒ 成本材料为空。
	if len(receipt.Materials) != 0 {
		t.Fatalf("materials = %+v, want none for the primer chain", receipt.Materials)
	}
	// 被换下去的 rarity 2 晶体 ⇒ 源 `[refund primer materials]` rare 分支 0 = 10415190 ×1。
	if len(receipt.Refunds) != 1 || receipt.Refunds[0].Template != 10415190 || receipt.Refunds[0].Amount != 1 {
		t.Fatalf("refunds = %+v, want 10415190 x1", receipt.Refunds)
	}
	// 落库核对：金币扣了、槽里换了、旧件进账本、返还进了账号仓。
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
	if nextLedger.Counts[100401592] != 2 {
		t.Fatalf("source crystal not registered: counts=%v", nextLedger.Counts)
	}
	nextAccount, e := ReadAccountMaterials(accountNext)
	if e != nil {
		t.Fatal(e)
	}
	if nextAccount.Count(10415190) != 1 {
		t.Fatalf("refund missing in account materials: %v", nextAccount)
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
	updated, accountNext, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), plan)
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
	updated, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), plan)
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
		nextState, _, _, e := s.PreparePrimerTransform(role, primerAccountRaw(t), again)
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
	updated, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), plan)
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
	_, _, receipt, e := s.PreparePrimerTransform(role, primerAccountRaw(t), plan)
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
