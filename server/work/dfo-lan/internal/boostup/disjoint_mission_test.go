package boostup

import "testing"

// 第九关（真源 `[step info][no] 9 → [mission][type] disjoint`）的完成判定：
// 只认「**已经在这一关领过奖励**（Phase==2 && Claimed[Step]）+ 服务端真的删掉了装备」，
// 其它一律不推进、也不报错——分解是玩家的正常操作，不能被事件层拦死。
func TestDisjointMissionAdvanced(t *testing.T) {
	c := &Catalog{Steps: []Step{
		{Number: 1, Guide: "normal", Mission: "equip item"},
		{Number: 2, Guide: "normal", Mission: "disjoint"},
		{Number: 3, Guide: "normal", Mission: "none"},
	}}
	claimed := func(step byte) map[byte]bool { return map[byte]bool{step: true} }

	// 唯一应推进的组合：本关 mission 是 disjoint、已领取、真删了装备。
	on := State{Activated: true, Training: Training{Step: 2, Phase: 2, Claimed: claimed(2)}}
	got, changed, err := c.DisjointMissionAdvanced(on, true)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("claimed disjoint step with a real disassembly must advance")
	}
	if got.Training.Step != 3 || got.Training.Phase != 0 || got.Training.Finished {
		t.Fatalf("advance landed on %+v", got.Training)
	}
	if on.Training.Step != 2 {
		t.Fatal("input state mutated")
	}
	if got.Training.Claimed[2] != true {
		t.Fatal("claim history must survive the advance")
	}

	for _, tc := range []struct {
		name   string
		state  State
		proof  bool
		assign *Catalog
	}{
		{"event not activated", State{Training: Training{Step: 2, Phase: 2, Claimed: claimed(2)}}, true, c},
		{"reward not claimed yet", State{Activated: true, Training: Training{Step: 2, Phase: 1}}, true, c},
		{"claimed but guide still pending", State{Activated: true, Training: Training{Step: 2, Phase: 2}}, true, c},
		{"nothing was disassembled", on, false, c},
		{"current step is not a disjoint mission", State{Activated: true, Training: Training{Step: 1, Phase: 2, Claimed: claimed(1)}}, true, c},
		{"finished track", State{Activated: true, Training: Training{Step: 2, Phase: 2, Finished: true, Claimed: claimed(2)}}, true, c},
		{"step out of range", State{Activated: true, Training: Training{Step: 9, Phase: 2, Claimed: claimed(9)}}, true, c},
	} {
		if tc.assign == nil {
			tc.assign = c
		}
		out, changed, err := tc.assign.DisjointMissionAdvanced(tc.state, tc.proof)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", tc.name, err)
		}
		if changed {
			t.Fatalf("%s: must not advance, got %+v", tc.name, out.Training)
		}
		if out.Training.Step != tc.state.Training.Step || out.Training.Phase != tc.state.Training.Phase {
			t.Fatalf("%s: state rewritten anyway: %+v -> %+v", tc.name, tc.state.Training, out.Training)
		}
	}

	// 目录缺失（活动未启用）时同样静默不推进。
	if _, changed, err := (*Catalog)(nil).DisjointMissionAdvanced(on, true); err != nil || changed {
		t.Fatalf("nil catalog: changed=%v err=%v", changed, err)
	}
}
