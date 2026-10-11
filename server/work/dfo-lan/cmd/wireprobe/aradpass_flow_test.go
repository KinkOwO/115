package main

import (
	"context"
	"dfolan/internal/aradpass"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/savecontract"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"
)

// aradPassFlowFixture 起一个一次性 SQLite 库 + 一个装着战令目录的会话。
//
// 与 internal/workflow/loot_aradpass_test.go 的分工：那边钉"事务/幂等/回滚"，
// 这边钉"帧序 / 路由 / 状态投影"。
func aradPassFlowFixture(t *testing.T) (context.Context, *database.Store, *worldSession, *aradpass.Catalog) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	})
	store := fixture.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "arad-pass-flow")
	if err != nil {
		t.Fatal(err)
	}
	version := savecontract.Identity()
	raw, err := inventory.SaveBag(json.RawMessage(`{"level":115}`), inventory.Bag{Version: "ordinary-bag-v1"})
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{
		AccountID: account, Name: "AradPassFlow", Profession: 0, Request: []byte{0},
		ConfigVersion: version, State: raw,
	}, 100)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	items := map[uint32]catalog.LootItem{}
	for _, id := range []uint32{590722601, 590722598, 590722593, 590722579} {
		items[id] = catalog.LootItem{ID: id, Kind: "stackable", StackableType: "[etc]", StackLimit: 1000}
	}
	ls := &loot.Service{
		Catalog:  catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}, Items: items},
		BagRules: rules,
	}
	source := &aradpass.Catalog{
		Levels: []aradpass.Level{{Level: 1, Exp: 0}, {Level: 2, Exp: 600}, {Level: 3, Exp: 1200}},
		Rewards: []aradpass.Reward{
			{Track: aradpass.TrackNormal, Slot: 1, Item: 590722601, Count: 2, Level: 1},
			{Track: aradpass.TrackNormal, Slot: 2, Item: 590722601, Count: 1, Level: 3},
			// 皇家轨一档（Lv2）：位图要按**等级**落位，与普通轨同一个 50 位空间。
			{Track: aradpass.TrackRoyal, Slot: 1, Item: 590722646, Count: 1, Level: 2},
		},
		// 源 `[normal dungeon index list]` 的等价物（夹具只放一张，够验映射）。
		NormalDungeons: []uint32{100004140},
		// 任务表：只放本用例需要的那两条（形状与源 `[normal mission]` / `[season mission]` 一致）。
		NormalMissions:   []aradpass.Mission{{Index: 1, Key: "daily_mission", Count: 4, Exp: 300}},
		SeasonMissions:   []aradpass.Mission{{Index: 1, Key: "proper_dungeon_clear", Count: 100, Exp: 800}},
		RoyalFirstReward: []uint32{590722598, 590722593},
		PurchaseExp:      5000,
		PassItem:         []uint32{590722579},
	}
	w := &worldSession{account: account, role: role, loot: ls, store: store, aradPass: source}
	return ctx, store, w, source
}

// 状态投影：等级由经验换算、皇家字节按 Royal 走、0x1C2 按**领奖状态**推、
// 当前角色登记进 0x08A2 第 0 槽、任务行带真实计数。
func TestAradPassPayloadsReflectCharacterState(t *testing.T) {
	_, _, w, source := aradPassFlowFixture(t)
	w.role.WireID = 0x1234
	raw, err := aradpass.WriteState(w.role.State, aradpass.State{
		Version: "arad-pass-v1", Exp: 600, Royal: true,
		// 赛季轨第 1 条（`proper_dungeon_clear`）推进 1 次、未做满。
		Missions: map[string]uint32{"2:1": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	w.role.State = raw
	event, charac, err := aradPassPayloads(source, w.role)
	if err != nil {
		t.Fatal(err)
	}
	if len(event) != 760 {
		t.Fatalf("0x08A1 正文 = %d 字节，期望 760", len(event))
	}
	if len(charac) != 4+20*55 {
		t.Fatalf("0x08A2 正文 = %d 字节，期望 %d", len(charac), 4+20*55)
	}
	if event[4] != 2 {
		t.Fatalf("等级字节 = %d，exp=600 应当是 2 级", event[4])
	}
	if got := binary.LittleEndian.Uint32(event[12:]) / 256; got != 600 {
		t.Fatalf("定点累计经验 = %d，期望 600", got)
	}
	if event[0x18] != 1 {
		t.Fatal("Royal 应当写进已购候选字节 0x18（DFO_ARAD_PASS_ROYAL_BYTE 可覆盖）")
	}
	// ★ 2209 正文前 4 字节 = 2 才是 ROYAL 轨解锁的开关（实机双向验证）。
	if got := binary.LittleEndian.Uint32(event[0:4]); got != protocol.AradPassBodyPurchased {
		t.Fatalf("已购时 2209 正文头 = %d，期望 %d", got, protocol.AradPassBodyPurchased)
	}
	// ★ 0x1C2：未领过任何档 ⇒ 取抓包**登录首帧**那一态（0）。
	// 此前硬写 1，等于对客户端宣称"已经领过"，面板把 Free 第 1 档画成绿勾 Complete。
	if event[0x1C2] != 0 {
		t.Fatalf("未领奖时 0x1C2 应为 0（抓包登录首帧），实际 %d", event[0x1C2])
	}
	// 领过一档之后 → 取抓包"领奖之后"那一态（1）。
	claimed, err := aradpass.WriteState(w.role.State, aradpass.State{
		Version: "arad-pass-v1", Exp: 600, Royal: true,
		Claimed: map[string]bool{"0:1": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	w.role.State = claimed
	event, _, err = aradPassPayloads(source, w.role)
	if err != nil {
		t.Fatal(err)
	}
	if event[0x1C2] != 1 {
		t.Fatalf("领过奖之后 0x1C2 应为 1（抓包领奖后样本），实际 %d", event[0x1C2])
	}
	// 未购皇家 ⇒ 2209 头必须留 0（与官服抓包那台"未购买"的账号一致）。
	notRoyal, err := aradpass.WriteState(w.role.State, aradpass.State{Version: "arad-pass-v1", Exp: 600})
	if err != nil {
		t.Fatal(err)
	}
	w.role.State = notRoyal
	event, _, err = aradPassPayloads(source, w.role)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint32(event[0:4]); got != 0 {
		t.Fatalf("未购皇家时 2209 正文头 = %d，期望 0", got)
	}

	// 0x08A2：当前角色登记进**第 0 槽**（charID = WireID），其余 19 槽留空。
	if got := binary.LittleEndian.Uint32(charac[4+5:]); got != 0x1234 {
		t.Fatalf("第 0 槽 charID = %#x，期望当前角色的 WireID", got)
	}
	// 记录必须"存在"：客户端判据是 @+0 / @+1 / @+17 三者之一非 0，全 0 会被整条跳过。
	if got := binary.LittleEndian.Uint32(charac[4+1:]); got == 0 {
		t.Fatal("第 0 槽 @+1（键）为 0 ⇒ 整条记录会被客户端跳过")
	}
	// @+9 = 位掩码（bit1 进"皇家"那条列表，面板 `Royal Pass Character(n/10)` 数的就是它）；
	// @+15 是 bit1 那条列表要求的非零标志。
	slot := aradPassCharSlot(w.role)
	if got := charac[4+0]; got != slot.Lead {
		t.Fatalf("第 0 槽 @+0 = %d，期望 %d（源 [royal registable count] 1 10 的下界）", got, slot.Lead)
	}
	if got := binary.LittleEndian.Uint32(charac[4+1:]); got != slot.Key {
		t.Fatalf("第 0 槽 @+1 = %d，期望 %d", got, slot.Key)
	}
	if got := binary.LittleEndian.Uint32(charac[4+9:]); got != slot.Kind {
		t.Fatalf("第 0 槽 @+9 = %d，期望 %d", got, slot.Kind)
	}
	if got := charac[4+15]; got != slot.Flag {
		t.Fatalf("第 0 槽 @+15 = %d，期望 %d", got, slot.Flag)
	}
	if got := charac[4+16]; got != slot.State {
		t.Fatalf("第 0 槽 @+16 = %d，期望 %d（判定函数 sub_14229FF90 返回的就是这一格）", got, slot.State)
	}
	if got := binary.LittleEndian.Uint32(charac[4+17:]); got != slot.Extra {
		t.Fatalf("第 0 槽 @+17 = %d，期望 %d", got, slot.Extra)
	}
	// ★ 正文前 4 字节必须是 2 —— `sub_14229FF90` 的第一句就是 `!= 2 → return 0`。
	if got := binary.LittleEndian.Uint32(charac[0:]); got != protocol.AradPassCharacHeader {
		t.Fatalf("0x08A2 正文头 = %d，期望 %d（否则面板认定未购买）", got, protocol.AradPassCharacHeader)
	}
	// 缺省：Lead=1（1..10 的下界）、Key=1（非 0 才不被跳过）、Kind=3（两位都置）、Flag=1。
	if slot.Lead != 1 || slot.Key != 1 || slot.Kind != 3 || slot.Flag != 1 || slot.State != 1 {
		t.Fatalf("槽位缺省值 = %+v", slot)
	}
	for i := 1; i < 20; i++ {
		if got := binary.LittleEndian.Uint32(charac[4+i*55+5:]); got != 0xFFFFFFFF {
			t.Fatalf("槽 %d 的 charID = %#x，期望空白 0xFFFFFFFF", i, got)
		}
	}
}

// 已领标记是**逐档 1 字节**的数组（索引 = 每轨序号：普通 0..24、皇家 25..49），
// 不是位图 —— 两次实机对照：`0x1C2 = 0xff` 只点亮 2 格而不是 8 格。
func TestAradPassClaimedSlotsLayout(t *testing.T) {
	_, _, _, source := aradPassFlowFixture(t)
	st := aradpass.State{Version: "arad-pass-v1", Claimed: map[string]bool{
		"0:1": true, "0:2": true, "2:1": true, "9:9": true,
	}}
	got := aradPassClaimedSlots(st, source)
	if len(got) != aradPassClaimedSlotCount {
		t.Fatalf("数组长度 = %d，期望 %d", len(got), aradPassClaimedSlotCount)
	}
	// 普通轨第 1、2 档 → 下标 0、1；皇家轨第 1 档 → 下标 25；不存在的轨道/槽不动数组。
	if got[0] != aradPassClaimedMark || got[1] != aradPassClaimedMark || got[25] != aradPassClaimedMark {
		t.Fatalf("下标 0/1/25 应为已领标记，实际 %d/%d/%d", got[0], got[1], got[25])
	}
	for _, i := range []int{2, 24, 26, 49} {
		if got[i] != 0 {
			t.Fatalf("下标 %d 不该被标记（实际 %d）", i, got[i])
		}
	}
	// 空状态 ⇒ 全 0（就是"四格全 Get"那一态）。
	empty := aradPassClaimedSlots(aradpass.State{}, source)
	for i, v := range empty {
		if v != 0 {
			t.Fatalf("空状态下标 %d = %d，应为 0", i, v)
		}
	}
}

// 任务行：序号必须写成 1..N（写 0 会让客户端从这里停止收行），
// 且赛季轨第 1 条的进度落到 @+3、次数落到 @+5。
func TestAradPassMissionRowsCarryCounts(t *testing.T) {
	_, _, w, source := aradPassFlowFixture(t)
	raw, err := aradpass.WriteState(w.role.State, aradpass.State{
		Version: "arad-pass-v1", Missions: map[string]uint32{"2:1": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	w.role.State = raw
	event, _, err := aradPassPayloads(source, w.role)
	if err != nil {
		t.Fatal(err)
	}
	row := func(list, i int) []byte {
		if list == 0 {
			return event[0x1B+7*i : 0x1B+7*i+7]
		}
		return event[0xEE+7*i : 0xEE+7*i+7]
	}
	for i := 0; i < 16; i++ {
		if row(0, i)[0] != byte(i+1) {
			t.Fatalf("普通任务第 %d 行序号 = %d，应为 %d", i, row(0, i)[0], i+1)
		}
	}
	for i := 0; i < 12; i++ {
		if row(1, i)[0] != byte(i+1) {
			t.Fatalf("赛季任务第 %d 行序号 = %d，应为 %d", i, row(1, i)[0], i+1)
		}
	}
	first := row(1, 0) // 赛季轨第 1 条 = proper_dungeon_clear
	// ★ 进度在 **@+1**：实机用探针定案（@+1=41 时面板 `n / m` 列显示 `41 / 100`）。
	// 第一版写在 @+3 是错的 —— 任务页因此一直显示 `0 / m`。
	if got := binary.LittleEndian.Uint16(first[1:]); got != 1 {
		t.Fatalf("赛季任务 1 的进度 @+1 = %d，期望 1（该角色打过 1 次）", got)
	}
	// @+3 = 累计经验（面板 `Total`）：未做满 ⇒ 次数 0 ⇒ 0。
	// @+3 = 累计经验（面板 `Total`），**单位十分之一**：未做满 ⇒ 次数 0 ⇒ 0。
	if got := binary.LittleEndian.Uint16(first[3:]); got != 0 {
		t.Fatalf("赛季任务 1 的累计经验 @+3 = %d，期望 0", got)
	}
	// 做满 1 次（源 `proper_dungeon_clear` 单次 800 经验）⇒ @+3 = 800/10 = 80。
	// 实测锚点：@+3 = 300 时面板显示 `Total 3000`（见 protocol.AradPassMissionRow.TotalExp）。
	done, err := aradpass.WriteState(w.role.State, aradpass.State{
		Version: "arad-pass-v1", Missions: map[string]uint32{"2:1": 100},
		Rewarded: map[string]bool{"2:1": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	w.role.State = done
	event, _, err = aradPassPayloads(source, w.role)
	if err != nil {
		t.Fatal(err)
	}
	if got := binary.LittleEndian.Uint16(row(1, 0)[3:]); got != 80 {
		t.Fatalf("做满后赛季任务 1 的累计经验 @+3 = %d，期望 80（= 800/10）", got)
	}
	if got := binary.LittleEndian.Uint16(row(1, 0)[5:]); got != 1 {
		t.Fatalf("做满后赛季任务 1 的次数 @+5 = %d，期望 1", got)
	}
	// @+5 = 完成次数（面板 `k time(s)`）；也是客户端的"变化"判据。
	if got := binary.LittleEndian.Uint16(first[5:]); got != 0 {
		t.Fatalf("赛季任务 1 的次数 @+5 = %d，期望 0（未做满）", got)
	}
}

// 领奖：680 + 16B 才发奖；帧序是「2210 → 2209 → 14 → 应答（最后）」。
func TestAradPassRequestClaimsAndPushesStateBeforeReply(t *testing.T) {
	ctx, store, w, _ := aradPassFlowFixture(t)
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint32(payload[0:], aradpass.ClaimScheduleID)
	binary.LittleEndian.PutUint32(payload[4:], aradpass.TrackNormal)
	binary.LittleEndian.PutUint32(payload[8:], 1)
	plan, err := w.aradPassRequest(ctx, payload, 680)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) < 4 {
		t.Fatalf("计划至少要有 2210/2209/14/应答，实际 %d 帧", len(plan))
	}
	if plan[0].ID != aradPassCharacInfoNotiID || plan[1].ID != aradPassEventInfoNotiID {
		t.Fatalf("前两帧必须是 2210/2209，实际 %d/%d", plan[0].ID, plan[1].ID)
	}
	last := plan[len(plan)-1]
	if last.ID != 680 || last.Kind != 1 {
		t.Fatalf("最后一帧必须是 680 应答，实际 id=%d kind=%d", last.ID, last.Kind)
	}
	// 回包 = `01 <事件号 516> <10×u32>`，第一格是轨道。
	if len(last.Payload) != 45 || last.Payload[0] != 1 {
		t.Fatalf("应答正文 = %d 字节（首字节 %d），期望 45/1", len(last.Payload), last.Payload[0])
	}
	if got := binary.LittleEndian.Uint32(last.Payload[1:]); got != aradpass.ClaimScheduleID {
		t.Fatalf("应答事件号 = %d，期望 %d", got, aradpass.ClaimScheduleID)
	}
	if got := binary.LittleEndian.Uint32(last.Payload[5:]); got != aradpass.TrackNormal {
		t.Fatalf("应答第一格（轨道）= %d，期望 %d", got, aradpass.TrackNormal)
	}
	// 存档真值：已领标记 + 产物入包。
	fresh := aradPassFlowReload(t, ctx, store, w.role)
	st, err := aradpass.ReadState(fresh.State)
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsClaimed(aradpass.TrackNormal, 1) {
		t.Fatal("已领标记没有落库")
	}
	bag, err := inventory.ReadBag(fresh.State)
	if err != nil {
		t.Fatal(err)
	}
	total := uint32(0)
	for _, item := range bag.Items {
		if item.Template == 590722601 {
			total += item.Amount
		}
	}
	if total != 2 {
		t.Fatalf("产物入包数量 = %d，期望 2", total)
	}
}

// 681 是查询：只重推状态 + 应答，**不发奖**。
func TestAradPassQueryDoesNotGrant(t *testing.T) {
	ctx, store, w, _ := aradPassFlowFixture(t)
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint32(payload[0:], aradpass.ClaimScheduleID)
	binary.LittleEndian.PutUint32(payload[4:], aradpass.TrackNormal)
	binary.LittleEndian.PutUint32(payload[8:], 1)
	plan, err := w.aradPassRequest(ctx, payload, 681)
	if err != nil {
		t.Fatal(err)
	}
	if plan[0].ID != aradPassCharacInfoNotiID {
		t.Fatalf("查询也要重推状态，实际首帧 id=%d", plan[0].ID)
	}
	for _, p := range plan {
		if p.ID == 14 || p.ID == 13 {
			t.Fatal("查询不该带库存刷新（那意味着它发了奖）")
		}
	}
	if plan[len(plan)-1].ID != 681 {
		t.Fatalf("应答要按请求 id 回（681），实际 %d", plan[len(plan)-1].ID)
	}
	fresh := aradPassFlowReload(t, ctx, store, w.role)
	st, err := aradpass.ReadState(fresh.State)
	if err != nil {
		t.Fatal(err)
	}
	if st.IsClaimed(aradpass.TrackNormal, 1) {
		t.Fatal("查询不该写已领标记")
	}
}

// 等级不足：拒绝路径也必须带应答（清在途标志），且错误里说明原因。
func TestAradPassRequestRefusesBelowLevel(t *testing.T) {
	ctx, _, w, _ := aradPassFlowFixture(t)
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint32(payload[0:], aradpass.ClaimScheduleID)
	binary.LittleEndian.PutUint32(payload[4:], aradpass.TrackNormal)
	binary.LittleEndian.PutUint32(payload[8:], 2) // 第 2 档要 3 级
	if _, err := w.aradPassRequest(ctx, payload, 680); err == nil {
		t.Fatal("等级不足必须报错（由调用方记事件）")
	}
}

// 激活：拥有通行证物品时登录收口一次，置 Royal + 加购票经验 + 发首发奖励。
func TestEnsureAradPassActivationOnOwnedPassItem(t *testing.T) {
	ctx, store, w, _ := aradPassFlowFixture(t)
	// 先用 GM 口径把通行证物品放进角色背包（直写存档）。
	awarder := &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.loot.Equipment}
	next, _, err := awarder.Grant(w.role.State, 590722579, 1)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := store.CreateCharacter(ctx, database.Character{
		AccountID: w.role.AccountID, Name: "AradPassOwned", Profession: 0, Request: []byte{0},
		ConfigVersion: w.role.ConfigVersion, State: next,
	}, 100)
	if err != nil {
		t.Fatal(err)
	}
	w.role = fresh
	activated, err := w.ensureAradPassActivation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !activated {
		t.Fatal("持有通行证物品时应当激活")
	}
	st, err := aradpass.ReadState(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Royal || !st.FirstRewards || st.Exp != 5000 {
		t.Fatalf("激活后状态 = %+v", st)
	}
	// 幂等：再收口一次不会重复发。
	again, err := w.ensureAradPassActivation(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("已激活时应当零写入")
	}
}

// 账号级一次性发放通行证物品：第一个登录的角色拿到，同账号后续登录/换角色都不再发。
func TestAradPassPassItemGrantedOncePerAccount(t *testing.T) {
	// 业主 2026-10-11 决定**不再发这件道具**（改成静默解锁皇家轨）⇒ 缺省关，
	// 这条用例专门验"开了之后的账号级幂等"，所以显式打开开关。
	t.Setenv("DFO_ARAD_PASS_GRANT_ITEM", "1")
	ctx, store, w, source := aradPassFlowFixture(t)
	_ = source
	granted, err := w.ensureAradPassPassItem(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !granted {
		t.Fatal("账号第一次登录时应当发放通行证物品")
	}
	if !ownsAradPassItem(w.role.State, 590722579) {
		t.Fatal("通行证物品没有进背包")
	}
	// 幂等键是 (模板, 账号)：同一角色再来一次不发。
	again, err := w.ensureAradPassPassItem(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if again {
		t.Fatal("同一角色重复登录不该再发")
	}
	// 同账号的**另一个角色**登录：账号已经领过 ⇒ 也不发。
	fresh := aradPassFlowReload(t, ctx, store, w.role)
	other, err := store.CreateCharacter(ctx, database.Character{
		AccountID: w.role.AccountID, Name: "AradPassSecond", Profession: 0, Request: []byte{0},
		ConfigVersion: w.role.ConfigVersion, State: fresh.State,
	}, 100)
	if err != nil {
		t.Fatal(err)
	}
	empty, err := inventory.SaveBag(other.State, inventory.Bag{Version: "ordinary-bag-v1"})
	if err != nil {
		t.Fatal(err)
	}
	other.State = empty
	w2 := &worldSession{account: w.account, role: other, loot: w.loot, store: store, aradPass: w.aradPass}
	g2, err := w2.ensureAradPassPassItem(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if g2 {
		t.Fatal("同账号的第二个角色不该再拿到通行证物品")
	}
}

// Receive All（`680` 的 `slot == 0`）：把当前等级下**所有**可领档一次领完，且可重复点。
//
// 实机证据（2026-10-11 01:30:36）：客户端发了 `516, 0, 0, 0`，我们当时按 `RewardAt(0,0)`
// 找不到档、回了拒绝（`arad_pass_request_refused: no reward for track=0 slot=0`）。
func TestAradPassReceiveAllClaimsEveryUnlockedTier(t *testing.T) {
	ctx, store, w, _ := aradPassFlowFixture(t)
	// exp=1200 ⇒ 3 级 ⇒ 普通轨第 1 档（Lv1）与第 2 档（Lv3）都解锁。
	//
	// ★ 必须**落库**：`ClaimAradPassReward` 走 `CommitCharacterEvent`，事务里读的是
	// 角色行（不是内存里的 `w.role`）—— 只改内存的话事务看到的是旧等级
	// （第一版用例就这么错过：外面 level=3、事务里 level=1，第 2 档被"等级不够"挡掉）。
	raw, err := aradpass.WriteState(w.role.State, aradpass.State{Version: "arad-pass-v1", Exp: 1200})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := store.SaveCharacterState(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion,
		func(database.Character) (json.RawMessage, error) { return raw, nil })
	if err != nil {
		t.Fatal(err)
	}
	w.role = saved
	w.role.WireID = 0x1
	payload := make([]byte, 16)
	binary.LittleEndian.PutUint32(payload[0:], aradpass.ClaimScheduleID)
	// track 与 slot 都留 0 = Receive All
	plan, err := w.aradPassRequest(ctx, payload, 680)
	if err != nil {
		t.Fatal(err)
	}
	last := plan[len(plan)-1]
	if last.ID != 680 || last.Kind != 1 {
		t.Fatalf("应答必须是最后一帧：id=%d kind=%d", last.ID, last.Kind)
	}
	if plan[0].ID != aradPassCharacInfoNotiID {
		t.Fatalf("状态帧仍要先于应答，实际首帧 id=%d", plan[0].ID)
	}
	fresh := aradPassFlowReload(t, ctx, store, w.role)
	st, err := aradpass.ReadState(fresh.State)
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range []int{1, 2} {
		if !st.IsClaimed(aradpass.TrackNormal, slot) {
			t.Fatalf("Receive All 后普通轨第 %d 档仍未标记已领", slot)
		}
	}
	// 再点一次：所有档都已领 ⇒ 零入账，但**仍要回应答**（否则按钮卡在在途态）。
	plan2, err := w.aradPassRequest(ctx, payload, 680)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan2) == 0 || plan2[len(plan2)-1].ID != 680 {
		t.Fatalf("重复 Receive All 仍须应答，实际 %d 帧", len(plan2))
	}
}

// 缓冲探针：只碰保留区，保护段一律拒绝（写进去会破坏已取证的布局）。
func TestAradPassBodyPokeProtectsKnownLayout(t *testing.T) {
	// 自由区：0x00（正文 u32 头）、0x8B（98B 保留区首）、0x1BF（赛季行之后、1C0 之前）、0x1C9（标记之后）
	// 还要覆盖**闭区间**写法（`8C-8E:3` 一次写三格）。
	//
	// ★ 不能用 0x1C0/0x1C1：那两格 2026-10-11 已定案为「每日礼包已领日」（功能字段），
	// 已进保护段 —— 见 aradPassBodyPoke 的注释。
	t.Setenv("DFO_ARAD_PASS_BODY_POKE", "0:2,8B:1,1BF:1,1C9:2,8C-8E:3,1A:9,50:9,1C2:9,4:9,140:9")
	body := make([]byte, 760)
	body[0x1A] = 16
	body[0x1C2] = 0x1F
	aradPassBodyPoke(body)
	// 偏移按**十六进制**读：`0` 就是 0x00、`8B` 就是 0x8B；`8C-8E` 是闭区间。
	if body[0x00] != 2 || body[0x8B] != 1 || body[0x1BF] != 1 || body[0x1C9] != 2 {
		t.Fatalf("四个自由偏移应当被写入，实际 00=%d 8B=%d 1BF=%d 1C9=%d",
			body[0x00], body[0x8B], body[0x1BF], body[0x1C9])
	}
	if body[0x8C] != 3 || body[0x8D] != 3 || body[0x8E] != 3 || body[0x8F] != 0 {
		t.Fatalf("闭区间 8C-8E 应写 3，实际 %d/%d/%d/%d",
			body[0x8C], body[0x8D], body[0x8E], body[0x8F])
	}
	// 保护段：任务条数、任务行（含赛季行尾 0x140）、已领位图、等级/经验区，必须原样。
	// 0x1C0/0x1C1 也必须在保护段里（探针不许再写「已领日」）。
	if body[0x1C0] != 0 || body[0x1C1] != 0 {
		t.Fatalf("每日礼包已领日 0x1C0/0x1C1 被探针改了：%d/%d", body[0x1C0], body[0x1C1])
	}
	if body[0x1A] != 16 || body[0x50] != 0 || body[0x140] != 0 || body[0x1C2] != 0x1F || body[0x04] != 0 {
		t.Fatalf("保护段被改了：1A=%d 50=%d 140=%d 1C2=%#x 04=%d",
			body[0x1A], body[0x50], body[0x140], body[0x1C2], body[0x04])
	}
}

// 0x08A2 记录探针：偏移相对**第 0 条记录起点**（正文 +4），只动那一条记录、不碰正文头。
func TestAradPassCharacPokeWritesOnlyFirstRecord(t *testing.T) {
	t.Setenv("DFO_ARAD_PASS_CHARAC_POKE", "15-36:1")
	body, err := protocol.AradPassCharacInfo(protocol.AradPassCharacHeader,
		[]protocol.AradPassCharSlot{{CharID: 2, Key: 1, Lead: 1, Kind: 3, Flag: 1, State: 1}})
	if err != nil {
		t.Fatal(err)
	}
	aradPassCharacPoke(body)
	if body[4+0x15] != 1 || body[4+0x36] != 1 {
		t.Fatal("第 0 条记录的 0x15..0x36 应当被写入")
	}
	if body[4+0x14] != 0 || body[4+0x37] != 0 {
		t.Fatal("区间外不该被写")
	}
	// 正文头与第 1 条记录起点不能被误伤。
	if binary.LittleEndian.Uint32(body[0:4]) != protocol.AradPassCharacHeader {
		t.Fatal("0x08A2 正文头被改坏")
	}
	if body[4+55] != 0 {
		t.Fatal("第 1 条记录不该被写")
	}
}

// 通关副本 → 普通轨任务的映射：**只认有副本号证据的那几条**，其余故意不接。
func TestAradPassNormalMissionsForDungeon(t *testing.T) {
	_, _, _, source := aradPassFlowFixture(t)
	cases := []struct {
		name    string
		dungeon uint32
		want    []int
	}{
		{"沉月湖一层", 100004136, []int{aradPassMissionNormalMoonLake}},
		{"沉月湖二层", 100004137, []int{aradPassMissionNormalMoonLake}},
		{"蔚蓝号主层", loot.AzureMainDungeonID, []int{aradPassMissionNormalAzureMain}},
		{"蔚蓝号伴生层", 100004134, []int{aradPassMissionNormalAzureMain}},
		{"源白名单图", 100004140, []int{aradPassMissionNormalDungeon2}},
		{"以上都不是", 100005014, nil},
	}
	for _, c := range cases {
		got := aradPassNormalMissionsForDungeon(source, c.dungeon)
		if len(got) != len(c.want) {
			t.Fatalf("%s(%d): 得到 %v，期望 %v", c.name, c.dungeon, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("%s(%d): 得到 %v，期望 %v", c.name, c.dungeon, got, c.want)
			}
		}
	}
	// nil catalog 不能炸（也不该记任何普通轨任务）。
	if got := aradPassNormalMissionsForDungeon(nil, 100004136); len(got) != 1 || got[0] != aradPassMissionNormalMoonLake {
		t.Fatalf("nil catalog 下仍应按副本号命中沉月湖，实际 %v", got)
	}
}

func aradPassFlowReload(t *testing.T, ctx context.Context, store *database.Store, role database.Character) database.Character {
	t.Helper()
	roles, err := store.Characters(ctx, role.AccountID)
	if err != nil {
		t.Fatal(err)
	}
	for _, cur := range roles {
		if cur.ID == role.ID {
			return cur
		}
	}
	t.Fatalf("character %d not persisted", role.ID)
	return database.Character{}
}
