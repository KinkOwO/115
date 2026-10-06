package gamedata

import (
	"dfolan/internal/boostup"
	"fmt"
	"log"
)

// preparePVFBoostUp 直读活动 662（新手成长胶囊教学）：训练步骤、礼盒、胶囊、
// 可选挑战一次装载，任何一步失败都拒绝发布该活动。
//
// 装备分组（`[equip grouping]` 穿戴任务）不在这里再读一遍
// `equipmentgrouping.etc`：名望侧已经有同一张源的唯一解析器，分组号由
// `FameRules.Groups` 注入成 `boostup.GroupIndex` 视图（§0.2 单一规则）。
func preparePVFBoostUp(c *Catalogs, s *Source, selected map[string]bool, inputs CatalogInputs) error {
	if !selected["boostup"] {
		return nil
	}
	if c.Items == nil {
		return fmt.Errorf("boostup requires the native PVF items domain")
	}
	src, content, err := s.BoostUp(*c.Items)
	if err != nil {
		return err
	}
	if inputs.BoostChallenge {
		if err = content.BindChallenges(src); err != nil {
			return fmt.Errorf("optional post-graduation challenges: %w", err)
		}
	}
	if err = content.BindCapsules(src); err != nil {
		return fmt.Errorf("Starter Boost capsules: %w", err)
	}
	if c.FameRules == nil || len(c.FameRules.Groups) == 0 {
		return fmt.Errorf("Starter Boost wear missions need the fame equipment grouping source")
	}
	groups := boostup.GroupIndex(c.FameRules.Groups)
	content.Groups = groups
	if err = content.ValidateWearMissions(); err != nil {
		return fmt.Errorf("Starter Boost wear missions: %w", err)
	}
	if err = content.ValidatePointMissions(); err != nil {
		return fmt.Errorf("Starter Boost point missions: %w", err)
	}
	c.BoostUp = content
	log.Printf("PVF Starter Boost prepared: steps=%d gifts=%d capsules=%d challenge-buffs=%d groups=%d source=%s",
		len(content.Steps), len(content.Gifts), len(content.Capsules), len(content.ChallengeBuffs), len(groups), c.SourceChecksum)
	s.ReleaseReadCaches()
	return nil
}
