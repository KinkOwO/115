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

func (w *worldSession) onlineStatus(ctx context.Context, now time.Time, kind string, index uint32) (activity.RoutineProgress, error) {
	if w == nil || w.online == nil || w.attendanceClock() == nil || w.attendanceClock().Location == nil || w.role.ID <= 0 || w.role.AccountID != w.account {
		return activity.RoutineProgress{}, fmt.Errorf("online requires owned selected role and source calendar")
	}
	now = now.In(w.attendanceClock().Location)
	from := w.onlineClock
	if from.IsZero() || w.onlineRole != w.role.ID {
		from = now
	}
	r, e := w.online.Run(ctx, w.role, workflow.RoutineAction{From: from, Now: now, GameDay: w.attendanceClock().Day(now), ResetHour: w.attendanceClock().Rules.ResetHour, ClaimKind: kind, ClaimIndex: index, ObservationKey: fmt.Sprintf("extension-online:%d:%d:%d:%s:%d", w.role.ID, from.UnixNano(), now.UnixNano(), kind, index)})
	if e != nil {
		if errors.Is(e, activity.ErrRoutineInactive) && kind == "" {
			w.onlineClock = now
			w.onlineRole = w.role.ID
		}
		return activity.RoutineProgress{}, e
	}
	w.onlineClock = now
	w.onlineRole = w.role.ID
	if r.CreatedMail && w.notifyBoostMail != nil {
		w.notifyBoostMail(w.role.ID)
	}
	return r.Progress, nil
}
func (client *gameConnection) observeOnline(now time.Time) {
	if client.worldState == nil || client.worldState.online == nil || !client.bootstrapped || client.selectedCharacterID == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, e := client.worldState.onlineStatus(ctx, now, "", 0)
	if e != nil && !errors.Is(e, activity.ErrRoutineInactive) && client.event != nil {
		client.event(map[string]any{"kind": "online_progress_error", "reason": e.Error()})
	}
}
func (client *gameConnection) flushOnlineClock() {
	w := client.worldState
	if w == nil || w.online == nil || w.onlineClock.IsZero() {
		return
	}
	defer func() { w.onlineClock = time.Time{}; w.onlineRole = 0 }()
	if w.onlineRole != w.role.ID {
		return
	}
	client.observeOnline(time.Now())
}
func (w *worldSession) onlineView(ctx context.Context, now time.Time) (featurebridge.CampaignView, error) {
	c := featurebridge.CampaignView{Family: featurebridge.Online, State: 2, Title: "在线奖励", Detail: "当前资源未配置在线活动"}
	if w.online == nil {
		return c, nil
	}
	rules := w.online.Rules
	c.Source = rules.SourceChecksum
	c.Campaign = rules.CampaignKey()[:32]
	c.Detail = rules.Dates.Start + " ~ " + rules.Dates.End
	p, e := w.onlineStatus(ctx, now, "", 0)
	if errors.Is(e, activity.ErrRoutineInactive) {
		c.State = 1
		c.Detail = "活动未开始、已结束或角色等级未达要求"
		return c, nil
	}
	if e != nil {
		return c, e
	}
	c.State = 0
	day, e := time.ParseInLocation("2006-01-02", p.Day, now.Location())
	if e != nil {
		return c, e
	}
	key := "weekday"
	if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
		key = "weekend"
	}
	var threshold uint64
	for i, r := range rules.OnlineDaily[key] {
		threshold += uint64(r.Milliseconds)
		row := featurebridge.Row{Group: featurebridge.Day, Index: uint32(i + 1), Item: r.Reward.Item, Count: r.Reward.Count, Progress: p.DailyMillis / 1000, Goal: threshold / 1000, Label: routineRewardLabel(rules, fmt.Sprintf("每日在线%d分钟", threshold/60000), r.Reward)}
		if p.DailyMillis >= threshold {
			row.State = featurebridge.Claimable
		}
		if p.Claimed[fmt.Sprintf("%s:day:%d", p.Day, i+1)] {
			row.State = featurebridge.Claimed
		}
		c.Rows = append(c.Rows, row)
	}
	threshold = 0
	for i, r := range rules.OnlinePeriod {
		threshold += uint64(r.Milliseconds)
		row := featurebridge.Row{Group: featurebridge.Period, Index: uint32(i + 1), Item: r.Reward.Item, Count: r.Reward.Count, Progress: p.TotalMillis / 1000, Goal: threshold / 1000, Label: routineRewardLabel(rules, fmt.Sprintf("累计在线%d小时", threshold/3600000), r.Reward)}
		if p.TotalMillis >= threshold {
			row.State = featurebridge.Claimable
		}
		if p.Claimed[fmt.Sprintf("period:%d", i+1)] {
			row.State = featurebridge.Claimed
		}
		c.Rows = append(c.Rows, row)
	}
	return c, nil
}
