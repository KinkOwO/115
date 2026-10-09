package main

import (
	"testing"

	"dfolan/internal/legion"
)

// 参考序列回归（2026-10-08 实测 BUG：选完难度进不去副本）。
//
// 参考抓包 D:\zhuabao\captures\20261008-105227 的 NOTI2895 逐帧解出来是：
//
//	CMD2045 之后            stage=0 follow=FFFFFFFF marks=[00 01 ff ff ff ff]
//	CMD2062 载入攻坚房间后   stage=1 follow=FFFFFFFF marks=[00 01 ff ff ff ff]
//	CMD2062 载入第 1 关后    stage=1 follow=FFFFFFFF marks=[00 01 ff ff ff ff]
//	第 1 关击杀确认          stage=2 follow=FFFFFFFF marks=[00 01 02 ff ff ff]
//	CMD2062 载入第 2 关后    stage=2 follow=FFFFFFFF marks=[00 01 02 ff ff ff]
//
// 出问题的那一版把 @9 在进图后写成了 0（而不是 FFFFFFFF 哨兵），客户端就
// **不发 CMD2062**，症状正是「点难度后还在城镇」。这条测试把整条序列钉住。
func TestApocalypseInfoFollowsReferenceSequence(t *testing.T) {
	s := apocalypseSession(t)
	w := apocalypseTownSession(t, 7)
	w.legion = s
	s.world = w

	// CMD2043：等待态。
	if _, err := s.handle(w, startPayload(107), legion.CmdStart); err != nil {
		t.Fatalf("CMD2043: %v", err)
	}
	run := s.apocalypseRun()
	assertInfo(t, "after 2043", s, 0xff, 2, 0, [6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})

	// CMD2354 action2 确认难度 1。
	if _, err := s.handle(w, operationPayload(2, 0, legion.OperationChannelCode), legion.CmdOperationSelect); err != nil {
		t.Fatalf("CMD2354: %v", err)
	}
	assertInfo(t, "after 2354", s, 0x00, 2, 0, [6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})

	// CMD2045 确认作战：stage 仍是 0，但 marks 已经是 [00 01]。
	if _, err := s.handle(w, enterPayload(legion.OperationChannelCode, 0), legion.CmdEnterDungeon); err != nil {
		t.Fatalf("CMD2045: %v", err)
	}
	assertInfo(t, "after 2045", s, 0x00, 2, 0, [6]byte{0x00, 0x01, 0xff, 0xff, 0xff, 0xff})
	if run.Entered != true {
		t.Fatal("run not entered after CMD2045")
	}

	// CMD2062 载入攻坚房间：stage=1，marks 不变。
	run.Stage = 1
	assertInfo(t, "after 2062 nav", s, 0x00, 2, 1, [6]byte{0x00, 0x01, 0xff, 0xff, 0xff, 0xff})

	// CMD2062 载入第 1 关：stage 不变（客户端只是进关），marks 不变。
	assertInfo(t, "after 2062 stage1", s, 0x00, 2, 1, [6]byte{0x00, 0x01, 0xff, 0xff, 0xff, 0xff})

	// 第 1 关清完：stage=2、marks 三格。
	stageSession := apocalypseStageSession(t, 1, false)
	stageSession.legion = s
	stageSession.apocalypse = run
	s.world = stageSession
	if out := stageSession.apocalypseStageProjection(func(map[string]any) {}); len(out) != 1 {
		t.Fatalf("projection after stage1 clear = %+v", out)
	}
	assertInfo(t, "after stage1 clear", s, 0x00, 2, 2, [6]byte{0x00, 0x01, 0x02, 0xff, 0xff, 0xff})

	// 第 2 关清完：stage=3、marks 四格（参考抓包 82.83s 那一帧）。
	stageSession2 := apocalypseStageSession(t, 2, false)
	stageSession2.legion = s
	stageSession2.apocalypse = run
	s.world = stageSession2
	if out := stageSession2.apocalypseStageProjection(func(map[string]any) {}); len(out) != 1 {
		t.Fatalf("projection after stage2 clear = %+v", out)
	}
	assertInfo(t, "after stage2 clear", s, 0x00, 2, 3, [6]byte{0x00, 0x01, 0x02, 0x03, 0xff, 0xff})
}

// 清关态的 N2895（终局翻牌前那一帧）必须与 2 号实录同形。
//
// 契约按 2 号实录校正（2026-10-08，roles_persist_..._20261008_160138_795939_next37
// 的 apocalypse_source_clear_outcome 帧，本实现已逐字节对齐到 diff=0）：
//
//	@9..12  Following 随阶段递增（**不是哨兵 FFFFFFFF**）
//	@13..16 Stage = **刚清掉的那一间**（= run.Stage - 1），不是「下一间」
//	@29+12i 六个到达标记 = ApocalypseMarksFor(run.MarkCount())
//
// 此前这条测试断言的是 @13 = run.Stage 且 @9 = FFFFFFFF —— 那是本实现自己的
// 旧契约，与 2 号 不符。
func TestApocalypseInfoStageClearKeepsFollowSentinel(t *testing.T) {
	s := apocalypseSession(t)
	w := apocalypseTownSession(t, 7)
	if _, err := s.handle(w, startPayload(107), legion.CmdStart); err != nil {
		t.Fatalf("CMD2043: %v", err)
	}
	run := s.apocalypseRun()
	run.Choice = 0
	run.Entered = true
	// 第 4 关刚清完：Stage 已推到 5（= 已进入过的房间数）、Cleared=4。
	run.Stage = 5
	run.Cleared = 4
	run.PhaseCleared = 5
	run.RoleSet = true
	p := apocalypseStageClearInfo(run)
	// 载荷 @9 是 **Outcome**（不是 Following）：清关帧的权威实测是路线终点 5。
	// Following 在载荷 @17，且整场恒为 ffffffff（2 号三次采样一致）。
	if got := u32(t, p, 9); got != 4 {
		t.Fatalf("clear-state outcome @9 = %d, want 4 (the route endpoint cleared so far)", got)
	}
	if got := u32(t, p, 17); got != 0xffffffff {
		t.Fatalf("clear-state following @17 = %#x, want ffffffff", got)
	}
	// Stage 报告刚清掉的那一间。
	if got := u32(t, p, 13); got != 4 {
		t.Fatalf("clear-state stage @13 = %d, want 4 (run.Stage-1, the room just cleared)", got)
	}
	// 清关那一刻 marks 覆盖到刚清掉的那关：MarkCount = Cleared+2 = 6，全满。
	for i, want := range legion.ApocalypseMarksFor(run.MarkCount()) {
		if got := p[29+12*i]; got != want {
			t.Fatalf("clear-state mark %d = %#x, want %#x", i, got, want)
		}
	}
	// 目标标记：2 号恒为 [1,0,0,0,0,0]。
	if got := u32(t, p, 128); got != 1 {
		t.Fatalf("clear-state target mark 0 @128 = %d, want 1", got)
	}
	for i := 1; i < 6; i++ {
		if got := u32(t, p, 128+4*i); got != 0 {
			t.Fatalf("clear-state target mark %d @%d = %d, want 0", i, 128+4*i, got)
		}
	}
}

func assertInfo(t *testing.T, label string, s *legionSession, choice byte, state uint32, stage uint32, marks [6]byte) {
	t.Helper()
	p := s.apocalypseInfo()
	if len(p) != legion.LegionInfoSize {
		t.Fatalf("%s: N2895 length %d, want %d", label, len(p), legion.LegionInfoSize)
	}
	if p[4] != choice {
		t.Errorf("%s: choice @4 = %#x, want %#x", label, p[4], choice)
	}
	if got := u32(t, p, 5); got != state {
		t.Errorf("%s: state @5 = %d, want %d", label, got, state)
	}
	if got := u32(t, p, 17); got != 0xffffffff {
		t.Errorf("%s: follow @17 = %#x, want FFFFFFFF (sentinel)", label, got)
	}
	if got := u32(t, p, 13); got != stage {
		t.Errorf("%s: stage @13 = %d, want %d", label, got, stage)
	}
	for i := 0; i < 6; i++ {
		if got := p[29+12*i]; got != marks[i] {
			t.Errorf("%s: mark[%d] = %#x, want %#x", label, i, got, marks[i])
		}
	}
}

func u32(t *testing.T, p []byte, off int) uint32 {
	t.Helper()
	if off+4 > len(p) {
		t.Fatalf("offset %d out of range (len %d)", off, len(p))
	}
	return uint32(p[off]) | uint32(p[off+1])<<8 | uint32(p[off+2])<<16 | uint32(p[off+3])<<24
}
