package character

import "testing"

// adv0Prereq 让每个技能自身的 [pre required skill] 视为已满足：
// 这里只验证"growtype 0 觉醒让技能可学"，不验证前置链路。
func adv0Prereq(d LearningDefinition) map[uint16]byte {
	known := map[uint16]byte{}
	pre := d.Ints("[pre required skill]")
	for i := 0; i+1 < len(pre); i += 2 {
		known[uint16(pre[i])] = byte(pre[i+1])
	}
	return known
}

// 无转职分支职业（demonic swordman job 9 / creator mage job 10）在源里是
// `[max grow count] 1` + 只有 `[growtype 1]` 一段，角色 Advancement 恒为 0，
// 觉醒（stage 1/2/3）就挂在 growtype 0 上，技能觉醒矩阵的非零值也落在 growtype 列 0。
// ForAwakening 必须读取该列，否则这两个职业的 36 个觉醒技能永远不可学。
// 证据与影响面见 analysis/tasks/next52-adv0-awakening-jobs.md。
func TestAdvancementZeroAwakeningUsesGrowtypeZeroColumn(t *testing.T) {
	l := loadLearningForTest(t)

	// demonicswordman/timestop：三个 stage 的 growtype 0 均为 40，fitness 覆盖 0..4。
	d, ok := l.index[9][257]
	if !ok || d.Path != "skill/demonicswordman/timestop.skl" {
		t.Fatalf("swordman skill 257: %+v", d)
	}
	for stage := 1; stage <= 3; stage++ {
		if !d.ForAwakening(0, stage) {
			t.Fatalf("stage %d: growtype 0 column not read", stage)
		}
		st := State{Level: 60, Advancement: 0, Awakening: byte(stage)}
		if _, err := d.costForState(st, 1, adv0Prereq(d)); err != nil {
			t.Fatalf("stage %d refused at growtype 0: %v", stage, err)
		}
	}
	// stage 0 与负数 advancement 仍必须被拒。
	if d.ForAwakening(0, 0) || d.ForAwakening(-1, 1) {
		t.Fatal("stage 0 / negative advancement accepted")
	}
	if _, err := d.costForState(State{Level: 115, Advancement: 0, Awakening: 0}, 1, nil); err == nil {
		t.Fatal("unawakened character learned an awakening-only skill")
	}

	// creator mage：iceage 在三个 stage 上都有 growtype 0 上限，
	// artefactofcreation 只在 stage 3 上有。
	for _, tc := range []struct {
		id    uint16
		stage int
	}{{258, 1}, {258, 2}, {258, 3}, {407, 3}} {
		dd := l.index[10][tc.id]
		if !dd.ForAwakening(0, tc.stage) {
			t.Fatalf("creator mage skill %d stage %d lost its growtype 0 cap", tc.id, tc.stage)
		}
		if _, err := dd.costForState(State{Level: 115, Advancement: 0, Awakening: byte(tc.stage)}, 1, adv0Prereq(dd)); err != nil {
			t.Fatalf("creator mage skill %d stage %d: %v", tc.id, tc.stage, err)
		}
	}
}

// 放宽 adv 0 的影响面必须收敛：当前源里只有 job 6（thief 的一个普通技能自身带
// growtype 0 上限）、job 9、job 10 的技能矩阵在 growtype 列 0 有非零值，
// 其余 13 个职业的觉醒矩阵该列全为 0，因此不得有任何技能因此泄漏。
func TestGrowtypeZeroAwakeningColumnStaysInsideBranchlessJobs(t *testing.T) {
	l := loadLearningForTest(t)
	seen := map[byte]int{}
	for job, defs := range l.index {
		for _, d := range defs {
			for stage := 1; stage <= 3; stage++ {
				if d.ForAwakening(0, stage) {
					seen[job]++
					break
				}
			}
		}
	}
	for job, n := range seen {
		if job != 6 && job != 9 && job != 10 {
			t.Fatalf("job %d leaked %d skills through the growtype 0 column", job, n)
		}
	}
	if seen[9] == 0 || seen[10] == 0 {
		t.Fatalf("branchless jobs missing from the growtype 0 column: %v", seen)
	}
}
