package main

import (
	"net"
	"testing"
	"time"

	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
)

// ★ 回归护栏（2026-10-08 实机）：角色在副本内**死亡** → 10 秒倒计时判负 →
// 被请离回城。此后玩家点右上角面板的「进入」，客户端会带**保存阶段**发 CMD2045
// （实测 plain `…006b000000 02000000…`，即 stage=2），服务端必须把它当**续关**
// 受理。
//
// 这条死亡链在 `player_death.go` 的 `deathFailLeave`：它原先只为维纳斯清了阶段
// 时钟，**没有给末世录置 `Suspended`** ⇒ `apocalypseEnter` 落进「首次入场」分支，
// 把请求拒成 `apocalypse first entry carries stage 2, want 0`，客户端拿不到结果码
// ⇒ 症状正是业主报的「点右上角 UI 的进入没有任何反应」。
//
// 注意这条与「阶段时限超时」（`apocalypseStageTimeout`）是**两条不同的路径**：
// 后者本来就置了 Suspended（见 TestApocalypseStageTimeoutKeepsProgress），
// 所以只测超时那条会把本 BUG 漏过去。
func TestApocalypseDeathLeaveSuspendsRunForResume(t *testing.T) {
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := peer.Read(buf); err != nil {
				return
			}
		}
	}()

	events := make(chan map[string]any, 64)
	logf := func(e map[string]any) { events <- e }
	keys := make([]byte, wire.SessionKeyBytes)

	// 死在第 2 关（下标 1 ⇒ run.Stage = 2）。
	w := apocalypseStageSession(t, 1, false)
	w.role = database.Character{ID: 7, WireID: 7}
	w.pilotDeath = &odysseyDeath{Run: "dungeon-run", Sequence: 1, Dead: true}
	c := &gameConnection{
		gatewayRuntime: &gatewayRuntime{},
		worldState:     w,
		bootstrapped:   true,
		event:          logf,
		output:         newConnectionOutput(server, keys, "test", logf),
	}
	if w.apocalypse.Suspended {
		t.Fatal("run was already suspended before the death leave")
	}
	savedStage, savedChoice := w.apocalypse.Stage, w.apocalypse.Choice

	c.deathFailLeave(0)

	if w.activeDungeon != nil {
		t.Fatalf("active dungeon was not cleared: %+v", w.activeDungeon)
	}
	if !w.apocalypse.Suspended {
		t.Fatal("death leave did not suspend the apocalypse run — the client's resume CMD2045 " +
			"would be refused as `first entry carries stage N, want 0` and pressing 进入 does nothing")
	}
	// 进度、难度、RunID 必须原样保留（续关就靠它们重建当前未完成 Boss）。
	if w.apocalypse.Stage != savedStage || w.apocalypse.Choice != savedChoice {
		t.Fatalf("progress lost: stage %d→%d choice %#x→%#x",
			savedStage, w.apocalypse.Stage, savedChoice, w.apocalypse.Choice)
	}
	// 续关刻度：客户端发的 @17（= ContinueStage()）必须非 0，否则仍会被当首次入场。
	if want := w.apocalypse.ContinueStage(); want == 0 {
		t.Fatal("ContinueStage() = 0 after a death on stage 2 — the client would still be treated as a first entry")
	}
}

// 阶段倒计时（NOTI1474）契约，与维纳斯同款。规格见
// D:\115US-001\moshilu\10-军团末世录专有\1474-DUNGEONTIMEOUTTIME.md：
//
//   - 正文 8B = [阶段时限秒 @0, 阶段开始 Unix 秒 @4]，频道 119 走**秒**分支；
//   - 「真实共享加载和资源释放完成后，紧随本人的 N30 发送。后续同阶段换房使用
//     原开始时间」——重复调用必须复用冻结的开始时刻，否则每次发包都会把倒计时
//     续满，超时判死永远不成立；
//   - 攻坚房间（下标 0）不设限时，不发。
func TestApocalypseStageTimerFreezesStart(t *testing.T) {
	w := apocalypseStageSession(t, 1, true)
	// 给一条来源时限：plan.PhaseSeconds 是 [phase info] 的实值（秒）。
	w.legion.plan = &legion.RunPlan{PhaseSeconds: []float64{90, 300, 300, 300, 600, 600}}

	start := time.Unix(1_700_000_000, 0)
	first := w.apocalypseStageTimer(start)
	if len(first) != 1 || first[0].ID != legion.NotiDungeonTimeoutTime || first[0].Kind != 0 {
		t.Fatalf("first timer packet = %+v", first)
	}
	if got := w.apocalypse.StageClock[1]; !got.Equal(start) {
		t.Fatalf("stage clock = %v, want %v", got, start)
	}
	// 123 秒后再问一次：开始时刻必须是**原来那个**（规格「不能以当前发送时间覆盖
	// 阶段最初开始时间」），正文逐字节相同。
	later := start.Add(123 * time.Second)
	again := w.apocalypseStageTimer(later)
	if len(again) != 1 {
		t.Fatalf("second timer packet = %+v", again)
	}
	if string(again[0].Payload) != string(first[0].Payload) {
		t.Fatalf("timer payload changed after 123s:\n first=%x\nagain=%x", first[0].Payload, again[0].Payload)
	}

	// 攻坚房间（下标 0）不发。
	nav := apocalypseStageSession(t, 0, true)
	nav.legion.plan = &legion.RunPlan{PhaseSeconds: []float64{90, 300, 300, 300, 600, 600}}
	if out := nav.apocalypseStageTimer(start); out != nil {
		t.Fatalf("the navigation room must not carry a countdown: %+v", out)
	}
}

// 阶段倒计时到期 = 挑战失败：走 N33 开路 + 回城 + 保留原阶段。
//
// 业主口径（2026-10-08）：「副本内的倒计时，倒计时结束自动退出副本，**再次进入
// 就是刚刚退出的那一关**」。所以 Stage / Cleared / Choice 一律不动，只置 Suspended。
func TestApocalypseStageTimeoutKeepsProgress(t *testing.T) {
	w := apocalypseStageSession(t, 2, true)
	w.legion.plan = &legion.RunPlan{PhaseSeconds: []float64{90, 300, 300, 300, 600, 600}}
	w.apocalypse.Choice = 1
	w.apocalypse.Stage = 3   // 在第 2 关（下标 2）
	w.apocalypse.Cleared = 2 // 已清 2 关

	started := time.Unix(1_700_000_000, 0)
	w.apocalypse.StageClock[2] = started

	// 用**有效时限**推进，不写死秒数（诊断覆盖 apocalypsePhaseLimitOverride
	// 会改变它；见 apocalypse_timer.go）。
	limit, ok := w.apocalypsePhaseLimit(2)
	if !ok || limit == 0 {
		t.Fatal("stage 2 has no countdown limit")
	}
	// 未到期：什么都不发。
	if out := w.apocalypseStageTimeout(started.Add(time.Duration(limit-1)*time.Second), nil); out != nil {
		t.Fatalf("timeout fired before its limit: %+v", out)
	}
	// 到期：发 N33 开路，并保留进度。
	var notes []map[string]any
	out := w.apocalypseStageTimeout(started.Add(time.Duration(limit)*time.Second), func(n map[string]any) { notes = append(notes, n) })
	if len(out) == 0 {
		t.Fatal("timeout did not emit anything at its limit")
	}
	if out[0].ID != 33 || out[0].Kind != 0 {
		t.Fatalf("timeout must lead with N33 (FAIL_CLEAR_DUNGEON): %+v", out[0])
	}
	if !hasPacket(out, legion.NotiLegionInfo) {
		t.Fatalf("timeout must end with the waiting-state N2895: %+v", out)
	}
	if len(notes) != 1 || notes[0]["kind"] != "apocalypse_stage_timeout" {
		t.Fatalf("timeout notes = %+v", notes)
	}
	// ★ 进度必须保留：再次进入就是刚刚退出的那一关。
	if w.apocalypse.Choice != 1 || w.apocalypse.Stage != 3 || w.apocalypse.Cleared != 2 {
		t.Fatalf("progress was lost on timeout: %+v", w.apocalypse)
	}
	if !w.apocalypse.Suspended {
		t.Fatal("the run was not suspended after a timeout")
	}
	if got := w.apocalypse.ContinueStage(); got != 3 {
		t.Fatalf("ContinueStage after timeout = %d, want 3 (the room just left)", got)
	}
	if notes[0]["resume_stage"] != 3 {
		t.Fatalf("resume_stage = %v, want 3", notes[0]["resume_stage"])
	}
}

// 结算期不拽人：翻牌链已经挂起或副本已完成时，超时判定必须让路。
func TestApocalypseStageTimeoutYieldsToSettlement(t *testing.T) {
	w := apocalypseStageSession(t, 2, true)
	w.legion.plan = &legion.RunPlan{PhaseSeconds: []float64{90, 300, 300, 300, 600, 600}}
	w.apocalypse.StageClock[2] = time.Unix(1_700_000_000, 0)
	due := time.Unix(1_700_000_000, 0).Add(400 * time.Second)

	w.completionSent = true
	if out := w.apocalypseStageTimeout(due, nil); out != nil {
		t.Fatalf("timeout fired during settlement: %+v", out)
	}
	w.completionSent = false
	w.resultSent = true
	if out := w.apocalypseStageTimeout(due, nil); out != nil {
		t.Fatalf("timeout fired after the result was sent: %+v", out)
	}
}

// 难度框强制关闭（业主 2026-10-08：「识别到难度选择框，15 秒后没有选择难度
// 进入副本，就强制关闭」）。
//
// 关窗帧用的是规范明写的「操作记录 byte@5 非 0 走关闭/取消分支」，也就是正文 @8=1
// —— 上一轮我误把 deadline 压到这一位上，客户端直接走取消分支、点 Open 毫无反应，
// 反证了这一位就是开关。
func TestApocalypseSelectWindowForceClose(t *testing.T) {
	now := time.Now()
	newWindow := func() *worldSession {
		w := apocalypseStageSession(t, 1, true)
		w.apocalypse.Choice = 0xff
		w.apocalypse.Entered = false
		return w
	}

	// 开窗 → 挂计时；未到期不发。
	w := newWindow()
	w.armApocalypseSelectWindow(now)
	if pkts, notes := w.apocalypseSelectWindowDue(now.Add(time.Second)); len(pkts) != 0 || len(notes) != 0 {
		t.Fatalf("the window closed before its deadline: %+v %+v", pkts, notes)
	}
	// 到期 → 发关闭帧 + 复位 N2895。
	pkts, notes := w.apocalypseSelectWindowDue(now.Add(apocalypseSelectWindowSeconds * time.Second))
	if len(notes) != 1 || notes[0]["kind"] != "apocalypse_select_window_forced_closed" {
		t.Fatalf("notes = %+v", notes)
	}
	if len(pkts) != 2 {
		t.Fatalf("close plan = %+v", pkts)
	}
	if pkts[0].ID != legion.CmdOperationSelect || pkts[0].Kind != 1 {
		t.Fatalf("first packet must be the CMD2354 close ack: %+v", pkts[0])
	}
	// 关闭位 = 正文 @8 = 1（操作记录 byte@5）。
	if len(pkts[0].Payload) < 9 || pkts[0].Payload[8] != 1 {
		t.Fatalf("close byte @8 = %v, want 1", pkts[0].Payload)
	}
	if pkts[1].ID != legion.NotiLegionInfo {
		t.Fatalf("second packet must reset the panel state: %+v", pkts[1])
	}
	// 只关一次。
	if pkts, _ := w.apocalypseSelectWindowDue(now.Add(time.Minute)); len(pkts) != 0 {
		t.Fatalf("the window was closed twice: %+v", pkts)
	}

	// 确认难度（action2）后不再关窗。
	confirmed := newWindow()
	confirmed.armApocalypseSelectWindow(now)
	confirmed.cancelApocalypseSelectWindow()
	if pkts, notes := confirmed.apocalypseSelectWindowDue(now.Add(time.Minute)); len(pkts) != 0 || len(notes) != 0 {
		t.Fatalf("a confirmed difficulty was force-closed: %+v %+v", pkts, notes)
	}

	// 已经进图（Entered）时也不关窗。
	entered := newWindow()
	entered.apocalypse.Entered = true
	entered.armApocalypseSelectWindow(now)
	if pkts, notes := entered.apocalypseSelectWindowDue(now.Add(time.Minute)); len(pkts) != 0 || len(notes) != 0 {
		t.Fatalf("an entered run was force-closed: %+v %+v", pkts, notes)
	}
}
