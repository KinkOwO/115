package catalog

import (
	"testing"

	"dfolan/internal/catalog/pvf"
)

// cell 组词助手：`[booster info]` 的正文由「标记（type 3）+ 数值（type 0）」交错组成。
func cellTag(text string) pvf.Token { return pvf.Token{Type: 3, Text: text} }
func cellNum(v int32) pvf.Token     { return pvf.Token{Type: 0, Value: v} }

func cellRow(tokens ...pvf.Token) []pvf.Token { return tokens }

// TestParseBoosterInfoKeepsTheNoDropPlaceholder 锁住 2026-10-07 定位的那条缺陷。
//
// 真实源（大深渊固定盒 10419725/10419728 的 `[booster info]`）里每个池的正文是
//
//	<drawCount> [ <template> <weight> <count> ] …
//
// 而「本次没有」那一项写成 **-1**。旧实现用 `Value > 0` 过滤，把 -1 丢掉 ⇒ 三元组
// 整体错位一格 ⇒ drawCount 被当成模板、权重被当成数量。实机表现：每次通关地上都多出
//
//	template 1  = stackable/coin.stk             （复活币）        权重 911200/1e6
//	template 6  = stackable/cash/store_silver.stk（金库升级道具）  权重 474500/1e6
//	template 12 = 空面（不是物品，被当空奖丢掉）
//
// 同时真正的 `[draw count]`（6 / 12 次）被静默降成 1 次，誓约那条线大面积少发。
// 本测试用源里的原样数字断言：-1 必须留在候选表里当「本次没有」，且不能再出现 1 / 6。
func TestParseBoosterInfoKeepsTheNoDropPlaceholder(t *testing.T) {
	// v=10419725/10419726 的 `[etc] 1 -1 911200 1 10420672 60000 …`
	cells := cellRow(
		cellTag("[booster info]"),
		cellTag("[etc]"),
		cellNum(1),
		cellNum(-1), cellNum(911200), cellNum(1),
		cellNum(10420672), cellNum(60000), cellNum(1),
		cellNum(10420673), cellNum(18000), cellNum(1),
		cellTag("[/etc]"),
		cellTag("[/booster info]"),
	)
	pools := parseBoosterInfo(cells)
	if len(pools) != 1 {
		t.Fatalf("pools = %d, want 1", len(pools))
	}
	got := pools[0]
	if got.DrawCount != 1 {
		t.Fatalf("draw count = %d, want 1 (the leading number of the body)", got.DrawCount)
	}
	want := []BoosterRewardCandidate{
		{Template: BoosterNoDropTemplate, Weight: 911200, Count: 1},
		{Template: 10420672, Weight: 60000, Count: 1},
		{Template: 10420673, Weight: 18000, Count: 1},
	}
	if len(got.Candidates) != len(want) {
		t.Fatalf("candidates = %+v, want %d", got.Candidates, len(want))
	}
	for i := range want {
		if got.Candidates[i] != want[i] {
			t.Fatalf("candidate[%d] = %+v, want %+v", i, got.Candidates[i], want[i])
		}
	}
	for _, c := range got.Candidates {
		if c.Template == 1 || c.Template == 6 {
			t.Fatalf("candidate %+v is the misaligned reading: the weights must never become templates", c)
		}
	}
}

// TestParseBoosterInfoHonoursTheDrawCountAfterThePlaceholder 锁住同一处错位的第二半：
// 池开头的那个数**是抽次数**，不是模板。
//
// `[etc] 6 -1 474500 1 100401592 272000 1 10419720 253500 1`（10419725 的第三池）
// 的正确读法是「抽 6 次，候选 = {本次没有 47.45%, 100401592 27.2%, 10419720 25.35%}」。
// 旧实现把它读成「模板 6」——地上出现金库升级道具，真正的 6 次抽取一次都没发生。
func TestParseBoosterInfoHonoursTheDrawCountAfterThePlaceholder(t *testing.T) {
	cells := cellRow(
		cellTag("[booster info]"),
		cellTag("[etc]"),
		cellNum(6),
		cellNum(-1), cellNum(474500), cellNum(1),
		cellNum(100401592), cellNum(272000), cellNum(1),
		cellNum(10419720), cellNum(253500), cellNum(1),
		cellTag("[/etc]"),
		cellTag("[/booster info]"),
	)
	pools := parseBoosterInfo(cells)
	if len(pools) != 1 {
		t.Fatalf("pools = %d, want 1", len(pools))
	}
	if pools[0].DrawCount != 6 {
		t.Fatalf("draw count = %d, want 6", pools[0].DrawCount)
	}
	if len(pools[0].Candidates) != 3 {
		t.Fatalf("candidates = %+v, want 3", pools[0].Candidates)
	}
	if pools[0].Candidates[0].Template != BoosterNoDropTemplate {
		t.Fatalf("candidate[0] = %+v, want the no-drop placeholder", pools[0].Candidates[0])
	}
	if pools[0].Candidates[1].Template != 100401592 || pools[0].Candidates[2].Template != 10419720 {
		t.Fatalf("real prizes lost: %+v", pools[0].Candidates)
	}
}

// TestParseBoosterInfoKeepsZeroOutOfTheBody 说明 0 仍然被过滤：源码只在段与段之间
// 写 0，它不属于池正文，保留它反而会把三元组推歪。这条与上一条不矛盾 ——
// 被保留的是**负数**这个明确的占位符，不是任意非正数。
func TestParseBoosterInfoKeepsZeroOutOfTheBody(t *testing.T) {
	cells := cellRow(
		cellTag("[booster info]"),
		cellTag("[etc]"),
		cellNum(1),
		cellNum(0),
		cellNum(10362432), cellNum(10000), cellNum(70),
		cellTag("[/etc]"),
		cellTag("[/booster info]"),
	)
	pools := parseBoosterInfo(cells)
	if len(pools) != 1 || len(pools[0].Candidates) != 1 {
		t.Fatalf("pools = %+v, want one pool with one candidate", pools)
	}
	if got := pools[0].Candidates[0]; got.Template != 10362432 || got.Weight != 10000 || got.Count != 70 {
		t.Fatalf("candidate = %+v, want {10362432 10000 70}", got)
	}
}
