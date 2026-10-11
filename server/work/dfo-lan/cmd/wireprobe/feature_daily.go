package main

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/featurebridge"
	"dfolan/internal/workflow"
	"errors"
	"fmt"
	"time"
)

func (w *worldSession) dailyStatus(ctx context.Context, now time.Time, kind string, index uint32) (activity.RoutineProgress, error) {
	if w == nil || w.daily == nil || w.attendanceClock() == nil || w.attendanceClock().Location == nil || w.role.ID <= 0 || w.role.AccountID != w.account {
		return activity.RoutineProgress{}, fmt.Errorf("daily requires owned role and source calendar")
	}
	now = now.In(w.attendanceClock().Location)
	r, e := w.daily.Run(ctx, w.role, workflow.RoutineAction{From: now, Now: now, GameDay: w.attendanceClock().Day(now), ResetHour: w.attendanceClock().Rules.ResetHour, ClaimKind: kind, ClaimIndex: index, ObservationKey: fmt.Sprintf("extension-daily:%d:%d:%s:%d", w.role.ID, now.UnixNano(), kind, index)})
	if e == nil && r.CreatedMail && w.notifyBoostMail != nil {
		w.notifyBoostMail(w.role.ID)
	}
	return r.Progress, e
}
func (w *worldSession) dailyView(ctx context.Context, now time.Time) (featurebridge.CampaignView, error) {
	c := featurebridge.CampaignView{Family: featurebridge.Daily, State: 2, Title: "十日签到", Detail: "当前资源未配置累计签到"}
	if w.daily == nil {
		return c, nil
	}
	rules := w.daily.Rules
	c.Source = rules.SourceChecksum
	c.Campaign = rules.CampaignKey()[:32]
	c.Detail = fmt.Sprintf("每日通关%d次推荐地下城，累计一天；每天最多一天，漏签可继续。%s ~ %s", rules.ProperDungeonCount, rules.Dates.Start, rules.Dates.End)
	p, e := w.dailyStatus(ctx, now, "", 0)
	if errors.Is(e, activity.ErrRoutineInactive) {
		c.State = 1
		c.Detail = "活动未开始、已结束或角色等级未达要求"
		return c, nil
	}
	if e != nil {
		return c, e
	}
	c.State = 0
	n := uint64(0)
	for _, r := range rules.Daily {
		row := featurebridge.Row{Group: featurebridge.Day, Index: r.Index, Item: r.Reward.Item, Count: r.Reward.Count, Progress: uint64(p.Clears[r.Index]), Goal: uint64(rules.ProperDungeonCount), Label: routineRewardLabel(rules, fmt.Sprintf("累计第%d天", r.Index), r.Reward)}
		if rules.DailyEligible(p, r.Index) {
			row.State = featurebridge.Claimable
		}
		if p.Claimed[fmt.Sprintf("day:%d", r.Index)] {
			row.State = featurebridge.Claimed
			n++
		}
		c.Rows = append(c.Rows, row)
	}
	for _, r := range rules.Cumulative {
		row := featurebridge.Row{Group: featurebridge.Cumulative, Index: r.Index, Item: r.Reward.Item, Count: r.Reward.Count, Progress: n, Goal: uint64(r.Required), Label: routineRewardLabel(rules, fmt.Sprintf("累计领取%d天", r.Required), r.Reward)}
		if n >= row.Goal {
			row.State = featurebridge.Claimable
		}
		if p.Claimed[fmt.Sprintf("cumulative:%d", r.Index)] {
			row.State = featurebridge.Claimed
		}
		c.Rows = append(c.Rows, row)
	}
	return c, nil
}
