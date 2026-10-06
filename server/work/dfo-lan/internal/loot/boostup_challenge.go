package loot

import (
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
)

// BoostChallengeSnapshot 只读取角色状态里的挑战进度（纯投影，无存储事务）。
// 挑战的对账与领取事务在 workflow.LootService
// （ReconcileBoostChallenge / ClaimBoostChallenge，§7.2 E13）。
func BoostChallengeSnapshot(c *boostup.Catalog, role Role) ([]byte, error) {
	st, e := boostup.ReadState(role.State)
	if e != nil {
		return nil, e
	}
	// Rows 必须先建好：`var out` 的 map 是 nil，写第一条挑战行就 panic。
	// 实机 2026-10-06 15:34:49（会话 ..._20261006_153257_795354_next37）第 11 关领奖
	// `960200000b000000` → `connection_panic_recovered: assignment to entry in nil map`
	// → 掉线。旧端把 665 门控着，这条投影从未跑到有行的一次；665 按源常开后立刻暴露。
	out := protocol.BoostChallengeState115{Rows: map[byte]protocol.BoostChallengeRow115{}}
	if st.Challenge != nil && st.Challenge.Enrolled {
		if e = c.ValidateChallengeState(st.Challenge); e != nil {
			return nil, e
		}
		out.Enrolled = true
		if st.LevelBonusSent {
			out.LevelMarker = uint16(c.GoalLevel)
		}
		for id, p := range st.Challenge.Rows {
			out.Rows[id] = protocol.BoostChallengeRow115{Unlocked: p.Unlocked, UnlockClaimed: p.UnlockClaimed, Progress: p.Progress, Claims: p.Claims}
		}
	}
	return protocol.BoostChallengeStatus115(out)
}
