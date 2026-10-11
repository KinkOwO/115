package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
)

// FreezePrepared freezes a prepared content/policy award exactly once, then
// uses the existing atomic Claim and login Recover path. The caller owns
// eligibility and the source/policy rule; this method invents no reward table.
func (s *LegionRewardService) FreezePrepared(ctx context.Context, role database.Character, run, content string, dungeon uint32, draw func() ([]loot.Award, []loot.Award, error)) (database.Character, LegionRewardPlan, error) {
	var empty LegionRewardPlan
	if s == nil || s.Store == nil || s.Awarder == nil || role.ID <= 0 || role.AccountID <= 0 || run == "" || content == "" || dungeon == 0 || draw == nil || s.Awarder.Catalog.Source.Checksum == "" || role.ConfigVersion != s.Awarder.Catalog.Source.SaveIdentity() {
		return role, empty, fmt.Errorf("prepared legion reward source/owner missing")
	}
	saved, frozen, err := commitEquipmentEvent(ctx, s.Store, role, "legion-phase-freeze:"+run, "legion-phase-source-v1", func(current database.Character) (json.RawMessage, LegionRewardPlan, error) {
		state, progress, err := readLegionRewards(current.State)
		if err != nil {
			return nil, empty, err
		}
		basic, additional, err := draw()
		if err != nil {
			return nil, empty, err
		}
		if len(basic) > 35 || len(additional) > 15 || len(basic)+len(additional) == 0 {
			return nil, empty, fmt.Errorf("prepared legion reward display capacity")
		}
		for _, rows := range [][]loot.Award{basic, additional} {
			for _, item := range rows {
				definition, ok := s.Awarder.Catalog.Items[item.Template]
				valid := ok && definition.Kind == "stackable" && definition.StackableType != "[booster]" && definition.StackableType != "[virtual]"
				// Loot contains stackables; equipment has its own current source
				// reader and the same Reward validation used by atomic Grant.
				if !ok || definition.Kind == "equipment" {
					eq := s.Awarder.Equipment
					if eq != nil && eq.Source.Checksum == s.Awarder.Catalog.Source.Checksum {
						_, err := eq.Reward(item.Template)
						valid = err == nil
					}
				}
				if item.Template == 0 || item.Amount == 0 || !valid {
					return nil, empty, fmt.Errorf("invalid prepared legion product %d", item.Template)
				}
			}
		}
		plan := LegionRewardPlan{Run: run, Source: current.ConfigVersion, ContentChecksum: s.Awarder.Catalog.Source.Checksum, Content: content, Dungeon: dungeon, Eligible: true, Basic: basic, Additional: additional}
		progress.Pending[run] = plan
		next, err := saveLegionRewards(state, progress)
		return next, plan, err
	})
	if err == nil && (frozen.Run != run || frozen.Content != content || frozen.Dungeon != dungeon || frozen.Source != role.ConfigVersion || frozen.ContentChecksum != s.Awarder.Catalog.Source.Checksum) {
		err = fmt.Errorf("prepared legion receipt belongs to another source phase")
	}
	return saved, frozen, err
}
