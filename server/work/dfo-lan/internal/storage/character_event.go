package storage

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	"time"
)

func (s *Store) MigrateCharacterEvents(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_events (
 character_id bigint NOT NULL REFERENCES characters(id), event_key text NOT NULL,
 config_version text NOT NULL, model text NOT NULL, outcome jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(character_id,event_key));`)
	return e
}

func (s *Store) CharacterEventReceipt(ctx context.Context, account, id int64, key string) (json.RawMessage, error) {
	var p json.RawMessage
	e := s.DB.QueryRow(ctx, `SELECT e.outcome FROM character_events e JOIN characters c ON c.id=e.character_id WHERE c.account_id=$1 AND c.id=$2 AND e.event_key=$3`, account, id, key).Scan(&p)
	return p, e
}

// CommitCharacterEvent owns atomic state/receipt persistence. Its callback is
// pure domain code and runs only under the owning character's PostgreSQL lock.
// Retries never recompute a reward using a later level or changed rules.
func (s *Store) CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(Character) (json.RawMessage, json.RawMessage, error)) (Character, bool, error) {
	return s.commitCharacterEvent(ctx, account, id, version, key, model, apply, nil)
}

// CommitCharacterPremiumEvent 将背包变化、账号契约续期和幂等回执放在同一事务中。
func (s *Store) CommitCharacterPremiumEvent(ctx context.Context, account, id int64, version, key, model string, apply func(Character) (json.RawMessage, json.RawMessage, []CashPremiumActivation, error)) (Character, bool, error) {
	if apply == nil {
		return Character{}, false, fmt.Errorf("角色契约事件缺少处理函数")
	}
	var rewards []CashPremiumActivation
	return s.commitCharacterEvent(ctx, account, id, version, key, model, func(role Character) (json.RawMessage, json.RawMessage, error) {
		state, outcome, premiums, err := apply(role)
		rewards = premiums
		return state, outcome, err
	}, func() []CashPremiumActivation { return rewards })
}

func (s *Store) commitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(Character) (json.RawMessage, json.RawMessage, error), premiums func() []CashPremiumActivation) (Character, bool, error) {
	var role Character
	decoded, e := hex.DecodeString(version)
	if e != nil || len(decoded) != 32 || key == "" || len(key) > 200 || model == "" || len(model) > 100 || apply == nil {
		return role, false, fmt.Errorf("invalid character event")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return role, false, e
	}
	defer tx.Rollback(ctx)
	if premiums != nil {
		// 与商城相同：先锁账号货币行，再锁角色，串行化跨角色的契约续期。
		if _, e = tx.Exec(ctx, `INSERT INTO account_currency(account_id,cera) VALUES($1,0) ON CONFLICT DO NOTHING`, account); e != nil {
			return role, false, e
		}
		var lockedAccount int64
		if e = tx.QueryRow(ctx, `SELECT account_id FROM account_currency WHERE account_id=$1 FOR UPDATE`, account).Scan(&lockedAccount); e != nil {
			return role, false, e
		}
	}
	e = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if e != nil {
		return role, false, e
	}
	if role.ConfigVersion != version {
		return role, false, fmt.Errorf("character event source mismatch")
	}
	var prior string
	e = tx.QueryRow(ctx, `SELECT model FROM character_events WHERE character_id=$1 AND event_key=$2`, id, key).Scan(&prior)
	if e == nil {
		if prior != model {
			return role, false, fmt.Errorf("character event model mismatch")
		}
		return role, false, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return role, false, e
	}
	state, outcome, e := apply(role)
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
			var oldEnd int64
			e = tx.QueryRow(ctx, `SELECT end_time FROM account_premiums WHERE account_id=$1 AND premium_type=$2 FOR UPDATE`, account, reward.Type).Scan(&oldEnd)
			if e != nil && !errors.Is(e, pgx.ErrNoRows) {
				return role, false, e
			}
			base := max(now, oldEnd)
			if base > math.MaxInt64-reward.DurationSecond {
				return role, false, fmt.Errorf("契约到期时间溢出")
			}
			end := base + reward.DurationSecond
			if _, e = tx.Exec(ctx, `INSERT INTO account_premiums(account_id,premium_type,end_time,updated_at) VALUES($1,$2,$3,now()) ON CONFLICT(account_id,premium_type) DO UPDATE SET end_time=EXCLUDED.end_time,updated_at=now()`, account, reward.Type, end); e != nil {
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
	_, e = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,$4,$5)`, id, key, version, model, outcome)
	if e != nil {
		return role, false, e
	}
	_, e = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state)
	if e != nil {
		return role, false, e
	}
	if e = tx.Commit(ctx); e != nil {
		return role, false, e
	}
	role.State = state
	s.Cache.Del(ctx, fmt.Sprintf("%scharacters:%d", s.prefix, account))
	return role, true, nil
}
