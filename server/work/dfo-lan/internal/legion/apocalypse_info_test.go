package legion

import (
	"encoding/binary"
	"testing"
)

// The encoder must reproduce the byte ranges the 2026-10-08 capture
// (D:\zhuabao\captures\20261008-105227, difficulty 1 and 2 full clears) shows
// changing across a run. Every vector below is read straight off those frames.
//
// Payload layout: u16 content at [0:2], then the state body, so body offset b
// sits at payload offset b+2. The body fields the client reads are
//
//	@2  u8  operation choice (0xff = none, else the CTP key)
//	@3  u32 state (2 in the run, 0 after CMD2046)
//	@7  u32 outcome (0 in the run)
//	@11 u32 stage (0 in town, 1 in the waiting room, +1 per cleared phase)
//	@15 u32 follow (FFFFFFFF in town, the entered stage right after a load,
//	               0 on that stage's kill confirmation)
//	@27+12i u8 stage-reach mark (0xff until reached, else the stage number)
//	@107 u8  assigned-role counter
//
// The capture's own payload is 208 bytes while this repository's
// LegionInfoBody is 204; the two extra bytes sit in the trailing padding and
// every field above reads at the same offset on both sides, so the encoder
// keeps the repository's body length.
//
// A regression here does not crash the client: it silently stops rendering the
// run's stage/role panel, which is exactly why the offsets need a gate.
func TestApocalypseInfoMatchesCaptureVectors(t *testing.T) {
	unset := [6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	afterNav := [6]byte{0x00, 0x01, 0xff, 0xff, 0xff, 0xff}
	afterTwo := [6]byte{0x00, 0x01, 0x02, 0xff, 0xff, 0xff}
	afterFull := [6]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0xff}
	sentinel := ^uint32(0)

	waiting := ApocalypseWaitingInfo()

	confirmed := ApocalypseWaitingInfo()
	confirmed.OperationChoice = 0

	// CMD2045 之后：Stage 推到 1、marks 到 [00 01]，@9 仍是哨兵 FFFFFFFF。
	entered := confirmed
	entered.Stage = 1
	entered.StageMarks = afterNav

	roled := entered
	roled.RoleCount = 1

	// 第 3 关清完（stage=3、marks 三格）。
	twoCleared := roled
	twoCleared.Stage = 3
	twoCleared.StageMarks = afterTwo

	full := roled
	full.OperationChoice = 1
	full.Stage = 5
	full.StageMarks = afterFull

	ended := full
	ended.State = 0
	ended.RoleCount = 0

	cases := []struct {
		name      string
		state     ApocalypseInfoState
		choice    byte
		stateVal  uint32
		following uint32
		stage     uint32
		marks     [6]byte
		roleCount byte
	}{
		{"waiting", waiting, 0xff, 2, sentinel, 0, unset, 0},
		{"difficulty-confirmed", confirmed, 0x00, 2, sentinel, 0, unset, 0},
		{"navigation-loaded", entered, 0x00, 2, sentinel, 1, afterNav, 0},
		{"role-set", roled, 0x00, 2, sentinel, 1, afterNav, 1},
		{"two-phases-cleared", twoCleared, 0x00, 2, sentinel, 3, afterTwo, 1},
		{"full-clear", full, 0x01, 2, sentinel, 5, afterFull, 1},
		{"reward-ended", ended, 0x01, 0, sentinel, 5, afterFull, 0},
	}
	for _, c := range cases {
		p := ApocalypseInfo(c.state)
		if len(p) != 2+LegionInfoBody {
			t.Fatalf("%s: length %d, want %d", c.name, len(p), 2+LegionInfoBody)
		}
		if got := binary.LittleEndian.Uint16(p[0:]); got != ApocalypseContent {
			t.Fatalf("%s: content %d, want %d", c.name, got, ApocalypseContent)
		}
		// Offsets below are payload offsets (the u16 content prefix included),
		// exactly as the capture is read: p[4] is the choice byte, p[5:] the
		// state dword, and so on.
		if p[4] != c.choice {
			t.Errorf("%s: choice @4 = %#x, want %#x", c.name, p[4], c.choice)
		}
		if got := binary.LittleEndian.Uint32(p[5:]); got != c.stateVal {
			t.Errorf("%s: state @5 = %d, want %d", c.name, got, c.stateVal)
		}
		if got := binary.LittleEndian.Uint32(p[9:]); got != 0 {
			t.Errorf("%s: outcome @9 = %d, want 0", c.name, got)
		}
		if got := binary.LittleEndian.Uint32(p[13:]); got != c.stage {
			t.Errorf("%s: stage @13 = %d, want %d", c.name, got, c.stage)
		}
		if got := binary.LittleEndian.Uint32(p[17:]); got != c.following {
			t.Errorf("%s: following @17 = %d, want %d", c.name, got, c.following)
		}
		var marks [6]byte
		for i := range marks {
			marks[i] = p[29+12*i]
		}
		if marks != c.marks {
			t.Errorf("%s: marks = % x, want % x", c.name, marks, c.marks)
		}
		if p[109] != c.roleCount {
			t.Errorf("%s: role count @109 = %d, want %d", c.name, p[109], c.roleCount)
		}
		// The six 12-byte destination slots keep the constructor's unset shape
		// (u64 @+4 = FF..FF) so the client's own reader can still walk them;
		// only byte0 is rewritten, and only for a reached stage.
		for i := 0; i < 6; i++ {
			off := 29 + 12*i
			if binary.LittleEndian.Uint64(p[off+4:]) != ^uint64(0) {
				t.Errorf("%s: slot %d destination tail was overwritten", c.name, i)
			}
		}
	}
}

// marks 的布局是「前导一格」：slot0 = 00 当 highestStage>=1，
// slot i（i>=1）= i 当 highestStage > i，其余 0xff。
// 参考抓包锚点见 ApocalypseMarksFor 的注释。
func TestApocalypseMarksFollowStage(t *testing.T) {
	for highest := 0; highest <= 6; highest++ {
		marks := ApocalypseMarksFor(highest)
		// ApocalypseMarksFor 的入参就是「填几格」：slot i 在 i < filled 时写 i。
		filled := highest
		if filled > 6 {
			filled = 6
		}
		want := [6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
		for i := 0; i < filled; i++ {
			want[i] = byte(i)
		}
		if marks != want {
			t.Fatalf("stage %d marks = % x, want % x", highest, marks, want)
		}
	}
	// 入参 = 填几格；锚点全部来自抓包逐帧解出。
	for _, c := range []struct {
		filled int
		want   [6]byte
	}{
		{0, [6]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{1, [6]byte{0x00, 0xff, 0xff, 0xff, 0xff, 0xff}},
		{2, [6]byte{0x00, 0x01, 0xff, 0xff, 0xff, 0xff}},
		{3, [6]byte{0x00, 0x01, 0x02, 0xff, 0xff, 0xff}},
		{5, [6]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0xff}},
		{6, [6]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05}},
	} {
		if got := ApocalypseMarksFor(c.filled); got != c.want {
			t.Fatalf("filled %d marks = % x, want % x", c.filled, got, c.want)
		}
	}
}

// ApocalypseStageOfDungeon is what the CMD2062 handler uses to decide which
// phase the client is asking for, so the table order is contract.
func TestApocalypseStageDungeonTable(t *testing.T) {
	if ApocalypseStageDungeons[0] != ApocalypseNavigationDungeon {
		t.Fatalf("navigation dungeon %d is not stage0", ApocalypseNavigationDungeon)
	}
	for i, id := range ApocalypseStageDungeons {
		if got := ApocalypseStageOfDungeon(id); got != i {
			t.Fatalf("stage of %d = %d, want %d", id, got, i)
		}
	}
	if !IsApocalypseStageDungeon(ApocalypseNavigationDungeon) {
		t.Fatal("navigation dungeon not reported as a stage dungeon")
	}
	if ApocalypseStageOfDungeon(100004131) != -1 {
		t.Fatal("a foreign dungeon was accepted as an apocalypse stage")
	}
	if got := len(ApocalypseCombatStageDungeons()); got != 5 {
		t.Fatalf("combat stages = %d, want 5", got)
	}
}

// A fresh run must report "no difficulty chosen", because the zero value would
// mean difficulty 1 to the client.
func TestNewApocalypseRunStateHasNoChoice(t *testing.T) {
	run := NewApocalypseRunState()
	if run.Choice != 0xff {
		t.Fatalf("fresh choice = %#x, want 0xff", run.Choice)
	}
	run.Choice = 0
	run.Entered = true
	run.PhaseCleared = 4
	run.RoleSet = true
	run.Reset()
	if run.Choice != 0xff || run.Entered || run.PhaseCleared != 0 || run.RoleSet {
		t.Fatalf("reset left state behind: %+v", run)
	}
}

// CMD2354 成功响应的字节布局是**契约**，不能顺手塞字段。
//
// 实机事故（2026-10-08 17:32）：为了做「难度框 15 秒倒计时」，把 absolute
// deadline 写到正文 @7..10、close 写到 @11 —— 规格
// D:\115US-001\moshilu\10-军团末世录专有\2354-LEGIONOPERATION成功响应.md 写明
// 操作记录内部是「u32@0 = Action、**byte@5 非 0 走关闭/取消分支**、
// u32@6 = 子面板+1000 的倒计时截止值」，也就是正文 @8 是那个关闭位。
// 我的 @7..10 正好压在它上面（@8 变成 deadline 的高字节，非 0），客户端于是
// 走取消分支 —— 点右上角 Open 完全没反应，并连发 9 次 action1 重试。
//
// ★ 回归护栏：倒计时截止值（正文 @9..12）与关闭位（正文 @8）必须互不干扰。
//
// 2026-10-08 实机事故：把截止值写到正文 @7..10、关闭位写到 @11，**整体提前 2B**，
// 压掉了真正的关闭位 @8 ⇒ 客户端走关闭/取消分支，点 Open 完全没反应
// （客户端连发 9 次 action1 重试）。当时的错误结论是「客户端不认这个字段」，
// 实际是偏移写错。本测试就是把这两个偏移钉死，防止再犯。
func TestApocalypseOperationDeadlineDoesNotClobberCloseBit(t *testing.T) {
	const deadline = 0x11223344
	open := ApocalypseOperationAck(1, deadline)
	if len(open) != OperationAckSize {
		t.Fatalf("open ack length %d, want %d", len(open), OperationAckSize)
	}
	// @8 = 操作记录 byte@5 = 关闭/取消位。开窗时必须为 0。
	if open[8] != 0 {
		t.Fatalf("open ack close bit @8 = %#x, want 0", open[8])
	}
	// @7 是操作记录 byte@4，当前实现留 0；写偏移时最容易误伤就是这一格与 @8。
	if open[7] != 0 {
		t.Fatalf("open ack @7 = %#x, want 0", open[7])
	}
	// @9..12 = 操作记录 byte@6..9 = 截止值。
	if got := binary.LittleEndian.Uint32(open[9:]); got != deadline {
		t.Fatalf("open ack deadline @9..12 = %#x, want %#x", got, deadline)
	}
	// 关闭响应的关闭位必须是 1，**Action 必须与开窗同为 1**，且不携带倒计时。
	//
	// ★ 2026-10-08 第二次实机事故：关闭位写对了、Action 却留 0
	// （`016b000000000000010000000000000000`），服务端 15 秒确实发了这一帧，
	// 客户端完全不理 ⇒「倒计时走到 0 之后什么都不发生」。原生 reader 的关闭
	// 分支判据是「Action1 + close=1」（维纳斯可用先例同款），所以这里把
	// Action 一并钉死。
	closed := ApocalypseOperationClose()
	if got := binary.LittleEndian.Uint32(closed[3:]); got != 1 {
		t.Fatalf("close ack action @3..6 = %d, want 1 (native reader branches to close only on Action1+close=1)", got)
	}
	if closed[8] != 1 {
		t.Fatalf("close ack close bit @8 = %#x, want 1", closed[8])
	}
	if got := binary.LittleEndian.Uint32(closed[9:]); got != 0 {
		t.Fatalf("close ack deadline @9..12 = %#x, want 0", got)
	}
	// 两者只应在关闭位（@8）与 action 之外的附加字段上不同：内容号相同。
	if got := binary.LittleEndian.Uint16(open[1:]); got != binary.LittleEndian.Uint16(closed[1:]) {
		t.Fatalf("content differs between open/close ack: %d vs %d", got, binary.LittleEndian.Uint16(closed[1:]))
	}
	for i := 7; i < len(open); i++ {
		if i == 8 || (i >= 9 && i <= 12) {
			continue
		}
		if open[i] != 0 {
			t.Fatalf("open ack byte %d = %#x, want 0", i, open[i])
		}
	}
}

// CMD2354 成功响应的公共前缀与 action 回显。
//
// ⚠️ 本测试**不再**断言「附加字段必须为 0」。参考抓包 20261008-105227 的 action1
// 正文除内容号与 action 外全 0，但**那一帧本身就是带 BUG 的参考**：2 号的难度框
// 同样不会自动关闭（业主确认），成因正是它的倒计时截止值为 0。
// 旧版本这条「必须全 0」的断言把 BUG 钉成了契约，已按规格改写：
// 截止值必须下发，见 TestApocalypseOperationDeadlineDoesNotClobberCloseBit。
func TestApocalypseOperationAckMatchesReferenceLayout(t *testing.T) {
	for _, action := range []uint32{1, 2} {
		p := ApocalypseOperationAck(action, 0)
		if len(p) != OperationAckSize {
			t.Fatalf("action %d: length %d, want %d", action, len(p), OperationAckSize)
		}
		if p[0] != 1 {
			t.Fatalf("action %d: common success flag = %d, want 1", action, p[0])
		}
		if got := binary.LittleEndian.Uint16(p[1:]); got != ApocalypseContent {
			t.Fatalf("action %d: content = %d, want %d", action, got, ApocalypseContent)
		}
		if got := binary.LittleEndian.Uint32(p[3:]); got != action {
			t.Fatalf("action %d: recorded action = %d", action, got)
		}
		// 关闭位在成功响应里必须为 0（非 0 会把开窗打成取消）。
		if p[8] != 0 {
			t.Fatalf("action %d: close bit @8 = %#x, want 0", action, p[8])
		}
	}
}

// 通关关闭态必须与 2 号权威抓包 idx=654 同形（CMD2046 之后那一帧）。
//
// 业主 2026-10-08 报「翻牌结束后右上角 UI 还在、退出也有 UI 残留」——
// 成因是本实现通关后发的是**等待态 State2**，客户端因此保留军团面板。
// 规格 G0452 说明「**State0 才是收起军团窗口的那一帧**」。
//
// 权威抓包 D:\zhuabao\captures\20261008-184805 idx=654：
//
//	@4..8   = 01 00 00 00   State=0（收起），choice 保留 01
//	@9..12  = 05 00 00 00   Following=5
//	@13..16 = 05 00 00 00   Stage=5（本次打到第 5 关）
//	@29+12i = [0,1,2,3,4,5] 阶段记录保留
//	@128+   = [1,0,0,0,0,0] 目标标记保留
func TestApocalypseCompletedInfoMatchesCapture(t *testing.T) {
	p := ApocalypseInfo(ApocalypseCompletedInfo(1, 5))
	if len(p) != LegionInfoSize {
		t.Fatalf("length %d, want %d", len(p), LegionInfoSize)
	}
	// ★ 字节位置（用权威抓包逐字节校准）：choice 在 **@4**、State 在 **@5**、
	// Stage 在 **@13**。本文件最初把 State 断到 @4，测试立刻抓到偏差。
	if got := p[5]; got != 0 {
		t.Fatalf("State @5 = %d, want 0 (State0 is what collapses the legion window)", got)
	}
	if got := p[4]; got != 1 {
		t.Fatalf("choice @4 = %d, want 1 (the difficulty stays on the frame)", got)
	}
	// 载荷 @9..12 是 **Outcome**（不是 Following）；权威 idx=654 该位是 5。
	if got := binary.LittleEndian.Uint32(p[9:]); got != 5 {
		t.Fatalf("Outcome @9 = %d, want 5", got)
	}
	// Following 在载荷 @17，恒为 ffffffff。
	if got := binary.LittleEndian.Uint32(p[17:]); got != 0xffffffff {
		t.Fatalf("Following @17 = %#x, want ffffffff", got)
	}
	if got := binary.LittleEndian.Uint32(p[13:]); got != 5 {
		t.Fatalf("Stage @13 = %d, want 5 (the room just cleared)", got)
	}
	for i, want := range [6]byte{0, 1, 2, 3, 4, 5} {
		if got := p[29+12*i]; got != want {
			t.Fatalf("stage record %d @%d = %d, want %d (records must survive the close)", i, 29+12*i, got, want)
		}
	}
	if got := binary.LittleEndian.Uint32(p[128:]); got != 1 {
		t.Fatalf("target mark 0 @128 = %d, want 1", got)
	}
	// 与「撤退清空态」必须不同：那个是全 ff + Stage0。
	closed := ApocalypseInfo(ApocalypseClosedInfo())
	if closed[13] == p[13] {
		t.Fatal("the retreat clear state must differ from the completed state")
	}
}
