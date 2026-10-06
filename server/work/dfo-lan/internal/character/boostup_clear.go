package character

import (
	"dfolan/internal/boostup"
	"encoding/json"
)

// Called only inside clear:<RunID>'s character transaction, after Completed()
// validation. Its existing receipt prevents a replay from counting twice.
//
// 只有「清理秩序终结者」这一类关卡能由副本号直接判定：它的判据就是活动源自己的
// `[go contents dungeon index]`（ApplyChallengeClear 里 dungeon == GuideDungeon）。
//
// 「高阶/军团」与「团队」关卡的判据是源里的内容号（`[challenge condition]`
// 26/31/35/36、14），而**副本号 → 内容号**这张映射在本树没有直读来源：
// etc/clientchannelinfo.etc 只给 `[guide dungeon index]` 和 isSemiRaid/isLegion
// 这类标记，不含内容号；旧活动树是由共享包 internal/conquest 提供的，该包不在
// 交付快照里。按 §0.3 不猜表：这两类关卡在拿到内容号真源之前不由通关事件计数。
func (s *ProgressionService) applyBoostChallengeClear(raw json.RawMessage, dungeon uint32) (json.RawMessage, error) {
	if s.Boost == nil {
		return raw, nil
	}
	st, err := boostup.ReadState(raw)
	if err != nil {
		return nil, err
	}
	if !st.Activated || !st.Training.Finished || st.Challenge == nil || !st.Challenge.Enrolled {
		return raw, nil
	}
	next, changed, err := s.Boost.ApplyChallengeClear(st.Challenge, "clear endkeeper of order", 0, dungeon)
	if err != nil {
		return nil, err
	}
	if !changed {
		return raw, nil
	}
	st.Challenge = next
	return boostup.WriteState(raw, st)
}
