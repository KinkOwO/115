package character

import (
	"encoding/json"
	"testing"

	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
)

func emptyRows(n int) []protocol.SkillVariation {
	rows := make([]protocol.SkillVariation, n)
	for i := range rows {
		rows[i] = protocol.SkillVariation{ID: 0, Choice: 3}
	}
	return rows
}

// CMD483 的 Reset 只该退回"玩家用 SP 买过的等级"。初始技能与
// [growtype N] 自动授予的等级是白给的，一起退回去等于凭空发 SP。
// 例：Spatha Noctis（62）在 job11/adv2 由自动授予到 rank1，Reset 后
// 它必须原样留 rank1、SP 不变。
func TestResetAutoSetKeepsSourceFloorWithoutRefund(t *testing.T) {
	s, role, st := autoSkillFixture(t)
	st.SkillPoints[0] = 0
	st.LearnedSkills[0] = map[uint16]byte{62: 1}
	st.SkillSlots[0] = map[uint16]uint16{62: 0}

	if e := s.resetAutoState(t.Context(), role, &st, 0, 1); e != nil {
		t.Fatal(e)
	}
	if st.SkillPoints[0] != 0 {
		t.Fatalf("白给的等级被退成 SP: %d", st.SkillPoints[0])
	}
	if st.LearnedSkills[0][62] != 1 {
		t.Fatalf("源授予的 rank1 被清掉: %v", st.LearnedSkills[0])
	}
	if len(st.SkillSlots[0]) != 0 {
		t.Fatalf("旧槽位没清，新推荐技能会挤到 14+ palette: %v", st.SkillSlots[0])
	}
}

// 超过源授予的部分才是玩家花 SP 买的：Reset 必须只退这一段的钱。
// 夹具里挑一个"有自动授予、还能继续加点"的技能，避免写死某个技能的等级上限。
func TestResetAutoSetRefundsOnlyPurchasedRanks(t *testing.T) {
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
	known, e = s.knownSkills(role, st, 0)
	if e != nil {
		t.Fatal(e)
	}
	var want, full int
	for lv := 1; lv <= int(target); lv++ {
		cost, e := def.costForLevel(st, 115, lv, known)
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
	if e = s.resetAutoState(t.Context(), role, &st, 0, 1); e != nil {
		t.Fatal(e)
	}
	if st.SkillPoints[0] != uint16(want) {
		t.Fatalf("SP 退款 = %d，应为 %d（只退源授予之上的 rank）", st.SkillPoints[0], want)
	}
	if st.LearnedSkills[0][id] != floor {
		t.Fatalf("skill%d 应回落到源授予 rank%d: %v", id, floor, st.LearnedSkills[0])
	}
	t.Logf("skill%d floor=%d target=%d refund=%d（全额 %d）", id, floor, target, want, full)
}

// 再次 Reset 必须幂等：SP 不再变、整份 state 也不再变（含 VP 块）。
func TestResetAutoSetIsIdempotent(t *testing.T) {
	s, role, st := autoSkillFixture(t)
	st.Awakening = 3
	st.SkillPoints[0] = 120
	st.LearnedSkills[0] = map[uint16]byte{62: 1}
	st.SkillVariations[0] = SkillVariationState{
		Intensions: []protocol.SkillVariation{{ID: 62, Choice: 1}, {ID: 0, Choice: 3}, {ID: 0, Choice: 3}},
		Options:    emptyRows(5),
	}
	if e := s.resetAutoState(t.Context(), role, &st, 0, 7); e != nil {
		t.Fatal(e)
	}
	first, e := json.Marshal(st)
	if e != nil {
		t.Fatal(e)
	}
	points := st.SkillPoints[0]
	if e = s.resetAutoState(t.Context(), role, &st, 0, 7); e != nil {
		t.Fatal(e)
	}
	second, _ := json.Marshal(st)
	if string(first) != string(second) || st.SkillPoints[0] != points {
		t.Fatalf("第二次 Reset 改变了状态: sp %d -> %d", points, st.SkillPoints[0])
	}
}

// mask&2 清 Enhance、mask&4 清 Evolve，并且清完要补满固定宽度
// （3 个 intension + 5 个 option），否则客户端拿到空块显示空白。
func TestResetAutoSetClearsVariationsByMask(t *testing.T) {
	s, role, st := autoSkillFixture(t)
	st.Awakening = 3
	full := SkillVariationState{
		Intensions: []protocol.SkillVariation{{ID: 62, Choice: 1}, {ID: 62, Choice: 2}, {ID: 0, Choice: 3}},
		Options:    []protocol.SkillVariation{{ID: 62, Choice: 1}, {ID: 62, Choice: 1}, {ID: 62, Choice: 2}, {ID: 0, Choice: 3}, {ID: 0, Choice: 3}},
	}
	st.SkillVariations[0] = full

	onlySkills := st
	if e := s.resetAutoState(t.Context(), role, &onlySkills, 0, 1); e != nil {
		t.Fatal(e)
	}
	if onlySkills.SkillVariations[0].Intensions[0].ID != 62 || onlySkills.SkillVariations[0].Options[0].ID != 62 {
		t.Fatal("mask=1 不该动 VP 分配")
	}

	st.SkillVariations[0] = full
	if e := s.resetAutoState(t.Context(), role, &st, 0, 7); e != nil {
		t.Fatal(e)
	}
	v := st.SkillVariations[0]
	if len(v.Intensions) != 3 || len(v.Options) != 5 {
		t.Fatalf("VP 块宽度 = %d/%d，应为 3/5", len(v.Intensions), len(v.Options))
	}
	for i, r := range v.Intensions {
		if r.ID != 0 || r.Choice != 3 || r.Status != 2 {
			t.Fatalf("intension%d 未清空: %+v", i, r)
		}
	}
	for i, r := range v.Options {
		if r.ID != 0 || r.Choice != 3 || r.Empty != 1 {
			t.Fatalf("option%d 未清空: %+v", i, r)
		}
	}
}

// VP 面板由三觉解锁，与 115 级无关：100 级三觉角色原来会被
// "variation unlock progression not satisfied" 拒掉。
func TestVariationUnlockFollowsAwakeningNotLevel(t *testing.T) {
	s := &Service{}
	for _, tc := range []struct {
		name      string
		awakening byte
		level     byte
		want      bool
	}{
		{"三觉但不到 115", 3, 100, true},
		{"三觉已满级", 3, 115, true},
		{"二觉", 2, 115, false},
		{"未觉醒", 0, 115, false},
	} {
		st := State{Level: tc.level, Advancement: 2, Awakening: tc.awakening}
		e := s.applyVariations(11, &st, map[uint16]byte{}, protocol.SkillPurchase{Options: emptyRows(5)})
		if tc.want && e != nil {
			t.Fatalf("%s: 被拒 %v", tc.name, e)
		}
		if !tc.want && e == nil {
			t.Fatalf("%s: 不该放行", tc.name)
		}
		if tc.want && len(st.SkillVariations[0].Options) != 5 {
			t.Fatalf("%s: VP 分配没落进 state", tc.name)
		}
	}
}

// 三觉有一部分技能源里没有 [variation point] 块。原来会把整次 Apply 拒掉，
// 现在只有"声明了该块但缺这个 tag"才拒。
func TestVariationAcceptsSkillWithoutSourcePointBlock(t *testing.T) {
	s, _, _ := autoSkillFixture(t)
	tested := 0
	for job, defs := range s.Learning.index {
		if tested > 0 {
			break
		}
		for id, d := range defs {
			if len(d.Fields["[variation point]"]) != 0 {
				continue
			}
			adv := -1
			for candidate := 0; candidate < 16; candidate++ {
				if d.ForAdvancement(candidate) || d.ForAwakening(candidate, 3) {
					adv = candidate
					break
				}
			}
			if adv < 0 {
				continue
			}
			intensions := emptyRows(3)
			intensions[0] = protocol.SkillVariation{ID: id, Choice: 1}
			st := State{Level: 100, Advancement: byte(adv), Awakening: 3}
			req := protocol.SkillPurchase{Intensions: intensions, Options: emptyRows(5)}
			if e := s.applyVariations(job, &st, map[uint16]byte{id: 1}, req); e != nil {
				t.Fatalf("job%d skill%d 无 [variation point] 被拒: %v", job, id, e)
			}
			t.Logf("无 [variation point] 的样本：job%d skill%d (%s)", job, id, d.Path)
			tested++
			break
		}
	}
	if tested == 0 {
		t.Skip("当前源里没有缺 [variation point] 的技能")
	}
}

// VariationRestore 要把空块补满宽度，且二觉以下不下发 VP 块。
func TestVariationRestoreFillsSlotsForThirdAwakening(t *testing.T) {
	s := &Service{}
	role := func(st State) storage.Character {
		raw, e := json.Marshal(st)
		if e != nil {
			t.Fatal(e)
		}
		return storage.Character{State: raw}
	}
	blank := role(State{Awakening: 3, Level: 100})
	out, e := s.VariationRestore(blank)
	if e != nil {
		t.Fatal(e)
	}
	if len(out) == 0 {
		t.Fatal("三觉角色必须收到 VP 块，否则面板显示空白")
	}
	filled := State{Awakening: 3, Level: 100}
	fillVariationSlots(&filled.SkillVariations[0])
	p, e := protocol.SkillPurchaseSuccess(0, 0, 0, nil)
	if e != nil {
		t.Fatal(e)
	}
	want, e := protocol.SkillPurchaseVariations(p, 0, filled.SkillVariations[0].Intensions, filled.SkillVariations[0].Options)
	if e != nil {
		t.Fatal(e)
	}
	if string(out) != string(want) {
		t.Fatalf("补满后的 VP 块与期望不一致: %d vs %d 字节", len(out), len(want))
	}
	below, e := s.VariationRestore(role(State{Awakening: 2, Level: 115}))
	if e != nil || below != nil {
		t.Fatalf("未三觉不该收到 VP 块: %v %v", below, e)
	}
}
