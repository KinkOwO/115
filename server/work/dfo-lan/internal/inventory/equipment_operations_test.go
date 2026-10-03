package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"
)

// 「装备生成」的成本挑选：**玩家在窗口里点的是哪一支，就扣哪一支**。
//
// ★ 分仓：三档登记证 `10361512`~`10361515` 是**账号共享材料（灵魂仓库 / 容器 35）**，
// 必须从 `AccountMaterials` 扣、而不是从背包 —— 实机 `have 0` 之谜就是没做这件事。
// ★ 不回退：`payOption` 来自请求头 `[13]`。实机 2026-09-29 13:52 那笔（`[13]=2`，
// 玩家选的是**巡礼之印**）被旧实现"挑第一支付得起的"错扣成了 35,000 金币。
func TestPickCraftCost(t *testing.T) {
	// 组 2 的真实两支（源自 configs/equipment-create-cost.generated.json）：
	//   1 = 登记证 10361514×1 + 金币 35000；2 = 登记证 10361514×1 + 巡礼之印 10401346×7
	group := catalog.CreateCostGroup{
		Index: 2,
		Items: []uint32{100391056},
		Costs: []catalog.CreateCostOption{
			{Number: 1, Pairs: []catalog.CreateCostItem{
				{Template: 10361514, Amount: 1},
				{Template: 0, Amount: 35000},
			}},
			{Number: 2, Pairs: []catalog.CreateCostItem{
				{Template: 10361514, Amount: 1},
				{Template: 10401346, Amount: 7},
			}},
		},
	}
	empty := NewAccountMaterials()
	acct := empty
	var e error
	if acct, _, e = acct.Add(10361514, 5); e != nil {
		t.Fatal(e)
	}

	// 1) 选 cost 1（登记证 + 金币）：登记证在**账号仓库**、金币也够 ⇒ 材料标记 FromAccount。
	rich := Bag{Gold: 40000}
	opt, gold, bagMats, acctMats, e := pickCraftCost(rich, acct, group, 1)
	if e != nil {
		t.Fatal(e)
	}
	if opt != 1 || gold != 35000 {
		t.Fatalf("rich: option=%d gold=%d, want 1/35000", opt, gold)
	}
	if len(bagMats) != 0 {
		t.Fatalf("rich: bag materials should be empty, got %+v", bagMats)
	}
	if len(acctMats) != 1 || acctMats[0].Template != 10361514 || acctMats[0].Count != 1 {
		t.Fatalf("rich: account materials = %+v", acctMats)
	}

	// 2) ★ 选 cost 2（登记证 + 巡礼之印）：必须扣背包里的 `10401346`×7，**不碰金币**。
	seals := Bag{Gold: 40000, Items: []BagItem{
		{Slot: 127, Template: 10401346, Amount: 120},
	}}
	opt, gold, bagMats, acctMats, e = pickCraftCost(seals, acct, group, 2)
	if e != nil {
		t.Fatal(e)
	}
	if opt != 2 || gold != 0 {
		t.Fatalf("seals: option=%d gold=%d, want 2/0", opt, gold)
	}
	if len(bagMats) != 1 || bagMats[0].Template != 10401346 || bagMats[0].Count != 7 {
		t.Fatalf("seals: bag materials = %+v", bagMats)
	}
	if len(acctMats) != 1 || acctMats[0].Template != 10361514 {
		t.Fatalf("seals: account materials = %+v", acctMats)
	}

	// 3) ★ 选了 cost 2、金币管够但巡礼之印不够 ⇒ **必须拒绝，不许回退 cost 1**。
	poorSeals := Bag{Gold: 999999, Items: []BagItem{
		{Slot: 127, Template: 10401346, Amount: 3},
	}}
	if _, _, _, _, e = pickCraftCost(poorSeals, acct, group, 2); e == nil {
		t.Fatalf("cost 2 with only 3 seals must be refused, not silently downgraded to cost 1")
	}

	// 4) ★ 选了 cost 1、巡礼之印管够但金币不够 ⇒ 也必须拒绝（不许自动换 cost 2）。
	//    这正是实机 13:52 的错扣形态反过来：不许用"另一支付得起"来顶替玩家的选择。
	poorGold := Bag{Gold: 100, Items: []BagItem{
		{Slot: 127, Template: 10401346, Amount: 120},
	}}
	if _, _, _, _, e = pickCraftCost(poorGold, acct, group, 1); e == nil {
		t.Fatalf("cost 1 with 100 gold must be refused, not silently downgraded to cost 2")
	}

	// 5) ★ 登记证**只在背包里**（旧实机的情形）：必须拒绝 —— 背包里那份其实属于账号仓库，
	//    客户端不会拿背包那份去付（`SweepAccountMaterials` 迟早把它搬走）。
	bagOnly := Bag{Gold: 40000, Items: []BagItem{
		{Slot: 124, Template: 10361514, Amount: 5},
	}}
	if _, _, _, _, e := pickCraftCost(bagOnly, empty, group, 1); e == nil {
		t.Fatalf("bag-only ticket must be refused (regression: 实机 have 0)")
	}

	// 6) 付法序号不存在 ⇒ 拒绝，并点名可用的序号（不许猜一支扣下去）。
	if _, _, _, _, e = pickCraftCost(rich, acct, group, 3); e == nil {
		t.Fatalf("unknown pay option must be refused")
	} else if !contains(e.Error(), "no cost option 3") {
		t.Fatalf("refusal should name the requested option, got %v", e)
	}
}

// 金币行必须被识别成金币（模板 0），别当成物品 0 去扣。
func TestCreateCostGoldRow(t *testing.T) {
	if !(catalog.CreateCostItem{Template: 0, Amount: 1}).Gold() {
		t.Fatalf("template 0 must be gold")
	}
	if (catalog.CreateCostItem{Template: 10361513, Amount: 1}).Gold() {
		t.Fatalf("material must not be gold")
	}
}

// 三档登记证必须被认成账号共享材料 —— 这是分仓的判据本身。
func TestCraftTicketsAreAccountMaterials(t *testing.T) {
	for _, tpl := range []uint32{10361512, 10361513, 10361514, 10361515, 10361516} {
		if _, ok := AccountMaterialSlot(tpl); !ok {
			t.Fatalf("template %d must be an account-shared material", tpl)
		}
	}
	// 反过来：10401346 不是（它留在背包里扣）。
	if _, ok := AccountMaterialSlot(10401346); ok {
		t.Fatalf("10401346 must stay in the ordinary bag")
	}
}

func contains(hay, needle string) bool {
	return indexOf(hay, needle) >= 0
}

func indexOf(hay, needle string) int {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// ★ 幂等键必须在**每次成功之后都变** —— 否则同物二次生成会被当成重放静默吞掉
// （实机 2026-09-29 14:21：两笔全被吞，日志却是 DONE + 全零回执）。
func TestCraftEventKeyChangesPerAttempt(t *testing.T) {
	before := json.RawMessage(`{"bag":{"gold":100}}`)
	after := json.RawMessage(`{"bag":{"gold":65}}`)

	if craftEventKey(100391056, 25, 2, before) == craftEventKey(100391056, 25, 2, after) {
		t.Fatalf("key must differ when the character state differs, else the 2nd craft is swallowed")
	}
	// 同状态同请求 ⇒ 同键：重复帧仍然是幂等的（不会扣两次）。
	if craftEventKey(100391056, 25, 2, before) != craftEventKey(100391056, 25, 2, before) {
		t.Fatalf("key must be stable for the same state, else a duplicate frame double-charges")
	}
	// 不同物/槽/档必须分开。
	if craftEventKey(100391056, 25, 2, before) == craftEventKey(100354178, 23, 2, before) {
		t.Fatalf("key must include template and slot")
	}
}

// transformEquipmentCatalog uses the historical flow fixture.
// It contains all explicit test anchors and unchanged create-cost group members;
// complete native bindings and definition parity are checked separately.
func transformEquipmentCatalog(t *testing.T) (*EquipmentCatalog, error) {
	t.Helper()
	cat, err := catalog.LoadLoot(testfixture.LootLevel150Path(t))
	if err != nil {
		return nil, fmt.Errorf("load loot catalog: %w", err)
	}
	base, err := LoadEquipmentCatalog("../../configs/equipment.current37.json", cat.Source.Checksum)
	if err != nil {
		return nil, fmt.Errorf("load base equipment catalog: %w", err)
	}
	full, err := OpenFullEquipmentCatalog("../inventory/testdata/equipment-flow", cat.Source.Checksum)
	if err != nil {
		return nil, fmt.Errorf("open equipment flow fixture: %w", err)
	}
	gear := *base
	gear.Full = full
	return &gear, nil
}

// loadTransformFixtures 载入变换用到的三张真实表。
func loadTransformFixtures(t *testing.T) (*ItemService, *catalog.EquipmentCreateCost) {
	t.Helper()
	cat, e := catalog.LoadLoot(testfixture.LootLevel150Path(t))
	if e != nil {
		t.Fatalf("load loot catalog: %v", e)
	}
	gear, e := transformEquipmentCatalog(t)
	if e != nil {
		t.Fatalf("load equipment catalog: %v", e)
	}
	cc, e := catalog.LoadEquipmentCreateCost("../../configs/equipment-create-cost.generated.json", cat.Source.Checksum)
	if e != nil {
		t.Fatalf("load create cost: %v", e)
	}
	return &ItemService{CreateCost: &cc, Equipment: gear}, &cc
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

	bag := Bag{
		Worn: []BagEquipment{
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

	var got BagEquipment
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
	cat, e := catalog.LoadLoot(testfixture.LootLevel150Path(t))
	if e != nil {
		t.Fatalf("load loot catalog: %v", e)
	}
	rules, e := catalog.LoadEquipmentJournalRules("../../configs/equipment-journal.generated.json", cat.Source.Checksum)
	if e != nil {
		t.Fatalf("load journal rules: %v", e)
	}
	ledger, e := ReadEquipmentJournal(json.RawMessage(`{}`))
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
	s.WearRules = WearRules{Slots: map[string]uint16{
		"[weapon]": 12, "[coat]": 14, "[amulet]": 19,
	}}

	const (
		wornCoat  = 100051282 // 身上穿的 coat
		bagWeapon = 117010280 // 背包里的武器（源）
		target    = 117010253 // 变换目标
	)

	// 情形 A：该部位（12）本来空着 ⇒ 源直接穿上，背包那一格腾出。
	bagA := Bag{
		Worn:      []BagEquipment{{Slot: 14, Template: wornCoat, Durability: 60}},
		Equipment: []BagEquipment{{Slot: 11, Template: bagWeapon, Durability: 100}},
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
	bagB := Bag{
		Worn:      []BagEquipment{{Slot: 12, Template: 117010280, Durability: 50}},
		Equipment: []BagEquipment{{Slot: 11, Template: bagWeapon, Durability: 100}},
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
	s.WearRules = WearRules{Slots: map[string]uint16{"[weapon]": 12, "[coat]": 14}}

	// 身上有 ⇒ 用它，且 bagSlot=0。
	onBody := Bag{
		Worn:      []BagEquipment{{Slot: 12, Template: 117010280}},
		Equipment: []BagEquipment{{Slot: 11, Template: 117010253}},
	}
	if tpl, bagSlot, ok := s.transformSource(onBody, 12); !ok || tpl != 117010280 || bagSlot != 0 {
		t.Fatalf("身上有装备时应优先用它：tpl=%d bagSlot=%d ok=%v", tpl, bagSlot, ok)
	}

	// 身上没有 ⇒ 从背包按部位类型找（[weapon] → 槽 12）。
	inBag := Bag{Equipment: []BagEquipment{{Slot: 11, Template: 117010280}}}
	if tpl, bagSlot, ok := s.transformSource(inBag, 12); !ok || tpl != 117010280 || bagSlot != 11 {
		t.Fatalf("身上没有时应从背包找：tpl=%d bagSlot=%d ok=%v", tpl, bagSlot, ok)
	}

	// 背包里同部位有两件 ⇒ 无法确定，放弃。
	ambiguous := Bag{Equipment: []BagEquipment{
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
		if _, ok := AccountMaterialSlot(tpl); !ok {
			t.Fatalf("灵魂 %d 不在账号材料槽里，变换会扣不到", tpl)
		}
	}
}

// fakeEquipmentCatalog 以**接口**形式注入装备定义。
//
// 本测试在 loot 包内，够不到 EquipmentCatalog 的未导出索引（index 是小写），
// 而 JournalLimit 只要求 EquipmentDefinitioner —— 正好可以用最小实现顶上。
// 生产路径传的是服务端真实目录（Service.Equipment），同一套判据。
type fakeEquipmentCatalog map[uint32]EquipmentDefinition

func (f fakeEquipmentCatalog) Definition(id uint32) (EquipmentDefinition, error) {
	d, ok := f[id]
	if !ok {
		return d, fmt.Errorf("equipment definition missing: %d", id)
	}
	return d, nil
}

// journalDef 造一条装备定义。收录判据只读三个字段：
// [minimum level]（必须 == 115）、[rarity]（必须在 {2,3,4,6,8}）、[equipment type]（收紧上限用）。
func journalDef(id uint32, minimumLevel, rarity int32, kind string) EquipmentDefinition {
	fields := map[string][]pvf.Token{
		"[minimum level]": {{Type: 0, Value: minimumLevel}},
		"[rarity]":        {{Type: 0, Value: rarity}},
	}
	if kind != "" {
		fields["[equipment type]"] = []pvf.Token{{Type: 3, Text: kind}}
	}
	return EquipmentDefinition{ID: id, Fields: fields}
}

// 规格 CMD/0026-DISJOINTITEM：CMD26「分解」同时就是客户端的「装备库添加」。
// 玩家 2026-09-30 报告里那只耳环 100391006（minimum level 115 / rarity 6）正是该被收录的那类。
func TestJournalRegistrationsAddsDeletedEquipment(t *testing.T) {
	rules := catalog.EquipmentJournalRules{Maximum: 99}
	cat := fakeEquipmentCatalog{
		100391006: journalDef(100391006, 115, 6, "[earring]"),
		100051285: journalDef(100051285, 115, 2, ""),
	}
	// 登记用的模板来自**服务端背包**那一行（bySlot 由 Disjoint 之前的背包快照构建），
	// 不是请求里客户端上报的 Template。
	bySlot := map[uint16]uint32{12: 100391006, 13: 100051285}

	ledger, added, skipped, e := journalRegistrations(
		EquipmentJournal{}, bySlot, []uint16{12, 13}, cat, &rules)
	if e != nil {
		t.Fatalf("registrations: %v", e)
	}
	if len(skipped) != 0 {
		t.Fatalf("unexpected skips: %+v", skipped)
	}
	if len(added) != 2 {
		t.Fatalf("added = %+v, want 2", added)
	}
	if ledger.Counts[100391006] != 1 || ledger.Counts[100051285] != 1 {
		t.Fatalf("counts = %+v, want both 1", ledger.Counts)
	}
	// 上限回落：[max equipment count by equipment type] 在源里只显式声明了 `[oath]`，
	// 所以 `[earring]` 这类走普通上限 99（一度被"过度收紧"成拒绝，见 catalog 的注释）。
	for _, r := range added {
		if r.Limit != 99 || r.After != 1 {
			t.Fatalf("registration = %+v, want limit 99 after 1", r)
		}
	}
}

// 达上限**只跳过收录、不拒绝分解**；背包里查不到那一行也必须有可见原因（不静默放行）。
func TestJournalRegistrationsSkipsWithoutFailing(t *testing.T) {
	rules := catalog.EquipmentJournalRules{
		Maximum:       99,
		MaximumByType: []catalog.JournalTypeLimit{{Kind: "[oath]", Rarity: 2, Maximum: 1}},
	}
	cat := fakeEquipmentCatalog{
		100051285: journalDef(100051285, 115, 2, ""),       // 普通 ⇒ 上限 99
		900000001: journalDef(900000001, 115, 2, "[oath]"), // 誓约 ⇒ 按类型收紧到 1
		100401610: journalDef(100401610, 110, 2, ""),       // 等级不是 115 ⇒ 不可登记
	}
	ledger := EquipmentJournal{Counts: map[uint32]uint32{
		100051285: 99, // 已满
		900000001: 1,  // 誓约上限 1，已满
	}}
	bySlot := map[uint16]uint32{10: 100051285, 20: 900000001, 30: 100401610}
	// 31 号槽故意不在 bySlot 里：模拟"背包快照与删除结果对不上"。
	slots := []uint16{10, 20, 30, 31}

	next, added, skipped, e := journalRegistrations(ledger, bySlot, slots, cat, &rules)
	if e != nil {
		t.Fatalf("cap must not fail the disassembly: %v", e)
	}
	if len(added) != 0 {
		t.Fatalf("added = %+v, want none", added)
	}
	want := []struct {
		slot   uint16
		reason string
	}{
		{10, "cap reached"},
		{20, "cap reached"},
		{30, "not registrable"},
		{31, "no bag row"},
	}
	if len(skipped) != len(want) {
		t.Fatalf("skipped = %+v, want %d entries", skipped, len(want))
	}
	for i, w := range want {
		if skipped[i].Slot != w.slot || skipped[i].Reason != w.reason {
			t.Fatalf("skip[%d] = %+v, want slot %d reason %q", i, skipped[i], w.slot, w.reason)
		}
	}
	if next.Counts[100051285] != 99 || next.Counts[900000001] != 1 || len(next.Counts) != 2 {
		t.Fatalf("ledger must be untouched: %+v", next.Counts)
	}
}

// 规则表 / 装备目录没装 ⇒ 不收录、不报错、也**不记 skip**：整条特性是关的，
// 不是"这一件被跳过"（否则回执里会刷满假的跳过原因）。
func TestJournalRegistrationsDisabledWithoutRulesOrCatalog(t *testing.T) {
	bySlot := map[uint16]uint32{12: 100391006}
	cat := fakeEquipmentCatalog{100391006: journalDef(100391006, 115, 6, "[earring]")}
	slots := []uint16{12}

	next, added, skipped, e := journalRegistrations(
		EquipmentJournal{}, bySlot, slots, cat, nil)
	if e != nil || len(added) != 0 || len(skipped) != 0 || len(next.Counts) != 0 {
		t.Fatalf("nil rules: next=%+v added=%+v skipped=%+v err=%v", next, added, skipped, e)
	}

	next, added, skipped, e = journalRegistrations(
		EquipmentJournal{}, bySlot, slots, nil, &catalog.EquipmentJournalRules{Maximum: 99})
	if e != nil || len(added) != 0 || len(skipped) != 0 || len(next.Counts) != 0 {
		t.Fatalf("nil catalog: next=%+v added=%+v skipped=%+v err=%v", next, added, skipped, e)
	}
}
