package quest

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/savecontract"
	"encoding/json"
	"fmt"
	"sort"
)

// BoostStorySkipPlan 是 662 直升胶囊落地后的主线清除判据：源里 `[grade] [epic]`
// 的任务（客户端任务手册里的 Act 章节线），最低等级落在 (0, goal] 区间、职业与
// 转职阶段允许。goal 取活动自身的 `[goal level]`，**含等于**：115 级当前章主线
// （源里 min=115 的 epic 共 60 条）也是直升角色该一次清掉的，业主 2026-10-06
// 实机反馈「还剩 115 级的任务」即角色 1 库里剩下的 22841/23028（`[epic]` min=115）。
// 这与 2026-10-05 奥德赛毕业那次「删掉 min>=115 过滤」的裁决同一口径。
//
// 与奥德赛毕业共用 epic 扫源判据，但**不**读 `aradodyssey.etc [quest clear]` 那张表：
// 那是奥德赛课的源，普通角色没有对应字段（662 的 `boostup.evt` 只声明
// `[quest clear item]` 三张 `[side]` 墙清券，见
// docs/protocol/boostup662-story-skip-20261006.md），所以这里按业主裁决取服侧策略。
func (s *Service) BoostStorySkipPlan(role character.Character, goal byte) ([]uint16, error) {
	if goal == 0 || len(s.Catalog.Quests) == 0 ||
		role.ConfigVersion != savecontract.Identity() || role.ConfigVersion != s.Catalog.Source.SaveIdentity() || role.ConfigVersion != s.Professions.Source.SaveIdentity() {
		return nil, fmt.Errorf("boost skip catalogs are missing or mismatched")
	}
	var state character.State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, e
	}
	if state.Level < goal {
		return nil, fmt.Errorf("boost skip requires the event goal level")
	}
	profession, ok := s.Professions.Professions[role.Profession]
	if !ok {
		return nil, fmt.Errorf("boost skip profession absent from source")
	}
	ids := make([]uint16, 0, 64)
	for id, q := range s.Catalog.Quests {
		grade := cells(q.Script.Cells, "[grade]")
		if len(grade) != 1 || grade[0].Type != 6 || grade[0].Text != "[epic]" || q.MinimumLevel == 0 || q.MinimumLevel > uint32(goal) {
			continue
		}
		allowed := jobAllowed(q.Jobs, profession.Job)
		for _, grow := range cells(q.Script.Cells, "[grow type]") {
			if grow.Type != 0 {
				return nil, fmt.Errorf("boost skip quest %d grow type unresolved", id)
			}
			if grow.Value >= 0 && grow.Value != int32(state.Advancement) {
				allowed = false
			}
		}
		if allowed {
			ids = append(ids, uint16(id))
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// BoostStorySkip 幂等：character_events 的 boost-story-skip-v2 行就是标记；
// 只持有 v1 收据的角色（清除时漏了 min=goal 那批）在这里补一次，见
// database.CommitBoostStorySkip。
func (s *Service) BoostStorySkip(ctx context.Context, role character.Character, goal byte) (int, bool, error) {
	if s.Store == nil || goal == 0 || role.ID == 0 {
		return 0, false, fmt.Errorf("boost skip store/role missing")
	}
	return s.Store.CommitBoostStorySkip(ctx, role.AccountID, role.ID, savecontract.Identity(),
		func(current character.Character) ([]uint16, error) { return s.BoostStorySkipPlan(current, goal) })
}
