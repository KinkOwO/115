package loot

import "testing"

// 小深渊征兆的保底（官方设定，业主 2026-10-08 提供）：
//
//	记录玩家没触发征兆的次数，一旦有征兆了就归 0，即便征兆当轮不结算；
//	直到征兆被结算后，重新计数；累计 30 次不触发就保底补 1 阶。
//
// 钉住四件事：① 第 30 次才发、第 29 次不发；② 获得或结算都归零；③ 持有 ≥1 时计数不动；
// ④ 大深渊那三个副本（没有 [coupon drop table]）完全不碰这条线。

// omenSeedWithOutcome 找一个落在指定分支的种子。
//
// 行 0（持有 0）只有「获得」与「无事发生」两支 —— 它的 [drop prob] 是 0，永远不会结算。
// 想找「结算」那一支要用持有 ≥1 的行。
func omenSeedWithOutcome(t *testing.T, a *AttunementRewards, held uint32, gained, paid bool) uint32 {
	t.Helper()
	for i := uint32(1); i <= 200000; i++ {
		seed := i*2654435761 + 1
		out, err := a.AdvanceOmen(seed, omenDungeon, held, 0)
		if err != nil {
			t.Fatalf("advance: %v", err)
		}
		if out.Gained == gained && out.Paid == paid {
			return seed
		}
	}
	t.Fatalf("找不到 held=%d gained=%v paid=%v 的种子", held, gained, paid)
	return 0
}

func TestOmenPityFiresExactlyOnTheThirtiethMiss(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	seed := omenSeedWithOutcome(t, a, 0, false, false)

	t.Run("第 29 次还不发", func(t *testing.T) {
		out, err := a.AdvanceOmen(seed, omenDungeon, 0, OmenPityMisses-2)
		if err != nil {
			t.Fatal(err)
		}
		if out.Pity || out.Gained {
			t.Fatalf("第 %d 次就保底了：gained=%v pity=%v", OmenPityMisses-1, out.Gained, out.Pity)
		}
		if out.MissesAfter != OmenPityMisses-1 {
			t.Fatalf("计数 = %d, want %d", out.MissesAfter, OmenPityMisses-1)
		}
		if out.After != 0 || out.Paid {
			t.Fatalf("无事发生那一场不该动持有数：after=%d paid=%v", out.After, out.Paid)
		}
	})

	t.Run("第 30 次补一阶", func(t *testing.T) {
		out, err := a.AdvanceOmen(seed, omenDungeon, 0, OmenPityMisses-1)
		if err != nil {
			t.Fatal(err)
		}
		if !out.Pity || !out.Gained {
			t.Fatalf("保底没触发：gained=%v pity=%v", out.Gained, out.Pity)
		}
		if out.After != 1 {
			t.Fatalf("持有 = %d, want 1（保底只补一阶）", out.After)
		}
		if out.MissesAfter != 0 {
			t.Fatalf("保底后计数 = %d, want 0", out.MissesAfter)
		}
		// 补的那一阶是「获得一个征兆」，不是结算：不掷 list、也不发东西。
		if out.Paid || len(out.Awards) != 0 {
			t.Fatalf("保底不该结算阶段奖励：paid=%v awards=%v", out.Paid, out.Awards)
		}
	})

	t.Run("持有 ≥1 时计数不动", func(t *testing.T) {
		heldSeed := omenSeedWithOutcome(t, a, 1, false, false)
		out, err := a.AdvanceOmen(heldSeed, omenDungeon, 1, 4)
		if err != nil {
			t.Fatal(err)
		}
		if out.MissesAfter != 4 {
			t.Fatalf("持有 1 个征兆时计数 = %d, want 4（要等这次征兆结算之后才重新计数）", out.MissesAfter)
		}
		// 即使计数已经堆到保底线，也不该在「持有中」的那一场补第二阶。
		out, err = a.AdvanceOmen(heldSeed, omenDungeon, 1, OmenPityMisses-1)
		if err != nil {
			t.Fatal(err)
		}
		if out.Pity {
			t.Fatal("持有 ≥1 时不该触发保底")
		}
	})
}

func TestOmenMissCounterResetsOnGainAndOnSettlement(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	t.Run("获得一个征兆就归 0", func(t *testing.T) {
		seed := omenSeedWithOutcome(t, a, 0, true, false)
		out, err := a.AdvanceOmen(seed, omenDungeon, 0, 7)
		if err != nil {
			t.Fatal(err)
		}
		if !out.Gained || out.After != 1 {
			t.Fatalf("gained=%v after=%d", out.Gained, out.After)
		}
		// 「即便征兆当轮不结算」——这一场本来就没结算，计数也要归 0。
		if out.Paid {
			t.Fatal("行 0 不该结算")
		}
		if out.MissesAfter != 0 {
			t.Fatalf("获得征兆后计数 = %d, want 0", out.MissesAfter)
		}
	})

	t.Run("结算之后重新计数", func(t *testing.T) {
		seed := omenSeedWithOutcome(t, a, 2, false, true)
		out, err := a.AdvanceOmen(seed, omenDungeon, 2, 9)
		if err != nil {
			t.Fatal(err)
		}
		if !out.Paid || out.After != 0 {
			t.Fatalf("paid=%v after=%d", out.Paid, out.After)
		}
		if out.MissesAfter != 0 {
			t.Fatalf("结算后计数 = %d, want 0", out.MissesAfter)
		}
	})
}

// TestOmenPityNeverTouchesDungeonsWithoutStages 这条线**只属于小深渊**：大深渊三个副本的
// [coupon drop table] 一行都没有（见 configs/attunement-rewards.generated.json），
// 所以它们连计数都不该碰，种子也不该被消耗。
func TestOmenPityNeverTouchesDungeonsWithoutStages(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, dungeon := range []uint32{100005066, 100005067, 100005068} {
		if n := a.OmenStagesCount(dungeon); n != 0 {
			t.Fatalf("dungeon %d has %d omen stages, want 0", dungeon, n)
		}
		const seed = uint32(20261008)
		out, err := a.AdvanceOmen(seed, dungeon, 0, OmenPityMisses-1)
		if err != nil {
			t.Fatalf("dungeon %d: %v", dungeon, err)
		}
		if out.Pity || out.Gained || out.Paid {
			t.Fatalf("dungeon %d 不该被征兆线碰到：%+v", dungeon, out)
		}
		if out.After != 0 || out.MissesAfter != OmenPityMisses-1 || out.Seed != seed {
			t.Fatalf("dungeon %d 状态被改动了：after=%d misses=%d seed=%d",
				dungeon, out.After, out.MissesAfter, out.Seed)
		}
	}
}
