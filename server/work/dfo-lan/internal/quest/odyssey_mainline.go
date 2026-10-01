package quest

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/savecontract"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// OdysseyMainlinePlan is the pure graduation mainline strategy (IO-free): the
// story quests an Arad Odyssey character counts as cleared at its current
// level, plus the branch quests that must stay reachable so the player is
// never blocked by a story quest they were never offered.
func OdysseyMainlinePlan(g *catalog.OdysseyGrowth, profession byte, level byte) (clear []uint16, branches []uint16, err error) {
	if g == nil || g.Quests == nil {
		return nil, nil, fmt.Errorf("Odyssey quest tables not loaded")
	}
	if level > catalog.OdysseyGraduationLevelV {
		return nil, nil, fmt.Errorf("level %d beyond Odyssey graduation level", level)
	}
	for _, id := range g.Quests.ClearedAt(level) {
		clear = append(clear, uint16(id))
	}
	// Double insurance: a branch quest must never end up inside the cleared
	// set, even if a future source edit reintroduces the overlap.
	skip := map[uint16]bool{}
	for _, id := range clear {
		skip[id] = true
	}
	for _, id := range g.Quests.BranchQuestsUpTo(level, profession) {
		if !skip[uint16(id)] {
			branches = append(branches, uint16(id))
		}
	}
	return clear, branches, nil
}

// OdysseyMainline applies the plan: it writes the cleared rows (idempotent,
// existing rows untouched, progress_model='odyssey-skip-v1') and returns the
// branch quests that remain reachable. Branch quests are not written; they are
// reported so the operator can audit that the level 115 abyss guide survived.
func (s *Service) OdysseyMainline(ctx context.Context, role storage.Character) (int, []uint16, error) {
	if s.Odyssey == nil || s.Store == nil || !character.CreatedAsOdyssey(role) {
		return 0, nil, nil
	}
	var state character.State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return 0, nil, e
	}
	clear, branches, e := OdysseyMainlinePlan(s.Odyssey, role.Profession, state.Level)
	if e != nil {
		return 0, nil, e
	}
	// ⚠️ `ClearQuests` 的 version 是 **L1 存档身份**：它去匹配 `character_quests.config_version`，
	// 而该列在存里是契约身份（`savecontract.Identity()`）。`OdysseyGrowth.Source` 存的是
	// **内层真哈希**（`ImportOdysseyGrowth` 按 `a.Snapshot().Checksum` 赋值）⇒ 直接传它
	// **恒不匹配**，主线清任务会静默清 0 行（返回 0 且不报错）。
	cleared, e := s.Store.ClearQuests(ctx, role.AccountID, role.ID, savecontract.Identity(), clear)
	if e != nil {
		return 0, nil, e
	}
	return cleared, branches, nil
}
