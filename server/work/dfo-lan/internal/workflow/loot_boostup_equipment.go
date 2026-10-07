package workflow

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/database"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var errBoostWearNotReady = errors.New("boost worn mission not yet satisfied")

// BoostSetPoints 提供角色穿戴装备的原始套装积分（按套装号）。
// 真源是 character.Service.BoostWornSetPoints（名望侧唯一的 setpointinfo 解析器）；
// 为 nil 时依赖积分的关卡直接判未满足，不猜。
type BoostSetPoints func(database.Character) (map[int32]uint32, error)

// Separate from the equipment transaction but recoverable: recompute from
// the CURRENT locked row, not from an ACK or a client declaration. A failure
// never rolls back an already-successful equipment move or consumes a reward.
// （§7.2 E13：带权益视图的提交编排在 workflow；事实/满足判定是 loot 纯计算。）
func (s *LootService) ReconcileBoostEquipment(ctx context.Context, role database.Character, c *boostup.Catalog, setPoints BoostSetPoints) (database.Character, bool, error) {
	if s == nil || s.Loot == nil || s.Store == nil || c == nil {
		return role, false, nil
	}
	before, err := boostup.ReadState(role.State)
	if err != nil {
		return role, false, err
	}
	st := before.Training
	if !before.Activated || st.Finished || st.Phase != 2 || !st.Claimed[st.Step] {
		return role, false, nil
	}
	if st.Step == 0 || int(st.Step) > len(c.Steps) {
		return role, false, fmt.Errorf("invalid boost mission step")
	}
	row := c.Steps[int(st.Step)-1]
	req, err := row.WearRequirement()
	if err != nil {
		return role, false, err
	}
	pointReq, err := row.PointRequirement()
	if err != nil || req == nil && pointReq == nil {
		return role, false, err
	}
	if pointReq != nil && setPoints == nil {
		return role, false, fmt.Errorf("boost set point source missing")
	}
	apply := func(current database.Character, premiums map[uint8]bool) (json.RawMessage, json.RawMessage, error) {
		state, err := boostup.ReadState(current.State)
		if err != nil {
			return nil, nil, err
		}
		if !state.Activated || state.Training.Step != st.Step || state.Training.Phase != 2 {
			return nil, nil, errBoostWearNotReady
		}
		facts, err := s.Loot.BoostWornFacts(LootRole(current))
		if err != nil {
			return nil, nil, err
		}
		var met bool
		var proof any = facts
		if pointReq != nil {
			points, pointErr := setPoints(current)
			if pointErr != nil {
				return nil, nil, pointErr
			}
			met = pointReq.Satisfied(points, premiums)
			proof = map[string]any{"worn": facts, "set_points": points, "active_contracts": premiums}
		} else {
			if req.Enchanted {
				cards, cardErr := s.Loot.BoostWornEnchantCards(LootRole(current))
				if cardErr != nil {
					return nil, nil, cardErr
				}
				met, err = req.Satisfied(facts, c.Groups, cards)
				proof = map[string]any{"worn": facts, "enchant_cards": cards}
			} else {
				met, err = req.Satisfied(facts, c.Groups)
			}
		}
		if err != nil {
			return nil, nil, err
		}
		if !met {
			return nil, nil, errBoostWearNotReady
		}
		state.Training, err = c.MissionCompleted(state.Training, row.Mission)
		if err != nil {
			return nil, nil, err
		}
		raw, err := boostup.WriteState(current.State, state)
		if err != nil {
			return nil, nil, err
		}
		receipt, err := json.Marshal(proof)
		return raw, receipt, err
	}
	var next database.Character
	var applied bool
	key := fmt.Sprintf("boostup-mission:%d", st.Step)
	if pointReq != nil {
		// 权益视图按本树既有读法取（database.ActivePremiumSet），再进角色事务：
		// 权益是账号级状态，事务锁保护的是角色行的关卡进度，不是权益行。
		premiums, premiumErr := s.Store.ActivePremiumSet(ctx, role.AccountID, time.Now())
		if premiumErr != nil {
			return role, false, premiumErr
		}
		next, applied, err = s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, "boostup-worn-proof-v1", func(cur database.Character) (json.RawMessage, json.RawMessage, error) { return apply(cur, premiums) })
	} else {
		next, applied, err = s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, "boostup-worn-proof-v1", func(cur database.Character) (json.RawMessage, json.RawMessage, error) { return apply(cur, nil) })
	}
	if errors.Is(err, errBoostWearNotReady) {
		return role, false, nil
	}
	if err != nil {
		return role, false, err
	}
	next.WireID = role.WireID
	return next, applied, nil
}
