package loot

import (
	"fmt"
	"testing"
)

// 大深渊（100005066/67/68）的 additional 一张表有 10~14 支，而**档位标签在多支之间重复**：
// epic 同时挂在涌起波动、同样的命运、祝福气息武器/首饰/防具/特殊、神兵认主、宝匣上。
// 上一版 PlanRun 只把 tier 带进 RunTiers，RollPlanned 再按 tier 反查 ⇒ 永远命中第一支，
// 其余全被遮蔽（实测 100005068 的 14 支里只有 3 支付得到东西）。下面三条钉住这个洞。

// TestPlanRunReachesEveryAdditionalBranch 要求：扫一遍种子，**每一支的每一个条目**
// 都要至少被掷中一次。任何一支永远掷不到 = 那一支对应的事件在游戏里不存在。
func TestPlanRunReachesEveryAdditionalBranch(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, dg := range []uint32{100005014, 100005066, 100005067, 100005068} {
		tbl := a.byDungeon[dg]
		if tbl == nil {
			t.Fatalf("dungeon %d not loaded", dg)
		}
		// 同一个条目可以挂在多支上（涌起 2/3/5 次那三支共用同一张 7 项列表），
		// 所以这里记的是「该条目允许出现在哪些 drop count 上」。
		declared := map[uint32]map[uint32]bool{}
		for _, br := range tbl.Additional {
			for _, e := range br.Entries {
				if declared[e.Item] == nil {
					declared[e.Item] = map[uint32]bool{}
				}
				declared[e.Item][br.DropCount] = true
			}
		}
		seen := map[uint32]bool{}
		seed := uint32(1)
		for i := 0; i < 200000; i++ {
			tiers, next, err := a.PlanRun(seed, dg, 0)
			if err != nil {
				t.Fatalf("dungeon %d PlanRun: %v", dg, err)
			}
			if tiers.OathTemplate != 0 {
				seen[tiers.OathTemplate] = true
				if !declared[tiers.OathTemplate][tiers.OathAmount] {
					t.Fatalf("dungeon %d: 条目 %d 带下来的 drop count = %d，源表里那几条支上都没有这个值",
						dg, tiers.OathTemplate, tiers.OathAmount)
				}
			}
			seed = next
		}
		if len(seen) != len(declared) {
			var missing []uint32
			for item := range declared {
				if !seen[item] {
					missing = append(missing, item)
				}
			}
			t.Fatalf("dungeon %d: 20 万个种子只掷到 %d/%d 个条目，永远掷不到的有 %v",
				dg, len(seen), len(declared), missing)
		}
	}
}

// TestRollPlannedPaysTheRolledBranchVerbatim 是核心契约：掉落阶段发的必须**就是**
// 进本时掷中的那一条，数量必须**就是**那一支的 `[drop count]`，不许按档位重选。
func TestRollPlannedPaysTheRolledBranchVerbatim(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	seed := uint32(7)
	for i := 0; i < 20000; i++ {
		for _, dg := range []uint32{100005066, 100005067, 100005068} {
			tiers, next, err := a.PlanRun(seed, dg, 0)
			if err != nil {
				t.Fatalf("dungeon %d PlanRun: %v", dg, err)
			}
			awards, _, err := a.RollPlanned(next, dg, 0, tiers)
			if err != nil {
				t.Fatalf("dungeon %d RollPlanned: %v", dg, err)
			}
			// RollPlanned 的奖单形状：先一条 fixed（珠子线在场时），再 OathAmount 条
			// 掷中的那一条 additional。同一个条目可能同时挂在 fixed 与 additional 上
			// （10419721 就是），所以只能按位置切，不能按模板号数。
			fixedN := 0
			if tiers.Primer != 0 || tiers.PrimerLabel != "" {
				fixedN = 1
			}
			if len(awards) != fixedN+int(tiers.OathAmount) {
				t.Fatalf("dungeon %d seed %d: 奖单 %d 条，应为 fixed %d + oath %d（%v）",
					dg, seed, len(awards), fixedN, tiers.OathAmount, awards)
			}
			for _, aw := range awards[fixedN:] {
				if aw.Template != tiers.OathTemplate {
					t.Fatalf("dungeon %d seed %d: 掷中的是 %d，实发 %v（遮蔽回归）",
						dg, seed, tiers.OathTemplate, awards)
				}
			}
			// 掷中的条目必须真的在该表 additional 里。
			declared := false
			for _, br := range a.byDungeon[dg].Additional {
				for _, e := range br.Entries {
					if e.Item == tiers.OathTemplate {
						declared = true
					}
				}
			}
			if !declared {
				t.Fatalf("dungeon %d: 发出去的 %d 不在该表 additional 里", dg, tiers.OathTemplate)
			}
			seed = next
		}
	}
}

// TestRollPlannedWithoutTemplateStillFollowsTier 钉住兼容面：两条诊断覆盖
// （`-oath-grades` / `--oath-from-gear`）只给档位数字、不给掷中的条目，
// 这时仍按档位反查选池，行为与旧版逐字节相同。
func TestRollPlannedWithoutTemplateStillFollowsTier(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	// 小深渊 maze 0 每个档位只有一个 additional 条目 ⇒ 反查是确定的。
	awards, _, err := a.RollPlanned(7, omenDungeon, 0, RunTiers{Oath: 45})
	if err != nil {
		t.Fatal(err)
	}
	if len(awards) != 1 || awards[0].Template != 10416144 {
		t.Fatalf("只给 oath=45 应发 10416144，实发 %v", awards)
	}
	// 大深渊 epic 档：反查命中的是第一支（涌起 3 次波动），数量是它的 drop count。
	awards, _, err = a.RollPlanned(7, 100005068, 0, RunTiers{Oath: 44})
	if err != nil {
		t.Fatal(err)
	}
	if len(awards) != 3 {
		t.Fatalf("诊断覆盖按档位反查应发该支的 3 件，实发 %v", awards)
	}
	for _, aw := range awards {
		if aw.Template != 10419735 {
			t.Fatalf("诊断覆盖 epic 档应发 10419735，实发 %v", awards)
		}
	}
}

// TestPlanRunCarriesTheBranchIdentity 钉住「掷中的是源表里哪一行」这件事本身。
//
// 业主 2026-10-12 的问题就是这一条：`[additional drop table]` 那 12 行（涌起 2/3/5/10 次
// 波动、同样的命运、祝福气息武器/首饰/防具/特殊、神兵认主、虚无的气息×3、宝匣、希望之光）
// 到底有没有触发？只带档位与模板号答不了这个问题 —— 涌起那四支**共用同一张 7 项清单**
// （10419730..36），epic 这个标签又挂在 10 支上，所以「掷中哪一行」必须由 `[effect index]`
// 与 `[select prob]` 认。虚无的气息在源里是**同一个 effect index 挂三条**（50000/100000/30000），
// 因此这里按 (effect, prob) 认支，不按 index 单独认。
func TestPlanRunCarriesTheBranchIdentity(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	type key struct{ effect, prob uint32 }
	for _, dg := range []uint32{100005066, 100005067, 100005068, 100005014} {
		tbl := a.byDungeon[dg]
		if tbl == nil || len(tbl.Additional) == 0 {
			t.Fatalf("dungeon %d has no additional table", dg)
		}
		branches := map[key]*attunementAdditional{}
		order := []key{}
		for i := range tbl.Additional {
			br := &tbl.Additional[i]
			k := key{br.EffectIndex, br.SelectProb}
			if _, dup := branches[k]; dup {
				t.Fatalf("dungeon %d: (%d,%d) 这一支在源表里出现两次，认行的判据要重写", dg, k.effect, k.prob)
			}
			branches[k] = br
			order = append(order, k)
		}
		seen := map[key]bool{}
		seed := uint32(7)
		for i := 0; i < 400000; i++ {
			tiers, next, err := a.PlanRun(seed, dg, 0)
			if err != nil {
				t.Fatalf("dungeon %d PlanRun: %v", dg, err)
			}
			seed = next
			if tiers.OathTemplate == 0 {
				continue
			}
			k := key{tiers.OathEffect, tiers.OathProb}
			br, ok := branches[k]
			if !ok {
				t.Fatalf("dungeon %d: 带下来的 (effect=%d, prob=%d) 不是源表里的任何一支", dg, k.effect, k.prob)
			}
			// 掷中的条目、份数都必须落在**这一支**上，不能只是档位像。
			if tiers.OathAmount != br.DropCount {
				t.Fatalf("dungeon %d 支 (effect=%d): drop count=%d，源表这支写的是 %d", dg, k.effect, tiers.OathAmount, br.DropCount)
			}
			hit := false
			for _, e := range br.Entries {
				if e.Item == tiers.OathTemplate {
					hit = true
					break
				}
			}
			if !hit {
				t.Fatalf("dungeon %d 支 (effect=%d, prob=%d) 的清单里没有条目 %d", dg, k.effect, k.prob, tiers.OathTemplate)
			}
			seen[k] = true
		}
		if len(seen) != len(order) {
			var missing []string
			for _, k := range order {
				if !seen[k] {
					missing = append(missing, fmt.Sprintf("effect %d/prob %d", k.effect, k.prob))
				}
			}
			t.Fatalf("dungeon %d: 40 万个种子没掷到这些支 ⇒ 那些事件在游戏里一次都不会触发: %v", dg, missing)
		}
	}
}
