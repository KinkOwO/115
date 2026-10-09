package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

// SkillTreeWireIndex 把存档里的技能类型选择投影成客户端原生选择字节
// （0xff = 第二技能页未解锁；0 = 技能类型 1；1 = 技能类型 2）。
// 语义与 protocol.SkillTreeWireIndex 完全一致，这里只是接受 State。
func SkillTreeWireIndex(state State) byte {
	return protocol.SkillTreeWireIndex(state.SkillTreeType)
}

// SetSkillTreeType 落地 CMD260（ENUM_CMDPACKET_CHANGE_ANOTHER_SKILL_TREE）：
// 把角色的技能类型选择写成 selection（1 = 技能类型 1，2 = 技能类型 2）。
//
// 未解锁第二技能页（State.SkillTreeType == 0）时拒绝 —— 与 86JP
// SkillHandler.Handle_CHANGE_ANOTHER_SKILL_TREE 的 locked 分支同语义
// （那边回 LockedWireValue 并保持原状态）。
//
// 与其它技能变更共用 CommitCharacterEvent，因此同一个 key 的重发是幂等的
// （重放不会再翻一次页）。
func (s *Service) SetSkillTreeType(ctx context.Context, role Character, key string, selection byte) (Character, bool, error) {
	if selection < 1 || selection > 2 {
		return role, false, fmt.Errorf("invalid skill tree selection")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "source-change-skill-tree-v1", func(cur Character) (json.RawMessage, json.RawMessage, error) {
		var st State
		if e := json.Unmarshal(cur.State, &st); e != nil {
			return nil, nil, e
		}
		if st.SkillTreeType < 1 || st.SkillTreeType > 2 {
			return nil, nil, fmt.Errorf("second skill page is not unlocked")
		}
		st.SkillTreeType = selection
		p, e := mergeSkillState(cur.State, st)
		if e != nil {
			return nil, nil, e
		}
		receipt, _ := json.Marshal(map[string]any{"skill_tree_type": selection})
		return p, receipt, nil
	})
	if e != nil {
		return role, false, e
	}
	saved.WireID = role.WireID
	return saved, applied, nil
}
