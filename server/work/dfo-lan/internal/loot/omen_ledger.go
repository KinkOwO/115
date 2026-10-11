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
	misses map[int64]uint32
	last map[int64]OmenOutcome
	seq  uint64
}

// NewOmenLedger 建一本账。reward 为 nil 时整本是哑的（所有查询返回零值）。
func NewOmenLedger(reward *AttunementRewards) *OmenLedger {
	return &OmenLedger{
		reward: reward,
		held:   map[int64]uint32{},
		misses: map[int64]uint32{},
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
//
// 计数一并归 0：把玩家摆到某个阶段，语义上等价于「刚拿到那个征兆的那一刻」。
func (o *OmenLedger) Set(character int64, held uint32) {
	o.SetState(character, held, 0)
}

// SetState 直接写这本书的两个数：进本装载存档（loadOmenRunState）与诊断都走它。
func (o *OmenLedger) SetState(character int64, held, misses uint32) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.held[character] = held
	o.misses[character] = misses
}

// Misses 报告角色当前「连续未触发」的计数（0 表示刚拿到过征兆）。
func (o *OmenLedger) Misses(character int64) uint32 {
	if o == nil {
		return 0
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.misses[character]
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
	out, err := o.reward.AdvanceOmen(seed, dungeon, o.held[character], o.misses[character])
	if err != nil {
		return out, nil, err
	}
	o.seq++
	out.Seq = o.seq
	o.held[character] = out.After
	o.misses[character] = out.MissesAfter
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
	out, err := o.reward.AdvanceOmen(seed, dungeon, o.held[character], o.misses[character])
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
	// 计数同理：预览之后、发布之前没人该动过它。
	if misses := o.misses[character]; misses != out.Misses {
		return fmt.Errorf("omen miss counter changed before drop publication")
	}
	o.seq++
	out.Seq = o.seq
	o.held[character] = out.After
	o.misses[character] = out.MissesAfter
	o.last[character] = out
	return nil
}
