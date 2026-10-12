package loot

// 活动 1093「额外掉落奖励」**固定掉落**模式的语义守卫（业主 2026-10-13 定案）。
//
// 口径：`ExtraDropRewards.FixedPerRun > 0` 时，能量不走稀有度表，改成
// **每局固定给一次**，触发点是**深渊（调律/天平）的结算那一步**
//（`Session.Death` 里 `attunementPending` 为真的那一帧），
// 而不是「本局第一只掉出东西的怪」——后者只是个代理信号，语义上不稳定。
//
// 这里三条测试各自钉一个方向：
//  1. 结算那一步**确实**发，且只发一份、数量正确；
//  2. **非结算**的死亡（哪怕它自己掉了一堆东西）**不发**、也不消耗「本局已发」标记；
//  3. `FixedPerRun == 0` 时回落官方稀有度表，固定口径的标记**不出现**。
//
// 挂具复用 `border_plan_test.go` 的 `borderTestRun`：它已经给出
// 「带调律表的深渊副本 100005068 + 源领主 123（Rank 3）」这个最小现场。

import (
	"dfolan/internal/extradrop"
	"dfolan/internal/game/protocol"
	"strings"
	"testing"
)

// fixedTestEnergyItem 是本套测试用的能量物品号（与源里的 `Mystical Energy` 相同）。
const fixedTestEnergyItem uint32 = 590015808

const fixedTestScanPrefix = "extradrop1093_scan:"

// fixedTestExtraDrop 造一份「只认 100005068、固定给 perRun 个」的 1093 掉落侧。
//
// `TargetDungeonIDs` 用**副本 id**（与生产同一条判据，见 `ExtraDropRewards.Matches`）；
// `Groups` 里放一档就够 —— `EnergyItem()` 取的是「第一张能找到的非零 `Item`」。
func fixedTestExtraDrop(perRun uint32) *ExtraDropRewards {
	return &ExtraDropRewards{
		Catalog: &extradrop.Catalog{
			TargetDungeonIDs: []uint32{100005068},
			Groups: []extradrop.Group{{
				Type:  extradrop.GroupGetItem,
				Tiers: []extradrop.RewardTier{{Rarity: "rare", Item: fixedTestEnergyItem, Amount: 1}},
			}},
		},
		FixedPerRun: perRun,
	}
}

// fixedTestEnergyCount 数**这一次**死亡发了几份能量、一共多少个。
//
// 走 `s.Objects`（内部表示）而不是解析线上行：`SceneDrop.Item` 是编好的字节，
// 回读要再引一个解码器，而 `Drop.Award` 本来就是真值。
func fixedTestEnergyCount(s *Session, rows []protocol.SceneDrop) (copies int, total uint32) {
	for _, row := range rows {
		drop, ok := s.Objects[row.Object]
		if !ok {
			continue
		}
		if drop.Award.Template == fixedTestEnergyItem {
			copies++
			total += drop.Award.Amount
		}
	}
	return copies, total
}

func fixedTestHasMarker(s *Session, entity uint16, want string) bool {
	for _, k := range s.Skipped[entity] {
		if k == want {
			return true
		}
	}
	return false
}

func fixedTestHasScanMarker(s *Session, entity uint16) bool {
	for _, k := range s.Skipped[entity] {
		if strings.HasPrefix(k, fixedTestScanPrefix) {
			return true
		}
	}
	return false
}

// 结算那一步发，且只有一份；重放不重复发。
func TestExtraDropFixedEnergyIsPaidAtTheAbyssSettlement(t *testing.T) {
	s, d := borderTestRun(t, 4)
	s.ExtraDrop = fixedTestExtraDrop(10)
	d.Loaded = true

	rows, err := s.Death(d, 7) // entity 7 = 源领主（Template 123 == Definition.SourceBoss，Rank 3）
	if err != nil {
		t.Fatal(err)
	}
	if copies, total := fixedTestEnergyCount(s, rows); copies != 1 || total != 10 {
		t.Fatalf("结算应发 1 份共 10 个，实际 copies=%d total=%d", copies, total)
	}
	if !fixedTestHasMarker(s, 7, "extradrop1093_fixed:10") {
		t.Fatalf("缺少固定口径标记，skipped=%v", s.Skipped[7])
	}
	if !s.extraDropFixedPaid {
		t.Fatal("结算后应置「本局已发」")
	}

	// 重放同一次死亡必须走缓存、不重复发。
	again, err := s.Death(d, 7)
	if err != nil {
		t.Fatal(err)
	}
	if copies, _ := fixedTestEnergyCount(s, again); copies != 1 {
		t.Fatalf("重放不得重复发，实际 copies=%d", copies)
	}
}

// 非结算的死亡：即使它自己掉了一堆东西，也**不发**、也**不消耗**标记。
//
// 造场景：奥德赛货币（必掉）⇒ 这次死亡 `result.Awards` **非空** ——
// 正是旧实现（`len(result.Awards) > 0`）会误发固定能量的形态。
func TestExtraDropFixedEnergyIgnoresANonSettlementDeath(t *testing.T) {
	s, d := borderTestRun(t, 4)
	s.ExtraDrop = fixedTestExtraDrop(10)
	// ★ 关掉礼盒目录：`OpenRewardBoxes` 会把「不是盒子、不是容器、也不在物品表里」的奖励
	// **直接丢掉**（`attunement_empty_prize_*`）。本测试只关心 1093 的触发点，
	// 让掉落原样落地最省事 —— 于是这次死亡的 awards 一定非空。
	s.RewardBoxes = nil
	// 必掉的货币：Denominator==10000 且 rank 全 10000 ⇒ Roll 必命中。
	s.Currency = &OdysseyCurrency{
		Source:      "test", // 必须等于 s.Catalog.Source.Checksum，否则会返回 source mismatch
		Denominator: 10000,
		Rates:       [4]uint32{10000, 10000, 10000, 10000},
		Templates:   [4]uint32{10418035, 10418035, 10418035, 10418035},
	}
	d.Definition.Odyssey = true
	d.Monsters = append(d.Monsters, protocol.DungeonMonster{Entity: 8, Template: 999, Rank: 1, Level: 1})
	d.Dead[8] = true
	d.Loaded = true

	rows, err := s.Death(d, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("前置条件不成立：这次死亡本应有掉落，否则测不出触发点")
	}
	if copies, _ := fixedTestEnergyCount(s, rows); copies != 0 {
		t.Fatalf("非结算死亡不该发能量，实际 copies=%d", copies)
	}
	if s.extraDropFixedPaid {
		t.Fatal("非结算死亡不该消耗「本局已发」标记")
	}

	// 随后打源领主：能量应当**在这里**才发出来。
	bossRows, err := s.Death(d, 7)
	if err != nil {
		t.Fatal(err)
	}
	if copies, total := fixedTestEnergyCount(s, bossRows); copies != 1 || total != 10 {
		t.Fatalf("结算应发 1 份共 10 个，实际 copies=%d total=%d", copies, total)
	}
}

// FixedPerRun == 0：回落官方稀有度表，固定口径的标记不出现。
func TestExtraDropFixedEnergyOffFallsBackToTheRarityTable(t *testing.T) {
	s, d := borderTestRun(t, 4)
	s.ExtraDrop = fixedTestExtraDrop(0)
	d.Loaded = true

	if _, err := s.Death(d, 7); err != nil {
		t.Fatal(err)
	}
	if s.extraDropFixedPaid {
		t.Fatal("关掉固定模式后不该置标记")
	}
	if fixedTestHasMarker(s, 7, "extradrop1093_fixed:10") {
		t.Fatal("关掉固定模式后不该出现固定口径标记")
	}
	// 稀有度表那条路仍然跑（会留下 scan 诊断标记）。
	if !fixedTestHasScanMarker(s, 7) {
		t.Fatalf("应走稀有度表并留下 scan 标记，skipped=%v", s.Skipped[7])
	}
}
