package loot

import (
	"fmt"
	"sync"
)

// OmenLedger 按角色保存征兆累积数。
//
// 征兆是**跨场累积**的玩家状态（官方：「通关时随机累积」），所以它既不属于
// 掉落会话（Session 每场新建），也不能挂在副本状态上。
//
// 这本账只是**进程内缓存**：权威状态在角色存档里（character_omen_state，见
// internal/database/omen_state.go）。cmd/wireprobe 在每场开始时用存档刷新它、
// 结算后写回，所以重启不再归零。这里不直接持有 Store 是为了让掉落会话不知道
// 数据库的存在。
type OmenLedger struct {
	reward *AttunementRewards

	mu   sync.Mutex
	held map[int64]uint32
	last map[int64]OmenOutcome
	seq  uint64
}

// NewOmenLedger 建一本账。reward 为 nil 时整本是哑的（所有查询返回零值）。
func NewOmenLedger(reward *AttunementRewards) *OmenLedger {
	return &OmenLedger{
		reward: reward,
		held:   map[int64]uint32{},
		last:   map[int64]OmenOutcome{},
	}
}

// Enabled 报告这本账是否会推进：奖励表里至少有一个副本带征兆阶段。
func (o *OmenLedger) Enabled() bool {
	if o == nil || o.reward == nil {
		return false
	}
	for _, d := range o.reward.Dungeons() {
		if o.reward.OmenStagesCount(d) > 0 {
			return true
		}
	}
	return false
}

// Held 报告角色当前持有的征兆数。
func (o *OmenLedger) Held(character int64) uint32 {
	if o == nil {
		return 0
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.held[character]
}

// Set 直接写持有数（诊断用：把玩家放到指定阶段，省掉刷场次）。
func (o *OmenLedger) Set(character int64, held uint32) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.held[character] = held
}

// Last 返回角色最近一次结算，供调用方记事件。
func (o *OmenLedger) Last(character int64) (OmenOutcome, bool) {
	if o == nil {
		return OmenOutcome{}, false
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	v, ok := o.last[character]
	return v, ok
}

// Advance 推进一场通关：读持有数、按阶段表判定、写回新持有数。
//
// 返回的 awards 是**包装**，调用方必须走与固定/追加奖励同一条开箱路径展开 ——
// 走别的路就等于同一件东西有两个分布。
func (o *OmenLedger) Advance(character int64, dungeon, seed uint32) (OmenOutcome, []Award, error) {
	if o == nil || o.reward == nil {
		return OmenOutcome{Dungeon: dungeon, Seed: seed}, nil, nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	out, err := o.reward.AdvanceOmen(seed, dungeon, o.held[character])
	if err != nil {
		return out, nil, err
	}
	o.seq++
	out.Seq = o.seq
	o.held[character] = out.After
	o.last[character] = out
	return out, out.Awards, nil
}

// preview computes a candidate without consuming a ledger sequence or state.
func (o *OmenLedger) preview(character int64, dungeon, seed uint32) (OmenOutcome, []Award, error) {
	if o == nil || o.reward == nil {
		return OmenOutcome{Dungeon: dungeon, Seed: seed}, nil, nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	out, err := o.reward.AdvanceOmen(seed, dungeon, o.held[character])
	return out, out.Awards, err
}

// commit rejects concurrent changes rather than overwriting another clear.
func (o *OmenLedger) commit(character int64, out OmenOutcome) error {
	if o == nil || o.reward == nil {
		return nil
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	// AdvanceOmen clamps invalid held values; compare using that same rule.
	held := o.held[character]
	if n := o.reward.OmenStagesCount(out.Dungeon); n > 0 && int(held) >= n {
		held = uint32(n - 1)
	}
	if held != out.Held {
		return fmt.Errorf("omen state changed before drop publication")
	}
	o.seq++
	out.Seq = o.seq
	o.held[character] = out.After
	o.last[character] = out
	return nil
}
