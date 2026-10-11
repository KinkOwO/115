package workflow

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/dungeon"
	"dfolan/internal/loot"
	"dfolan/internal/database"
	"encoding/json"
	"fmt"
)

// Entry handlers must first verify the operational event is open and resolve
// g from the loaded PVF. Reuse the established SQL state+receipt transaction.
// Full bags roll back the claim; there is no unverified mail fallback.
// （§7.2 E13：事务编排归 workflow，纯领奖计算在 loot.PrepareBoostGift。）
func (s *LootService) ClaimBoostGift(ctx context.Context, role database.Character, g boostup.Gift) (database.Character, loot.BoostGiftReceipt, bool, error) {
	var out loot.BoostGiftReceipt
	if s == nil || s.Loot == nil || s.Store == nil {
		return role, out, false, fmt.Errorf("gift store unavailable")
	}
	key := fmt.Sprintf("boostup-gift:%d", g.ID)
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, "boostup-gift-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		raw, receipt, err := s.Loot.PrepareBoostGift(LootRole(current), g)
		if err != nil {
			return nil, nil, err
		}
		b, err := json.Marshal(receipt)
		return raw, b, err
	})
	if e != nil {
		return role, out, false, e
	}
	b, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return role, out, false, e
	}
	if e = json.Unmarshal(b, &out); e != nil {
		return role, out, false, e
	}
	if out.Gift != g.ID || len(out.Items) == 0 || len(out.Items) != len(out.Slots) {
		return role, out, false, fmt.Errorf("invalid gift receipt")
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

func (s *LootService) BoostStepRequest(ctx context.Context, role database.Character, c *boostup.Catalog, step byte, claim bool) (database.Character, loot.BoostStepReceipt, bool, error) {
	var out loot.BoostStepReceipt
	if s == nil || s.Loot == nil || s.Store == nil {
		return role, out, false, fmt.Errorf("boost step storage missing")
	}
	if claim && c != nil && step > 0 && int(step) <= len(c.Steps) && c.Steps[step-1].AutoOpen {
		return s.boostAutoStep(ctx, role, c, step)
	}
	key := fmt.Sprintf("boostup-guide:%d", step)
	if claim {
		key = fmt.Sprintf("boostup-reward:%d", step)
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, "boostup-step-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		raw, r, e := s.Loot.PrepareBoostStep(LootRole(current), c, step, claim)
		if e != nil {
			return nil, nil, e
		}
		b, e := json.Marshal(r)
		return raw, b, e
	})
	if e != nil {
		return role, out, false, e
	}
	b, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return role, out, false, e
	}
	if e = json.Unmarshal(b, &out); e != nil {
		return role, out, false, e
	}
	// 普通领奖：一条奖励行至少落一个槽位。自动开盒那两步不在这里校验，
	// 它们在 boostAutoStep 里按账户材料事务单独收口。
	if out.Step != step || out.Claim != claim || out.AutoOpened || claim && (len(out.Rewards) == 0 || len(out.Slots) < len(out.Rewards)) {
		return role, out, false, fmt.Errorf("boost step receipt mismatch")
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

// CompleteBoostGuide 记录「教学副本真正通关」这一事实（step 归属校验在事务内）。
func (s *LootService) CompleteBoostGuide(ctx context.Context, role database.Character, c *boostup.Catalog, expected uint32, run *dungeon.Session) (database.Character, bool, error) {
	if s == nil || s.Loot == nil || s.Store == nil || c == nil || run == nil || !run.Completed() || run.RunID == "" || run.Definition.ID != expected {
		return role, false, fmt.Errorf("boost guide lacks actual completed instance")
	}
	st, e := boostup.ReadState(role.State)
	if e != nil {
		return role, false, e
	}
	step := st.Training.Step
	next, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), fmt.Sprintf("boostup-guide-clear:%d", step), "boostup-guide-clear-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		state, e := boostup.ReadState(current.State)
		if e != nil {
			return nil, nil, e
		}
		if !state.Activated || state.Training.Step != step {
			return nil, nil, fmt.Errorf("training owner/step changed")
		}
		state.Training, e = c.GuideDungeonCleared(state.Training, expected, run.Definition.ID)
		if e != nil {
			return nil, nil, e
		}
		raw, e := boostup.WriteState(current.State, state)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(map[string]any{"step": step, "run": run.RunID, "dungeon": run.Definition.ID})
		return raw, receipt, e
	})
	next.WireID = role.WireID
	return next, applied, e
}
