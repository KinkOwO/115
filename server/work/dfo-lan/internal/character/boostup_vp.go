package character

import (
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

// LOCAL rule approved by the user: allocate the Skill Evolve Points the source
// step asks for and save; no extra Enhance combination is required. The number
// of points comes from this step's [mission][condition] (the 2026 source says
// 3, not all five). Called only AFTER the skill save has passed
// ownership/source/point validation, inside its existing transaction.
func (s *Service) completeBoostVPSave(raw json.RawMessage, state State, req protocol.SkillPurchase) (json.RawMessage, bool, error) {
	if s.Boost == nil || req.Options == nil && req.Intensions == nil {
		return raw, false, nil
	}
	st, e := boostup.ReadState(raw)
	if e != nil {
		return nil, false, e
	}
	if !st.Activated || st.Training.Finished || st.Training.Phase != 2 || !st.Training.Claimed[st.Training.Step] {
		return raw, false, nil
	}
	if st.Training.Step == 0 || int(st.Training.Step) > len(s.Boost.Steps) {
		return nil, false, fmt.Errorf("invalid boost VP step")
	}
	if s.Boost.Steps[int(st.Training.Step)-1].Mission != "skill vp option" {
		return raw, false, nil
	}
	if req.Tree > 1 || state.Awakening != 3 {
		return nil, false, fmt.Errorf("VP save lacks third awakening")
	}
	need, e := s.Boost.Steps[int(st.Training.Step)-1].VPPointCondition()
	if e != nil {
		return nil, false, e
	}
	// 花掉的积分数直接取自 activeEvolutions：VP 选项的槽位数、选择项与技能归属
	// 已经在 applyVariations 里按源校验过，这里不再重复一遍点数结算。
	if uint32(activeEvolutions(state.SkillVariations[req.Tree].Options)) < need {
		return raw, false, nil
	}
	st.Training, e = s.Boost.MissionCompleted(st.Training, "skill vp option")
	if e != nil {
		return nil, false, e
	}
	raw, e = boostup.WriteState(raw, st)
	return raw, e == nil, e
}
