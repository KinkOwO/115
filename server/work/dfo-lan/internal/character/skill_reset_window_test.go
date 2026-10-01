package character

import (
	"context"
	"encoding/json"
	"testing"

	"dfolan/internal/game/protocol"
)

// The single VP-panel gate is the third awakening alone; level never
// participates.
func TestVariationUnlockedHelper(t *testing.T) {
	for _, tc := range []struct {
		name      string
		awakening byte
		level     byte
		want      bool
	}{
		{"三觉但不到115", 3, 100, true},
		{"三觉满级", 3, 115, true},
		{"二觉满级", 2, 115, false},
		{"未觉醒", 0, 115, false},
	} {
		st := State{Level: tc.level, Awakening: tc.awakening}
		if got := variationUnlocked(&st); got != tc.want {
			t.Fatalf("%s: variationUnlocked = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Reset 窗口（CMD483 Confirm，mask 位）mask=1 只退源地板之上的 rank：地板
// 保留、SP 回退、槽位经 skillRows 重建。退款单价走 refundCostForState（跳过
// Cost 的资格/前置检查——整树重置时前置技能可能已被先退掉）。
func TestResetWindowRefundsOnlyPurchasedRanks(t *testing.T) {
	s, role, st := autoSkillFixture(t)
	st.Level = 115
	free, e := s.automaticSkills(role, st)
	if e != nil {
		t.Fatal(e)
	}
	known, e := s.knownSkills(role, st, 0)
	if e != nil {
		t.Fatal(e)
	}
	var id uint16
	var floor byte
	var def LearningDefinition
	for candidate, rank := range free {
		d, ok := s.Learning.index[11][candidate]
		if !ok || rank == 0 || rank > 250 {
			continue
		}
		if _, e := d.costForLevel(st, 115, int(rank)+1, known); e != nil {
			continue
		}
		id, floor, def = candidate, rank, d
		break
	}
	if id == 0 {
		t.Skip("夹具里没有可继续加点的自动授予技能")
	}
	target := floor + 2
	st.SkillPoints[0] = 0
	st.LearnedSkills[0] = map[uint16]byte{id: target}
	var want, full int
	for lv := 1; lv <= int(target); lv++ {
		cost, e := def.refundCostForState(st, lv)
		if e != nil {
			t.Fatal(e)
		}
		full += cost
		if lv > int(floor) {
			want += cost
		}
	}
	if want == full {
		t.Skip("该技能的源授予段单价为 0，退多退少无法区分")
	}
	if e = s.applySkillReset(t.Context(), role, &st, 0, ResetOrdinarySkills); e != nil {
		t.Fatal(e)
	}
	if st.SkillPoints[0] != uint16(want) {
		t.Fatalf("SP 退款 = %d，应为 %d（只退源授予之上的 rank）", st.SkillPoints[0], want)
	}
	if st.LearnedSkills[0][id] != floor {
		t.Fatalf("skill%d 应回落到源授予 rank%d: %v", id, floor, st.LearnedSkills[0])
	}
	if len(st.SkillSlots[0]) == 0 && len(st.LearnedSkills[0]) > 0 {
		t.Fatal("Reset 窗口没有重建槽位")
	}
}

// mask=2 清 Enhance 三槽（不动 VP）；mask=4 清 Evolve 五槽并把池恢复为满 5 点；
// mask=7（All）两者都清。Enhance 永不触碰 VP 余额。
func TestResetWindowClearsVariationsByMask(t *testing.T) {
	s, role, st := autoSkillFixture(t)
	st.Awakening = 3
	st.TechniquePoints[0] = 2
	full := SkillVariationState{
		Intensions: []protocol.SkillVariation{{ID: 62, Choice: 1}, {ID: 62, Choice: 2}, {ID: 0, Choice: 3}},
		Options:    []protocol.SkillVariation{{ID: 62, Choice: 1}, {ID: 0, Choice: 3}, {ID: 0, Choice: 3}, {ID: 0, Choice: 3}, {ID: 0, Choice: 3}},
	}
	st.SkillVariations[0] = full

	onlyEnhance := st
	if e := s.applySkillReset(t.Context(), role, &onlyEnhance, 0, ResetEnhance); e != nil {
		t.Fatal(e)
	}
	v := onlyEnhance.SkillVariations[0]
	if len(v.Intensions) != 3 || v.Intensions[0].ID != 0 || v.Intensions[0].Status != 2 {
		t.Fatalf("Enhance 槽未清空: %+v", v.Intensions)
	}
	if v.Options[0].ID != 62 {
		t.Fatal("Enhance 复选框不该动 Evolve")
	}
	if onlyEnhance.TechniquePoints[0] != 2 {
		t.Fatalf("Enhance 复选框改了 VP 余额: %d", onlyEnhance.TechniquePoints[0])
	}

	onlyEvolve := st
	if e := s.applySkillReset(t.Context(), role, &onlyEvolve, 0, ResetEvolve); e != nil {
		t.Fatal(e)
	}
	v = onlyEvolve.SkillVariations[0]
	if len(v.Options) != 5 || v.Options[0].ID != 0 || v.Options[0].Empty != 1 {
		t.Fatalf("Evolve 槽未清空: %+v", v.Options)
	}
	if v.Intensions[0].ID != 62 {
		t.Fatal("Evolve 复选框不该动 Enhance")
	}
	if onlyEvolve.TechniquePoints[0] != 5 {
		t.Fatalf("Evolve 复选框未把池恢复为 5: %d", onlyEvolve.TechniquePoints[0])
	}

	all := st
	if e := s.applySkillReset(t.Context(), role, &all, 0, ResetOrdinarySkills|ResetEnhance|ResetEvolve); e != nil {
		t.Fatal(e)
	}
	v = all.SkillVariations[0]
	if len(v.Intensions) != 3 || v.Intensions[0].ID != 0 || len(v.Options) != 5 || v.Options[0].ID != 0 {
		t.Fatalf("All 未全部清空: %+v", v)
	}
	if all.TechniquePoints[0] != 5 {
		t.Fatalf("All 未把池恢复为 5: %d", all.TechniquePoints[0])
	}
}

// ResetSkills 的 mask==0 在触碰数据库之前就必须拒绝。
func TestResetWindowRefusesEmptyMask(t *testing.T) {
	s := &Service{} // no Store: the mask gate fires first
	role := Character{AccountID: 1, ID: 2, ConfigVersion: "a"}
	if _, _, e := s.ResetSkills(context.Background(), role, "k", 0, 0); e == nil {
		t.Fatal("mask=0 必须拒绝")
	}
	if _, _, e := s.ResetSkills(context.Background(), role, "k", 1, 8); e == nil {
		t.Fatal("合法位之外的 mask 剔除后为 0，必须拒绝")
	}
}

// Reset 窗口响应帧：三觉角色带满宽度空槽（TP 头=5）；未三觉无变体块。
func TestResetWindowResponseCarriesFilledBlocks(t *testing.T) {
	s := &Service{}
	role := func(st State) Character {
		raw, e := json.Marshal(st)
		if e != nil {
			t.Fatal(e)
		}
		return Character{State: raw}
	}
	awakened := State{Awakening: 3, Level: 100, SkillPoints: [2]uint16{120, 120}, TechniquePoints: [2]uint16{5, 5}}
	out, e := s.ResetResponse(role(awakened), 0)
	if e != nil {
		t.Fatal(e)
	}
	filled := awakened
	fillVariationSlots(&filled.SkillVariations[0])
	p, e := protocol.SkillPurchaseSuccess(0, 120, 5, nil)
	if e != nil {
		t.Fatal(e)
	}
	want, e := protocol.SkillPurchaseVariations(p, 0, filled.SkillVariations[0].Intensions, filled.SkillVariations[0].Options)
	if e != nil {
		t.Fatal(e)
	}
	if string(out) != string(want) {
		t.Fatalf("三觉 Reset 响应帧不匹配: %d vs %d 字节", len(out), len(want))
	}
	below := State{Awakening: 2, Level: 115, SkillPoints: [2]uint16{10, 10}}
	out, e = s.ResetResponse(role(below), 0)
	if e != nil {
		t.Fatal(e)
	}
	p2, e := protocol.SkillPurchaseSuccess(0, 10, 0, nil)
	if e != nil {
		t.Fatal(e)
	}
	want2, e := protocol.SkillPurchaseVariations(p2, 0, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if string(out) != string(want2) {
		t.Fatalf("未三觉 Reset 响应帧不匹配: %d vs %d 字节", len(out), len(want2))
	}
}
