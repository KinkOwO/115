package loot

import (
	"encoding/json"
	"testing"
)

// 源驱动奖单的行数上界必须与「结算帧每座 126 件」对齐，而不是老月湖策略的 16。
//
// 背景（2026-10-09 业主实机）：蔚蓝号的效率是「装备 10 / 誓约 4」（沉月湖 7/3）⇒
// 固定 2 件 + 基础产物 + 10 + 4 > 16 ⇒ 奖单写进事件表后 `ReadMoonReward` 读回来时
// 被 `DecodeMoonReward` 判 `foreign/corrupt Moon reward proof` ⇒ 冻结整条失败
// ⇒ CMD46 被拒 ⇒ **结算面板根本不出现**。沉月湖恰好卡在 16 以内才没露出来。
func TestDecodeMoonRewardRowBudgetMatchesFrameLimit(t *testing.T) {
	if MoonRewardMaxRows != 126 {
		t.Fatalf("MoonRewardMaxRows=%d，期望 126（ConquestClearReward115 的每座上限）", MoonRewardMaxRows)
	}
	s := moonRewardFixture()
	role := Role{ConfigVersion: s.Catalog.Source.SaveIdentity(), AccountID: 7, ID: 9}
	const run = "0123456789abcdef0123456789abcdef"
	build := func(n int) json.RawMessage {
		p := MoonRewardPlan{Source: role.ConfigVersion, Run: run, Rules: "seed", Account: role.AccountID, Character: role.ID}
		for i := 0; i < n; i++ {
			p.Grants = append(p.Grants, MoonRewardGrant{Template: 101, Count: 1})
		}
		b, e := json.Marshal(p)
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	// 16 = 旧上界；17 = 蔚蓝号这种「固定+基础+装备10+誓约4」的单子必然越过的第一条线。
	for _, n := range []int{1, 16, 17, MoonRewardMaxRows} {
		if _, e := s.DecodeMoonReward(role, run, build(n)); e != nil {
			t.Fatalf("%d 件应当被接受，实际 %v", n, e)
		}
	}
	if _, e := s.DecodeMoonReward(role, run, build(MoonRewardMaxRows+1)); e == nil {
		t.Fatalf("%d 件应当被拒（超过结算帧每座上限）", MoonRewardMaxRows+1)
	}
	if _, e := s.DecodeMoonReward(role, run, build(0)); e == nil {
		t.Fatal("空奖单应当被拒")
	}
	// 身份仍要卡住：换一个 run 必须拒绝（修上界不能顺手把归属校验放宽）。
	if _, e := s.DecodeMoonReward(role, "ffffffffffffffffffffffffffffffff", build(20)); e == nil {
		t.Fatal("换了 run 的奖单应当被拒")
	}
}
