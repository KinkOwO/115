package main

import (
	"testing"
	"time"

	"dfolan/internal/legion"
)

// 清关后的**服务端主动推进**：参考抓包 47.54 s2c N2895 → 47.56 c2s CMD2062
// → 47.57 下一关的整套进图帧。1 号的 N2895 与参考逐字节相同、前后帧序也一致，
// 客户端仍然不发 CMD2062（2026-10-08 三局实测），所以推进不能只依赖它。
//
// 契约：
//  1. 中间关清关 → 挂起一次推进，目标是「下一关」；
//  2. 客户端自己发 CMD2062 → 取消挂起（快路径，避免载入两次）；
//  3. 到期后由服务端发全套进图帧（丢掉 select_ack），并把 Stage 推到下一关。
func TestApocalypseServerDrivenStageAdvance(t *testing.T) {
	// 1) 攻坚房间清关：挂起推进，目标 = 第 1 关（下标 1）。
	nav := apocalypseStageSession(t, 0, false)
	nav.apocalypseStageCleared = [6]bool{}
	nav.apocalypse.Stage = 1
	nav.apocalypse.Cleared = 0
	if out := nav.apocalypseStageProjection(func(map[string]any) {}); len(out) != 1 {
		t.Fatalf("navigation projection %+v", out)
	}
	if nav.apocalypseAdvancePending == nil {
		t.Fatal("clearing the navigation room did not arm an advance")
	}
	if nav.apocalypseAdvancePending.stage != 1 {
		t.Fatalf("armed stage = %d, want 1", nav.apocalypseAdvancePending.stage)
	}

	// 2) 客户端发 CMD2062 → 取消。
	nav.cancelApocalypseAdvance()
	if pkts, _ := nav.apocalypseAdvanceDue(time.Now().Add(time.Second)); len(pkts) != 0 {
		t.Fatalf("a cancelled advance still fired: %+v", pkts)
	}

	// 3) 未取消时到期才发；未到期不发。
	mid := apocalypseStageSession(t, 1, false)
	mid.apocalypseStageCleared = [6]bool{}
	mid.apocalypse.Stage = 2
	mid.apocalypse.Cleared = 1
	if out := mid.apocalypseStageProjection(func(map[string]any) {}); len(out) != 1 {
		t.Fatalf("mid-stage projection %+v", out)
	}
	if mid.apocalypseAdvancePending == nil || mid.apocalypseAdvancePending.stage != 2 {
		t.Fatalf("armed stage = %+v, want 2", mid.apocalypseAdvancePending)
	}
	if pkts, _ := mid.apocalypseAdvanceDue(time.Now()); len(pkts) != 0 {
		t.Fatalf("advance fired before its deadline: %+v", pkts)
	}

	// 终点关不挂推进（走终局翻牌链）。
	last := len(legion.ApocalypseStageDungeons) - 1
	term := apocalypseStageSession(t, last, false)
	term.apocalypseStageCleared = [6]bool{}
	term.apocalypse.Stage = last + 1
	if out := term.apocalypseStageProjection(func(map[string]any) {}); out != nil {
		t.Fatalf("terminal projection %+v", out)
	}
	if term.apocalypseAdvancePending != nil {
		t.Fatal("the terminal stage must not arm a stage advance")
	}
}

// ★ A2（2026-10-09 实机对照结论）：这两条推进路径**不得**在房间里补发 N2895。
//
// 演进全过程（两次实机，务必别再翻回去）：
//  1. 第4/5轮：业主报「死亡后点进入没反应 / 回到的是前一关」。当时判定成因是
//     「客户端把 N2895 @13 原样回送成 CMD2045 @17，而这两条路径不发 N2895 ⇒
//     客户端手上的阶段是旧的」，于是在**进图帧序之后补发**权威 N2895；
//  2. 第5轮实测：续关确实回到正确关卡 ✓；
//  3. 但业主后续实机对照（同一客户端跑老 exe 20:42）发现地图内三个同源症状：
//     复活币数字 99（老版本 8）、难度2 死亡出现复活提示、药水被禁；
//  4. 会话 `20261009_023423` 帧序比对锁定：**老版本从不发这类「房间里补发的
//     N2895」，而它地图内一切正常**；这两条补发每进一间房就多给客户端一帧作战
//     信息，把客户端的当前作战状态冲掉 ⇒ 症状随房间切换出现。
//
// ⇒ 两处补发**已删除**。续关正确性改由服务端侧兜底：
//
//	`apocalypse_run.go` 的 resumeStale 分支（客户端报旧阶段也按**保存阶段**续关），
//	由 TestApocalypseResumeToleratesStaleClientStage 钉住。
//
// 本用例因此钉「**不补发**」：推进/直进后的帧序里不得出现 N2895。
func TestApocalypseAdvancePathsDoNotRepublishStage(t *testing.T) {
	// —— 服务端主动推进 ——
	nav := apocalypseStageSession(t, 0, false)
	nav.level = 200
	nav.dungeons = apocalypseTestDungeons(t)
	nav.apocalypseStageCleared = [6]bool{}
	if out := nav.apocalypseStageProjection(func(map[string]any) {}); len(out) != 1 {
		t.Fatalf("navigation projection %+v", out)
	}
	if nav.apocalypseAdvancePending == nil || nav.apocalypseAdvancePending.stage != 1 {
		t.Fatalf("armed advance = %+v, want stage 1", nav.apocalypseAdvancePending)
	}
	pkts, notes := nav.apocalypseAdvanceDue(time.Now().Add(apocalypseStageAdvanceDelay + time.Second))
	if len(pkts) == 0 {
		t.Fatalf("the advance did not fire (notes %+v)", notes)
	}
	if nav.apocalypse.Stage != 2 {
		t.Fatalf("run.Stage = %d after advancing into room index 1, want 2", nav.apocalypse.Stage)
	}
	for _, p := range pkts {
		if p.ID == legion.NotiLegionInfo {
			t.Fatalf("服务端推进补发了 N2895（%q）—— 老版本从不发这类帧，补发会把客户端的"+
				"作战状态冲掉（复活币数字 99 / 难度2 出现复活提示）", p.Name)
		}
	}

	// —— 客户端 CMD2062 直进 ——
	mid := apocalypseStageSession(t, 0, true)
	mid.level = 200
	mid.dungeons = apocalypseTestDungeons(t)
	res, err := mid.legion.ApocalypseDirectMove(mid, legion.ApocalypseStageDungeons[1])
	if err != nil {
		t.Fatalf("direct move into the first combat stage: %v", err)
	}
	if mid.apocalypse.Stage != 2 {
		t.Fatalf("run.Stage = %d after loading room index 1, want 2", mid.apocalypse.Stage)
	}
	for _, p := range res.Packets {
		if p.ID == legion.NotiLegionInfo {
			t.Fatalf("客户端直进补发了 N2895（%q）—— 同上，已删除", p.Name)
		}
	}
}

// sweep 是推进链的权威兜底：客户端**有时不发 CMD117**（2026-10-08 15:25 实机
// 第 1 关就是），于是清怪投影一次都没跑、N2895 没发、下一关也没排，
// 角色停在副本里。sweep 只看房间状态，因此与客户端发了什么无关。
func TestApocalypseAdvanceSweepCatchesMissedProjection(t *testing.T) {
	// 第 2 关已清空，但投影从未跑过（apocalypseStageCleared 全 false），
	// 也没有任何挂起推进 —— 正是实机那一刻的状态。
	w := apocalypseStageSession(t, 2, false)
	w.apocalypseStageCleared = [6]bool{}
	w.apocalypseAdvancePending = nil
	w.apocalypse.Stage = 3
	w.apocalypse.Cleared = 2

	notes := w.apocalypseAdvanceSweep(time.Now())
	if len(notes) != 1 || notes[0]["kind"] != "apocalypse_advance_swept" {
		t.Fatalf("sweep notes = %+v", notes)
	}
	if w.apocalypseAdvancePending == nil || w.apocalypseAdvancePending.stage != 3 {
		t.Fatalf("sweep armed %+v, want stage 3", w.apocalypseAdvancePending)
	}

	// 房间里还有活怪 → 不清关，不排。
	living := apocalypseStageSession(t, 2, true)
	living.apocalypseAdvancePending = nil
	if notes := living.apocalypseAdvanceSweep(time.Now()); len(notes) != 0 {
		t.Fatalf("sweep fired with a living monster: %+v", notes)
	}
	if living.apocalypseAdvancePending != nil {
		t.Fatal("sweep armed an advance with a living monster")
	}

	// 已有挂起推进 → 不重复排。
	armed := apocalypseStageSession(t, 2, false)
	armed.apocalypseAdvancePending = &apocalypseAdvance{at: time.Now().Add(time.Second), stage: 3}
	if notes := armed.apocalypseAdvanceSweep(time.Now()); len(notes) != 0 {
		t.Fatalf("sweep duplicated a pending advance: %+v", notes)
	}

	// 终点关不排（走终局翻牌链）。
	last := len(legion.ApocalypseStageDungeons) - 1
	term := apocalypseStageSession(t, last, false)
	term.apocalypseAdvancePending = nil
	if notes := term.apocalypseAdvanceSweep(time.Now()); len(notes) != 0 {
		t.Fatalf("sweep fired on the terminal stage: %+v", notes)
	}
}

// 重新排推进不能被上一关的残留挂起挡住（实机 BUG：第 1 关的挂起残留导致
// 第 2 关清关时排不上）。
func TestApocalypseArmAdvanceReplacesStalePending(t *testing.T) {
	w := apocalypseStageSession(t, 2, false)
	w.apocalypseAdvancePending = &apocalypseAdvance{at: time.Now().Add(time.Second), stage: 2}
	if !w.armApocalypseAdvance(3, apocalypseStageAdvanceDelay) {
		t.Fatal("a stale pending advance blocked re-arming for a different stage")
	}
	if w.apocalypseAdvancePending.stage != 3 {
		t.Fatalf("armed stage = %d, want 3", w.apocalypseAdvancePending.stage)
	}
	// 同一目标且还在计时 → 不重复排。
	again := apocalypseStageSession(t, 2, false)
	if !again.armApocalypseAdvance(3, time.Minute) {
		t.Fatal("first arm failed")
	}
	if again.armApocalypseAdvance(3, time.Minute) {
		t.Fatal("the same target was armed twice while still counting down")
	}
}

// 撤退续关的**阶段授权**（实机 2026-10-08 16:24 反例回归）。
//
// 现场：打通攻坚房间+第1关+第2关 → 在第3关点撤退 → 再点开始。
// 客户端发 CMD2045 @17=3（**正确**，= 下一个要请求的阶段），
// 而当时服务端按「已清关数」算出 0，把正确的请求拒了：
//
//	legion_refused id=2045 request_hex="…6b000000 03000000 0000"
//	reason="apocalypse resume stage 3 does not match the saved stage 0"
//
// 正确公式：ContinueStage = 下一个要请求的阶段 = 已清关数 + 1。
func TestApocalypseResumeStageMatchesClientRequest(t *testing.T) {
	// 刻度：run.Stage == NOTI2895 @13 == CMD2045 @17 == 「已进入过的房间数」。
	// 载入的副本表下标 = ContinueStage() - 1。
	for _, c := range []struct {
		name  string
		stage int
		want  int // ContinueStage()
		index int // 实际要重建的副本表下标
	}{
		{"只进过攻坚房", 1, 1, 0},
		{"在第 1 关", 2, 2, 1},
		{"在第 2 关", 3, 3, 2},
		// 第 5 关（终点）对应下标 5，也就是「已进入过 6 间」。
		{"在第 5 关", 6, 6, 5},
	} {
		run := legion.NewApocalypseRunState()
		run.Entered = true
		run.Stage = c.stage
		run.Suspended = true
		if got := run.ContinueStage(); got != c.want {
			t.Fatalf("%s: ContinueStage() = %d, want %d (the value the client sends in @17)", c.name, got, c.want)
		}
		if idx := run.ContinueStage() - 1; idx != c.index {
			t.Fatalf("%s: dungeon index = %d, want %d", c.name, idx, c.index)
		}
	}
	// 未进图时 @17 必须是 0，且不能当作合法的续关值（会载入攻坚房下标 0 之外）。
	fresh := legion.NewApocalypseRunState()
	fresh.Entered = true
	if got := fresh.ContinueStage(); got != 0 {
		t.Fatalf("ContinueStage before any room = %d, want 0", got)
	}
}
