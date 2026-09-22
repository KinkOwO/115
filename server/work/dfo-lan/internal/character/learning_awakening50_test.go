package character

import (
	"testing"
)

// 实机缺陷（2026-09-21，角色 test-jh，剑魂觉醒成剑圣后 60 级）：加点"流星剑"
// （skill/swordman/meteorsword.skl）时服务端回 "unsupported skill advancement/level"。
//
// 根因是目录缺字段，不是学习逻辑：这些技能在源里 [growtype maximum level] 全 0
// （任何基础 growtype 都不可学），只在 [awakening maximum level] 的觉醒矩阵里给值
// （meteorsword 为 stage1..3 × adv1 = 40）。服务端 ForAwakening 要求
// len([awakening maximum level]) == 3*len([growtype maximum level]) 才会把觉醒上限写回，
// 而 configs/skills.next27.json 是工具还不支持该字段时生成的 —— 3224 行里
// [awakening maximum level] 出现 0 次，于是 1141 个觉醒专属技能一律被判"不可学"。
// 用当前 cmd/skillaudit 重新生成后，逐字段比对只有新增（觉醒相关 1024 行 + variation
// point/awakening 字段）与 5 行"源里确实没有自身 [type]"的修正，没有其它改动。
func TestAwakeningOnlySkillsLearnableAfterAwakening(t *testing.T) {
	l := loadLearningForTest(t)
	d, ok := l.index[0][236]
	if !ok || d.Path != "skill/swordman/meteorsword.skl" {
		t.Fatalf("swordman skill 236: %+v", d)
	}
	// 觉醒矩阵必须已在目录里，否则 ForAwakening 永远为 false。
	if !d.ForAwakening(1, 1) {
		t.Fatal("[awakening maximum level] missing from the learning catalog")
	}
	// 剑魂（advancement 1）觉醒（stage 1）后在 60 级可以学。
	cost, e := d.costForLevel(State{Level: 60, Advancement: 1, Awakening: 1}, 60, 1, map[uint16]byte{})
	if e != nil {
		t.Fatalf("awakened swordman refused an awakening skill: %v", e)
	}
	if cost <= 0 {
		t.Fatalf("cost: %d", cost)
	}
	// 未觉醒时仍不可学：基础 growtype 上限全 0，觉醒矩阵 stage 0 不存在。
	if _, e := d.costForLevel(State{Level: 60, Advancement: 1, Awakening: 0}, 60, 1, map[uint16]byte{}); e == nil {
		t.Fatal("unawakened character learned an awakening-only skill")
	}
	// 觉醒等级不够时也不可学（stage 由 50/75/100 级门槛推进）。
	if _, e := d.costForLevel(State{Level: 49, Advancement: 1, Awakening: 1}, 49, 1, map[uint16]byte{}); e == nil {
		t.Fatal("skill learned below its [required level]")
	}
}

// 目录里所有"基础 growtype 上限全 0"的技能都必须带觉醒矩阵，否则它们永远不可学。
func TestAwakeningOnlySkillsCarryAwakeningCaps(t *testing.T) {
	l := loadLearningForTest(t)
	base := 0
	checked := 0
	for _, d := range l.index[0] {
		caps := d.Ints("[growtype maximum level]")
		if len(caps) == 0 {
			continue
		}
		allZero := true
		for _, c := range caps {
			if c > 0 {
				allZero = false
				break
			}
		}
		if !allZero {
			continue
		}
		base++
		if len(d.Ints("[awakening maximum level]")) == 3*len(caps) {
			checked++
		}
	}
	if base == 0 {
		t.Fatal("no awakening-only skills in the swordman catalog")
	}
	if checked == 0 {
		t.Fatalf("%d swordman skills have all-zero growtype caps but no awakening matrix", base)
	}
}
