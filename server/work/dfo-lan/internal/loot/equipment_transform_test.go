package loot

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

// transformEquipmentCatalog 载入一份**全量**装备目录，整个测试进程只载一次。
//
// ★ 本仓的全量目录不是单文件：外部包里的 `configs/equipment-full.json` 是 `cmd/equipmentfull`
// 的导出物，本仓没有它 —— 本仓用的是 `configs/equipment-full.index.json` +
// `configs/equipment-full.data` 两件套（.data 326 MB），由 `OpenFullEquipmentCatalog` 打开，
// 再挂到基础目录的 `Full` 字段上（此后 `Definition` / `Reward` 都走全量，见
// internal/inventory/equipment_full.go 的 Definition）。
//
// 加载不便宜（53 MB 的 index 要整体 decode），所以用 sync.Once 摊到整个测试进程；
// 变换的每个用例都要 115 级装备，只有全量目录里才有。
var (
	transformGearOnce sync.Once
	transformGear     *inventory.EquipmentCatalog
	transformGearErr  error
)

func transformEquipmentCatalog() (*inventory.EquipmentCatalog, error) {
	transformGearOnce.Do(func() {
		cat, e := catalog.LoadLoot("../../configs/loot.level150.json")
		if e != nil {
			transformGearErr = fmt.Errorf("load loot catalog: %w", e)
			return
		}
		base, e := inventory.LoadEquipmentCatalog("../../configs/equipment.current37.json", cat.Source.Checksum)
		if e != nil {
			transformGearErr = fmt.Errorf("load base equipment catalog: %w", e)
			return
		}
		full, e := inventory.OpenFullEquipmentCatalog("../../configs/equipment-full", cat.Source.Checksum)
		if e != nil {
			transformGearErr = fmt.Errorf("open full equipment catalog (configs/equipment-full.data): %w", e)
			return
		}
		gear := *base
		gear.Full = full
		transformGear = &gear
	})
	return transformGear, transformGearErr
}

// loadTransformFixtures 载入变换用到的三张真实表。
func loadTransformFixtures(t *testing.T) (*Service, *catalog.EquipmentCreateCost) {
	t.Helper()
	cat, e := catalog.LoadLoot("../../configs/loot.level150.json")
	if e != nil {
		t.Fatalf("load loot catalog: %v", e)
	}
	gear, e := transformEquipmentCatalog()
	if e != nil {
		t.Fatalf("load equipment catalog: %v", e)
	}
	cc, e := catalog.LoadEquipmentCreateCost("../../configs/equipment-create-cost.generated.json", cat.Source.Checksum)
	if e != nil {
		t.Fatalf("load create cost: %v", e)
	}
	return &Service{CreateCost: &cc, Equipment: gear}, &cc
}

// costGroupFor 必须能靠**档位**找到组：实测请求里的目标模板一个都不在
// 任何组的 `[item index]` 里（2026-09-30 复核），组是按 `[grade]`+`[rarity]` 分档的。
func TestCostGroupForFallsBackToGradeRarity(t *testing.T) {
	s, cc := loadTransformFixtures(t)

	// 实测请求里的目标之一：100061157（coat，grade121/rarity4）。
	const target = 100061157
	if _, ok := cc.GroupFor(target); ok {
		t.Fatalf("前提失效：%d 已在某个组的 items 里，这条回退用例不再有意义", target)
	}
	grade, rarity, ok := s.equipmentGradeRarity(target)
	if !ok {
		t.Fatalf("读不到 %d 的档位", target)
	}
	if grade != 121 || rarity != 4 {
		t.Fatalf("%d 的档位应为 (121,4)，实际 (%d,%d)", target, grade, rarity)
	}

	g, ok := s.costGroupFor(target)
	if !ok {
		t.Fatalf("档位回退失败：%d 找不到 [create cost] 组", target)
	}
	// 组 3 是列表里第一个 (121,4) 的组 ⇒ 可复现地落到它。
	if g.Index != 3 {
		t.Fatalf("期望落到组 3（(121,4) 的第一组），实际组 %d", g.Index)
	}
	opt, ok := g.Option(1)
	if !ok || len(opt.Pairs) == 0 {
		t.Fatalf("组 %d 的 [cost] 1 不可用：ok=%v pairs=%d", g.Index, ok, len(opt.Pairs))
	}
	// 组 3 的 cost 1 必须是「登记证 10361515 ×1 + 40000 金币」这一支。
	var gold uint32
	var mats []uint32
	for _, p := range opt.Pairs {
		if p.Gold() {
			gold += p.Amount
			continue
		}
		mats = append(mats, p.Template)
	}
	if gold != 40000 || len(mats) != 1 || mats[0] != 10361515 {
		t.Fatalf("组 3 的 [cost] 1 与源不符：mats=%v gold=%d", mats, gold)
	}
}

// 精确命中优先于档位回退：组内已有的模板不该被改判到别的组。
func TestCostGroupForPrefersExactItem(t *testing.T) {
	s, cc := loadTransformFixtures(t)
	const inGroup = 100051317 // 组 1 的 items[0]（(119,3)）
	if _, ok := cc.GroupFor(inGroup); !ok {
		t.Fatalf("前提失效：%d 不在任何组的 items 里", inGroup)
	}
	g, ok := s.costGroupFor(inGroup)
	if !ok {
		t.Fatalf("%d 找不到组", inGroup)
	}
	want, _ := cc.GroupFor(inGroup)
	if g.Index != want.Index {
		t.Fatalf("精确命中被改判：期望组 %d，实际组 %d", want.Index, g.Index)
	}
}

// 分解的幂等键必须覆盖**整批**：旧实现只取第一件，导致"第一件相同、其余不同"的两批互相
// 顶掉 —— 第二批被当成重放、取回上一批的 receipt，接着触发 `disjoint receipt conflict`、
// **整批被拒**（装备没删、图鉴没登记）。实测 2026-09-30 02:36/02:41 各一次。
func TestDisjointEventKeyCoversWholeBatch(t *testing.T) {
	a := []protocol.DisjointItemEntry{{Slot: 11, Template: 100401598}, {Slot: 12, Template: 100313753}}
	b := []protocol.DisjointItemEntry{{Slot: 11, Template: 100401598}, {Slot: 13, Template: 100201078}}

	ka := disjointEventKey(0xFFFF, a)
	if ka == disjointEventKey(0xFFFF, b) {
		t.Fatal("第一件相同、其余不同的两批必须得到不同 key（否则整批会被当重放拒掉）")
	}
	// 同一批换顺序 ⇒ 同一个 key（顺序不该影响幂等语义）。
	rev := []protocol.DisjointItemEntry{a[1], a[0]}
	if disjointEventKey(0xFFFF, rev) != ka {
		t.Fatal("同一批的不同排列应当是同一个 key")
	}
	// 工具槽参与。
	if disjointEventKey(1, a) == ka {
		t.Fatal("tool 不同应当是不同 key")
	}
	// 旧实现（只看第一件）在这两批上会相等 —— 显式钉住这条教训。
	if len(a) != len(b) || a[0] != b[0] {
		t.Fatal("用例前提变了：两批应当第一件相同、其余不同")
	}
	if len(ka) > 64 || len(ka) == 0 {
		t.Fatalf("key 长度不合理：%d", len(ka))
	}
}

// 武器 / 誓约的档位在 `[create cost]` 里根本不存在（那张表只有 (119,3)/(120,6)/(121,4) 三档、
// 只覆盖 11 个防具首饰部位 + 融合石）。但官方规则明确要求「武器页签里能对所有分解过的武器做
// 装备变换」，所以 `costGroupFor` 必须按 rarity 兜底，且**不能**改判防具那种能精确命中的。
func TestCostGroupForWeaponAndOathFallback(t *testing.T) {
	s, cc := loadTransformFixtures(t)

	for _, tpl := range []uint32{117010280 /*武器 [weapon]*/, 100610079 /*誓约核心 [oath]*/, 100401606 /*星蕴石 [primer]*/} {
		if _, ok := cc.GroupFor(tpl); ok {
			t.Fatalf("前提失效：%d 已在组的 items 里", tpl)
		}
		g, ok := s.costGroupFor(tpl)
		if !ok {
			t.Fatalf("%d 应能靠 rarity 兜底找到档位（武器/誓约必须可变换）", tpl)
		}
		t.Logf("%d -> 组 %d", tpl, g.Index)
	}

	// 武器 rarity 8 ⇒ 表里最高 rarity 是 6 ⇒ 落到 (120,6) 那一档 = 组 2。
	if g, _ := s.costGroupFor(117010280); g.Index != 2 {
		t.Fatalf("武器应落到组 2（(120,6)），实际组 %d", g.Index)
	}
	// 防具 (121,4) 仍精确命中组 3，不被兜底改判。
	if g, _ := s.costGroupFor(100051282); g.Index != 3 {
		t.Fatalf("防具被改判到组 %d（应为 3）", g.Index)
	}
}

// `CommitAccountMaterialEvent` 整批拒绝（`invalid account material event`，2026-09-30 01:16 实测）。
func TestTransformKeyFitsEventKeyLimit(t *testing.T) {
	slots := []uint32{14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 25}
	templates := []uint32{100061157, 100161038, 100111127, 100261101, 100211061,
		100301815, 100313518, 100323408, 100345953, 100354128, 100391006}
	k := transformKey(slots, templates, json.RawMessage(`{"v":1}`))
	if k == "" {
		t.Fatal("event_key 不能为空")
	}
	if len(k) > 200 {
		t.Fatalf("event_key 超长（storage 会拒绝）：%d 字节", len(k))
	}
	if again := transformKey(slots, templates, json.RawMessage(`{"v":1}`)); again != k {
		t.Fatalf("同一输入应得到同一 key：%q vs %q", k, again)
	}
	if other := transformKey(slots, templates, json.RawMessage(`{"v":2}`)); other == k {
		t.Fatal("前置状态变了，key 必须跟着变（否则合法重换会被当成重放）")
	}
	if diff := transformKey(slots, []uint32{100061157}, json.RawMessage(`{"v":1}`)); diff == k {
		t.Fatal("请求内容不同，key 必须不同")
	}
}

// 换装只改 `Template` 与 `Durability`：**打造效果必须原样保留**。
//
// 打造效果存在 181 字节的 `Record` 里：offset 10 = 等级字节（强化/增幅共用，bit0-4），
// offset 19 = 次元属性类型，offset 20 = 次元属性数值。早先版本清空了 `Record`
// ⇒ 等于把强化/增幅/附魔全抹掉（2026-09-30 修正，见 applyTransform 注释）。
func TestApplyTransformKeepsBuildEffects(t *testing.T) {
	s, _ := loadTransformFixtures(t)

	const (
		from = 100051282 // 实测身上穿的 coat（cloth）
		to   = 100061157 // 实测请求里的目标 coat（leather）
	)
	rec := make([]byte, protocol.CurrentItemRecordSize)
	binary.LittleEndian.PutUint32(rec[2:], from) // 行内 offset 2..6 = 模板号（ValidateRecord 会断言）
	rec[10] = 0x07                               // +7（bit0-4 = 等级，bit5-7 = 再封装次数）
	rec[19] = 1                                  // 次元属性类型（红字）
	rec[20] = 42                                 // 次元属性数值

	bag := inventory.Bag{
		Worn: []inventory.BagEquipment{
			{Slot: 14, Template: from, Durability: 60, Refine: 7,
				Record: rec, AvatarOptions: []byte{9, 9}, AvatarSockets: []byte{8}},
			{Slot: 19, Template: 100301825, Durability: 0},
		},
	}
	out, e := s.applyTransform(bag, []EquipmentTransformPair{{Slot: 14, From: from, To: to, Group: 3}})
	if e != nil {
		t.Fatalf("applyTransform: %v", e)
	}

	wantDurability, e := s.Equipment.Reward(to)
	if e != nil {
		t.Fatalf("Reward(%d): %v", to, e)
	}

	var got inventory.BagEquipment
	for _, w := range out.Worn {
		if w.Slot == 14 {
			got = w
		}
		if w.Slot == 19 && w.Template != 100301825 {
			t.Fatalf("无关槽 19 被改了：%d", w.Template)
		}
	}
	if got.Template != to {
		t.Fatalf("模板没换：%d", got.Template)
	}
	if got.Durability != wantDurability {
		t.Fatalf("耐久应按新模板重置：期望 %d，实际 %d", wantDurability, got.Durability)
	}
	// ★ 打造效果：一个字节都不能丢。
	if len(got.Record) != protocol.CurrentItemRecordSize {
		t.Fatalf("Record 被丢了（长度 %d）：%v", len(got.Record), got.Record)
	}
	// ★ 行内模板号必须同步 —— 否则 `ValidateRecord` 报 equipment instance template
	//   mismatch，服务端**整个角色都读不出来**（2026-09-30 02:10 实机教训：选角界面
	//   一个角色都不显示）。
	if in := binary.LittleEndian.Uint32(got.Record[2:]); in != to {
		t.Fatalf("Record 里的模板号没同步：%d（应为 %d）", in, to)
	}
	if e := got.ValidateRecord(); e != nil {
		t.Fatalf("换装后 ValidateRecord 必须通过：%v", e)
	}
	if got.Record[10] != 0x07 {
		t.Fatalf("强化/增幅等级被抹掉：offset10=%#x", got.Record[10])
	}
	if got.Record[19] != 1 || got.Record[20] != 42 {
		t.Fatalf("次元属性被抹掉：type=%d value=%d", got.Record[19], got.Record[20])
	}
	if got.Refine != 7 {
		t.Fatalf("锻造等级应保留：期望 7，实际 %d", got.Refine)
	}
	if len(got.AvatarOptions) != 2 || len(got.AvatarSockets) != 1 {
		t.Fatalf("Avatar* 不该被清：opts=%v sockets=%v", got.AvatarOptions, got.AvatarSockets)
	}

	// 原 Bag 不能被就地改动（applyTransform 先拷切片）。
	for _, w := range bag.Worn {
		if w.Slot == 14 && w.Template != from {
			t.Fatalf("原 Bag 被就地改了：%d", w.Template)
		}
	}
}

// 方案「甲」：**换下去的源装备必须被登记进图鉴**，否则它"换出去即消失"、再也选不回来。
// （实机 2026-09-30 01:31：11 件里 A 套只有 3 件在 counts 里 ⇒ 只有那 3 件能换回。）
func TestRegisterTransformedSourcesAddsSource(t *testing.T) {
	s, _ := loadTransformFixtures(t)
	cat, e := catalog.LoadLoot("../../configs/loot.level150.json")
	if e != nil {
		t.Fatalf("load loot catalog: %v", e)
	}
	rules, e := catalog.LoadEquipmentJournalRules("../../configs/equipment-journal.generated.json", cat.Source.Checksum)
	if e != nil {
		t.Fatalf("load journal rules: %v", e)
	}
	ledger, e := inventory.ReadEquipmentJournal(json.RawMessage(`{}`))
	if e != nil {
		t.Fatalf("empty journal: %v", e)
	}
	pairs := []EquipmentTransformPair{{Slot: 14, From: 100051282, To: 100061157, Group: 3}}

	next := registerTransformedSources(s.Equipment, &rules, ledger, pairs)
	if next.Counts[100051282] != 1 {
		t.Fatalf("源装备没被登记：counts=%v", next.Counts)
	}
	// 再来一次（换出去又换回来）应累加，而不是停在 1。
	again := registerTransformedSources(s.Equipment, &rules, next, pairs)
	if again.Counts[100051282] != 2 {
		t.Fatalf("第二次没累加：counts=%v", again.Counts)
	}
	// 目标不该被误登记（只有"源"进池子）。
	if again.Counts[100061157] != 0 {
		t.Fatalf("目标不该被登记：%v", again.Counts)
	}
	// From == To 时跳过。
	noop := registerTransformedSources(s.Equipment, &rules, ledger,
		[]EquipmentTransformPair{{Slot: 14, From: 100051282, To: 100051282, Group: 3}})
	if noop.Counts[100051282] != 0 {
		t.Fatalf("From==To 不该登记：%v", noop.Counts)
	}
}

// 源在背包（客户端允许把背包里的装备放进界面「变换前」槽，请求只带部位码）：
// 目标应被**穿上**到该部位，而该部位原来那件（若有）退回源所在的背包格。
// 实机 2026-09-30 03:24 的失败就是这条没实现 —— 源在 backpack，旧代码只查 worn。
func TestApplyTransformWithBagSource(t *testing.T) {
	s, _ := loadTransformFixtures(t)
	s.WearRules = inventory.WearRules{Slots: map[string]uint16{
		"[weapon]": 12, "[coat]": 14, "[amulet]": 19,
	}}

	const (
		wornCoat  = 100051282 // 身上穿的 coat
		bagWeapon = 117010280 // 背包里的武器（源）
		target    = 117010253 // 变换目标
	)

	// 情形 A：该部位（12）本来空着 ⇒ 源直接穿上，背包那一格腾出。
	bagA := inventory.Bag{
		Worn:      []inventory.BagEquipment{{Slot: 14, Template: wornCoat, Durability: 60}},
		Equipment: []inventory.BagEquipment{{Slot: 11, Template: bagWeapon, Durability: 100}},
	}
	outA, e := s.applyTransform(bagA, []EquipmentTransformPair{
		{Slot: 12, From: bagWeapon, To: target, Group: 2, FromBag: true, BagSlot: 11},
	})
	if e != nil {
		t.Fatalf("applyTransform A: %v", e)
	}
	var worn12 bool
	for _, w := range outA.Worn {
		if w.Slot == 12 {
			worn12 = true
			if w.Template != target {
				t.Fatalf("A: 槽 12 应为目标 %d，实际 %d", target, w.Template)
			}
		}
		if w.Slot == 14 && w.Template != wornCoat {
			t.Fatalf("A: 无关槽 14 被改了：%d", w.Template)
		}
	}
	if !worn12 {
		t.Fatal("A: 目标没有穿到槽 12")
	}
	for _, x := range outA.Equipment {
		if x.Slot == 11 {
			t.Fatalf("A: 源所在的背包格 11 应已腾出，实际还有 %d", x.Template)
		}
	}

	// 情形 B：该部位已被占用 ⇒ 目标穿上、原件退回源那一格（交换）。
	bagB := inventory.Bag{
		Worn:      []inventory.BagEquipment{{Slot: 12, Template: 117010280, Durability: 50}},
		Equipment: []inventory.BagEquipment{{Slot: 11, Template: bagWeapon, Durability: 100}},
	}
	outB, e := s.applyTransform(bagB, []EquipmentTransformPair{
		{Slot: 12, From: bagWeapon, To: target, Group: 2, FromBag: true, BagSlot: 11},
	})
	if e != nil {
		t.Fatalf("applyTransform B: %v", e)
	}
	for _, w := range outB.Worn {
		if w.Slot == 12 && w.Template != target {
			t.Fatalf("B: 槽 12 应为目标 %d，实际 %d", target, w.Template)
		}
	}
	var returned bool
	for _, x := range outB.Equipment {
		if x.Slot == 11 {
			returned = true
			if x.Template != 117010280 {
				t.Fatalf("B: 换下来的那件应退回槽 11，实际 %d", x.Template)
			}
		}
	}
	if !returned {
		t.Fatal("B: 换下来的那件没退回背包")
	}
}

// transformSource 的三条路径：身上 → 背包（按部位类型）→ 同部位多件时放弃。
func TestTransformSourcePrefersWornThenBag(t *testing.T) {
	s, _ := loadTransformFixtures(t)
	s.WearRules = inventory.WearRules{Slots: map[string]uint16{"[weapon]": 12, "[coat]": 14}}

	// 身上有 ⇒ 用它，且 bagSlot=0。
	onBody := inventory.Bag{
		Worn:      []inventory.BagEquipment{{Slot: 12, Template: 117010280}},
		Equipment: []inventory.BagEquipment{{Slot: 11, Template: 117010253}},
	}
	if tpl, bagSlot, ok := s.transformSource(onBody, 12); !ok || tpl != 117010280 || bagSlot != 0 {
		t.Fatalf("身上有装备时应优先用它：tpl=%d bagSlot=%d ok=%v", tpl, bagSlot, ok)
	}

	// 身上没有 ⇒ 从背包按部位类型找（[weapon] → 槽 12）。
	inBag := inventory.Bag{Equipment: []inventory.BagEquipment{{Slot: 11, Template: 117010280}}}
	if tpl, bagSlot, ok := s.transformSource(inBag, 12); !ok || tpl != 117010280 || bagSlot != 11 {
		t.Fatalf("身上没有时应从背包找：tpl=%d bagSlot=%d ok=%v", tpl, bagSlot, ok)
	}

	// 背包里同部位有两件 ⇒ 无法确定，放弃。
	ambiguous := inventory.Bag{Equipment: []inventory.BagEquipment{
		{Slot: 11, Template: 117010280}, {Slot: 20, Template: 117010253},
	}}
	if _, _, ok := s.transformSource(ambiguous, 12); ok {
		t.Fatal("同部位多件时应当放弃（不能猜）")
	}

	// 部位映射缺失 ⇒ 放弃。
	if _, _, ok := s.transformSource(inBag, 99); ok {
		t.Fatal("部位映射缺失时应当放弃")
	}
}

// 装备变换的成本口径 = 客户端「变换确认」界面：**目标稀有度 → 对应灵魂 ×1** + 固定金币。
// （源里的 `[create cost]` 只按 (grade,rarity) 分三档、没有太初档，武器永远匹配不到，
// 所以变换不走那张表。）
func TestTransformCostByRarity(t *testing.T) {
	s, _ := loadTransformFixtures(t)

	// 实测目标：117010280 / 117010253 都是 rarity 8（太初）⇒ 客户端界面写「1 太初(s)」。
	if soul, gold, e := s.transformCost(117010280); e != nil || soul != 10361516 || gold != transformGoldCost {
		t.Fatalf("rarity8 应为太初灵魂 10361516 + %d 金币，实际 soul=%d gold=%d err=%v",
			transformGoldCost, soul, gold, e)
	}
	// 五个稀有度一对一。
	for rarity, want := range map[int32]uint32{2: 10361512, 3: 10361513, 4: 10361514, 6: 10361515, 8: 10361516} {
		if got, ok := soulFor(rarity); !ok || got != want {
			t.Fatalf("rarity %d 应映射到 %d，实际 %d ok=%v", rarity, want, got, ok)
		}
	}
	// 表外的稀有度必须明确拒绝，不能猜。
	for _, rarity := range []int32{0, 1, 5, 7, 9} {
		if _, ok := soulFor(rarity); ok {
			t.Fatalf("rarity %d 不该有映射", rarity)
		}
	}
	// 灵魂必须都在**账号材料槽**里（否则变换扣不到）。
	for _, tpl := range []uint32{10361512, 10361513, 10361514, 10361515, 10361516} {
		if _, ok := inventory.AccountMaterialSlot(tpl); !ok {
			t.Fatalf("灵魂 %d 不在账号材料槽里，变换会扣不到", tpl)
		}
	}
}
