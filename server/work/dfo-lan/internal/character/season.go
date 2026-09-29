package character

import (
	"dfolan/internal/adventure"
	"encoding/json"
	"time"
)

func seasonLevel(state adventure.SeasonState) uint32 {
	rules, err := adventure.CurrentSeason()
	if err != nil {
		return 1
	}
	return rules.DisplayLevel(state)
}

// 只接受服务端已确认的最终通关事件；军团/攻坚区域不能拿子副本编号冒充整次通关。
func awardSeasonClear(raw json.RawMessage, dungeonID uint32, difficulty byte, now time.Time) (json.RawMessage, uint32, error) {
	var state State
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, 0, err
	}
	rules, err := adventure.CurrentSeason()
	if err != nil {
		return nil, 0, err
	}
	if state.Level < rules.MinimumLevel {
		return raw, 0, nil
	}
	content, ok := rules.Content(dungeonID, 0, false)
	if !ok || content.Category > 2 {
		return raw, 0, nil
	}
	// 2026-09-28 千海之天实机 CMD16：副本100005064，难度字节为0。
	// 源经验表同样从0起算，直接按原始难度查表，不能跳过0或统一减1。
	gain, err := rules.Gain(&state.SeasonLevel, content, uint32(difficulty), now)
	if err != nil {
		return nil, 0, err
	}
	next, err := adventure.SaveSeason(raw, state.SeasonLevel)
	return next, gain, err
}
