package catalog

import (
	"math/rand"
	"reflect"
	"testing"

	"dfolan/internal/catalog/pvf"
)

func fixedAvatarCells(ids []uint32) []pvf.Token {
	cells := []pvf.Token{cellTag("[booster info]")}
	for _, id := range ids {
		cells = append(cells, cellTag("[avatar]"), cellNum(1), cellNum(int32(id)), cellNum(1000), cellNum(1), cellNum(1), cellNum(0), cellTag("[/avatar]"))
	}
	return append(cells, cellTag("[/booster info]"))
}

func TestBoosterFixedAvatarRewards(t *testing.T) {
	// Native Egypt look box 50041807: eight independent guaranteed pools.
	ids := []uint32{508550408, 508560408, 508570408, 508500408, 508510408, 508520408, 508530408, 508540408}
	pools := parseBoosterInfo(fixedAvatarCells(ids))
	if len(pools) != len(ids) {
		t.Fatalf("got %d pools, want all eight", len(pools))
	}
	for i, pool := range pools {
		want := []BoosterRewardCandidate{{Template: ids[i], Weight: 1000, Count: 1}}
		if pool.DrawCount != 1 || !reflect.DeepEqual(pool.Candidates, want) || !reflect.DeepEqual(pool.Pick(rand.New(rand.NewSource(1))), want) {
			t.Fatalf("pool %d: %+v, want %+v", i, pool, want)
		}
	}
}

func TestBoosterFixedAvatarRejectsPartialOrUnsupportedRecords(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func([]pvf.Token) []pvf.Token
	}{
		{"zero count", func(c []pvf.Token) []pvf.Token { c[13].Value = 0; return c }},
		{"zero weight", func(c []pvf.Token) []pvf.Token { c[12].Value = 0; return c }},
		{"missing closing tag", func(c []pvf.Token) []pvf.Token { return c[:16] }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if pools := parseBoosterInfo(tc.edit(fixedAvatarCells([]uint32{508550408, 508560408}))); len(pools) != 0 {
				t.Fatalf("invalid second record must not grant a partial outfit: %+v", pools)
			}
		})
	}
}

func TestBoosterUnknownAvatarVariantPreservesOtherRewards(t *testing.T) {
	// Native package_challenger2008sm has an unsupported 30/0 suffix.
	// It must not suppress the equipment/stackable rewards already supported.
	cells := fixedAvatarCells([]uint32{40808})
	cells[6].Value = 30
	cells = append(cells[:len(cells)-1], cellTag("[equipment]"), cellNum(1), cellNum(26151), cellNum(1000), cellNum(1), cellTag("[/equipment]"), cellTag("[/booster info]"))
	pools := parseBoosterInfo(cells)
	want := []BoosterRewardPool{{DrawCount: 1, Candidates: []BoosterRewardCandidate{{Template: 26151, Weight: 1000, Count: 1}}}}
	if !reflect.DeepEqual(pools, want) {
		t.Fatalf("unknown avatar variant changed existing package rewards: %+v", pools)
	}
}

func TestBoosterFixedAvatarMultiRecordRejectsUnsupportedTail(t *testing.T) {
	cells := []pvf.Token{cellTag("[booster info]"), cellTag("[avatar]"), cellNum(1), cellNum(508550408), cellNum(1000), cellNum(1), cellNum(1), cellNum(0), cellNum(508560408), cellNum(1000), cellNum(1), cellNum(30), cellNum(0), cellTag("[/avatar]"), cellTag("[/booster info]")}
	if pools := parseBoosterInfo(cells); len(pools) != 0 {
		t.Fatalf("recognized pool must not grant partial records: %+v", pools)
	}
}

func TestBoosterAvatarOrdinaryTriplesRemainSupported(t *testing.T) {
	cells := []pvf.Token{cellTag("[booster info]"), cellTag("[avatar]"), cellNum(1), cellNum(508550408), cellNum(1000), cellNum(1), cellTag("[/avatar]"), cellTag("[/booster info]")}
	pools := parseBoosterInfo(cells)
	if len(pools) != 1 || pools[0].Candidates[0] != (BoosterRewardCandidate{Template: 508550408, Weight: 1000, Count: 1}) {
		t.Fatalf("ordinary avatar triple changed: %+v", pools)
	}
}

func TestBoosterAvatarFiveOrdinaryCandidatesRemainSupported(t *testing.T) {
	cells := []pvf.Token{cellTag("[booster info]"), cellTag("[avatar]"), cellNum(1)}
	var want []BoosterRewardCandidate
	for i := int32(0); i < 5; i++ {
		cells = append(cells, cellNum(508550408+i), cellNum(1000+i), cellNum(1))
		want = append(want, BoosterRewardCandidate{Template: uint32(508550408 + i), Weight: uint32(1000 + i), Count: 1})
	}
	cells = append(cells, cellTag("[/avatar]"), cellTag("[/booster info]"))
	pools := parseBoosterInfo(cells)
	if len(pools) != 1 || pools[0].DrawCount != 1 || !reflect.DeepEqual(pools[0].Candidates, want) {
		t.Fatalf("ordinary five-candidate pool mistaken for extended avatar records: %+v", pools)
	}
}

func TestBoosterFixedAvatarNativeArchive(t *testing.T) {
	a := openChannelArchive(t)
	for _, path := range []string{
		"stackable/dfo/cash/2018/1218/egypt/look/egypt_trade_ava_pr1.stk",
		"stackable/dfo/cash/2019/1119/2ndawake/look/grow1/2ndawake_trade_ava_pr4.stk",
		"stackable/dfo/cash/2020/0804/look/grow1/2ndawake_trade_ava_pr2.stk",
	} {
		t.Run(path, func(t *testing.T) {
			cells, err := a.Tokens(path)
			if err != nil {
				t.Fatal(err)
			}
			pools := parseBoosterInfo(cells)
			if len(pools) != 8 {
				t.Fatalf("native box has %d pools, want 8", len(pools))
			}
			for _, p := range pools {
				if p.DrawCount != 1 || len(p.Candidates) != 1 || p.Candidates[0].Template < 500000000 || p.Candidates[0].Weight != 1000 || p.Candidates[0].Count != 1 {
					t.Fatalf("incorrect native avatar reward: %+v", p)
				}
			}
		})
	}
}
