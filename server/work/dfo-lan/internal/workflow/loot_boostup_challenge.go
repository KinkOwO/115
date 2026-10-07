package workflow

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"dfolan/internal/database"
	"encoding/json"
	"errors"
	"fmt"
)

var errChallengeUnchanged = errors.New("challenge state unchanged")

type BoostChallengeFacts func(database.Character) (byte, uint32, error)

// ReconcileBoostChallenge 把「毕业+等级/名望」事实写入挑战解锁状态；
// 事务编排在 workflow（§7.2 E13），解锁判定是纯函数 c.UnlockChallenges。
func (s *LootService) ReconcileBoostChallenge(ctx context.Context, role database.Character, c *boostup.Catalog, facts BoostChallengeFacts) (database.Character, bool, error) {
	if c == nil || len(c.Challenges) == 0 || facts == nil {
		return role, false, nil
	}
	if s == nil || s.Loot == nil || s.Store == nil {
		return role, false, fmt.Errorf("challenge store missing")
	}
	st, e := boostup.ReadState(role.State)
	if e != nil {
		return role, false, e
	}
	if !st.Activated || !st.Training.Finished || st.Training.Step <= 1 {
		return role, false, nil
	}
	last := st.Training.Step - 1
	// 已领事实就是毕业证明：Training.Claimed[last] 与最后一步的发放写在同一个
	// 事务里（loot.PrepareBoostStep / PrepareBoostAutoStep 都由角色行锁下的事件
	// 事务提交），发放没落地时这个标记也不会落地。不再另读一份发放回执。
	if !st.Training.Claimed[last] {
		return role, false, fmt.Errorf("challenge enrollment requires completed training")
	}
	key := fmt.Sprintf("boost-challenge-sync:%x", sha256.Sum256(role.State))
	unchanged := role
	next, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, "boost-challenge-unlock-v1", func(cur database.Character) (json.RawMessage, json.RawMessage, error) {
		current, err := boostup.ReadState(cur.State)
		if err != nil {
			return nil, nil, err
		}
		if !current.Activated || !current.Training.Finished || current.Training.Step != st.Training.Step || !current.Training.Claimed[last] {
			return nil, nil, fmt.Errorf("graduation changed")
		}
		level, fame, err := facts(cur)
		if err != nil {
			return nil, nil, err
		}
		unlocked, changed, err := c.UnlockChallenges(current.Challenge, level, fame)
		if err != nil {
			return nil, nil, err
		}
		if !changed {
			unchanged = cur
			unchanged.WireID = role.WireID
			return nil, nil, errChallengeUnchanged
		}
		current.Challenge = unlocked
		raw, err := boostup.WriteState(cur.State, current)
		if err != nil {
			return nil, nil, err
		}
		receipt, err := json.Marshal(map[string]any{"level": level, "fame": fame, "unlocked": unlocked.Rows})
		return raw, receipt, err
	})
	if errors.Is(e, errChallengeUnchanged) {
		return unchanged, false, nil
	}
	if e != nil {
		return role, false, e
	}
	next.WireID = role.WireID
	return next, applied, nil
}

// Mail and the reward flag/counter share one transaction: the item rows are
// built by the mailbox owner (database.SystemMailAssets) and the claim flag
// commits with the mail insert, so a retried request can never pay twice.
// The transport-derived request key is the idempotency gate.
func (s *LootService) ClaimBoostChallenge(ctx context.Context, role database.Character, c *boostup.Catalog, r protocol.BoostChallengeRequest115, key string) (database.Character, bool, error) {
	if c == nil || len(c.Challenges) == 0 {
		return role, false, fmt.Errorf("challenge source unavailable")
	}
	if s == nil || s.Loot == nil || s.Store == nil {
		return role, false, fmt.Errorf("challenge store missing")
	}
	st, e := boostup.ReadState(role.State)
	if e != nil {
		return role, false, e
	}
	if !st.Activated || !st.Training.Finished {
		return role, false, fmt.Errorf("challenge requires graduated event character")
	}
	d, e := c.Challenge(r.Index)
	if e != nil {
		return role, false, e
	}
	rewards := d.UnlockRewards
	if r.Action == 1 {
		rewards = d.ClearRewards
	}
	if r.Action > 1 {
		return role, false, fmt.Errorf("unsupported challenge action %d", r.Action)
	}
	var grants []database.GrantItem
	for _, g := range rewards {
		item, ok := s.Loot.Catalog.Items[g.Item]
		if !ok || item.Kind != "stackable" {
			return role, false, fmt.Errorf("challenge mail item %d unavailable", g.Item)
		}
		grants = append(grants, database.GrantItem{Template: g.Item, Amount: g.Count})
	}
	assets, e := database.SystemMailAssets(0, grants)
	if e != nil {
		return role, false, e
	}
	mailKey := fmt.Sprintf("boost-challenge:%d:%d:%x", r.Index, r.Action, sha256.Sum256([]byte(key)))
	_, sent, e := s.Store.SendSystemMailWithCharacterUpdate(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), mailKey, "Starter Boost", "Starter Boost Challenge reward.", assets, func(cur database.Character) (json.RawMessage, error) {
		st, err := boostup.ReadState(cur.State)
		if err != nil {
			return nil, err
		}
		if !st.Activated || !st.Training.Finished {
			return nil, fmt.Errorf("challenge character changed")
		}
		st.Challenge, _, err = c.ClaimChallenge(st.Challenge, r.Index, byte(r.Action))
		if err != nil {
			return nil, err
		}
		return boostup.WriteState(cur.State, st)
	})
	if e != nil {
		return role, false, e
	}
	// 本树的 storage 只有名单读取（Characters），没有单角色 getter：领奖后按账号取回
	// 刚提交的那一份，WireID 沿用连接侧的值。
	roles, e := s.Store.Characters(ctx, role.AccountID)
	if e != nil {
		return role, sent, e
	}
	var next database.Character
	for _, r := range roles {
		if r.ID == role.ID {
			next = r
			break
		}
	}
	if next.ID == 0 {
		return role, sent, fmt.Errorf("challenge character disappeared")
	}
	next.WireID = role.WireID
	return next, sent, nil
}
