package main

import (
	"encoding/binary"
	"testing"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
)

// apocalypseRewardSession 是一个末世录会话：频道 Type 119、已确认难度、
// 终局副本已载入。奖励链本身不发包给客户端以外的东西，所以不需要 loot 服务
// 也能验证帧序（loot 为 nil 时跳过入库，只发链）。
func apocalypseRewardSession(t *testing.T, choice byte, stage int) *worldSession {
	t.Helper()
	s := apocalypseSession(t)
	w := &worldSession{
		role:        database.Character{ID: 7, WireID: 7},
		channelType: apocalypseChannelType,
		legion:      s,
	}
	s.world = w
	s.apocalypse = legion.NewApocalypseRunState()
	s.apocalypse.ID = "test-run"
	s.apocalypse.Choice = choice
	s.apocalypse.Entered = true
	s.apocalypse.Stage = stage
	s.apocalypse.PhaseCleared = stage
	w.apocalypse = s.apocalypse
	return w
}

// 终局翻牌链的帧序与抓包/US-Local 事件一致，且每一帧的 opcode 都对得上。
func TestApocalypseTerminalRewardChain(t *testing.T) {
	w := apocalypseRewardSession(t, 0, 6)

	grant, err := w.apocalypseTerminalReward(legion.CmdRewardEnd, "apocalypse_terminal_settlement")
	if err != nil {
		t.Fatalf("terminal reward: %v", err)
	}
	if len(grant.Plan) == 0 {
		t.Fatal("terminal reward produced no packets")
	}
	if w.apocalypseGrant == nil {
		t.Fatal("the grant was not recorded on the session")
	}
	// opcode 序列（规格 G0454 / 2252 / 2253）：
	//	N2895 清关态 → N2895 **终点投影/Outcome0** → N2252 翻牌 → (N2) → N2253
	//	→ N2895 **同终点/Outcome3** → (冒险团三帧) → N9。
	//
	// 规格 2252：「生产在本包之前增加 N2895 终点投影/Outcome0，随后 2252→2253→
	// N2895 同终点/Outcome3」；规格 2253：「apocalypseResultPackets **最后追加
	// N2895 Outcome3**。完成状态只在奖励展示批次生成」。
	ids := make([]uint16, 0, len(grant.Plan))
	for _, p := range grant.Plan {
		ids = append(ids, p.ID)
	}
	wantPrefix := []uint16{legion.NotiLegionInfo, 31, legion.NotiLegionInfo, legion.NotiClearRewardBasic}
	if len(ids) < len(wantPrefix) {
		t.Fatalf("chain too short: %v", ids)
	}
	for i, want := range wantPrefix {
		if ids[i] != want {
			t.Fatalf("chain[%d] = %d, want %d (full: %v)", i, ids[i], want, ids)
		}
	}
	if !hasID(ids, legion.NotiClearRewardAdditional) {
		t.Fatalf("chain has no N2253: %v", ids)
	}
	// ★ N2253 之后必须还有一帧 N2895（Outcome3）。这是通关演出/结算窗的触发点，
	// 此前实现缺失，客户端因此不进终局演出。
	n2253 := -1
	for i, id := range ids {
		if id == legion.NotiClearRewardAdditional {
			n2253 = i
		}
	}
	trailing := false
	for _, id := range ids[n2253+1:] {
		if id == legion.NotiLegionInfo {
			trailing = true
		}
	}
	if !trailing {
		t.Fatalf("N2253 must be followed by the terminal N2895 Outcome3: %v", ids)
	}
	if ids[len(ids)-1] != 9 {
		t.Fatalf("chain must end with the party-steady N9: %v", ids)
	}
	// N2252 必须是 7772B、N2253 必须是 2405B。
	for _, p := range grant.Plan {
		switch p.ID {
		case legion.NotiClearRewardBasic:
			if len(p.Payload) != 7772 {
				t.Fatalf("N2252 length %d, want 7772", len(p.Payload))
			}
		case legion.NotiClearRewardAdditional:
			if len(p.Payload) != 2405 {
				t.Fatalf("N2253 length %d, want 2405", len(p.Payload))
			}
		}
	}
	// 完成事件必须写明奖励来源、难度与随机装备位的 roll 结果。
	// 本测试会话没有 loot 服务，所以 roll_gear 为空——这正是「降级为只发固定项」
	// 的可观测形态，绝不能悄悄伪装成发全。
	if !hasEvent(grant.Events, "apocalypse_full_clear_reward_committed") {
		t.Fatalf("events %v", grant.Events)
	}
	note := grant.Events[len(grant.Events)-1]
	if note["reward_source"] == nil || note["difficulty"] != "normal" {
		t.Fatalf("completion note %v", note)
	}
	if slots, ok := note["roll_gear_slots"].(int); !ok || slots != 8 {
		t.Fatalf("roll_gear_slots = %v, want 8 for difficulty 1", note["roll_gear_slots"])
	}
	if gear, ok := note["roll_gear"].([]uint32); !ok || len(gear) != 0 {
		t.Fatalf("roll_gear = %v, want empty without a loot service", note["roll_gear"])
	}
}

// 第二次调用不能再发一份奖励（run 已标记 Rewarded）。
func TestApocalypseTerminalRewardIsIdempotent(t *testing.T) {
	w := apocalypseRewardSession(t, 1, 6)
	first, err := w.apocalypseTerminalReward(legion.CmdRewardEnd, "first")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Plan) == 0 {
		t.Fatal("first call produced nothing")
	}
	second, err := w.apocalypseTerminalReward(legion.CmdRewardEnd, "second")
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Plan) != 0 {
		t.Fatalf("second call re-sent %d packets", len(second.Plan))
	}
	if !hasEvent(second.Events, "apocalypse_reward_skipped") {
		t.Fatalf("second call events %v", second.Events)
	}
}

// 未声明难度不得发放。
func TestApocalypseTerminalRewardRefusesUnknownDifficulty(t *testing.T) {
	w := apocalypseRewardSession(t, 3, 6)
	if _, err := w.apocalypseTerminalReward(legion.CmdRewardEnd, "bad"); err == nil {
		t.Fatal("an unsourced difficulty was paid")
	}
}

// 清关挂起：只有**该难度的终点关**才挂起终局结算。
//
// 规格 2252-LEGIONBASICREWARD.md：「检查模式当前阶段是否等于难度终点
// （**第一档 3，第二档 5**）」——所以难度1 打完第 3 关就通关，不是第 5 关。
// 此前写死最后一关（恒为 5），实机后果是难度1 与难度2 流程完全一样。
func TestCompleteApocalypseStageArmsOnlyTheTerminal(t *testing.T) {
	for _, choice := range []byte{0, 1} {
		endpoint := legion.ApocalypseEndpoint(choice)
		for stage := 0; stage < len(legion.ApocalypseStageDungeons); stage++ {
			w := apocalypseRewardSession(t, choice, stage+1)
			w.activeDungeon = &dungeon.Session{
				Definition: catalog.DungeonDefinition{ID: legion.ApocalypseStageDungeons[stage]},
				RunID:      "dungeon-run",
			}
			plan, err := w.completeApocalypseStage()
			if err != nil {
				t.Fatalf("choice %d stage %d: %v", choice, stage, err)
			}
			if len(plan) != 0 {
				t.Fatalf("choice %d stage %d returned packets; the chain must come from the timer", choice, stage)
			}
			terminal := stage >= endpoint
			if got := w.apocalypsePending != nil; got != terminal {
				t.Fatalf("choice %d (endpoint %d) stage %d armed=%v, want %v", choice, endpoint, stage, got, terminal)
			}
		}
	}
	// 两个难度的终点必须不同，否则这条修复没有意义。
	if legion.ApocalypseEndpoint(0) == legion.ApocalypseEndpoint(1) {
		t.Fatalf("both difficulties share the endpoint %d", legion.ApocalypseEndpoint(0))
	}
}

// 挂起任务到期后发出翻牌链，并且只能发一次。
func TestApocalypseSettlementDueFiresOnce(t *testing.T) {
	w := apocalypseRewardSession(t, 0, 6)
	if !w.armApocalypseSettlement(legion.CmdRewardEnd, "terminal", 0) {
		t.Fatal("the settlement was not armed")
	}
	if w.armApocalypseSettlement(legion.CmdRewardEnd, "terminal", 0) {
		t.Fatal("a second settlement was armed on top of the pending one")
	}
	packets, events := w.apocalypseSettlementDue(time.Now().Add(time.Second))
	if len(packets) == 0 {
		t.Fatalf("due settlement produced nothing (events %v)", events)
	}
	if !hasEvent(events, "apocalypse_full_clear_reward_committed") {
		t.Fatalf("events %v", events)
	}
	again, _ := w.apocalypseSettlementDue(time.Now().Add(2 * time.Second))
	if len(again) != 0 {
		t.Fatalf("settlement fired twice: %d packets", len(again))
	}
	// 未到期的挂起不发。
	w2 := apocalypseRewardSession(t, 0, 6)
	w2.armApocalypseSettlement(legion.CmdRewardEnd, "terminal", time.Hour)
	if p, _ := w2.apocalypseSettlementDue(time.Now()); len(p) != 0 {
		t.Fatalf("a not-yet-due settlement fired: %d packets", len(p))
	}
}

func hasID(ids []uint16, want uint16) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// apocalypseStageSession 是一个正在打某一关的会话：副本号 = 该阶段，房间里
// 有一只还没死的怪。
func apocalypseStageSession(t *testing.T, stage int, living bool) *worldSession {
	t.Helper()
	w := apocalypseRewardSession(t, 0, stage+1)
	s := &dungeon.Session{
		Definition: catalog.DungeonDefinition{ID: legion.ApocalypseStageDungeons[stage]},
		RunID:      "dungeon-run",
		Loaded:     true,
		Dead:       map[uint16]bool{},
		Visited:    map[uint32][]protocol.DungeonMonster{},
		Monsters:   []protocol.DungeonMonster{{Entity: 4096, Template: 65000000, Rank: 3, Level: 115, Team: 100}},
	}
	if !living {
		s.Dead[4096] = true
	}
	w.activeDungeon = s
	w.apocalypse.Stage = stage + 1
	w.apocalypse.PhaseCleared = stage
	return w
}

// 阶段清怪投影：房间里还有活怪时什么都不发；怪清光后只推一次权威 N2895，
// 并把阶段号推进到「已到达下一阶段」（客户端据此发 CMD2062）。
//
// ⚠️ 调用时机是契约的一部分：**必须在 bossCheck（CMD117）里调**，不能在
// monsterDeath 那一批里调。参考抓包 47.51 N38 → 47.53 c2s CMD117 →
// 47.54 s2c N2895 → 47.56 c2s CMD2062 —— 客户端是 CMD117 被受理之后才对
// N2895 起反应的；发早了客户端永远不走进下一间（实机两局症状：站在门里没反应）。
// 见 TestApocalypseStageProjectionRunsOnBossCheck。
func TestApocalypseStageProjection(t *testing.T) {
	// 怪还活着：不推。
	w := apocalypseStageSession(t, 2, true)
	if out := w.apocalypseStageProjection(func(map[string]any) {}); out != nil {
		t.Fatalf("projection fired with a living monster: %+v", out)
	}
	if w.apocalypseStageCleared[2] {
		t.Fatal("a stage with a living monster was marked cleared")
	}
	// 怪清光：推一次 N2895，stage 推进到 3。
	w = apocalypseStageSession(t, 2, false)
	notes := 0
	out := w.apocalypseStageProjection(func(map[string]any) { notes++ })
	if len(out) != 1 || out[0].ID != legion.NotiLegionInfo || out[0].Kind != 0 {
		t.Fatalf("projection packets %+v", out)
	}
	if len(out[0].Payload) != legion.LegionInfoSize {
		t.Fatalf("N2895 length %d, want %d", len(out[0].Payload), legion.LegionInfoSize)
	}
	if notes != 1 {
		t.Fatalf("events = %d, want 1", notes)
	}
	if w.apocalypse.Stage != 3 || w.apocalypse.PhaseCleared != 3 {
		t.Fatalf("run after projection = %+v", w.apocalypse)
	}
	// 同一阶段不重复推（重复的死亡上报不能再推一次）。
	if again := w.apocalypseStageProjection(func(map[string]any) {}); again != nil {
		t.Fatalf("projection fired twice for one stage: %+v", again)
	}
	// 攻坚房间（stage0）清空同样要发权威 N2895（参考抓包 47.54s），但不推进阶段号。
	nav := apocalypseStageSession(t, 0, false)
	navNotes := 0
	navOut := nav.apocalypseStageProjection(func(map[string]any) { navNotes++ })
	if len(navOut) != 1 || navOut[0].ID != legion.NotiLegionInfo || navOut[0].Kind != 0 {
		t.Fatalf("the navigation room must still project N2895: %+v", navOut)
	}
	if navNotes != 1 {
		t.Fatalf("navigation events = %d, want 1", navNotes)
	}
	if !nav.apocalypseStageCleared[0] || nav.apocalypse.Stage != 1 {
		t.Fatalf("navigation room state = %+v (stage %d)", nav.apocalypse, nav.apocalypse.Stage)
	}
	// 终点关：标记副本完成（终局翻牌链由 completeApocalypseStage 挂起）。
	term := apocalypseStageSession(t, len(legion.ApocalypseStageDungeons)-1, false)
	if out := term.apocalypseStageProjection(func(map[string]any) {}); out != nil {
		t.Fatalf("the terminal stage produced an advance frame: %+v", out)
	}
	if !term.activeDungeon.Completed() {
		t.Fatal("the terminal stage did not mark the dungeon completed")
	}
	// 非末世录副本不接手。
	foreign := apocalypseStageSession(t, 1, false)
	foreign.activeDungeon.Definition.ID = 100004131
	if out := foreign.apocalypseStageProjection(func(map[string]any) {}); out != nil {
		t.Fatalf("a foreign dungeon produced a projection: %+v", out)
	}
}

// 阶段投影的**调用时机**是契约：参考抓包 47.51 N38 → 47.53 c2s CMD117 →
// 47.54 s2c N2895 → 47.56 c2s CMD2062。所以：
//
//  1. 死亡确认（CMD39 → completeDungeon）那一批**不能**带 N2895；
//  2. CMD117 被受理（bossCheck）时才发 N2895。
//
// 这条测试直接驱动真实入口，钉住顺序 —— 发早了客户端永远不走进下一间
// （实机两局症状都是「站在门里没反应」，而服务端帧与参考逐字节相同，只有顺序不同）。
func TestApocalypseStageProjectionRunsOnBossCheck(t *testing.T) {
	// 攻坚房间（stage0）：怪物已死、房间已清，只差客户端上报 CMD117。
	nav := apocalypseStageSession(t, 0, false)
	nav.apocalypseStageCleared = [6]bool{}
	nav.apocalypse.Stage = 1
	nav.apocalypse.Cleared = 0

	// 1) 死亡确认那一批（completeDungeon）不得带 N2895。
	deathBatch, err := nav.completeDungeon()
	if err != nil {
		t.Fatalf("completeDungeon: %v", err)
	}
	if hasPacket(deathBatch, legion.NotiLegionInfo) {
		t.Fatalf("the death batch must not carry N2895: %+v", deathBatch)
	}
	if nav.apocalypseStageCleared[0] {
		t.Fatal("the stage was projected before CMD117 arrived")
	}

	// 2) CMD117 被受理那一刻才发 N2895，并标记该阶段已清。
	body := make([]byte, 16)
	binary.LittleEndian.PutUint16(body, 7) // actor = the owned wire id
	binary.LittleEndian.PutUint16(body[2:], 4096)
	plan, err := nav.bossCheck(body, nil)
	if err != nil {
		t.Fatalf("bossCheck: %v", err)
	}
	if !hasPacket(plan, legion.NotiLegionInfo) {
		t.Fatalf("bossCheck did not project N2895: %+v", plan)
	}
	if !nav.apocalypseStageCleared[0] {
		t.Fatal("the navigation stage was not marked cleared at the bossCheck step")
	}
}

// 翻牌卡组必须由末世录自己冻结（与 freezeVenusCards 同款），内容 = N2252 的
// 奖励行；不能落进通用 PlanCards 的金币卡组 —— 那正是「翻牌看不到物品」的成因
// （业主 2026-10-08：「可以把翻牌删除，参考维纳斯重做」）。
func TestApocalypseCardPlanCarriesRewardRows(t *testing.T) {
	// 用难度2：固定行 8 条 + 随机装备，正好覆盖「定长截断到 8」的边界。
	table, err := legion.ApocalypseRewardFor(1)
	if err != nil {
		t.Fatal(err)
	}
	gear := []uint32{100251140, 100323445}
	plan := apocalypseCardPlan(1, table, gear, "run-abc", "fixture", 115)
	if plan.Run != "run-abc" || plan.Model != "apocalypse-terminal-v1" {
		t.Fatalf("plan identity = %+v", plan)
	}
	// 前 8 项 = 通关证明 + 固定材料 + 随机装备（按定长截断）。
	want := apocalypseRewardItems(table, gear)
	if len(want) < 8 {
		t.Fatalf("reward rows = %d, want at least 8", len(want))
	}
	for i := 0; i < 8; i++ {
		if plan.Items[i].Template != want[i].Template || plan.Items[i].Amount != want[i].Amount {
			t.Fatalf("item %d = %+v, want %+v", i, plan.Items[i], want[i])
		}
	}
	// 前两格是随机装备（与维纳斯同序：装备在前、材料在后），
	// 这样固定材料再多也不会把随机位挤掉。
	if plan.Items[0].Template != gear[0] || plan.Items[1].Template != gear[1] {
		t.Fatalf("the first two cards = %d/%d, want the rolled gear %d/%d",
			plan.Items[0].Template, plan.Items[1].Template, gear[0], gear[1])
	}
	if plan.Items[2].Template != legion.ApocalypseRewardProofExpert {
		t.Fatalf("third card = %d, want the clear proof %d", plan.Items[2].Template, legion.ApocalypseRewardProofExpert)
	}
	// 随机装备必须在卡组里（翻牌界面的随机位就靠它）。
	for _, g := range gear {
		found := false
		for _, it := range plan.Items {
			if it.Template == g {
				found = true
			}
		}
		if !found {
			t.Fatalf("the rolled gear %d is missing from the card plan: %+v", g, plan.Items)
		}
	}
}

// 终局奖励门的两帧 N2895（规格 G0454 / 2252 / 2253）。
//
// 规格原文（三处互证）：
//
//	2895-LEGIONINFO末世录状态.md G0454：「仅个人最终结果阶段将线级 Stage 投影到
//	  该难度固定终点 3/5，并同步目标记录；**N2252 之前以 Outcome0 安装，N2253
//	  之后才 Outcome3**。持续结果通知保持该终点。」
//	2252-LEGIONBASICREWARD.md：「生产在本包之前增加 N2895 终点投影/Outcome0，
//	  随后 2252→2253→**N2895 同终点/Outcome3**。」
//	2253-LEGIONADDITIONALREWARD.md：「apocalypseResultPackets **最后追加 N2895
//	  Outcome3**。完成状态只在奖励展示批次生成。」
//
// 这一帧是通关演出/结算窗的触发点；本实现此前完全没有，所以客户端不进终局演出。
func TestApocalypseTerminalEndpointProjection(t *testing.T) {
	cases := []struct {
		choice   byte
		endpoint int
		marks    []byte
	}{
		{0, 3, []byte{0, 1, 2, 3, 0xff, 0xff}},
		{1, 5, []byte{0, 1, 2, 3, 4, 5}},
	}
	for _, c := range cases {
		if got := legion.ApocalypseEndpoint(c.choice); got != c.endpoint {
			t.Fatalf("Endpoint(%d) = %d, want %d", c.choice, got, c.endpoint)
		}
		run := legion.NewApocalypseRunState()
		run.Entered = true
		run.Choice = c.choice
		run.RoleSet = true

		before := apocalypseTerminalEndpointInfo(run, 0)
		after := apocalypseTerminalEndpointInfo(run, 3)
		// Outcome0 在前，Outcome3 在后 —— 两帧除 Outcome 外同终点。
		// 载荷 @5 起是 State/Outcome **共用的那个 u32** 的最低位（客户端读 @5）：
		//	Outcome0 → State2 的高位 0 → @5 的值为 2
		//	Outcome3 → 高位 3          → @5 的值为 2，@6 为 3
		// 2 号权威抓包 idx=640 正是 `01 02 00 00`（choice=1、@5=02、@6=00）。
		// State（载荷 @5..8 u32）：两帧都是「作战进行中」的 2。
		// 权威抓包 idx=640 的 `@5..8 = 02 00 00 00` 就是它。
		for _, p := range [][]byte{before, after} {
			if got := binary.LittleEndian.Uint32(p[5:]); got != 2 {
				t.Fatalf("choice %d: State @5 = %d, want 2", c.choice, got)
			}
		}
		// ★ Outcome（载荷 @9..12 u32）：**规格 G0454 的关键字段** ——
		// N2252 之前 0，N2253 之后 3。权威抓包 idx=640 该位是 5（路线终点），
		// 本实现的「终点投影」按规格取难度端点。
		if got := binary.LittleEndian.Uint32(before[9:]); got != 0 {
			t.Fatalf("choice %d: pre-N2252 Outcome @9 = %d, want 0", c.choice, got)
		}
		if got := binary.LittleEndian.Uint32(after[9:]); got != 3 {
			t.Fatalf("choice %d: post-N2253 Outcome @9 = %d, want 3", c.choice, got)
		}
		// Stage（载荷 @13）投影到**固定终点**（不是本次实际关数），两帧一致。
		for _, p := range [][]byte{before, after} {
			if got := binary.LittleEndian.Uint32(p[13:]); got != uint32(c.endpoint) {
				t.Fatalf("choice %d: Stage @13 = %d, want %d", c.choice, got, c.endpoint)
			}
		}
		// Following（载荷 @17）必须保持恒定哨兵 ffffffff —— 这一格**不是**
		// Outcome，本实现曾把两者弄反。
		for _, p := range [][]byte{before, after} {
			if got := binary.LittleEndian.Uint32(p[17:]); got != 0xffffffff {
				t.Fatalf("choice %d: Following @17 = %#x, want ffffffff", c.choice, got)
			}
		}
		// 阶段记录同步到终点。
		for i, want := range c.marks {
			if got := after[29+12*i]; got != want {
				t.Fatalf("choice %d: mark %d = %#x, want %#x", c.choice, i, got, want)
			}
		}
	}
}
