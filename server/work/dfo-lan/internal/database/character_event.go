package database

import (
	"bytes"
	"context"
	"dfolan/internal/adventure"
	"dfolan/internal/database/sqlcgen"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

func (s *Store) MigrateCharacterEvents(ctx context.Context) error {
	return s.execMigration(ctx, "0002_character_events.sql")
}

func (s *Store) CharacterEventReceipt(ctx context.Context, account, id int64, key string) (json.RawMessage, error) {
	receipt, err := s.queries.CharacterEventReceipt(ctx, sqlcgen.CharacterEventReceiptParams{
		AccountID: account, CharacterID: id, EventKey: key,
	})
	return receipt, storageError(err)
}

// CommitCharacterEvent owns atomic state/receipt persistence. Its callback is
// pure domain code and runs only under the owning character's PostgreSQL lock.
// Retries never recompute a reward using a later level or changed rules.
func (s *Store) CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(Character) (json.RawMessage, json.RawMessage, error)) (Character, bool, error) {
	return s.commitCharacterEvent(ctx, account, id, version, key, model, apply, nil, nil)
}

// CommitCharacterPremiumEvent 将背包变化、账号契约续期和幂等回执放在同一事务中。
// CommitCharacterEventTx 与 CommitCharacterEvent 相同，但把事务句柄交给 apply。
//
// 用于「除了改角色状态，还要在**同一事务**里读写另一张表」的场景 —— NPC 商店限购就是：
// 校验已购次数 + 记录本次购买必须与背包变更原子完成，否则会出现「货到手但次数没记」，
// 或者重发请求重复计数。
//
// apply 在角色行已被 FOR UPDATE 锁住时执行，且只在**首次**应用时调用
// （重发的同 key 请求走幂等回执路径，不会重跑 apply）。
func (s *Store) CommitCharacterEventTx(ctx context.Context, account, id int64, version, key, model string,
	apply func(*Tx, Character) (json.RawMessage, json.RawMessage, error)) (Character, bool, error) {
	if apply == nil {
		return Character{}, false, fmt.Errorf("character event 缺少处理函数")
	}
	return s.commitCharacterEvent(ctx, account, id, version, key, model, nil, apply, nil)
}

func (s *Store) CommitCharacterPremiumEvent(ctx context.Context, account, id int64, version, key, model string, apply func(Character) (json.RawMessage, json.RawMessage, []CashPremiumActivation, error)) (Character, bool, error) {
	if apply == nil {
		return Character{}, false, fmt.Errorf("角色契约事件缺少处理函数")
	}
	var rewards []CashPremiumActivation
	return s.commitCharacterEvent(ctx, account, id, version, key, model, func(role Character) (json.RawMessage, json.RawMessage, error) {
		state, outcome, premiums, err := apply(role)
		rewards = premiums
		return state, outcome, err
	}, nil, func() []CashPremiumActivation { return rewards })
}

func (s *Store) commitCharacterEvent(ctx context.Context, account, id int64, version, key, model string,
	apply func(Character) (json.RawMessage, json.RawMessage, error),
	txApply func(*Tx, Character) (json.RawMessage, json.RawMessage, error),
	premiums func() []CashPremiumActivation) (Character, bool, error) {
	var role Character
	decoded, e := hex.DecodeString(version)
	if apply == nil && txApply == nil {
		return role, false, fmt.Errorf("character event 缺少处理函数")
	}
	if e != nil || len(decoded) != 32 || key == "" || len(key) > 200 || model == "" || len(model) > 100 || (apply == nil && txApply == nil) {
		return role, false, fmt.Errorf("invalid character event")
	}
	tx, e := s.engine.begin(ctx)
	if e != nil {
		return role, false, e
	}
	defer tx.rollback(ctx)
	q := tx.queries()
	// Callback operations may share account state across characters.
	// Lock accounts before characters, matching roster/archival order.
	if txApply != nil {
		if _, e = q.LockAccountState(ctx, account); e != nil {
			return role, false, e
		}
	}
	if premiums != nil {
		// 与商城相同：先锁账号货币行，再锁角色，串行化跨角色的契约续期。
		if e = q.EnsureAccountCurrency(ctx, account); e != nil {
			return role, false, e
		}
		if _, e = q.LockAccountCurrency(ctx, account); e != nil {
			return role, false, e
		}
	}
	role, e = lockCharacter(ctx, tx, account, id)
	if e != nil {
		return role, false, e
	}
	if role.ConfigVersion != version {
		return role, false, fmt.Errorf("character event source mismatch")
	}
	// 迷雾誓约按账号共享。沿用角色→账号的既有锁顺序，在业务回调前
	// 取同账号最新状态，避免两个角色同时通关/领奖造成经验覆盖或重复领奖。
	var accountProfile AccountAdventure
	if s.adventureEnabled {
		accountProfile, e = lockAdventure(ctx, tx, role)
		if e != nil {
			return role, false, e
		}
		role.State, e = adventure.SaveSeason(role.State, accountProfile.Data.SeasonLevel)
		if e != nil {
			return role, false, e
		}
	}
	prior, e := tx.queries().CharacterEventModel(ctx, sqlcgen.CharacterEventModelParams{CharacterID: id, EventKey: key})
	if e == nil {
		if prior != model {
			return role, false, fmt.Errorf("character event model mismatch")
		}
		return role, false, tx.commit(ctx)
	}
	if !isNoRows(e) {
		return role, false, e
	}
	var state, outcome json.RawMessage
	if txApply != nil {
		state, outcome, e = txApply(newTx(tx.queries(), account, id), role)
	} else {
		state, outcome, e = apply(role)
	}
	if e != nil {
		return role, false, e
	}
	if !json.Valid(state) || !json.Valid(outcome) {
		return role, false, fmt.Errorf("invalid event JSON")
	}
	if premiums != nil {
		now := time.Now().Unix()
		var activated []CashPremium
		for _, reward := range premiums() {
			if reward.Type == 0 || reward.DurationSecond <= 0 {
				return role, false, fmt.Errorf("契约奖励类型或时长无效")
			}
			oldEnd, err := q.LockPremiumExpiry(ctx, sqlcgen.LockPremiumExpiryParams{AccountID: account, PremiumType: int16(reward.Type)})
			e = err
			if e != nil && !isNoRows(e) {
				return role, false, e
			}
			base := max(now, oldEnd)
			if base > math.MaxInt64-reward.DurationSecond {
				return role, false, fmt.Errorf("契约到期时间溢出")
			}
			end := base + reward.DurationSecond
			if e = q.SavePremiumExpiry(ctx, sqlcgen.SavePremiumExpiryParams{AccountID: account, PremiumType: int16(reward.Type), EndTime: end}); e != nil {
				return role, false, e
			}
			activated = append(activated, CashPremium{Type: reward.Type, EndTime: end, RemainingSecond: end - now})
		}
		if len(activated) > 0 {
			var fields map[string]json.RawMessage
			if e = json.Unmarshal(outcome, &fields); e != nil || fields == nil {
				return role, false, fmt.Errorf("契约事件回执必须为对象")
			}
			fields["premiums"], e = json.Marshal(activated)
			if e != nil {
				return role, false, e
			}
			outcome, e = json.Marshal(fields)
			if e != nil {
				return role, false, e
			}
		}
	}
	e = tx.queries().RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{CharacterID: id, EventKey: key, ConfigVersion: version, Model: model, Outcome: outcome})
	if e != nil {
		return role, false, e
	}
	if s.adventureEnabled {
		profileChanged := false
		if strings.HasPrefix(key, "clear:") {
			// 只消费服务端通关回调生成的判定；回执重放在上面已经返回，不能重复计数。
			var clear struct {
				Recommended bool `json:"recommended_dungeon_clear"`
			}
			if e = json.Unmarshal(outcome, &clear); e != nil {
				return role, false, e
			}
			if clear.Recommended && accountProfile.Data.RecommendedDungeonClears < math.MaxUint32 {
				accountProfile.Data.RecommendedDungeonClears++
				profileChanged = true
			}
		}
		// 换装等旧回调可能重新组织角色JSON；不能让缺省字段清空账号共享进度。
		mutatesSeason := model == "season-level-v1" || strings.HasPrefix(key, "clear:") || strings.HasPrefix(key, "consume:")
		if mutatesSeason {
			previous, _ := json.Marshal(accountProfile.Data.SeasonLevel)
			nextSeason, err := adventure.ReadSeason(state)
			if err != nil {
				return role, false, err
			}
			updated, _ := json.Marshal(nextSeason)
			if !bytes.Equal(previous, updated) {
				accountProfile.Data.SeasonLevel = nextSeason
				profileChanged = true
			}
		} else {
			state, e = adventure.SaveSeason(state, accountProfile.Data.SeasonLevel)
			if e != nil {
				return role, false, e
			}
		}
		if profileChanged {
			if e = saveAdventure(ctx, tx, account, accountProfile); e != nil {
				return role, false, e
			}
		}
	}
	if e = s.commitAdventureExperience(ctx, tx, role, state); e != nil {
		return role, false, e
	}
	e = tx.queries().UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state})
	if e != nil {
		return role, false, e
	}
	if e = tx.commit(ctx); e != nil {
		return role, false, e
	}
	role.State = state
	return role, true, nil
}
