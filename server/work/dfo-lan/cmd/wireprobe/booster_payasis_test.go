package main

import (
	"dfolan/internal/loot"
	"testing"
)

// 10415192「Splendor Soul Crystal」声明了 [booster info]，但它是小深渊 maze 1 的
// **材料**（maze 0 的同槽位是根本不带 [booster info] 的银币），官方按原样发 2 个。
//
// 这条守两件事，少一件都会坏：它不被展开（不然玩家拿到的是产物），也不能被当成
// 「本 build 打不开的盒子」丢掉（那样连产物都没有）。
func TestPayAsIsMaterialIsPaidUntouched(t *testing.T) {
	src := boosterBoxSource{catalog: &BoosterCatalog{
		Definitions: map[uint32]BoosterDefinition{
			10415192: {Template: 10415192, Type: "[booster]", Pools: []BoosterRewardPool{{
				DrawCount:  1,
				Candidates: []BoosterRewardCandidate{{Template: 10415191, Weight: 1000, Count: 1}},
			}}},
		},
		Items: map[uint32]ItemIndexInfo{
			10415192: {ID: 10415192, Kind: "stackable", StackableType: "[booster]"},
			10415191: {ID: 10415191, Kind: "stackable"},
		},
	}}
	if _, ok := src.RewardBox(10415192); ok {
		t.Fatal("10415192 被当成可开的包装了")
	}
	if src.Container(10415192) {
		t.Fatal("10415192 被当成打不开的盒子 —— 那样它会被静默丢掉")
	}
	if !src.Item(10415192) {
		t.Fatal("10415192 必须仍被认成可发放的物品")
	}
	got, _, skipped := loot.OpenRewardBoxes(1, src, []loot.Award{{Template: 10415192, Amount: 2}})
	if len(got) != 1 || got[0].Template != 10415192 || got[0].Amount != 2 {
		t.Fatalf("展开结果 = %+v，期望原样发出 10415192×2", got)
	}
	if len(skipped) != 0 {
		t.Fatalf("不该有跳过项：%v", skipped)
	}
}

// 反过来：真正该展开的礼盒不能因为这条豁免被放松。10416103 是小深渊 maze 0 的
// 固定表顶层礼盒（stack_limit 1），它必须继续被展开。
func TestShippedWrapperIsStillOpened(t *testing.T) {
	src := boosterBoxSource{catalog: &BoosterCatalog{
		Definitions: map[uint32]BoosterDefinition{
			10416103: {Template: 10416103, Type: "[booster]", Pools: []BoosterRewardPool{{
				DrawCount:  1,
				Candidates: []BoosterRewardCandidate{{Template: 10362432, Weight: 10000, Count: 2}},
			}}},
		},
		Items: map[uint32]ItemIndexInfo{
			10416103: {ID: 10416103, Kind: "stackable", StackableType: "[booster]"},
			10362432: {ID: 10362432, Kind: "stackable", StackableType: "[material]"},
		},
	}}
	if _, ok := src.RewardBox(10416103); !ok {
		t.Fatal("固定表的顶层礼盒必须仍可展开")
	}
	got, _, _ := loot.OpenRewardBoxes(1, src, []loot.Award{{Template: 10416103, Amount: 1}})
	if len(got) != 1 || got[0].Template != 10362432 || got[0].Amount != 2 {
		t.Fatalf("展开结果 = %+v，期望 10362432×2", got)
	}
}
