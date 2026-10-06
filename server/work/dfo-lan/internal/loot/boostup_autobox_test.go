package loot

import (
	"reflect"
	"testing"

	"dfolan/internal/boostup"
)

// autoBoxSource 是本包既有的 fakeBoxes：自动开盒复用掉落线同一个
// RewardBoxSource，礼盒内容在这里是手搭的，判据是「展开规则」本身。
// 源表能否开出这些东西由 cmd 层的真实归档测试钉住。
func TestBoostAutoGrantsUnwrapRules(t *testing.T) {
	src := fakeBoxes{
		boxes: map[uint32]RewardBox{
			// 第九关：一件装备，直接型单候选。
			590015951: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 100051285, Weight: 1000, Count: 1}}}}},
			// 第十一关：材料 + **未开封的选择箱** + 账号共享材料。
			590015965: {Pools: []RewardBoxPool{
				{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 590015966, Weight: 1000, Count: 1000}}},
				{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 590015964, Weight: 1000, Count: 1}}},
				{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 3037, Weight: 1000, Count: 1000}}},
			}},
			// 带权重的池不是定值奖励：服务端替玩家掷骰就把礼盒变成了抽奖券。
			590015952: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{
				{Template: 11, Weight: 600, Count: 1}, {Template: 12, Weight: 400, Count: 1},
			}}}},
			// 目录里没有的模板是源留的空槽，不能凭空发明成物品。
			590015999: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{{Template: 777, Weight: 1, Count: 1}}}}},
		},
		items: map[uint32]bool{100051285: true, 590015966: true, 590015964: true, 3037: true},
	}
	s := &Service{RewardBoxes: src}
	role := Role{ID: 1, AccountID: 1}

	g, e := s.BoostAutoGrants(role, 9, []boostup.Reward{{Item: 590015951, Count: 1}})
	if e != nil || !reflect.DeepEqual(g, []BoostAutoGrant{{Template: 100051285, Amount: 1}}) {
		t.Fatal("equipment box did not unwrap to the equipment", g, e)
	}
	g, e = s.BoostAutoGrants(role, 11, []boostup.Reward{{Item: 590015965, Count: 1}})
	want := []BoostAutoGrant{{590015966, 1000}, {590015964, 1}, {3037, 1000}}
	if e != nil || !reflect.DeepEqual(g, want) {
		t.Fatal("final box did not unwrap one layer", g, e)
	}
	if g[1].Template != 590015964 {
		t.Fatal("selection box was opened for the player")
	}
	// 同一步重放必须给同一份内容：领奖标记之外没有第二份随机数来源。
	again, e := s.BoostAutoGrants(role, 11, []boostup.Reward{{Item: 590015965, Count: 1}})
	if e != nil || !reflect.DeepEqual(again, want) {
		t.Fatal("auto unwrap is not replay-stable", again, e)
	}

	if _, e = s.BoostAutoGrants(role, 9, []boostup.Reward{{Item: 590015952, Count: 1}}); e == nil {
		t.Fatal("weighted pool accepted as a direct reward")
	}
	if _, e = s.BoostAutoGrants(role, 9, []boostup.Reward{{Item: 590015999, Count: 1}}); e == nil {
		t.Fatal("unknown template invented as an item")
	}
	if _, e = s.BoostAutoGrants(role, 9, []boostup.Reward{{Item: 590015951, Count: 0}}); e == nil {
		t.Fatal("empty outer-box count accepted")
	}
	if _, e = (&Service{}).BoostAutoGrants(role, 9, []boostup.Reward{{Item: 590015951, Count: 1}}); e == nil {
		t.Fatal("auto unwrap without a box source")
	}
}
