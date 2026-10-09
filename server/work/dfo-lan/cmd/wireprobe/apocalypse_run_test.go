package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"sort"
	"testing"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"dfolan/internal/testfixture"
)

// apocalypseSession is a连接 on the 末世录 directory row with the compiled table
// loaded, which is what the client's CMD2043/2354/2045 sequence arrives on.
func apocalypseSession(t *testing.T) *legionSession {
	t.Helper()
	cat, err := catalog.LoadApocalypseCatalog("../../configs/apocalypse.generated.json")
	if err != nil {
		t.Fatalf("load apocalypse catalog: %v", err)
	}
	clock, err := legion.NewApocalypseClock(cat)
	if err != nil {
		t.Fatalf("apocalypse clock: %v", err)
	}
	return &legionSession{channelType: apocalypseChannelType, catalog: cat, clock: clock}
}

// apocalypseTestDungeons 给测试用的副本目录：借 testfixture 里的一套迷宫，
// 把末世录六个阶段副本号按同一形状登记进去，这样 CMD2045 的进图路径
// （dungeon.Select + dungeonEntryPlanImpl）在单测里也能真的走一遍。
// 与 internal/legion/entry_plan_test.go 的 donor 手法一致。
func apocalypseTestDungeons(t *testing.T) *catalog.DungeonCatalog {
	t.Helper()
	dc, err := catalog.LoadDungeons(testfixture.DungeonPath(t, "dungeons.odyssey-scenes-release.json"))
	if err != nil {
		t.Fatalf("load dungeon fixture: %v", err)
	}
	// ★ donor 必须**确定性且真能开出战斗关**地挑。
	//
	// 两个坑（都是新加的 TestApocalypseServerAdvancePublishesStage 真的载入第 1 关
	// 才暴露出来的既有夹具缺陷）：
	//  1. `range` 一个 map 的顺序在 Go 里是随机的 ⇒ donor 每次都不同；
	//  2. 不同 donor 的房间图不同，有的解析不出迷宫声明的「源起始房」
	//     （`dungeon.Select` 会报 missing source start room）。
	// 所以按 ID 升序，并用 `dungeon.Select` **本身当探针**，取第一个能开出
	// 第 1 关（战斗关）的 donor —— 既确定，又不需要先起一个会话。
	ids := make([]uint32, 0, len(dc.Dungeons))
	for id := range dc.Dungeons {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	cloneOf := func(d catalog.DungeonDefinition) catalog.DungeonDefinition {
		c := d
		c.Odyssey = false
		c.Tutorial = false
		c.MinimumLevel = 1
		c.Mazes = nil
		for _, mz := range d.Mazes {
			mz.Quest = 0
			mz.Pending = nil
			c.Mazes = append(c.Mazes, mz)
		}
		return c
	}
	probe := func(d catalog.DungeonDefinition) bool {
		for _, id := range legion.ApocalypseStageDungeons {
			c := cloneOf(d)
			c.ID = id
			dc.Dungeons[id] = c
		}
		_, err := dungeon.Select(dc, protocol.DungeonSelection{
			ID: legion.ApocalypseStageDungeons[1], Difficulty: 0, Party: 65535}, 200, nil)
		return err == nil
	}
	var donor catalog.DungeonDefinition
	var donorID uint32
	for _, id := range ids {
		d := dc.Dungeons[id]
		// donor 要求：有迷宫、不是奥德赛（奥德赛会强绑 DesignatedDifficulty）、
		// 不是教程（allowTutorial=false 时会被拒），且真能开出战斗关。
		if len(d.Mazes) > 0 && !d.Odyssey && !d.Tutorial && probe(d) {
			donor, donorID = d, id
			break
		}
	}
	if donorID == 0 {
		t.Skip("fixture carries no ordinary dungeon that can open an apocalypse combat stage")
	}
	for _, id := range legion.ApocalypseStageDungeons {
		// 无条件覆盖这六个 ID（不再因夹具里已有同名项而跳过），让被测状态完全由
		// 本 helper 决定；迷宫清 Quest 与 Pending 的细节见 cloneOf。
		clone := cloneOf(donor)
		clone.ID = id
		dc.Dungeons[id] = clone
	}
	return &dc
}

// apocalypseTownSession 是带副本目录的连接：CMD2045 现在会真的载入攻坚房间，
// 所以凡是走 2045 的用例都需要它。
func apocalypseTownSession(t *testing.T, characterID int64) *worldSession {
	t.Helper()
	w := townSession(characterID)
	w.level = 200
	w.dungeons = apocalypseTestDungeons(t)
	return w
}

// operationPayload mirrors the client's CMD2354 sender: int32 @13 (action),
// char @17 (the CTP key on a confirm), int32 @18 (107).
func operationPayload(action uint32, key byte, channel uint32) []byte {
	p := make([]byte, legion.EnvelopeSize+9)
	putU32(p, legion.EnvelopeSize, action)
	p[legion.EnvelopeSize+4] = key
	putU32(p, legion.EnvelopeSize+5, channel)
	return p
}

// enterPayload mirrors the client's CMD2045 sender: int32 @13 (107), int32 @17
// (the stage field the capture shows as 0 / 2).
func enterPayload(channel, stage uint32) []byte {
	p := make([]byte, legion.EnvelopeSize+8)
	putU32(p, legion.EnvelopeSize, channel)
	putU32(p, legion.EnvelopeSize+4, stage)
	return p
}

// infoOf finds the NOTI2895 frame in a result, failing when it is absent. The
// 2895 is what carries the run state the client renders, so a missing one is a
// regression even when the acknowledgement is present.
func infoOf(t *testing.T, packets []outboundPacket) []byte {
	t.Helper()
	for _, p := range packets {
		if p.ID == legion.NotiLegionInfo && p.Kind == 0 {
			if len(p.Payload) != legion.LegionInfoSize {
				t.Fatalf("NOTI2895 length %d, want %d", len(p.Payload), legion.LegionInfoSize)
			}
			return p.Payload
		}
	}
	t.Fatalf("no NOTI2895 in %+v", packets)
	return nil
}

func hasPacket(packets []outboundPacket, id uint16) bool {
	for _, p := range packets {
		if p.ID == id {
			return true
		}
	}
	return false
}

// ★ 回归护栏（2026-10-08 实机）：暂停中的同一作战，客户端报来的阶段若是**旧值**，
// 必须按保存阶段续关，**不能硬拒**。
//
// `requested` 来自客户端最后收到的 NOTI2895 @13（它把 @13 原样回送成 CMD2045 @17）。
// 一旦服务端漏发那一帧（例如服务端主动推进那条路径——根因已在
// apocalypse_stage.go 补掉），客户端手上就是旧值。硬拒的表现就是业主报的
// 「点右上角『进入』没有任何反应」，实机 16:36 的铁证：
//
//	legion_refused id=2045 request_hex="…006b000000 00000000 00000000"
//	reason="apocalypse resume stage 0 does not match the saved stage 2"
//
// 本场只有这一条 run（Suspended = 同一作战已暂停），服务端才是权威。
func TestApocalypseResumeToleratesStaleClientStage(t *testing.T) {
	// 真实形态：已确认难度、正在第 1 关（run.Stage=2），死亡被请离后已回城、
	// 只有 run 处于挂起。
	w := apocalypseStageSession(t, 1, false)
	w.level = 200
	w.dungeons = apocalypseTestDungeons(t)
	w.activeDungeon = nil
	s := w.legion
	run := w.apocalypse
	run.Suspended = true

	// 客户端报 0（它手上的 N2895 是旧的）。
	result, err := s.handle(w, enterPayload(legion.OperationChannelCode, 0), legion.CmdEnterDungeon)
	if err != nil {
		t.Fatalf("a stale resume stage was refused — the player sees 点进入没有任何反应: %v", err)
	}
	// 必须按**保存阶段**续关：ResumeStage = ContinueStage()-1 = 1（第 1 关）。
	if run.ResumeStage != 1 {
		t.Fatalf("ResumeStage = %d, want 1 (resume the saved stage, not the stale request)", run.ResumeStage)
	}
	if !run.Entered || run.Suspended {
		t.Fatalf("after resume: entered=%v suspended=%v, want true/false", run.Entered, run.Suspended)
	}
	// 陈旧值必须留痕，便于继续收敛「哪些路径漏发 NOTI2895」。
	var noted bool
	for _, e := range result.Events {
		if stale, _ := e["resume_stage_stale"].(bool); stale {
			noted = true
		}
	}
	if !noted {
		t.Fatalf("the stale resume stage was accepted silently; events = %+v", result.Events)
	}
}

// ★ A4（2026-10-09 03:0x 实机）：**客户端报什么值都不影响重建哪一间**。
//
// 实机反例（会话 roles_..._20261009_030620_232753_next37，同一条 run）：
//
//	19:10:43  服务端 run.Stage=3、客户端报 2（陈旧一格）
//	          → 旧代码命中 `case want-1` 并算 requested-1 = 1
//	          → 载入 100004995（第 1 关）✗ 业主报的「回到前一关」
//	19:11:06  服务端 3、客户端 3 → 载入 100005057（第 2 关）✓
//
// 同场两种结果并存，正是业主报的「不一定回到刚退出那一关」。
// 现在续关**一律**取 run.ContinueStage()-1，客户端值只作诊断（resume_stage_stale）。
func TestApocalypseResumeIgnoresClientStageDelta(t *testing.T) {
	for _, c := range []struct {
		name      string
		runStage  int // 服务端保存的 run.Stage（= 已进入过的房间数）
		requested uint32
		wantIndex int
	}{
		{"客户端与保存值一致", 3, 3, 2},
		{"客户端陈旧一格（旧代码会落早一关）", 3, 2, 2},
		{"客户端陈旧两格", 3, 1, 2},
		{"客户端超前一格", 3, 4, 2},
		{"客户端报 0", 2, 0, 1},
		{"客户端报 0、在第 5 关", 6, 0, 5},
	} {
		w := apocalypseStageSession(t, 1, false)
		w.level = 200
		w.dungeons = apocalypseTestDungeons(t)
		w.activeDungeon = nil
		s := w.legion
		run := w.apocalypse
		run.Stage = c.runStage
		run.Suspended = true

		result, err := s.handle(w, enterPayload(legion.OperationChannelCode, c.requested), legion.CmdEnterDungeon)
		if err != nil {
			t.Fatalf("%s: 续关被拒（玩家会看到点进入没反应）：%v", c.name, err)
		}
		if run.ResumeStage != c.wantIndex {
			t.Fatalf("%s: 服务端 run.Stage=%d、客户端报 %d ⇒ 重建下标 %d, want %d"+
				"（一律以服务端保存阶段为准）", c.name, c.runStage, c.requested, run.ResumeStage, c.wantIndex)
		}
		if len(result.Events) == 0 {
			t.Fatalf("%s: 没有事件", c.name)
		}
	}
}

// The whole town-side sequence of one difficulty-1 clear, as the 2026-10-08
// capture shows it: CMD2043 → CMD2354 action1 → CMD2354 action2(key 0) →
// CMD2045 → CMD2355. The stage itself is requested by CMD2062, which this test
// does not exercise (it needs the dungeon catalog).
func TestApocalypseTownFlowMatchesCapture(t *testing.T) {
	s := apocalypseSession(t)
	w := apocalypseTownSession(t, 7)

	// CMD2043: the generic handler answers, but the run opens at the waiting
	// state (choice unset, marks unset) - see TestLegionStartAnswersWithAckThenInfo.
	if _, err := s.handle(w, startPayload(107), legion.CmdStart); err != nil {
		t.Fatalf("CMD2043: %v", err)
	}
	run := s.apocalypseRun()
	if run.Choice != 0xff || run.Entered {
		t.Fatalf("fresh run = %+v", run)
	}

	// CMD2354 action1: the client opens/refreshes the operation screen. The
	// capture shows the key byte as a stale screen value (0x0d), which must not
	// be read as a difficulty.
	result, err := s.handle(w, operationPayload(1, 0x0d, legion.OperationChannelCode), legion.CmdOperationSelect)
	if err != nil {
		t.Fatalf("CMD2354 open: %v", err)
	}
	if got := run.Choice; got != 0xff {
		t.Fatalf("action1 changed the difficulty to %#x", got)
	}
	if !hasPacket(result.Packets, legion.CmdOperationSelect) {
		t.Fatal("action1 was not acknowledged")
	}
	if hasPacket(result.Packets, legion.NotiLegionInfo) {
		t.Fatal("action1 must not publish a difficulty state")
	}
	// ★ 端到端护栏：开窗（action1）的 ACK **必须**带非 0 的倒计时截止值。
	//
	// 业主 2026-10-08 报「难度框倒计时是 0、不会自动关闭、不选难度就卡死」，
	// 成因就是这一格全零（客户端对 0 不做任何事 ⇒ 永不关窗）。维纳斯的同类框
	// 正常，差别即它带了截止值。此处钉住调用点，防止再退回「暂不下发」。
	//
	// ★ 2026-10-09 第三段结论（业主要求把倒计时加回）：该格**必须带绝对 UNIX 秒**。
	//
	// 演进史（三段实机，别再翻回去）：
	//  1. 不下发（该格 = 0）⇒ 倒计时恒 0、永不自动关窗；
	//  2. 写入绝对 UNIX 秒 ⇒ 倒计时 15→0 与自动关窗正常 ✓；
	//  3. 同期地图内出现三症状（复活币数字 99 / 难度2 出现复活提示 / 药水被禁），
	//     当时**误判**为该格所致并把该格改回 0（A' 版）—— 实测数字**仍是 99**，
	//     ⇒ 该格无辜。真凶是第4/5轮在房间里补发的两帧 N2895，已删除
	//     （见 TestApocalypseAdvancePathsDoNotRepublishStage）。
	//  4. 补发删除后（A''）数字恢复 8 ✓、药水 ✓、续关 ✓ ⇒ 本版把截止值加回。
	{
		var ack []byte
		for _, p := range result.Packets {
			if p.ID == legion.CmdOperationSelect {
				ack = p.Payload
			}
		}
		if ack == nil {
			t.Fatal("action1 ack payload missing")
		}
		if len(ack) <= 12 {
			t.Fatalf("action1 ack length %d, want %d", len(ack), legion.OperationAckSize)
		}
		if ack[8] != 0 {
			t.Fatalf("action1 ack close bit @8 = %#x, want 0", ack[8])
		}
		deadline := binary.LittleEndian.Uint32(ack[9:])
		if deadline == 0 {
			t.Fatal("action1 ack 没有截止值 —— 客户端倒计时恒为 0 且永不自动关窗（2026-10-08 实机）")
		}
		want := uint32(time.Now().Unix()) + legion.ApocalypseSelectionSeconds
		if deadline+2 < want || deadline > want+2 {
			t.Fatalf("action1 deadline %d, want ~%d (now + %d)", deadline, want, legion.ApocalypseSelectionSeconds)
		}
		// 服务端自己的窗口调度也必须已排程（到点下发关闭帧，与客户端倒计时并列）。
		if run.WindowDeadline.IsZero() {
			t.Fatal("action1 之后 run.WindowDeadline 必须已排程（服务端到点关窗）")
		}
	}

	// CMD2354 action2: key 0 = difficulty 1. The capture answers with the
	// difficulty NOTI2895 first, then the 2354 acknowledgement.
	result, err = s.handle(w, operationPayload(2, 0x00, legion.OperationChannelCode), legion.CmdOperationSelect)
	if err != nil {
		t.Fatalf("CMD2354 confirm: %v", err)
	}
	if run.Choice != 0x00 {
		t.Fatalf("confirmed choice = %#x, want 0", run.Choice)
	}
	if result.Packets[0].ID != legion.NotiLegionInfo || result.Packets[1].ID != legion.CmdOperationSelect {
		t.Fatalf("confirm order = %d,%d", result.Packets[0].ID, result.Packets[1].ID)
	}
	// 确认（action2）不再需要计时：截止值回 0，且关闭位保持 0。
	if ack := result.Packets[1].Payload; binary.LittleEndian.Uint32(ack[9:]) != 0 || ack[8] != 0 {
		t.Fatalf("action2 ack deadline/close = %#x/%#x, want 0/0", binary.LittleEndian.Uint32(ack[9:]), ack[8])
	}
	info := infoOf(t, result.Packets)
	if info[4] != 0x00 || binary.LittleEndian.Uint32(info[5:]) != 2 {
		t.Fatalf("choice=%#x state=%d run=%+v full=% x", info[4], binary.LittleEndian.Uint32(info[5:]), run, info)
	}
	if !hasEvent(result.Events, "legion_difficulty_selected") {
		t.Fatalf("events %v", result.Events)
	}

	// CMD2045: the operation is confirmed. This only acknowledges and publishes
	// the initial destinations; the waiting room is loaded by CMD2062.
	result, err = s.handle(w, enterPayload(legion.OperationChannelCode, 0), legion.CmdEnterDungeon)
	if err != nil {
		t.Fatalf("CMD2045: %v", err)
	}
	if !hasPacket(result.Packets, legion.CmdEnterDungeon) || !hasPacket(result.Packets, legion.NotiLegionInfo) {
		t.Fatalf("CMD2045 packets %+v", result.Packets)
	}
	if !run.Entered {
		t.Fatal("run not marked entered after CMD2045")
	}
	// CMD2045 现在会**自己**载入攻坚房间（参考抓包 41.12–41.15s），所以这条
	// 事件的 kind 取决于载入是否成功：测试夹具的角色资料不完整时会是
	// apocalypse_stage_entry_failed（协议应答仍然照发，这是有意的降级）。
	if !hasEvent(result.Events, "apocalypse_navigation_prepared") && !hasEvent(result.Events, "apocalypse_stage_entry_failed") {
		t.Fatalf("events %v", result.Events)
	}

	// CMD2355: role assignment. The capture shows it arriving after the waiting
	// room loads, but this test cannot load one; it asserts the gate accepts it
	// as soon as the run is entered.
	result, err = s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect)
	if err != nil {
		t.Fatalf("CMD2355: %v", err)
	}
	if !run.RoleSet || run.Role != 1 {
		t.Fatalf("role not recorded: %+v", run)
	}
	if !hasPacket(result.Packets, legion.CmdRoleSelect) || !hasPacket(result.Packets, legion.NotiLegionInfo) {
		t.Fatalf("CMD2355 packets %+v", result.Packets)
	}
	if note := infoOf(t, result.Packets); note[109] != 1 {
		t.Fatalf("role count = %d, want 1", note[109])
	}

	// CMD2046: the reward screen is closed. This is the settlement signal, so the
	// answer must be the flip-card chain (or the ack alone when the chain was
	// already delivered by the clear-delay task) and must always end with the ack.
	result, err = s.handle(w, rewardEndPayload(0, 0, 0), legion.CmdRewardEnd)
	if err != nil {
		t.Fatalf("CMD2046: %v", err)
	}
	if !hasPacket(result.Packets, legion.CmdRewardEnd) {
		t.Fatalf("CMD2046 packets %+v", result.Packets)
	}
	note := result.Events[len(result.Events)-1]
	if note["reward_chain"] != "delivered" && note["reward_chain"] != "already_delivered" {
		t.Fatalf("CMD2046 note %v", note)
	}
	if last := result.Packets[len(result.Packets)-1]; last.ID != legion.CmdRewardEnd || last.Kind != 1 {
		t.Fatalf("CMD2046 must end with its acknowledgement: %+v", result.Packets)
	}
	if !run.Ended {
		t.Fatal("run not marked ended")
	}
	if run.Rewarded {
		// run.Rewarded is set by the terminal chain; the test session has no loot
		// service, so the chain still runs and marks the run as paid.
		if note["reward_chain"] != "delivered" {
			t.Fatalf("rewarded run must report delivered: %v", note)
		}
	}
}

// The difficulty key is validated against the compiled table: the release table
// declares operations 1/2/3/5, so key 3 (operation 4) must be refused instead of
// answered with an invented plan.
func TestApocalypseDifficultyKeyValidatedAgainstTable(t *testing.T) {
	s := apocalypseSession(t)
	w := apocalypseTownSession(t, 7)
	if _, err := s.handle(w, startPayload(107), legion.CmdStart); err != nil {
		t.Fatalf("CMD2043: %v", err)
	}
	for _, key := range []byte{3, 0xff} {
		if _, err := s.handle(w, operationPayload(2, key, legion.OperationChannelCode), legion.CmdOperationSelect); err == nil {
			t.Fatalf("difficulty key %#x was accepted", key)
		}
	}
	for _, key := range []byte{0, 1, 2, 4} {
		if _, err := s.handle(w, operationPayload(2, key, legion.OperationChannelCode), legion.CmdOperationSelect); err != nil {
			t.Fatalf("difficulty key %#x was refused: %v", key, err)
		}
	}
}

// Entering and assigning a role are both gated on the run actually having been
// confirmed, so a replay of the packets cannot open a run the client never
// started.
func TestApocalypseEntryAndRoleRequireConfirmedRun(t *testing.T) {
	s := apocalypseSession(t)
	w := apocalypseTownSession(t, 7)

	if _, err := s.handle(w, enterPayload(legion.OperationChannelCode, 0), legion.CmdEnterDungeon); err == nil {
		t.Fatal("CMD2045 accepted before a difficulty was confirmed")
	}
	if _, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect); err == nil {
		t.Fatal("CMD2355 accepted before the run was entered")
	}

	if _, err := s.handle(w, startPayload(107), legion.CmdStart); err != nil {
		t.Fatalf("CMD2043: %v", err)
	}
	if _, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect); err == nil {
		t.Fatal("CMD2355 accepted after CMD2043 but before CMD2045")
	}
	if _, err := s.handle(w, operationPayload(2, 0, legion.OperationChannelCode), legion.CmdOperationSelect); err != nil {
		t.Fatalf("CMD2354: %v", err)
	}
	if _, err := s.handle(w, enterPayload(legion.OperationChannelCode, 0), legion.CmdEnterDungeon); err != nil {
		t.Fatalf("CMD2045: %v", err)
	}
	if _, err := s.handle(w, rolePayload(0, legion.OperationChannelCode), legion.CmdRoleSelect); err == nil {
		t.Fatal("role 0 was accepted")
	}
	if _, err := s.handle(w, rolePayload(1, 999), legion.CmdRoleSelect); err == nil {
		t.Fatal("a wrong content channel was accepted")
	}
	if _, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect); err != nil {
		t.Fatalf("CMD2355: %v", err)
	}
}

// A fresh CMD2043 on the same connection must drop the previous run's
// difficulty, stage and role so the next entry starts clean.
func TestApocalypseStartResetsTheRun(t *testing.T) {
	s := apocalypseSession(t)
	w := apocalypseTownSession(t, 7)
	if _, err := s.handle(w, startPayload(107), legion.CmdStart); err != nil {
		t.Fatalf("CMD2043: %v", err)
	}
	if _, err := s.handle(w, operationPayload(2, 1, legion.OperationChannelCode), legion.CmdOperationSelect); err != nil {
		t.Fatalf("CMD2354: %v", err)
	}
	if _, err := s.handle(w, enterPayload(legion.OperationChannelCode, 0), legion.CmdEnterDungeon); err != nil {
		t.Fatalf("CMD2045: %v", err)
	}
	run := s.apocalypseRun()
	run.Stage = 4
	run.PhaseCleared = 4
	if _, err := s.handle(w, startPayload(107), legion.CmdStart); err != nil {
		t.Fatalf("second CMD2043: %v", err)
	}
	if run.Choice != 0xff || run.Entered || run.Stage != 0 || run.PhaseCleared != 0 || run.RoleSet {
		t.Fatalf("run not reset: %+v", run)
	}
	// The channel-entry info packet must be the waiting state again.
	result, err := s.handle(w, rolePayload(1, legion.OperationChannelCode), legion.CmdRoleSelect)
	if err == nil {
		t.Fatalf("role accepted after reset: %+v", result)
	}
}

// apocalypseRan is what lets the dungeon layer take over CMD2062 before its
// generic "no active dungeon" guard, so it must stay false until a run exists.
func TestApocalypseRanGatesDirectMoveHandover(t *testing.T) {
	s := &legionSession{channelType: apocalypseChannelType}
	if s.apocalypseRan() {
		t.Fatal("apocalypseRan true before any run")
	}
	if !s.isApocalypse() {
		t.Fatal("a Type 119 connection must be routed to the apocalypse layer")
	}
	if _, err := s.ApocalypseDirectMove(townSession(7), legion.ApocalypseNavigationDungeon); err == nil {
		t.Fatal("direct move accepted without a run")
	}
	s.apocalypseRun()
	if !s.apocalypseRan() {
		t.Fatal("apocalypseRan false after the run was opened")
	}
	// Without CMD2045 the run is open but not entered, so the waiting room is
	// still refused - the capture only ever shows the move after CMD2045.
	if _, err := s.ApocalypseDirectMove(townSession(7), legion.ApocalypseNavigationDungeon); err == nil {
		t.Fatal("direct move accepted before CMD2045")
	}
	// A foreign dungeon id is never an apocalypse stage.
	other := &legionSession{channelType: apocalypseChannelType}
	other.apocalypseRun().Entered = true
	if _, err := other.ApocalypseDirectMove(townSession(7), 100004131); err == nil {
		t.Fatal("a foreign dungeon was accepted as an apocalypse stage")
	}
}

func hasEvent(events []map[string]any, kind string) bool {
	for _, e := range events {
		if e["kind"] == kind {
			return true
		}
	}
	return false
}

// 实机请求向量（2026-10-08 13:43:16 会话，玩家在末世录频道点创建队伍输入
// 「55555」）：48B，队名 5、容量 4、类型字节 0x26、模式 1，末尾只有 9B 零尾
// （不含测试助手那 8B 槽过滤）。这一段必须逐字节被解码接受，否则就是
// 「输入队名点确定没反应」。
const apocalypseLivePartyRequestHex = "00000500000035353535350400000000000000002601000101020407070707ffffffff00000000000000000000000000"

func TestApocalypseStandbyPartyLiveVector(t *testing.T) {
	req, err := hex.DecodeString(apocalypseLivePartyRequestHex)
	if err != nil {
		t.Fatal(err)
	}
	if len(req) != 48 {
		t.Fatalf("live request length %d, want 48", len(req))
	}
	name, err := protocol.DecodeApocalypseStandbyParty(req)
	if err != nil {
		t.Fatalf("live request refused: %v", err)
	}
	if string(name) != "55555" {
		t.Fatalf("party name %q, want 55555", name)
	}
	w := &worldSession{
		channelType: apocalypseChannelType,
		role: database.Character{
			ID: 2, WireID: 2, Name: "ApocalypseCap",
			State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`),
		},
		characters: &character.Service{ChannelContext: [2]byte{0x03, 0x56}},
	}
	handled, packets, err := w.apocalypseStandbyPartyHandle(12, req)
	if !handled || err != nil {
		t.Fatalf("live request handled=%v err=%v", handled, err)
	}
	// 队长资料 ×2 + NOTI9 建队应答 + N2254 入场角色信息（抓包同一位置有它）。
	if len(packets) != 4 {
		t.Fatalf("plan = %+v, want captain basic/addition/NOTI9/N2254", packets)
	}
	if packets[3].ID != legion.NotiEntryCharacterInfo || len(packets[3].Payload) != 272 {
		t.Fatalf("N2254 frame = %+v (len %d), want 272 bytes", packets[3], len(packets[3].Payload))
	}
	if packets[2].ID != 9 || packets[2].Kind != 0 || len(packets[2].Payload) == 0 {
		t.Fatalf("party creation frame = %+v", packets[2])
	}
	// 应答里的队伍类型字节必须与请求一致（0x26），否则客户端不认这支队。
	if !bytes.Contains(packets[2].Payload, []byte{protocol.ApocalypsePartyMode115}) {
		t.Fatalf("reply does not carry the apocalypse party type 0x26")
	}
	if !w.soloPartyReady {
		t.Fatal("standby party creation must arm soloPartyReady")
	}
}

// 末世录待机区 CMD12：[u16 0][u32 名长][队名][u32 容量=4][5 零][类型 0x26][模式 1]
// [槽过滤 8B][ff x4][零尾 8B]。抓包 20261008-105227 的 61B 城镇 CMD12 就是
// 这个形状（类型字节 0x26 = protocol.ApocalypsePartyMode115）。
func apocalypseStandbyPartyRequest(name string, capacity uint32, partyType byte) []byte {
	p := make([]byte, 0, 64)
	p = append(p, 0, 0)
	var v [4]byte
	binary.LittleEndian.PutUint32(v[:], uint32(len(name)))
	p = append(p, v[:]...)
	p = append(p, []byte(name)...)
	binary.LittleEndian.PutUint32(v[:], capacity)
	p = append(p, v[:]...)
	p = append(p, 0, 0, 0, 0, 0)
	p = append(p, partyType, 1, 0)
	p = append(p, []byte{1, 1, 2, 4, 7, 7, 7, 7}...)
	p = append(p, 0xff, 0xff, 0xff, 0xff)
	p = append(p, make([]byte, 8)...)
	return p
}

// 末世录待机区建队/离队：只有 Type 119 频道、已选角、CMD12/13 才接管，
// 且队伍类型字节必须是 0x26（38）。
func TestApocalypseStandbyPartyHandler(t *testing.T) {
	build := func(channelType uint32, roleID int64) *worldSession {
		return &worldSession{
			channelType: channelType,
			role: database.Character{
				ID: roleID, WireID: uint16(roleID), Name: "ApocalypseCap",
				State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`),
			},
			characters: &character.Service{ChannelContext: [2]byte{0x03, 0x56}},
		}
	}
	// 回归护栏（2026-10-08 实机 BUG）：待机区建队 CMD12/13 **不是**军团家族命令，
	// 所以建队处理器必须放在 legion.Requests 门禁之前。这条把那个前提钉住：
	// 如果哪天 legion.Requests 把 12/13 收进去，这里会失败并提醒复核路由顺序。
	if legion.Requests(12) || legion.Requests(13) {
		t.Fatal("CMD12/13 became legion family commands; the standby handler routing must be revisited")
	}
	for _, id := range []uint16{legion.CmdStart, legion.CmdFail, legion.CmdEnterDungeon, legion.CmdRewardEnd, legion.CmdOperationSelect, legion.CmdRoleSelect} {
		if !legion.Requests(id) {
			t.Fatalf("family command %d is no longer claimed by legion.Requests", id)
		}
	}
	// 其它频道不接管。
	if handled, _, _ := build(99, 7).apocalypseStandbyPartyHandle(12, nil); handled {
		t.Fatal("the apocalypse handler claimed a Venus channel")
	}
	// 未选角不接管。
	if handled, _, _ := build(apocalypseChannelType, 0).apocalypseStandbyPartyHandle(12, nil); handled {
		t.Fatal("the handler ran without a selected character")
	}
	// 非 12/13 不接管（家族命令留给 dispatchLegion）。
	if handled, _, _ := build(apocalypseChannelType, 7).apocalypseStandbyPartyHandle(2043, nil); handled {
		t.Fatal("the handler claimed a family command")
	}
	w := build(apocalypseChannelType, 7)
	w.characters.ChannelContext = [2]byte{3, 0x56}
	req := apocalypseStandbyPartyRequest("2221", 4, protocol.ApocalypsePartyMode115)
	handled, packets, err := w.apocalypseStandbyPartyHandle(12, req)
	if !handled || err != nil {
		t.Fatalf("CMD12 handled=%v err=%v", handled, err)
	}
	if len(packets) != 4 || packets[2].ID != 9 || packets[2].Kind != 0 {
		t.Fatalf("CMD12 packets %+v", packets)
	}
	if !w.soloPartyReady {
		t.Fatal("standby party creation must arm soloPartyReady")
	}
	// 类型字节不对（维纳斯的 0x22）必须被拒。
	if _, _, err := w.apocalypseStandbyPartyHandle(12, apocalypseStandbyPartyRequest("2221", 4, 0x22)); err == nil {
		t.Fatal("a Venus party type was accepted")
	}
	// 容量不是 4 必须被拒。
	if _, _, err := w.apocalypseStandbyPartyHandle(12, apocalypseStandbyPartyRequest("2221", 8, protocol.ApocalypsePartyMode115)); err == nil {
		t.Fatal("an 8-seat party was accepted")
	}
	// 副本内离队必须被拒，待机区离队要清 soloPartyReady。
	w.activeDungeon = &dungeon.Session{}
	if _, _, err := w.apocalypseStandbyPartyHandle(13, make([]byte, 8)); err == nil {
		t.Fatal("leaving the party inside a dungeon was accepted")
	}
	w.activeDungeon = nil
	handled, packets, err = w.apocalypseStandbyPartyHandle(13, make([]byte, 8))
	if !handled || err != nil || len(packets) != 1 || packets[0].ID != 9 {
		t.Fatalf("CMD13 handled=%v packets=%+v err=%v", handled, packets, err)
	}
	if w.soloPartyReady {
		t.Fatal("leaving the party must clear soloPartyReady")
	}
}
