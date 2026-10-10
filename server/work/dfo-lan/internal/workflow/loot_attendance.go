package workflow

import (
	"context"
	"dfolan/internal/attendance"
	"dfolan/internal/database"
	"encoding/json"
	"fmt"
)

// AttendanceService 是活动 331（每日签到）的领取服务。
//
// 与 `LootService` 分开：领取只依赖「角色存档 + 系统邮件」，不需要掉落/物品目录，
// 硬凑进 LootService 会让签到无谓地依赖 loot 目录的存在。
type AttendanceService struct {
	Store   *database.Store
	Catalog *attendance.Catalog
}

// AttendanceReceipt 是一次领取的结果，写进角色事件回执（可事后审计）。
type AttendanceReceipt struct {
	Day        int                 `json:"day"`
	ClaimedDay int64               `json:"claimed_day"`
	Rewards    []attendance.Reward `json:"rewards"`
	MailID     int64               `json:"mail_id"`
	MailTitle  string              `json:"mail_title_key"`
	MailBody   string              `json:"mail_message_key"`
	MailSender string              `json:"mail_sender"`
	MailText   string              `json:"mail_text"`
	State      attendance.State    `json:"state"`
}

// Claim 领取第 day 天（0 起）的签到奖励。
//
// 返回 (更新后的角色, 回执, 本次是否真的发放, 错误)。`applied=false` 表示这是一次
// **重放**（同一事件键已经记过账），调用方照常回成功应答、但不要再发一次奖励 ——
// 幂等由 `CommitCharacterEvent` 的事件账本保证（与 boostup 的礼包/关卡领取同一条机制）。
//
// 校验都在**事务内、角色行加锁之后**做（sendSystemMailWithCharacterUpdate 的 mutate
// 里），否则两个并发连接可以各自通过校验、各领一次同一天的奖励。
func (s *AttendanceService) Claim(ctx context.Context, role database.Character, day int, today int64,
	cycleStart int64, sender, text string) (database.Character, AttendanceReceipt, bool, error) {
	var receipt AttendanceReceipt
	if s == nil || s.Store == nil || s.Catalog == nil {
		return role, receipt, false, fmt.Errorf("attendance service unavailable")
	}
	if day < 0 || day >= attendance.MaxDays {
		return role, receipt, false, fmt.Errorf("attendance day %d out of range", day)
	}
	rewards, ok := s.Catalog.DayRewards(day)
	if !ok || len(rewards) == 0 {
		return role, receipt, false, fmt.Errorf("attendance day %d has no rewards in the source script", day)
	}
	grants := make([]database.GrantItem, 0, len(rewards))
	for _, r := range rewards {
		grants = append(grants, database.GrantItem{Template: r.Template, Amount: r.Count})
	}
	// 附件的容器字节取 0（普通背包）：签到奖励都是普通可堆叠物品；将来若源里出现
	// 账号绑定/时装类，要按物品自己的归属改这里（届时先取源证据，不要猜）。
	assets, err := database.SystemMailAssets(0, grants)
	if err != nil {
		return role, receipt, false, err
	}

	// 事件键按「期 + 天」定：同一期的同一天只会入账一次；换期（cycleStart 变）后
	// 新的一天自然得到新键。
	key := fmt.Sprintf("attendance-331:%d:%d", cycleStart, day)
	mailID, applied, err := s.Store.SendSystemMailWithCharacterUpdate(ctx, role.AccountID, role.ID,
		role.ConfigVersion, key, sender, text, assets,
		func(current database.Character) (json.RawMessage, error) {
			st, e := attendance.ReadState(current.State)
			if e != nil {
				return nil, e
			}
			st = st.Normalize(cycleStart)
			// 行号必须正好是"下一个该领的那天"：客户端点的就是它 believing 可领的那一格。
			if st.ClaimedDays != day {
				return nil, fmt.Errorf("attendance day %d is not the next claimable day (claimed=%d)", day, st.ClaimedDays)
			}
			if !st.AvailableToday(today) {
				return nil, fmt.Errorf("attendance day %d is not claimable today", day)
			}
			next, e := st.Claim(today)
			if e != nil {
				return nil, e
			}
			return attendance.WriteState(current.State, next)
		})
	if err != nil {
		return role, receipt, false, err
	}

	// 回执里的状态与返回给会话的角色都要**从库里读回来**：重放路径（applied=false）
	// 不会执行上面的 mutate，闭包里捕获的值在那条路上是零值。会话拿到的角色必须带
	// 新状态，否则进城推的 op1379 会一直停在旧的那一天。
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
	st, err := attendance.ReadState(saved.State)
	if err != nil {
		return role, receipt, false, err
	}
	receipt = AttendanceReceipt{
		Day:        day,
		ClaimedDay: today,
		Rewards:    rewards,
		MailID:     mailID,
		MailTitle:  s.Catalog.MailTitleKey,
		MailBody:   s.Catalog.MailMessageKey,
		MailSender: sender,
		MailText:   text,
		State:      st.Normalize(cycleStart),
	}
	return saved, receipt, applied, nil
}
