package legion

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

// N2252 是客户端直读的固定长度帧（1424FE3F0 家族读 7772B）。长度或行步长
// 写错不会报错，只会让翻牌界面少一行/错行，所以这里按抓包逐项钉住。
func TestApocalypseBasicClearRewardLayout(t *testing.T) {
	table, err := ApocalypseRewardFor(0)
	if err != nil {
		t.Fatal(err)
	}
	proof := table.Show[0].Template
	items := table.Grant[1:]
	raw, err := ApocalypseBasicClearReward(proof, items, nil, 60000)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 7772 {
		t.Fatalf("N2252 length %d, want 7772", len(raw))
	}
	// 行 0 是通关证明（表里的 basic 那件，数量 1）。
	if got := binary.LittleEndian.Uint32(raw[0:]); got != proof {
		t.Fatalf("row0 template %d, want %d", got, proof)
	}
	if got := binary.LittleEndian.Uint32(raw[4:]); got != 1 {
		t.Fatalf("row0 value %d, want 1", got)
	}
	// 行 0 在 @0（40B 布局）；@40 **必须保持全零**（抓包两个难度都是 0），
	// 行 1 起落在 @1600 + 44*i。
	// 难度1 抓包：@0=10421367x1、@40=0、@1600=10421371x20、@1644=10421373x96、
	// @1688=10362429x240、@1732=10415196x5。
	if got := binary.LittleEndian.Uint32(raw[40:]); got != 0 {
		t.Fatalf("@40 template = %d, want 0 (the capture leaves it empty)", got)
	}
	if got := binary.LittleEndian.Uint32(raw[44:]); got != 0 {
		t.Fatalf("@44 value = %d, want 0", got)
	}
	want := []struct {
		off int
		tpl uint32
		val uint32
	}{
		{1600, ApocalypseRewardMaterialA, 20},
		{1644, ApocalypseRewardMaterialB, 96},
		{1688, ApocalypseRewardAbyssTicket, 240},
		{1732, apocalypseRewardFlagRow1, 5},
	}
	for i, w := range want {
		if got := binary.LittleEndian.Uint32(raw[w.off:]); got != w.tpl {
			t.Errorf("row %d @%d template = %d, want %d", i+1, w.off, got, w.tpl)
		}
		if got := binary.LittleEndian.Uint32(raw[w.off+4:]); got != w.val {
			t.Errorf("row %d @%d value = %d, want %d", i+1, w.off, got, w.val)
		}
	}
	// 最后一行之后必须是零（客户端会继续读下一行，非零会被当成奖励）。
	lastEnd := 1732 + 44
	for i := lastEnd; i < 7760; i++ {
		if raw[i] != 0 {
			t.Fatalf("byte %d after the last row = %#x, want 0", i, raw[i])
		}
	}
	// @7760 = 通关耗时毫秒（本用例传 60000）。
	if got := binary.LittleEndian.Uint64(raw[7760:]); got != 60000 {
		t.Fatalf("@7760 elapsed = %d, want 60000", got)
	}
}

// 难度2 抓包行（normal 之外的第二个已取证难度），同样逐项核对。
func TestApocalypseBasicClearRewardExpertRows(t *testing.T) {
	table, err := ApocalypseRewardFor(1)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ApocalypseBasicClearReward(table.Show[0].Template, table.Grant[1:], nil, 60000)
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		off int
		tpl uint32
		val uint32
	}{
		{0, ApocalypseRewardProofExpert, 1},
		{1600, ApocalypseRewardMaterialA, 30},
		{1644, apocalypseRewardMaterialExpertA, 10},
		{1688, ApocalypseRewardMaterialB, 96},
		{1732, apocalypseRewardMaterialExpertB, 24},
		{1776, ApocalypseRewardAbyssTicket, 260},
		{1820, apocalypseRewardFlagRow1, 15},
		{1864, ApocalypseRewardUnknownFlag, 1},
	}
	for _, w := range want {
		if got := binary.LittleEndian.Uint32(raw[w.off:]); got != w.tpl {
			t.Errorf("@%d template = %d, want %d", w.off, got, w.tpl)
		}
		if got := binary.LittleEndian.Uint32(raw[w.off+4:]); got != w.val {
			t.Errorf("@%d value = %d, want %d", w.off, got, w.val)
		}
	}
	// @40 与 @44 必须为零（参考难度2 @212.28s 的同一位置就是 0）。
	if got := binary.LittleEndian.Uint32(raw[40:]); got != 0 {
		t.Fatalf("expert @40 template = %d, want 0", got)
	}
	if got := binary.LittleEndian.Uint32(raw[44:]); got != 0 {
		t.Fatalf("expert @44 value = %d, want 0", got)
	}
}

// N2253 在末世录里是 2405B 全零占位（抓包 2408B 载荷、非零字节数 0）。
func TestApocalypseAdditionalClearRewardIsPlaceholder(t *testing.T) {
	raw, err := ApocalypseAdditionalClearReward()
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 2405 {
		t.Fatalf("N2253 length %d, want 2405", len(raw))
	}
	for i, b := range raw {
		if b != 0 {
			t.Fatalf("byte %d = %#x, want the zero placeholder the capture shows", i, b)
		}
	}
}

// 只有表里声明的难度键能拿到奖励；未声明的必须明确报错而不是给一份空的。
func TestApocalypseRewardForDeclaredDifficulties(t *testing.T) {
	for _, choice := range []byte{0, 1, 2, 4} {
		table, err := ApocalypseRewardFor(choice)
		if err != nil {
			t.Fatalf("choice %d: %v", choice, err)
		}
		if len(table.Show) == 0 || len(table.Grant) == 0 || table.Label == "" {
			t.Fatalf("choice %d has an empty reward table: %+v", choice, table)
		}
		if table.Show[0].Amount != 1 {
			t.Fatalf("choice %d proof amount = %d, want 1", choice, table.Show[0].Amount)
		}
	}
	for _, choice := range []byte{3, 5, 0xff} {
		if _, err := ApocalypseRewardFor(choice); err == nil {
			t.Fatalf("undeclared difficulty %#x was accepted", choice)
		}
	}
	// 只有难度1/难度2 有抓包实证；master/match 的固定项来自源表 basic +
	// 同族外推，必须能被日志区分出来（RandomGearSlots 记录抓包观察到的行数，
	// master/match 为 0 表示没有样本）。
	for _, choice := range []byte{2, 4} {
		table, _ := ApocalypseRewardFor(choice)
		if table.RandomGearSlots != 0 {
			t.Fatalf("choice %d claims %d captured gear rows without evidence", choice, table.RandomGearSlots)
		}
	}
	for _, choice := range []byte{0, 1} {
		table, _ := ApocalypseRewardFor(choice)
		if table.RandomGearSlots == 0 {
			t.Fatalf("choice %d lost its captured gear-row count", choice)
		}
	}
	if labels := ApocalypseRewardLabels(); len(labels) != 4 {
		t.Fatalf("reward labels = %v, want 4", labels)
	}
}

// 随机装备行：从**分档池**里按 15/35/50 抽（业主 2026-10-08 方案 A，新建池子）。
//
// 池子形状 = configs/apocalypse-flip-gear.generated.json：
//
//	rarity 2 = 魔法  15%
//	rarity 3 = 神器  35%
//	rarity 4 = 史诗  50%
func TestApocalypseFlipGearRoll(t *testing.T) {
	pool := ApocalypseFlipGearPool{}
	for _, tier := range []struct {
		rarity, weight int
		base           uint32
	}{
		{2, 15, 100000}, {3, 35, 200000}, {4, 50, 300000},
	} {
		var ids []uint32
		for i := uint32(0); i < 20; i++ {
			ids = append(ids, tier.base+i)
		}
		pool.Tiers = append(pool.Tiers, ApocalypseFlipGearTier{
			Rarity: tier.rarity, Weight: tier.weight, Templates: ids,
		})
	}
	allowed := map[uint32]bool{}
	for _, tier := range pool.Tiers {
		for _, id := range tier.Templates {
			allowed[id] = true
		}
	}
	for i := 0; i < 40; i++ {
		got := ApocalypseRollFlipGearTiers(pool, 2)
		if len(got) != 2 {
			t.Fatalf("rolled %d of 2: %v", len(got), got)
		}
		for _, id := range got {
			if !allowed[id] {
				t.Fatalf("rolled %d, which is outside the pool", id)
			}
		}
		if got[0] == got[1] {
			t.Fatalf("roll repeated a template: %v", got)
		}
	}
	// 空池 / 零需求不得产出。
	if got := ApocalypseRollFlipGearTiers(ApocalypseFlipGearPool{}, 3); got != nil {
		t.Fatalf("an empty pool rolled %v", got)
	}
	if got := ApocalypseRollFlipGearTiers(pool, 0); got != nil {
		t.Fatalf("zero slots rolled %v", got)
	}
	// 需求多于池子大小时只抽得到池子大小（池内 60 件）。
	if got := ApocalypseRollFlipGearTiers(pool, 100); len(got) != 60 {
		t.Fatalf("roll of 100 from a 60-item pool = %d", len(got))
	}
	// 行数按难度取值：抓包难度1=8、难度2=7。
	if ApocalypseFlipGearSlots(0) != 8 || ApocalypseFlipGearSlots(1) != 7 || ApocalypseFlipGearSlots(4) != 7 {
		t.Fatalf("gear slots = %d/%d/%d", ApocalypseFlipGearSlots(0), ApocalypseFlipGearSlots(1), ApocalypseFlipGearSlots(4))
	}
}

// 三档占比必须落在 15/35/50（各 ±5 个百分点），且三档都真的出现过。
func TestApocalypseFlipGearTierDistribution(t *testing.T) {
	pool := ApocalypseFlipGearPool{}
	tierOf := map[uint32]int{}
	for _, tier := range []struct {
		rarity, weight int
		base           uint32
	}{
		{2, 15, 100000}, {3, 35, 200000}, {4, 50, 300000},
	} {
		var ids []uint32
		for i := uint32(0); i < 60; i++ {
			ids = append(ids, tier.base+i)
			tierOf[tier.base+i] = tier.rarity
		}
		pool.Tiers = append(pool.Tiers, ApocalypseFlipGearTier{
			Rarity: tier.rarity, Weight: tier.weight, Templates: ids,
		})
	}
	counts := map[int]int{}
	const rounds = 4000
	for i := 0; i < rounds; i++ {
		// 每轮抽 1 件 ⇒ 每件的档位分布都是独立掷出来的。
		out := ApocalypseRollFlipGearTiers(pool, 1)
		if len(out) != 1 {
			t.Fatalf("round %d rolled %d", i, len(out))
		}
		counts[tierOf[out[0]]]++
	}
	for rarity, want := range map[int]int{2: 15, 3: 35, 4: 50} {
		got := float64(counts[rarity]) * 100 / rounds
		if diff := got - float64(want); diff > 5 || diff < -5 {
			t.Errorf("rarity %d share = %.1f%%, want %d%% (±5)", rarity, got, want)
		}
	}
	for _, rarity := range []int{2, 3, 4} {
		if counts[rarity] == 0 {
			t.Errorf("rarity %d never rolled (%v)", rarity, counts)
		}
	}
}

// 某档取空后，权重按条件概率分摊给剩余档位（而不是发不出来）。
func TestApocalypseFlipGearTierRenormalises(t *testing.T) {
	// 只留史诗(4, 50%) 与神器(3, 35%) 各 1 件 ⇒ 史诗占 50/(50+35)=58.8%。
	pool := ApocalypseFlipGearPool{}
	pool.Tiers = append(pool.Tiers,
		ApocalypseFlipGearTier{Rarity: 3, Weight: 35, Templates: []uint32{200001}},
		ApocalypseFlipGearTier{Rarity: 4, Weight: 50, Templates: []uint32{300001}},
	)
	epic := 0
	const rounds = 3000
	for i := 0; i < rounds; i++ {
		out := ApocalypseRollFlipGearTiers(pool, 1)
		if len(out) != 1 {
			t.Fatalf("round %d rolled %d", i, len(out))
		}
		if out[0] == 300001 {
			epic++
		}
	}
	share := float64(epic) * 100 / rounds
	if share < 53 || share > 65 {
		t.Fatalf("epic share = %.1f%%, want ~58.8%%", share)
	}
	// 某档为空时不参与，权重全给另一档。
	single := ApocalypseFlipGearPool{}
	single.Tiers = append(single.Tiers,
		ApocalypseFlipGearTier{Rarity: 2, Weight: 15, Templates: nil},
		ApocalypseFlipGearTier{Rarity: 4, Weight: 50, Templates: []uint32{300001, 300002}},
	)
	for i := 0; i < 20; i++ {
		out := ApocalypseRollFlipGearTiers(single, 1)
		if len(out) != 1 || out[0] != 300001 && out[0] != 300002 {
			t.Fatalf("empty tier leaked: %v", out)
		}
	}
}

func TestApocalypseRewardFollowsSourceItemAndQuantity(t *testing.T) {
	cat := phaseFixture()
	for _, values := range [][2]float64{{17, 3}, {29, 7}} {
		cat.Operations[0].Reward.Values[0], cat.Operations[0].Reward.Values[1] = values[0], values[1]
		r, err := NormalClearReward(cat, 1)
		if err != nil || r.Template != uint32(values[0]) || r.Amount != uint32(values[1]) {
			t.Fatal(r, err)
		}
		p, err := ApocalypseSourceBasicClearReward(r, 125000)
		if err != nil || len(p) != 7772 || binary.LittleEndian.Uint32(p) != r.Template || binary.LittleEndian.Uint32(p[4:]) != r.Amount || binary.LittleEndian.Uint64(p[7760:]) != 125000 {
			t.Fatal("native standard item/time fields lost", err)
		}
	}
	if len(ApocalypseNoAdditionalReward()) != 2405 || ApocalypseNoAdditionalReward()[0] != 0 {
		t.Fatal("native disabled-additional branch lost")
	}
}

func TestApocalypseBasicCOSRulesAndNativeCapacity(t *testing.T) {
	source := strings.Repeat("b", 64)
	for _, change := range []struct{ id, amount int64 }{{17, 3}, {29, 7}} {
		contents := catalog.LegionContents{Source: pvf.ArchiveSnapshot{Checksum: source}, SHA256: strings.Repeat("c", 64), Contents: map[string]catalog.LegionContent{"Apocalypse": {Rewards: []catalog.LegionRewardRow{{Difficulty: "normal", Kind: "basic", Values: []int64{1000000, change.id, change.amount}}, {Difficulty: "expert", Kind: "pcroom", Values: []int64{1000000, 99, 5}}}}}}
		item, err := GuaranteedBasicClearReward(contents, source, "normal")
		if err != nil || item.Template != uint32(change.id) || item.Amount != uint32(change.amount) {
			t.Fatal(item, err)
		}
		if _, err = GuaranteedBasicClearReward(contents, source, "expert"); err == nil {
			t.Fatal("premium reward substituted for missing basic reward")
		}
		p, err := ApocalypseClearRewardItems(ClearReward{Template: 3, Amount: 1}, []ClearReward{item}, 125000)
		if err != nil || binary.LittleEndian.Uint32(p[1600:]) != item.Template || binary.LittleEndian.Uint32(p[1604:]) != item.Amount || binary.LittleEndian.Uint64(p[7760:]) != 125000 {
			t.Fatal(err)
		}
		for _, count := range []int{35, 36} {
			items := make([]ClearReward, count)
			for i := range items {
				items[i] = item
			}
			_, err = ApocalypseClearRewardItems(item, items, 1)
			if (err != nil) != (count > 35) {
				t.Fatal("native item capacity lost", count, err)
			}
		}
	}
}

func TestApocalypseRewardRefusesUnknownSourceBranches(t *testing.T) {
	for _, values := range [][]float64{{1, 1}, {0, 1, 1000000, 1, 1}, {1, 0, 1000000, 1, 1}, {1, 1, 500000, 1, 1}, {1, 1, 1000000, 2, 1}, {1, 1, 1000000, 1, 0}, {1, 0.5, 1000000, 1, 1}, {1, math.NaN(), 1000000, 1, 1}} {
		cat := phaseFixture()
		cat.Operations[0].Reward.Values = values
		if _, err := NormalClearReward(cat, 1); err == nil {
			t.Fatal("unknown reward branch accepted", values)
		}
	}
	cat := phaseFixture()
	cat.Operations[0].Reward.Label = "expert"
	if _, err := NormalClearReward(cat, 1); err == nil {
		t.Fatal("expert reward interpreted as normal")
	}
}

func TestApocalypseRewardOutcomeMatchesNativeChoiceClassifier(t *testing.T) {
	for choice, want := range []uint32{3, 5, 5, 3, 3} {
		if got := ApocalypseRewardOutcome(byte(choice)); got != want {
			t.Fatal(choice, got, want)
		}
	}
}

func TestGuaranteedExpertRewardUsesCurrentSourceTuple(t *testing.T) {
	cat := phaseFixture()
	op := &cat.Operations[1]
	op.Reward.Label = "expert"
	op.Reward.Values = []float64{29, 7, 1000000, 2, 1}
	r, err := GuaranteedClearReward(cat, 2)
	if err != nil || r.Template != 29 || r.Amount != 7 || r.Label != "expert" {
		t.Fatal(r, err)
	}
	for _, change := range []func(){func() { op.Reward.Values[3] = 1 }, func() { op.Reward.Label = "unknown" }, func() { op.Reward.Values[2] = 500000 }} {
		op.Reward.Label = "expert"
		op.Reward.Values = []float64{29, 7, 1000000, 2, 1}
		change()
		if _, err := GuaranteedClearReward(cat, 2); err == nil {
			t.Fatal("unclosed expert tuple accepted")
		}
	}
}
