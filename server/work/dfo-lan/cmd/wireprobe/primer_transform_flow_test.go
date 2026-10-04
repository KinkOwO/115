package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"dfolan/internal/database"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// CMD2381 装备库「誓约 / 晶体变换」的**端到端**回归（协议 → 计划 → 事务 → 刷新包）。
//
// 用专用 PostgreSQL 测试库（`DFO_TEST_POSTGRES_DSN`，临时 schema，跑完即删），
// 不接触玩家库。跳过条件：未设置 DSN。
func TestPrimerTransformFlowIntegration(t *testing.T) {
	dsn := os.Getenv("DFO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DFO_TEST_POSTGRES_DSN requires a dedicated PostgreSQL test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := database.Open(ctx, database.Config{PostgresDSN: dsn, MaxConnections: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("primer_transform_%d", time.Now().UnixNano())
	if err := admin.DiagnosticExec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := admin.DiagnosticExec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	store, err := database.Open(ctx, database.Config{PostgresDSN: dsn, PostgresSchema: schema, MaxConnections: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{
		store.Migrate, store.MigrateCharacterEvents, store.MigrateAccountMaterials, store.MigrateOathOptions,
	} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "primer-transform-fixture")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("c", 64)
	equipment, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: version},
		Rows: []inventory.EquipmentDefinition{
			primerDef(100401592, 2), // 微光星蕴石（通用基准件）
			primerDef(100401597, 6), // 同族传说档
			primerDef(100401599, 8), // 太初档（只能进 44..46）
		},
	}, version)
	if err != nil {
		t.Fatal(err)
	}
	transform, err := catalog.ParseEquipmentTransformSystem(primerTransformFlowTable)
	if err != nil {
		t.Fatal(err)
	}
	journal := catalog.EquipmentJournalRules{Maximum: 99}
	items := &inventory.ItemService{
		Model:     "primer-transform-test",
		Catalog:   catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}},
		Equipment: equipment,
		Journal:   &journal,
		Transform: &transform,
		BagRules:  inventory.BagRules{MissingStackLimit: 2147483647},
	}

	// 穿戴槽 36 = 稀有晶体（会被换下去 ⇒ 登记 + 返还微光灵魂），装备库里有传说档可换。
	raw, err := inventory.SaveBag(json.RawMessage(`{"level":115,"advancement":0}`),
		inventory.Bag{Version: "ordinary-bag-v1", Gold: 100_000,
			Worn: []inventory.BagEquipment{{Slot: 36, Template: 100401592}}})
	if err != nil {
		t.Fatal(err)
	}
	raw, err = inventory.SaveEquipmentJournal(raw, inventory.EquipmentJournal{
		Counts: map[uint32]uint32{100401592: 1, 100401597: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{
		AccountID: account, Name: "primer-transform", Profession: 11,
		ConfigVersion: savecontract.Identity(), Request: []byte{0}, State: raw,
	}, 24)
	if err != nil {
		t.Fatal(err)
	}

	w := &worldSession{account: account, store: store, role: role, items: items}
	plan, err := w.primerTransform(primerTransformFrame(t, 0, 100401597), nil)
	if err != nil {
		t.Fatalf("primer transform: %v", err)
	}
	if len(plan) == 0 {
		t.Fatal("no reply packet")
	}
	// 回包：kind 1、opcode 2381、**窗口字节 0**（窗口 2145 = EquipmentTransformWindow）。
	reply := plan[0]
	if reply.Kind != 1 || reply.ID != protocol.PrimerTransformOpcode {
		t.Fatalf("reply = %+v, want kind 1 id 2381", reply)
	}
	if want := []byte{1, 0, 0, 0, 0, 0, 0}; string(reply.Payload) != string(want) {
		t.Fatalf("reply payload = % x, want % x", reply.Payload, want)
	}
	// 刷新包：穿戴（14）、背包（13）、装备库账本（2610）都必须补发 —— handler 自己不回请。
	have := map[uint16]bool{}
	for _, p := range plan {
		have[p.ID] = true
	}
	for _, id := range []uint16{14, 13, protocol.EquipmentJournalOpcode} {
		if !have[id] {
			t.Fatalf("refresh packet %d missing from %v", id, plan)
		}
	}
	for _, p := range plan {
		if p.ID == protocol.EquipmentJournalOpcode && len(p.Payload) != protocol.EquipmentJournalBodySize {
			t.Fatalf("2610 body = %d bytes, want %d", len(p.Payload), protocol.EquipmentJournalBodySize)
		}
	}
	// 落库：金币扣 35000（源 `[need primer materials]` legendary）、槽 36 换成传说档、
	// 旧件进装备库、微光灵魂返还进账号仓。
	next, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if next.Gold != 65_000 {
		t.Fatalf("gold = %d, want 65000", next.Gold)
	}
	var worn uint32
	for _, item := range next.Worn {
		if item.Slot == 36 {
			worn = item.Template
		}
	}
	if worn != 100401597 {
		t.Fatalf("worn slot 36 = %d, want 100401597", worn)
	}
	ledger, err := inventory.ReadEquipmentJournal(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Counts[100401592] != 2 {
		t.Fatalf("replaced crystal not registered: %v", ledger.Counts)
	}
	accountRaw, err := store.AccountMaterials(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	accountMats, err := inventory.ReadAccountMaterials(accountRaw)
	if err != nil {
		t.Fatal(err)
	}
	if accountMats.Count(10415190) != 1 {
		t.Fatalf("refund missing: %v", accountMats)
	}
	// 事务状态确实写回了库（不是只在内存里改）。
	storedRole, err := store.AdminCharacter(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	stored := storedRole.State
	storedBag, err := inventory.ReadBag(stored)
	if err != nil {
		t.Fatal(err)
	}
	if storedBag.Gold != 65_000 {
		t.Fatalf("stored gold = %d, want 65000", storedBag.Gold)
	}

	// 重复帧：2259 的「变换 → 确定」是同一份正文发两次；2381 目前只有单帧样本。
	// 无论哪种，**第二次到达都不能二次扣费**：各槽已是目标 ⇒ 计划为空 ⇒ 只回窗口应答。
	// 这条用例把 §4.2 的口径钉住（回成功形状、存档一字不变）。
	repeat, err := w.primerTransform(primerTransformFrame(t, 0, 100401597), nil)
	if err != nil {
		t.Fatalf("repeat frame must not error: %v", err)
	}
	if len(repeat) != 1 || repeat[0].ID != protocol.PrimerTransformOpcode || repeat[0].Kind != 1 {
		t.Fatalf("repeat reply = %+v, want only the window reply", repeat)
	}
	if want := []byte{1, 0, 0, 0, 0, 0, 0}; string(repeat[0].Payload) != string(want) {
		t.Fatalf("repeat payload = % x, want % x", repeat[0].Payload, want)
	}
	again, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if again.Gold != 65_000 {
		t.Fatalf("repeat frame charged again: gold = %d", again.Gold)
	}
	againLedger, err := inventory.ReadEquipmentJournal(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if againLedger.Counts[100401592] != 2 {
		t.Fatalf("repeat frame re-registered the source: %v", againLedger.Counts)
	}
	accountRaw, err = store.AccountMaterials(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	if mats, err := inventory.ReadAccountMaterials(accountRaw); err != nil {
		t.Fatal(err)
	} else if mats.Count(10415190) != 1 {
		t.Fatalf("repeat frame refunded twice: %v", mats)
	}
}

// **实机缺陷回归（2026-10-04）**：用户反复点「变换」，晶体越变越多、最后 11 个槽全满。
//
// 复现序列：客户端每次换一个"下一格"来要同一件晶体（实机日志里的 rows=[0 1] → [0 2] →
// [0 3 4] → [5 6 7] → [8 9 10]）。修复口径：变换必须消耗一件真实来源，同一件实物不允许
// 被两行共用 ⇒ 无论点多少次，**晶体总数不能超过真实持有件数**。
func TestPrimerTransformFlowDoesNotMultiplyCrystals(t *testing.T) {
	dsn := os.Getenv("DFO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DFO_TEST_POSTGRES_DSN requires a dedicated PostgreSQL test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := database.Open(ctx, database.Config{PostgresDSN: dsn, MaxConnections: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("primer_dupe_%d", time.Now().UnixNano())
	if err := admin.DiagnosticExec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := admin.DiagnosticExec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	store, err := database.Open(ctx, database.Config{PostgresDSN: dsn, PostgresSchema: schema, MaxConnections: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{
		store.Migrate, store.MigrateCharacterEvents, store.MigrateAccountMaterials,
	} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "primer-dupe-fixture")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("f", 64)
	equipment, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: version},
		Rows:   []inventory.EquipmentDefinition{primerDef(100401597, 6)},
	}, version)
	if err != nil {
		t.Fatal(err)
	}
	transform, err := catalog.ParseEquipmentTransformSystem(primerTransformFlowTable)
	if err != nil {
		t.Fatal(err)
	}
	journal := catalog.EquipmentJournalRules{Maximum: 99}
	w := &worldSession{account: account, store: store, items: &inventory.ItemService{
		Model:     "primer-transform-test",
		Catalog:   catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}},
		Equipment: equipment, Journal: &journal, Transform: &transform,
		BagRules: inventory.BagRules{MissingStackLimit: 2147483647},
	}}
	// 只有 1 件实物：军械库登记 1 件，其余全空。实机那次是 2 件（617/592），机制相同。
	raw, err := inventory.SaveEquipmentJournal(
		json.RawMessage(`{"level":115,"inventory":{"version":"ordinary-bag-v1","gold":1000000}}`),
		inventory.EquipmentJournal{Counts: map[uint32]uint32{100401597: 1}})
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{
		AccountID: account, Name: "primer-dupe", Profession: 11,
		ConfigVersion: savecontract.Identity(), Request: []byte{0}, State: raw,
	}, 24)
	if err != nil {
		t.Fatal(err)
	}
	w.role = role
	crystals := func() int {
		bag, e := inventory.ReadBag(w.role.State)
		if e != nil {
			t.Fatal(e)
		}
		n := 0
		for _, worn := range bag.Worn {
			if worn.Slot >= protocol.PrimerTransformCrystalSlotBase && worn.Template != 0 {
				n++
			}
		}
		return n
	}
	// 连点 6 次，每次换一个目标槽（模拟客户端逐步填格）。
	for round := 0; round < 6; round++ {
		if _, err := w.primerTransform(primerTransformFrame(t, round%11, 100401597), nil); err != nil {
			t.Fatalf("round %d: %v", round+1, err)
		}
		if got := crystals(); got > 1 {
			t.Fatalf("round %d: crystal count = %d —— 又凭空造晶体了", round+1, got)
		}
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if got := crystals(); got != 1 {
		t.Fatalf("final crystal count = %d, want exactly the 1 real unit", got)
	}
	ledger, err := inventory.ReadEquipmentJournal(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Counts[100401597] != 0 {
		t.Fatalf("armory must not keep handing out free copies: %v", ledger.Counts)
	}
	if bag.Gold > 1_000_000 {
		t.Fatalf("gold grew to %d", bag.Gold)
	}
}

// 太初晶体（rarity 8）只能进 44..46：行 0..8（槽 36..44 之前的槽）必须被跳过而不是写出非法存档。
func TestPrimerTransformRejectsPrimevalInEarlySlot(t *testing.T) {
	dsn := os.Getenv("DFO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DFO_TEST_POSTGRES_DSN requires a dedicated PostgreSQL test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := database.Open(ctx, database.Config{PostgresDSN: dsn, MaxConnections: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("primer_guard_%d", time.Now().UnixNano())
	if err := admin.DiagnosticExec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := admin.DiagnosticExec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	store, err := database.Open(ctx, database.Config{PostgresDSN: dsn, PostgresSchema: schema, MaxConnections: 2})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{
		store.Migrate, store.MigrateCharacterEvents, store.MigrateAccountMaterials,
	} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "primer-guard-fixture")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("d", 64)
	equipment, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: version},
		Rows:   []inventory.EquipmentDefinition{primerDef(100401599, 8)},
	}, version)
	if err != nil {
		t.Fatal(err)
	}
	transform, err := catalog.ParseEquipmentTransformSystem(primerTransformFlowTable)
	if err != nil {
		t.Fatal(err)
	}
	journal := catalog.EquipmentJournalRules{Maximum: 99}
	w := &worldSession{account: account, store: store, items: &inventory.ItemService{
		Model:     "primer-transform-test",
		Catalog:   catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}},
		Equipment: equipment, Journal: &journal, Transform: &transform,
		BagRules: inventory.BagRules{MissingStackLimit: 2147483647},
	}}
	raw, err := inventory.SaveEquipmentJournal(
		json.RawMessage(`{"level":115,"inventory":{"version":"ordinary-bag-v1","gold":100000}}`),
		inventory.EquipmentJournal{Counts: map[uint32]uint32{100401599: 1}})
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{
		AccountID: account, Name: "primer-guard", Profession: 11,
		ConfigVersion: savecontract.Identity(), Request: []byte{0}, State: raw,
	}, 24)
	if err != nil {
		t.Fatal(err)
	}
	w.role = role
	// 行 0 → 槽 36：太初晶体放不进 ⇒ 计划阶段就该拒绝（只回窗口应答、不动存档）。
	plan, err := w.primerTransform(primerTransformFrame(t, 0, 100401599), nil)
	if err != nil {
		t.Fatalf("flow must not error: %v", err)
	}
	if len(plan) != 1 || plan[0].ID != protocol.PrimerTransformOpcode {
		t.Fatalf("plan = %+v, want only the window reply", plan)
	}
	next, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if next.Gold != 100_000 {
		t.Fatalf("gold changed to %d, must stay 100000", next.Gold)
	}
	for _, item := range next.Worn {
		if item.Template == 100401599 {
			t.Fatal("primeval crystal must not be written into an early slot")
		}
	}
}

// 付款方式解析：正文 `+13` 只在 1..2 时被采信，其余回落 1（金币）。
//
// 这条规则的动机是 docs §4 缺口 1：`+13` 到底是"恒 1 的窗口字段"还是"玩家选的付款方式"
// 尚未定论 —— "读它、只认 1..2" 在两种假设下都安全（恒 1 时读出 1，与固定 1 等价；
// 真可选时能跟着玩家切换）。真正扣哪一档由 `ItemService.transformPayment` 按源表决定
// （见 internal/inventory 的 `TestTransformPaymentByRarity` 覆盖 group 2 = 巡礼之印）。
func TestPrimerTransformPayOptionResolution(t *testing.T) {
	cases := []struct {
		key  uint32
		want int
	}{
		{1, 1}, // 实机样本 / 构造器默认
		{2, 2}, // 若该字段真是付款方式，玩家切到巡礼之印
		{0, primerTransformPayOptionDefault},
		{3, primerTransformPayOptionDefault},
		{0xFFFFFFFF, primerTransformPayOptionDefault}, // 未初始化的 -1 也得兜住
	}
	for _, c := range cases {
		r := protocol.PrimerTransformRequest{WindowKey: c.key}
		if got := primerTransformPayOption(r); got != c.want {
			t.Fatalf("window key %d -> pay option %d, want %d", c.key, got, c.want)
		}
	}
}

// primerTransformFrame 造一帧 203 字节的 CMD2381 正文（record 下标 → 槽 36+下标）。
func primerTransformFrame(t *testing.T, index int, template uint32) []byte {
	t.Helper()
	if index < 0 || index >= protocol.PrimerTransformCrystalSlotCount {
		t.Fatalf("record index %d out of range", index)
	}
	body := make([]byte, protocol.PrimerTransformBodySize)
	binary.LittleEndian.PutUint32(body[13:], 1) // 窗口对象字段（构造器恒 1）
	// 行 0（誓约核心槽 47）保持空；[32:36] 与实机一致写 1。
	binary.LittleEndian.PutUint32(body[32:], 1)
	body[17] = 0x2e
	binary.LittleEndian.PutUint32(body[20:], protocol.PrimerTransformEmptyTemplate)
	binary.LittleEndian.PutUint32(body[28:], protocol.PrimerTransformEmptyTemplate)
	for i := 0; i < protocol.PrimerTransformCrystalSlotCount; i++ {
		off := protocol.PrimerTransformHeaderSize + i*protocol.PrimerTransformEntrySize
		body[off] = 0x2e
		binary.LittleEndian.PutUint32(body[off+3:], protocol.PrimerTransformEmptyTemplate)
		binary.LittleEndian.PutUint32(body[off+11:], protocol.PrimerTransformEmptyTemplate)
	}
	off := protocol.PrimerTransformHeaderSize + index*protocol.PrimerTransformEntrySize
	body[off], body[off+1], body[off+2] = 0x2e, 0, 0
	binary.LittleEndian.PutUint32(body[off+3:], template)
	return body
}

// primerDef 造一条晶体定义（`[primer]` 在 durabilityOptional 里，所以不需要 [durability]）。
//
// `[grade]` 是必需的：`equipmentGradeRarity` 同时读 `[grade]` 与 `[rarity]`（真源里晶体是 116）。
func primerDef(id uint32, rarity int32) inventory.EquipmentDefinition {
	return inventory.EquipmentDefinition{
		ID: id, Path: fmt.Sprintf("primer/%d.equ", id), SHA256: strings.Repeat("e", 64),
		Fields: map[string][]pvf.Token{
			"[grade]":           {{Type: 0, Value: 116}},
			"[rarity]":          {{Type: 0, Value: rarity}},
			"[equipment type]":  {{Type: 6, Text: "[primer]"}},
			"[minimum level]":   {{Type: 0, Value: 115}},
			"[item group name]": {{Type: 6, Text: "primer"}},
		},
	}
}

// primerTransformFlowTable 与真实源同形的**最小**成本/返还表（真实源由 catalog 包的真实源用例钉住）。
const primerTransformFlowTable = `[need materials]
 [info]
  [condition] 115 ` + "`rare`" + `
  [cost]
   [group] 1
    0 25000
    10361512 1
   [/group]
   [group] 2
    10401346 5
    10361512 1
   [/group]
  [/cost]
 [/info]
[/need materials]
[refund materials]
 115 ` + "`rare`" + ` 0 1 10361512 1
[/refund materials]
[need amalgamation materials]
 [info]
  [condition] 115 ` + "`unique`" + `
  [cost]
   [group] 1
    0 30000
   [/group]
   [group] 2
    10401346 6
   [/group]
  [/cost]
 [/info]
[/need amalgamation materials]
[refund amalgamation materials]
 115 ` + "`rare`" + ` 0 -1 0
[/refund amalgamation materials]
[need primer materials]
 [info]
  [condition] 115 ` + "`rare`" + `
  [cost]
   [group] 1
    0 25000
   [/group]
   [group] 2
    10401346 5
   [/group]
  [/cost]
 [/info]
 [info]
  [condition] 115 ` + "`legendary`" + `
  [cost]
   [group] 1
    0 35000
   [/group]
   [group] 2
    10401346 7
   [/group]
  [/cost]
 [/info]
 [info]
  [condition] 115 ` + "`primeval`" + `
  [cost]
   [group] 1
    0 50000
   [/group]
   [group] 2
    10401346 10
   [/group]
  [/cost]
 [/info]
[/need primer materials]
[refund primer materials]
 115 ` + "`rare`" + ` 0 1 10415190 1
 115 ` + "`rare`" + ` 1 0
 115 ` + "`legendary`" + ` 0 1 10415190 10
 115 ` + "`legendary`" + ` 1 0
 115 ` + "`primeval`" + ` 0 1 10415190 100
 115 ` + "`primeval`" + ` 1 0
[/refund primer materials]
`
