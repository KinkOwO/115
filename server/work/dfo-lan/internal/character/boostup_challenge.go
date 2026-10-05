package character

import (
	"encoding/json"
	"fmt"
	"math"
)

// Same completed/available fame resolver as actor entry (EquipmentFame); no
// base-only or client-supplied substitute is used for challenge qualification.
func (s *Service) BoostChallengeFacts(role Character) (byte, uint32, error) {
	var st State
	if e := json.Unmarshal(role.State, &st); e != nil {
		return 0, 0, e
	}
	fame, e := s.EquipmentFame(role.State)
	return st.Level, fame, e
}

// BoostWornSetPoints 给出身上穿戴装备的**原始套装积分**（按套装号，未过名望阈值表）。
//
// 数据来自 EquipmentFameBreakdown.SetPointTotals：setpointinfo.cos /
// equipmentpartset.etc 只有名望侧一个解析器，活动关卡「集满 N 分」直接取这份事实，
// 不在活动包里再读一遍同一个源。Sets 是阈值换算后的名望值，判据要的是积分本身，
// 所以这里导出的是 SetPointTotals 而不是 Sets。
func (s *Service) BoostWornSetPoints(raw json.RawMessage) (map[int32]uint32, error) {
	breakdown, e := s.EquipmentFameBreakdown(raw)
	if e != nil {
		return nil, e
	}
	out := make(map[int32]uint32, len(breakdown.SetPointTotals))
	for set, point := range breakdown.SetPointTotals {
		if set <= 0 || point < 0 || point > math.MaxUint32 {
			return nil, fmt.Errorf("boost set point identity out of range")
		}
		out[int32(set)] = uint32(point)
	}
	return out, nil
}
