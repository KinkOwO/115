package character

import "testing"

// [purchase cost] 在当前源里是"按 rank 分段"的单价：rank1 用第一个值，
// rank2 及以后用第二个值。觉醒技能靠 costForLevel 选段，但普通技能也长这样 ——
// atgunner/twingunblade [0 15]、atfighter/crazymount [30 15]、
// knight/perfectguard [30 15]、knight/rosearmor [50 10]。
// 此前 Cost 要求单值，把这四类技能一律拒成 "skill learning fields unavailable"，
// 实机表现就是它们无法加点。
func TestPlainSkillsAcceptRankBandPurchaseCost(t *testing.T) {
	l := loadLearningForTest(t)
	for _, tc := range []struct {
		name             string
		job              byte
		id               uint16
		adv, level, rank int
		want             int
	}{
		{"atgunner_twingunblade_rank1_free", 5, 100, 1, 1, 1, 0},
		{"atfighter_crazymount_rank1", 7, 74, 3, 40, 1, 30},
		{"knight_perfectguard_rank1", 12, 8, 1, 5, 1, 30},
		{"knight_perfectguard_rank2", 12, 8, 1, 10, 2, 15},
		{"knight_rosearmor_rank1", 12, 40, 1, 15, 1, 50},
		{"knight_rosearmor_rank2", 12, 40, 1, 18, 2, 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, ok := l.index[tc.job][tc.id]
			if !ok {
				t.Fatalf("skill %d/%d missing from the learning catalog", tc.job, tc.id)
			}
			known := map[uint16]byte{}
			pre := d.Ints("[pre required skill]")
			for i := 0; i+1 < len(pre); i += 2 {
				known[uint16(pre[i])] = byte(pre[i+1])
			}
			cost, err := d.Cost(tc.level, tc.adv, tc.rank, known)
			if err != nil {
				t.Fatalf("rank %d refused: %v", tc.rank, err)
			}
			if cost != tc.want {
				t.Fatalf("cost=%d want=%d", cost, tc.want)
			}
		})
	}
}

// 分段价只影响"选哪个单价"，不得放开源里的成长上限或其它约束：
// crazymount（[growtype maximum level] = [0 0 0 1 0 0]）只在 growtype 3 上有 1 级。
func TestRankBandPurchaseCostKeepsGrowtypeCaps(t *testing.T) {
	d := loadLearningForTest(t).index[7][74]
	if d.Path != "skill/atfighter/crazymount.skl" {
		t.Fatal("unexpected fixture", d.Path)
	}
	known := map[uint16]byte{14: 1}
	if _, err := d.Cost(40, 3, 2, known); err == nil {
		t.Fatal("rank 2 accepted above the source growtype cap")
	}
	if _, err := d.Cost(40, 2, 1, known); err == nil {
		t.Fatal("growtype without a source cap accepted the skill")
	}
	if _, err := d.Cost(39, 3, 1, known); err == nil {
		t.Fatal("skill accepted below its [required level]")
	}
}
