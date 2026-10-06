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
	var out protocol.BoostChallengeState115
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
