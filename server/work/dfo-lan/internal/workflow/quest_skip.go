package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/quest"
	"fmt"
)

// SkipMainlineNow runs the client's one-click main quest button (CMD1422): it
// settles every main quest the character can take right now, in one transaction,
// and hands over the key items a skipped prerequisite would have paid.
//
// The rewards capability only exists for the five quests the source marks
// rewardable on force clear; everything else settles on zero experience, zero
// gold and zero items, which is what keeps the button from being a farm across
// the main line.
func (s *QuestService) SkipMainlineNow(ctx context.Context, role database.Character, operator, reason string, limit int) (quest.SkipResult, error) {
	var out quest.SkipResult
	if s == nil || s.Store == nil || s.Quest == nil {
		return out, fmt.Errorf("任务跳过需要有效角色存档")
	}
	return s.Quest.SkipAllNow(ctx, role, operator, reason, quest.MainlineGrades(), limit, quest.FinishRewards{Items: questItemRewards})
}
