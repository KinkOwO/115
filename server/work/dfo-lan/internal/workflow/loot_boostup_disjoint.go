package workflow

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/database"
	"encoding/json"
	"errors"
	"fmt"
)

var errBoostDisjointNotReady = errors.New("boost disjoint mission not yet satisfied")

// ReconcileBoostDisjoint 把「**这一笔分解真的删掉了装备**」当作第九关的服务端事实。
//
// 与 ReconcileBoostEquipment 同一套纪律：单独一次可恢复的角色事件（幂等键按关卡号），
// 判定在**锁住的当前行**里重算，失败绝不回滚已经提交的分解、也不重复消费。
// 关卡不匹配（玩家不在 disjoint 关、还没领奖励、活动没开）一律返回 applied=false、
// err=nil —— 分解本身是正常操作，不该被事件层拦住。
func (s *LootService) ReconcileBoostDisjoint(ctx context.Context, role database.Character, c *boostup.Catalog, disassembled bool) (database.Character, bool, error) {
	if s == nil || s.Loot == nil || s.Store == nil || c == nil {
		return role, false, nil
	}
	before, err := boostup.ReadState(role.State)
	if err != nil {
		return role, false, err
	}
	step := before.Training.Step
	if step == 0 {
		return role, false, nil
	}
	key := fmt.Sprintf("boostup-mission:%d", step)
	next, applied, err := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Loot.Catalog.Source.SaveIdentity(), key, "boostup-disjoint-proof-v1",
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			state, err := boostup.ReadState(current.State)
			if err != nil {
				return nil, nil, err
			}
			advanced, changed, err := c.DisjointMissionAdvanced(state, disassembled)
			if err != nil || !changed {
				if err != nil {
					return nil, nil, err
				}
				return nil, nil, errBoostDisjointNotReady
			}
			raw, err := boostup.WriteState(current.State, advanced)
			if err != nil {
				return nil, nil, err
			}
			receipt, err := json.Marshal(map[string]any{"mission": "disjoint", "step": state.Training.Step})
			return raw, receipt, err
		})
	if errors.Is(err, errBoostDisjointNotReady) {
		return role, false, nil
	}
	if err != nil {
		return role, false, err
	}
	next.WireID = role.WireID
	return next, applied, nil
}
