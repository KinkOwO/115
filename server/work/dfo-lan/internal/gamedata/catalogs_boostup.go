package gamedata

import (
	"dfolan/internal/boostup"
	"fmt"
	"log"
)

// preparePVFBoostUp 直读活动 662（新手成长胶囊教学）：训练步骤、礼盒、胶囊与
// 毕业后的可选挑战一次装载；除挑战本身按 warning 降级外，任何一步失败都拒绝发布该活动。
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
	// 毕业后的可选挑战（665）是玩法内容，不开第二套开关（§6）：一律按源绑定。
	// 绑定失败只降级挑战本身并记 warning —— 挑战的解析器不得挡住 662 的十一关
	// （见 internal/boostup/catalog.go 的 BindChallenges 注释），运行期由
	// boostup_challenge 的 fail-closed 分支拒绝查询与领奖。
	if err = content.BindChallenges(src); err != nil {
		log.Printf("warning: Starter Boost optional challenges (event 665) unavailable: %v", err)
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
	log.Printf("PVF Starter Boost prepared: steps=%d gifts=%d capsules=%d challenges=%d challenge-buffs=%d groups=%d event-window=%d..%d source=%s",
		len(content.Steps), len(content.Gifts), len(content.Capsules), len(content.Challenges),
		len(content.ChallengeBuffs), len(groups), boostup.EventStart, boostup.EventEnd, c.SourceChecksum)
	s.ReleaseReadCaches()
	return nil
}
