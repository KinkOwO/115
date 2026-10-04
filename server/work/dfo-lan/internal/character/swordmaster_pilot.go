package character

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

// RepairSwordmasterPilot projects only the captured, level-one Swordmaster
// request shape. The caller must select an isolated database explicitly.
func (s *Service) RepairSwordmasterPilot(role Character) (Character, error) {
	if !s.Rules.SwordmasterPilot || role.Profession != 0 || role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return role, fmt.Errorf("not a swordmaster pilot character")
	}
	req, err := protocol.DecodeCreateRequest(role.Request)
	if err != nil {
		return role, err
	}
	if req.Profession != 0 || req.Name != role.Name || len(req.Options) != 12 || req.Options[8] != 1 || (req.Options[10] != 0 && req.Options[10] != 2) {
		return role, fmt.Errorf("request differs from verified swordmaster samples")
	}
	var state State
	if err = json.Unmarshal(role.State, &state); err != nil {
		return role, err
	}
	prof := s.Catalog.Professions[0]
	// 2026-10-04：源身份用 SourcePath（L2 引用身份）而非 RawSHA256（.chr 原始哈希
	// 随字符串池偏移漂移，见 automatic_skills.go 注释）。
	if state.Level != 1 || state.Experience != 0 || prof.Path != state.SourcePath || len(prof.SwordmasterGrowth) == 0 || (state.Advancement != 0 && !(state.Advancement == 1 && state.SwordmasterPilot)) {
		return role, fmt.Errorf("pilot repair requires unchanged level-one source state")
	}
	// Merge only these fields; preserve all unrelated and future JSON fields.
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(role.State, &fields); err != nil {
		return role, err
	}
	fields["advancement"] = json.RawMessage("1")
	fields["swordmaster_pilot"] = json.RawMessage("true")
	fields["creation_mode"], _ = json.Marshal(req.Options[10])
	fields["creation_options"], _ = json.Marshal(req.Options)
	role.State, err = json.Marshal(fields)
	return role, err
}
