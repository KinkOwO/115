package gamedata_test

import (
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"os"
	"testing"
)

// TestBoostUpFullSourceClosure replays preparePVFBoostUp's binding order against
// the explicit inner archive: Load, capsules, optional challenges, the fame-side
// equipment-grouping injection, both mission validators and the reward-box closure.
//
// 胶囊绑定的源判据原来在 internal/boostup 的源测试里，那里需要一个「模板 → 脚本」
// 适配器；本树的适配器只有 gamedata 的 BoostUpSource 一份（归档已在手、物品索引
// 走原生 ItemIndex）。把判据合进本测试，是为了不在测试里再写第二份适配器。
func TestBoostUpFullSourceClosure(t *testing.T) {
	path := os.Getenv("US115_TEST_BOOST_PVF")
	if path == "" {
		t.Skip("explicit read-only source required")
	}
	s, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, ExpectedChecksum: os.Getenv("US115_TEST_BOOST_PVF_SHA256"), MaxBytes: gamedata.DefaultMaxBytes})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	index, e := s.ItemIndex("")
	if e != nil {
		t.Fatal(e)
	}
	src, c, e := s.BoostUp(index)
	if e != nil {
		t.Fatal(e)
	}
	if len(c.Steps) != 11 || c.GoalLevel != 115 || c.Town == 0 {
		t.Fatalf("unexpected step set: steps=%d goal=%d town=%d", len(c.Steps), c.GoalLevel, c.Town)
	}
	// 训练本体不会隐式打开 665 挑战，也不会丢掉毕业邮寄的源定义。
	if len(c.Challenges) != 0 || len(c.ChallengeBuffs) != 0 || c.CapsuleLevelMail(115) == nil {
		t.Fatal("training implicitly enabled optional challenges or lost the captured level mail")
	}
	if e = c.BindCapsules(src); e != nil {
		t.Fatal(e)
	}
	if len(c.Capsules) != 2 || c.Capsules[590015870].Variant != 0 || c.Capsules[590015871].Variant != 1 {
		t.Fatal("unexpected source capsule binding", c.Capsules)
	}
	if e = c.BindChallenges(src); e != nil {
		t.Fatal(e)
	}
	// 装备分组不另读 equipmentgrouping.etc：由名望侧唯一解析器注入（§0.2）。
	fame, e := s.Fame(index)
	if e != nil {
		t.Fatal(e)
	}
	groups := boostup.GroupIndex(fame.Groups)
	if len(groups) == 0 {
		t.Fatal("fame equipment grouping empty")
	}
	c.Groups = groups
	if e = c.ValidateWearMissions(); e != nil {
		t.Fatal(e)
	}
	if e = c.ValidatePointMissions(); e != nil {
		t.Fatal(e)
	}
	// 第三关的完成门槛只能来自源：[mission][condition] 写的是 3 点进化点，
	// 客户端引导条同样是「使用 3 点以上」。这里钉住它，避免服务端又用比源
	// 更严的「五点全花完」把玩家卡在第三关。
	vp := 0
	for _, step := range c.Steps {
		need, e := step.VPPointCondition()
		if e != nil {
			t.Fatal(e)
		}
		if need == 0 {
			continue
		}
		vp++
		if step.Number != 3 || need != 3 {
			t.Fatalf("boost VP mission: step=%d condition=%d", step.Number, need)
		}
	}
	if vp != 1 {
		t.Fatalf("expected one VP mission, got %d", vp)
	}
	roots := c.RewardRoots()
	if len(roots) == 0 {
		t.Fatal("no reward roots for booster closure")
	}
	// 生产接线（preparePVFBoostUp + cmd 的 boosterBoxSource）用通用 `[booster]`
	// 投影展开活动奖励盒：每个奖励根都必须真的有一层可开内容，闭环失败要在
	// 源测试里暴露，而不是启动时才熔断。
	boxes, e := s.Boosters(index)
	if e != nil {
		t.Fatal(e)
	}
	if len(boxes) == 0 {
		t.Fatal("booster projection empty")
	}
	for _, root := range roots {
		if len(boxes[root].Pools) == 0 {
			t.Fatalf("boost reward root %d has no [booster] content", root)
		}
	}
	t.Logf("boost source closure: steps=%d gifts=%d capsules=%d buffs=%d roots=%d boosters=%d",
		len(c.Steps), len(c.Gifts), len(c.Capsules), len(c.ChallengeBuffs), len(roots), len(boxes))
}

func TestBoostUpTeachingAPCSource(t *testing.T) {
	path := os.Getenv("US115_TEST_BOOST_PVF")
	if path == "" {
		t.Skip("explicit read-only source required")
	}
	s, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, ExpectedChecksum: os.Getenv("US115_TEST_BOOST_PVF_SHA256"), MaxBytes: gamedata.DefaultMaxBytes})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Loading event/NPC tokens does not need unrelated item/reward-box projections.
	_, c, err := s.BoostUp(catalog.ItemIndex{})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.TeachingAPCs) != 1 || c.TeachingAPCs[0] != (boostup.TeachingAPC{Index: 1, Template: 2504, Event: boostup.EventID}) {
		t.Fatal("source teaching APC binding lost", c.TeachingAPCs)
	}
}
