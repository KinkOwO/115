package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"

	"encoding/json"
	"testing"
)

// 奥德赛创建角色 + 满级 115 的 state（Options[10]=2, CreationMode=2）。
func graduateFixture(t *testing.T) (*ProgressionService, Character) {
	t.Helper()
	s, member := odysseyGrowthFixture(t)
	var options [12]byte
	options[10] = 2
	state := State{Level: OdysseyGraduationLevel, CreationOptions: options[:], CreationMode: 2}
	raw, e := json.Marshal(state)
	if e != nil {
		t.Fatal(e)
	}
	member.State = raw
	return s, member
}

// 满级毕业：标记落 state、持久化创建标记清零、奖励盒按目录发放。
func TestOdysseyGraduationFlow(t *testing.T) {
	s, role := graduateFixture(t)

	if !CreatedAsOdyssey(role) || OdysseyGraduated(role) {
		t.Fatal("前置状态错误")
	}
	graduated, _, e := s.ApplyOdysseyGraduation(role)
	if e != nil {
		t.Fatal(e)
	}
	var next State
	if e = json.Unmarshal(graduated, &next); e != nil {
		t.Fatal(e)
	}
	if !next.OdysseyGraduated || next.CreationMode != 0 || next.CreationOptions[10] != 0 {
		t.Fatalf("毕业标记未正确落 state: %+v", next)
	}
	// 原始建号请求包是历史，不被动。
	if !CreatedAsOdyssey(role) {
		t.Fatal("请求包被改写")
	}
	graduatedRole := role
	graduatedRole.State = graduated
	// 毕业后：OdysseyRole 恒 false（即使启动器强注模式），成员判定 false。
	t.Setenv("DFO_ODYSSEY_MODE", "1")
	if OdysseyRole(graduatedRole) {
		t.Fatal("毕业角色仍被当成奥德赛角色")
	}
	if OdysseyMember(graduatedRole) {
		t.Fatal("毕业角色仍算奥德赛成员")
	}
	if !OdysseyRole(role) || !OdysseyMember(role) {
		t.Fatal("未毕业角色判定错误")
	}

	// 奖励盒：目录只含毕业盒一个模板；发放一格。
	box := OdysseyGraduateBoxCatalog(s.Odyssey)
	if len(box.Items) != 1 || box.Items[10420561].ID != 10420561 {
		t.Fatalf("毕业奖励目录错误: %v", box.Items)
	}
	rawState, _, e := s.ApplyOdysseyGraduationReward(graduatedRole)
	if e != nil {
		t.Fatal(e)
	}
	bag, e := inventory.ReadBag(rawState)
	if e != nil {
		t.Fatal(e)
	}
	found := uint32(0)
	for _, it := range bag.Items {
		if it.Template == 10420561 {
			found += it.Amount
		}
	}
	if found != 1 {
		t.Fatalf("毕业盒应发 1 个 10420561，实际 %d", found)
	}
}

func TestOdysseyGraduationRejects(t *testing.T) {
	s, _ := graduateFixture(t)
	// 未满级
	var options [12]byte
	options[10] = 2
	low := State{Level: 114, CreationOptions: options[:]}
	rawLow, _ := json.Marshal(low)
	_, member := odysseyGrowthFixture(t)
	lowRole := member
	lowRole.State = rawLow
	if _, _, e := s.ApplyOdysseyGraduation(lowRole); e == nil {
		t.Fatal("未满级也被毕业")
	}
	// 普通角色（无奥德赛建号请求包）
	normal := member
	normal.Request = nil
	if _, _, e := s.ApplyOdysseyGraduation(normal); e == nil {
		t.Fatal("普通角色也被毕业")
	}
	// 已毕业
	done := State{Level: OdysseyGraduationLevel, CreationOptions: options[:], OdysseyGraduated: true}
	rawDone, _ := json.Marshal(done)
	doneRole := member
	doneRole.State = rawDone
	if _, _, e := s.ApplyOdysseyGraduation(doneRole); e == nil {
		t.Fatal("重复毕业被接受")
	}
	// 未毕业角色拿不到奖励盒
	if _, _, e := s.ApplyOdysseyGraduationReward(lowRole); e == nil {
		t.Fatal("未毕业也发了奖励盒")
	}
	// 奖励盒模板不得与等级赠装重复（真源校验兜底）
	for _, gift := range s.Odyssey.Gifts {
		if gift == s.Odyssey.GraduateReward {
			t.Fatal("毕业盒与赠装重复")
		}
	}
	if s.Odyssey.GraduateReward != catalog.OdysseyGraduateRewardTemplate {
		t.Fatalf("毕业盒模板漂移: %d", s.Odyssey.GraduateReward)
	}
}
