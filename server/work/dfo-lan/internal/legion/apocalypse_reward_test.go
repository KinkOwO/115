package legion

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestApocalypseRewardFollowsSourceItemAndQuantity(t *testing.T) {
	cat := phaseFixture()
	for _, values := range [][2]float64{{17, 3}, {29, 7}} {
		cat.Operations[0].Reward.Values[0], cat.Operations[0].Reward.Values[1] = values[0], values[1]
		r, err := NormalClearReward(cat, 1)
		if err != nil || r.Template != uint32(values[0]) || r.Amount != uint32(values[1]) {
			t.Fatal(r, err)
		}
		p, err := ApocalypseBasicClearReward(r, 125000)
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
