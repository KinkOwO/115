package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/honeytime"
	"encoding/json"
	"fmt"
)

// HoneyTimeService 是活动 313（HoneyTime 每日在线奖励）的领取服务。
//
// 与 `AttendanceService`（331 签到）分开的理由和它当初独立出来一样：脚本、活动号、
// 状态键、领奖行号口径都不同，硬凑一条线只会让两边互相耦合。
//
// 与签到唯一的结构性差别：**这里的奖励档不是"每天一档"，而是"同一天里有几档在线时长门槛"**，
// 所以幂等键要带**档**（同一档今天只能领一次，明天可以再领一次）。
type HoneyTimeService struct {
	Store   *database.Store
	Catalog *honeytime.Catalog
}

// 轨号：面板上面那行"每天的"是 `TrackDaily`（0），下面那行"累计在线时长"是 `TrackPeriod`（1）。
const (
	TrackDaily  = 0
	TrackPeriod = 1
)

func trackName(track int) string {
	if track == TrackPeriod {
		return "period"
	}
	return "daily"
}

// HoneyTimeReceipt 是一次领取的结果，写进角色事件回执（可事后审计）。
type HoneyTimeReceipt struct {
	// Track 是哪条轨（0 = 日档，1 = 期间轨）。
	Track int `json:"track"`
	// Row 是客户端发来的原始行号（0 起），原样留档 —— 判"行号口径对不对"就靠它。
	Row int `json:"row"`
	// TierIndex 是折算出的脚本行首值（`Tier.Index`）。
	TierIndex  int                 `json:"tier_index"`
	Rewards    []honeytime.Reward  `json:"rewards"`
	MailID     int64               `json:"mail_id"`
	MailSender string              `json:"mail_sender"`
	MailText   string              `json:"mail_text"`
	State      honeytime.State     `json:"state"`
}

// Claim 领取行号 row 对应的那一档在线奖励。
//
// 返回 (更新后的角色, 回执, 本次是否真的发放, 错误)。`applied=false` 表示这是一次**重放**
// （同一事件键已经记过账）：调用方照常回成功应答、但不要再发一次奖励 —— 幂等由
// `SendSystemMailWithCharacterUpdate` 的事件账本保证（与签到/礼包同一条机制）。
//
// 校验都在**事务内、角色行加锁之后**（mutate 里）做，否则两条并发连接可以各领一次同一档。
func (s *HoneyTimeService) Claim(ctx context.Context, role database.Character, track, row int, today int64,
	cycleStart int64, sender, text string) (database.Character, HoneyTimeReceipt, bool, error) {
	var receipt HoneyTimeReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, receipt, false, fmt.Errorf("honeytime service unavailable")
	}
	isPeriod := track == TrackPeriod
	var tier honeytime.Tier
	var ok bool
	if isPeriod {
		tier, ok = s.Catalog.PeriodTierForClaim(row)
	} else {
		tier, ok = s.Catalog.TierForClaim(row)
	}
	if !ok {
		return role, receipt, false,
			fmt.Errorf("honeytime %s row %d maps to no tier in the source script", trackName(track), row)
	}
	rewards := []honeytime.Reward{tier.Reward}
	grants := []database.GrantItem{{Template: tier.Reward.Template, Amount: tier.Reward.Count}}
	// 附件容器取 0（普通背包）：在线奖励都是普通物品。将来若源里出现账号绑定/时装类，
	// 要按物品自己的归属改这里（届时先取源证据，不要猜）。
	assets, err := database.SystemMailAssets(0, grants)
	if err != nil {
		return role, receipt, false, err
	}

	// 事件键按「期 + 轨 + 日(仅日档) + 档」定：
	// 日档同一档今天只入账一次、跨天自然得到新键；期间轨是**整期累计**，键里**不带日**
	// （否则跨天会重复入账）。
	keyTrack := trackName(track)
	var key string
	if isPeriod {
		key = fmt.Sprintf("honeytime-313:%d:%s:%d", cycleStart, keyTrack, tier.Index)
	} else {
		key = fmt.Sprintf("honeytime-313:%d:%s:%d:%d", cycleStart, keyTrack, today, tier.Index)
	}
	mailID, applied, err := s.Store.SendSystemMailWithCharacterUpdate(ctx, role.AccountID, role.ID,
		role.ConfigVersion, key, sender, text, assets,
		func(current database.Character) (json.RawMessage, error) {
			st, e := honeytime.ReadState(current.State)
			if e != nil {
				return nil, e
			}
			st = st.Normalize(cycleStart, today)
			// 门槛按**累计时长**算（客户端 sub_143CC72A0 也是逐档相减）。
			// 档号是 **1 起**（走到了第几档），行号是 **0 起的行序**
			// ⇒ "这一行解锁了" = 档号 > 行号（即档号至少到 row+1）。
			seconds := st.Seconds
			var step int
			var already bool
			if isPeriod {
				seconds = st.PeriodSeconds
				step = s.Catalog.PeriodStepNumber(st.PeriodSeconds)
				already = st.PeriodClaimedTier(tier.Index)
			} else {
				step = s.Catalog.StepNumber(st.Seconds)
				already = st.ClaimedTier(tier.Index)
			}
			if step <= row {
				return nil, fmt.Errorf("honeytime %s row %d is not unlocked yet (%d seconds, step %d)",
					keyTrack, row, seconds, step)
			}
			if already {
				return nil, fmt.Errorf("honeytime %s tier %d is already claimed", keyTrack, tier.Index)
			}
			next := st
			var claimErr error
			if isPeriod {
				next, claimErr = st.ClaimPeriod(tier.Index)
			} else {
				next, claimErr = st.Claim(tier.Index)
			}
			if claimErr != nil {
				return nil, claimErr
			}
			return honeytime.WriteState(current.State, next)
		})
	if err != nil {
		return role, receipt, false, err
	}

	// 回执里的状态与返回给会话的角色都要**从库里读回来**：重放路径（applied=false）不会
	// 执行上面的 mutate，闭包里捕获的值在那条路上是零值。会话拿到的角色必须带新状态，
	// 否则随后推的 op1294 会把"已领"又抹回去。
	saved := role
	roster, err := s.Store.Characters(ctx, role.AccountID)
	if err != nil {
		return role, receipt, false, err
	}
	for _, cand := range roster {
		if cand.ID == role.ID {
			saved = cand
			break
		}
	}
	st, err := honeytime.ReadState(saved.State)
	if err != nil {
		return role, receipt, false, err
	}
	receipt = HoneyTimeReceipt{
		Track:      track,
		Row:        row,
		TierIndex:  tier.Index,
		Rewards:    rewards,
		MailID:     mailID,
		MailSender: sender,
		MailText:   text,
		State:      st.Normalize(cycleStart, today),
	}
	return saved, receipt, applied, nil
}
