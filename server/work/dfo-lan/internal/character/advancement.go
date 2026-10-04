package character

import (
	"encoding/json"
	"fmt"
)

// ApplyAdvancement moves a town character to one of its profession's
// advancement branches (CMD1881 change grow-type). The branch must exist in
// the source catalog ([growtype N] growth block); a job change always lands
// unawakened, so the awakening stage resets to 0. Advancement skills are
// granted at read time by automaticSkills/knownSkills from the new branch, so
// only the advancement/awakening fields are persisted - no skill rows change.
func (s *Service) ApplyAdvancement(role Character, advancement byte) (json.RawMessage, error) {
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, err
	}
	if _, err := state.WireAdvancement(); err != nil {
		return nil, err
	}
	if advancement == 0 || advancement > 15 {
		return nil, fmt.Errorf("invalid advancement target")
	}
	// Idempotent at the domain level: selecting the branch the character is
	// already on is a no-op, which also covers a retransmitted request.
	if advancement == state.Advancement && state.Awakening == 0 {
		return role.State, nil
	}
	prof, ok := s.Catalog.Professions[role.Profession]
	// 2026-10-04：源身份用 SourcePath（L2 引用身份）而非 RawSHA256——.chr 原始哈希随
	// 字符串池偏移漂移（2.38.2.34 与 2.38.3.25 逐 token 一致但哈希不同），按 savecontract
	// 原则不得做拒档理由。
	if !ok || prof.Path != state.SourcePath || role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return nil, fmt.Errorf("advancement source mismatch")
	}
	if len(prof.AdvancementGrowth[advancement]) == 0 {
		return nil, fmt.Errorf("profession cannot advance to that branch")
	}
	state.Advancement = advancement
	state.Awakening = 0
	if _, err := state.WireAdvancement(); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(role.State, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	fields["advancement"], _ = json.Marshal(state.Advancement)
	fields["awakening"], _ = json.Marshal(state.Awakening)
	return json.Marshal(fields)
}
