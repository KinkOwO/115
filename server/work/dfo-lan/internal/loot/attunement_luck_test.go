package loot

import (
	"reflect"
	"testing"
)

// 「幸运事件」（小幸运 ×15 / 大幸运 ×50）在服务端**没有一行专门代码**：两个档就落在
// fixed 池里，抽中后走通用开箱路径，倍数写在盒子的 pool 里。这个测试钉住前半句 ——
// 如果哪天有人把它们从 fixed 池里挪走，或者权重被「补给高档」顺手改掉，这里会红。
func TestLuckTiersSitInTheFixedPool(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	got := a.LuckTemplates()
	want := []uint32{10416119, 10416120}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("luck templates = %v, want %v", got, want)
	}

	const dungeon = uint32(100005014)
	small, large := a.LuckWeights(dungeon, 0)
	if small != 3333 || large != 500 {
		t.Fatalf("maze 0 luck weights = %d/%d of 1e6, want 3333/500", small, large)
	}
	// 社区实测 1710 场里小幸运 7 次、大幸运 1 次（0.41% / 0.059%），与这两条权重同量级；
	// 这里只钉「maze 1 没有它们」——那是小深渊两张迷宫图之间一处容易忽略的差异。
	if s1, l1 := a.LuckWeights(dungeon, 1); s1 != 0 || l1 != 0 {
		t.Fatalf("maze 1 luck weights = %d/%d, want 0/0 (that segment gives the share to the common tier)", s1, l1)
	}
	// 大深渊的三张表都没有 lucky 档，所以对它们调用应当给出 0/0 而不是 panic。
	for _, d := range []uint32{100005066, 100005067, 100005068} {
		if s, l := a.LuckWeights(d, 0); s != 0 || l != 0 {
			t.Fatalf("dungeon %d reports luck weights %d/%d, want none", d, s, l)
		}
	}
}

// 调参层不能顺手改掉幸运事件：它属于另一条线（Mystical Fortune），
// 「削低档补高档」只该动普通与稀有。
func TestRebalanceKeepsLuckTiersIntact(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	const dungeon = uint32(100005014)
	beforeSmall, beforeLarge := a.LuckWeights(dungeon, 0)
	if _, _, err := a.ApplyRebalance(Rebalance{OmenHalveIdle: true, FixedTiltPercent: 25}); err != nil {
		t.Fatal(err)
	}
	small, large := a.LuckWeights(dungeon, 0)
	if small != beforeSmall || large != beforeLarge {
		t.Fatalf("rebalance moved the luck tiers: %d/%d -> %d/%d", beforeSmall, beforeLarge, small, large)
	}
}
