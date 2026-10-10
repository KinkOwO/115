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
	// 源身份按 SourcePath（引用身份）判断，不用 .chr 原始哈希：脚本字节里嵌的是字符串
	// 池偏移，重建字符串池（客户端版本升级）会让哈希整体漂移而引用不变。按存档契约原则，
	// 客户端资源哈希不得作为拒档理由（依据与取证见 automatic_skills.go 注释）。
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
