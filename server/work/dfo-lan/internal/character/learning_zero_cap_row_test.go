package character

import "testing"

// 当前源里 [growtype maximum level] 的零有两种含义，必须区分：
//
//   - 整行全 0（如 atgunner/quartermaster 的 [0 0 0 0 0 0]）＝ 该行不使用基础
//     growtype 上限表，归属由 [skill fitness growtype] / [skill fitness second
//     growtype] 决定，等级上限取 [maximum level]；
//   - 部分为 0（如 archer/latentability 的 [0 1 1 1 1 1]）＝ 那个 growtype
//     真的学不了。
//
// 把全 0 行当成"上限 0"会让弹药等职业觉醒后的技能无法加点（玩家实测反馈）；
// 把部分 0 行当成"不限"又会泄漏未转职/跨分支。这条测试钉住两者的分界。
func TestAllZeroGrowtypeRowKeepsFitnessOwnership(t *testing.T) {
	l := loadLearningForTest(t)

	// atgunner/quartermaster：全 0 行 + [skill fitness second growtype] 2。
	q := l.index[5][252]
	if q.Path != "skill/atgunner/quartermaster.skl" {
		t.Fatal("unexpected fixture", q.Path)
	}
	if !q.ForAdvancement(2) || q.ForAdvancement(1) || q.ForAdvancement(0) {
		t.Fatal("quartermaster ownership must come from [skill fitness second growtype] 2")
	}
	if cost, err := q.Cost(75, 2, 1, nil); err != nil || cost != 80 {
		t.Fatalf("quartermaster rank 1: cost=%d err=%v", cost, err)
	}
	// 上限来自 [maximum level] 20，而不是被全 0 行压成 0。
	if _, err := q.Cost(75, 2, 21, nil); err == nil {
		t.Fatal("rank 21 accepted above [maximum level] 20")
	}
	if _, err := q.Cost(74, 2, 1, nil); err == nil {
		t.Fatal("learned below [required level] 75")
	}
	// 同一 job 的另一个分支不能学。
	if _, err := q.Cost(75, 3, 1, nil); err == nil {
		t.Fatal("foreign growtype learned quartermaster")
	}

	// atgunner/g96thermobaricgranade：全 0 行、无任何 fitness 声明 —— 源里它只挂在
	// 该职业的技能树上，服务端不做 growtype 门禁，但等级与前置仍然生效。
	g := l.index[5][253]
	if g.Path != "skill/atgunner/g96thermobaricgranade.skl" {
		t.Fatal("unexpected fixture", g.Path)
	}
	if cost, err := g.Cost(80, 0, 1, map[uint16]byte{56: 1}); err != nil || cost != 90 {
		t.Fatalf("g96 rank 1: cost=%d err=%v", cost, err)
	}
	if _, err := g.Cost(80, 0, 1, map[uint16]byte{}); err == nil {
		t.Fatal("missing prerequisite accepted")
	}
	if _, err := g.Cost(79, 0, 1, map[uint16]byte{56: 1}); err == nil {
		t.Fatal("learned below [required level] 80")
	}

	// 部分 0 的行不得被放开：archer/latentability 的 growtype 0 仍然学不了。
	a := l.index[16][501]
	if a.Path != "skill/archer/latentability.skl" {
		t.Fatal("unexpected fixture", a.Path)
	}
	if a.ForAdvancement(0) {
		t.Fatal("partially zero row leaked the zero column")
	}
	if !a.ForAdvancement(1) {
		t.Fatal("partially zero row lost its non-zero column")
	}

	// 全 0 行 + 有效觉醒矩阵 = 觉醒专属：自身不可学，只有 forState 在对应
	// (growtype, stage) 写回上限之后才通过。
	s := l.index[0][86]
	if s.ForAdvancement(1) {
		t.Fatal("awakening-only row must stay refused without a matching stage")
	}
	if !s.ForAwakening(1, 1) {
		t.Fatal("awakening matrix lost")
	}
	if _, err := s.costForState(State{Level: 115, Advancement: 1, Awakening: 1}, 1, map[uint16]byte{86: 1}); err != nil {
		t.Fatalf("awakened character refused an awakening skill: %v", err)
	}
	if _, err := s.costForState(State{Level: 115, Advancement: 1, Awakening: 0}, 1, map[uint16]byte{86: 1}); err == nil {
		t.Fatal("unawakened character learned an awakening-only skill")
	}
}
