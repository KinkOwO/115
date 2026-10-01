package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"

	"encoding/json"

	"testing"
)

// findThirdAwakeningSample picks one profession/growtype that carries grants
// for all three awakening stages, so a full 1->2->3 progression can run.
func findThirdAwakeningSample(t *testing.T, c catalog.Characters) (byte, byte, catalog.Profession) {
	t.Helper()
	for job, p := range c.Professions {
		for adv := byte(1); adv < 16; adv++ {
			stages := p.AwakeningSkills[adv]
			if len(stages[1]) > 0 && len(stages[2]) > 0 && len(stages[3]) > 0 {
				return job, adv, p
			}
		}
	}
	t.Fatal("目录里没有完整三阶段的样本")
	return 0, 0, catalog.Profession{}
}

// 三觉（stage 3）在 ApplyAwakening 里落地 5 点 VP 池与 3+5 槽位布局，并把
// technique_points / skill_variations 持久化；stage 1/2 一律不得授予。
func TestAwakeningProgressionGrantsPoolOnlyAtStage3(t *testing.T) {
	s, c := loadAwakeningGrantFixture(t)
	job, adv, prof := findThirdAwakeningSample(t, c)
	role := Character{Profession: job, ConfigVersion: c.Source.SaveIdentity()}
	mk := func(level, aw byte) Character {
		st := State{Level: level, Advancement: byte(adv), Awakening: aw, SourceSHA256: prof.RawSHA256, InitialSkills: prof.InitialSkills, SkillPoints: [2]uint16{50, 50}}
		raw, e := json.Marshal(st)
		if e != nil {
			t.Fatal(e)
		}
		role.State = raw
		return role
	}
	decode := func(raw json.RawMessage) State {
		var st State
		if e := json.Unmarshal(raw, &st); e != nil {
			t.Fatal(e)
		}
		return st
	}
	u1, e := s.ApplyAwakening(mk(50, 0), 1)
	if e != nil {
		t.Fatal(e)
	}
	if out := decode(u1); out.TechniquePoints[0] != 0 {
		t.Fatalf("job%d adv%d: 一觉即授予 VP %d", job, adv, out.TechniquePoints[0])
	}
	u2, e := s.ApplyAwakening(mk(75, 1), 2)
	if e != nil {
		t.Fatal(e)
	}
	if out := decode(u2); out.TechniquePoints[0] != 0 {
		t.Fatalf("job%d adv%d: 二觉即授予 VP %d", job, adv, out.TechniquePoints[0])
	}
	u3, e := s.ApplyAwakening(mk(100, 2), 3)
	if e != nil {
		t.Fatal(e)
	}
	out := decode(u3)
	if out.Awakening != 3 || out.TechniquePoints[0] != 5 {
		t.Fatalf("job%d adv%d: 三觉后 TP=%d 觉醒=%d，应为 5/3", job, adv, out.TechniquePoints[0], out.Awakening)
	}
	if len(out.SkillVariations[0].Intensions) != 3 || len(out.SkillVariations[0].Options) != 5 {
		t.Fatalf("job%d adv%d: VP 槽位布局 = %d/%d，应为 3/5", job, adv, len(out.SkillVariations[0].Intensions), len(out.SkillVariations[0].Options))
	}
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(u3, &fields); e != nil {
		t.Fatal(e)
	}
	if _, ok := fields["technique_points"]; !ok {
		t.Fatalf("job%d adv%d: technique_points 未持久化", job, adv)
	}
	if _, ok := fields["skill_variations"]; !ok {
		t.Fatalf("job%d adv%d: skill_variations 未持久化", job, adv)
	}
}

// 三觉角色的 Learn 响应（CMD29）必须恒带 g1+g2 块：持久化变体块为空时，旧的
// 实现会下发 present=0 的响应，客户端每收到一次就把 VP 面板清零——这就是
// "Learn Skill 加点后 VP 消失"的机制。
func TestLearningResponseKeepsVariationBlocksForThirdAwakening(t *testing.T) {
	s, c := loadAwakeningGrantFixture(t)
	job, adv, prof := findThirdAwakeningSample(t, c)
	role := func(st State) Character {
		raw, e := json.Marshal(st)
		if e != nil {
			t.Fatal(e)
		}
		return Character{Profession: job, ConfigVersion: c.Source.SaveIdentity(), State: raw}
	}
	req := protocol.SkillPurchase{Tree: 0, Mode: 0}

	awakened := State{Level: 100, Advancement: byte(adv), Awakening: 3, SourceSHA256: prof.RawSHA256, InitialSkills: prof.InitialSkills, SkillPoints: [2]uint16{10, 10}, TechniquePoints: [2]uint16{5, 5}}
	got, e := s.LearningResponse(role(awakened), req)
	if e != nil {
		t.Fatal(e)
	}
	filled := awakened
	fillVariationSlots(&filled.SkillVariations[0])
	p, e := protocol.SkillPurchaseSuccess(0, 10, 5, nil)
	if e != nil {
		t.Fatal(e)
	}
	want, e := protocol.SkillPurchaseVariations(p, 0, filled.SkillVariations[0].Intensions, filled.SkillVariations[0].Options)
	if e != nil {
		t.Fatal(e)
	}
	if string(got) != string(want) {
		t.Fatalf("三觉 Learn 响应缺 g1/g2 块: %d vs %d 字节", len(got), len(want))
	}

	// 未三觉：无变体块（present=0）。
	below := State{Level: 115, Advancement: byte(adv), Awakening: 2, SourceSHA256: prof.RawSHA256, InitialSkills: prof.InitialSkills, SkillPoints: [2]uint16{10, 10}}
	got, e = s.LearningResponse(role(below), req)
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
	if string(got) != string(want2) {
		t.Fatalf("未三觉 Learn 响应帧不匹配: %d vs %d 字节", len(got), len(want2))
	}
}

// 存量三觉角色（VP 账本为 0）选角登录时补发到 5；已对齐与未三觉都是 no-op。
