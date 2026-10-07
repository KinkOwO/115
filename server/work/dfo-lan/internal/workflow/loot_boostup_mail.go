package workflow

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/database"
	"encoding/json"
	"fmt"
)

// Frozen intent is saved with capsule activation. Delivery and marking it
// complete are recoverable operations: the existing system-mail receipt is
// authoritative even if the process stops between those two commits.
// The bool reports a newly SETTLED promise, so recovery can notify an online
// client even when the mail insert already committed before a lost response.
// （§7.2 E13：两段提交+邮寄编排归 workflow；清单校验仍是纯函数。）
func (s *LootService) DeliverBoostGraduationMail(ctx context.Context, role database.Character, c *boostup.Catalog) (database.Character, bool, error) {
	return s.deliverBoostMail(ctx, role, c, false)
}

func (s *LootService) DeliverBoostLevelBonusMail(ctx context.Context, role database.Character, c *boostup.Catalog) (database.Character, bool, error) {
	return s.deliverBoostMail(ctx, role, c, true)
}

func boostMailPointers(st *boostup.State, bonus bool) (**boostup.GraduationMail, *bool) {
	if bonus {
		return &st.PendingLevelBonus, &st.LevelBonusSent
	}
	return &st.PendingMail, &st.MailSent
}

func (s *LootService) deliverBoostMail(ctx context.Context, role database.Character, c *boostup.Catalog, bonus bool) (database.Character, bool, error) {
	if s == nil || s.Loot == nil || s.Store == nil || c == nil {
		return role, false, nil
	}
	actor := role.WireID
	st, e := boostup.ReadState(role.State)
	if e != nil {
		return role, false, e
	}
	pending, done := boostMailPointers(&st, bonus)
	source := c.ReservedMail
	reserveKey, mailKey, doneKey := "boostup-mail-reserve", "boostup-max-level-v1", "boostup-mail-delivered"
	if bonus {
		var level struct {
			Level byte `json:"level"`
		}
		if e = json.Unmarshal(role.State, &level); e != nil {
			return role, false, e
		}
		source = c.CapsuleLevelMail(level.Level)
		reserveKey, mailKey, doneKey = "boostup-level-bonus-reserve", "boostup-challenge-level-v1", "boostup-level-bonus-delivered"
	}
	if st.Training.Step <= 1 {
		return role, false, nil
	}
	last := st.Training.Step - 1 // the completed generation, not a later edited PVF's step count
	if !st.Activated || !st.Training.Finished || !st.Training.Claimed[last] || *done {
		return role, false, nil
	}
	if *pending == nil && source == nil {
		return role, false, nil
	}
	// 发放事实由 Training.Claimed[last] 承担：它与最后一步的发放同事务提交
	// （loot.PrepareBoostAutoStep 在 CommitAccountMaterialEvent 的角色行锁里跑），
	// 发放没落地这个标记就不会落地，所以这里不再读一份发放回执来反推。
	if *pending == nil {
		intent := *source
		if e = intent.Validate(); e != nil {
			return role, false, e
		}
		role, _, e = s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), reserveKey, "boostup-mail-reserve-v1", func(cur database.Character) (json.RawMessage, json.RawMessage, error) {
			state, err := boostup.ReadState(cur.State)
			if err != nil {
				return nil, nil, err
			}
			if !state.Activated || !state.Training.Finished || !state.Training.Claimed[last] {
				return nil, nil, fmt.Errorf("graduation state changed")
			}
			p, d := boostMailPointers(&state, bonus)
			if *p == nil && !*d {
				*p = &intent
			}
			raw, err := boostup.WriteState(cur.State, state)
			if err != nil {
				return nil, nil, err
			}
			receipt, err := json.Marshal(intent)
			return raw, receipt, err
		})
		if e != nil {
			return role, false, e
		}
		role.WireID = actor
		st, e = boostup.ReadState(role.State)
		if e != nil {
			return role, false, e
		}
		pending, done = boostMailPointers(&st, bonus)
	}
	if *done {
		return role, false, nil
	}
	if *pending == nil {
		return role, false, fmt.Errorf("graduation mail intent missing after reserve")
	}
	intent := **pending
	if e = intent.Validate(); e != nil {
		return role, false, e
	}
	item, ok := s.Loot.Catalog.Items[intent.Item]
	if !ok || item.Kind != "stackable" {
		return role, false, fmt.Errorf("graduation mail item unavailable in owned catalog")
	}
	assets, e := database.SystemMailAssets(0, []database.GrantItem{{Template: intent.Item, Amount: intent.Count}})
	if e != nil {
		return role, false, e
	}
	mail, _, e := s.Store.CommitSystemMail(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), mailKey, "system-mail-v1", intent.Sender, intent.Body, assets)
	if e != nil {
		return role, false, e
	}
	updated, settled, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), doneKey, "boostup-mail-delivered-v1", func(cur database.Character) (json.RawMessage, json.RawMessage, error) {
		state, err := boostup.ReadState(cur.State)
		if err != nil {
			return nil, nil, err
		}
		p, d := boostMailPointers(&state, bonus)
		if *p != nil && **p != intent {
			return nil, nil, fmt.Errorf("graduation intent changed during delivery")
		}
		*p = nil
		*d = true
		raw, err := boostup.WriteState(cur.State, state)
		if err != nil {
			return nil, nil, err
		}
		receipt, err := json.Marshal(mail)
		return raw, receipt, err
	})
	if e != nil {
		return role, false, e
	}
	updated.WireID = actor
	return updated, settled, nil
}
