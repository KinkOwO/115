package main

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/featurebridge"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

func (w *worldSession) claimFeature(ctx context.Context, now time.Time, c featurebridge.ClaimRequest) error {
	if c.Family == featurebridge.Membership {
		if w.membership == nil || c.Source != w.membership.Rules.SourceChecksum || c.Campaign != w.membership.Rules.CampaignKey()[:32] {
			return fmt.Errorf("membership source mismatch")
		}
		group := ""
		if c.Group == featurebridge.Period {
			group = "period"
		}
		if c.Group == featurebridge.Cumulative {
			group = "cumulative"
		}
		if group == "" {
			return fmt.Errorf("invalid membership claim group")
		}
		r, e := w.membership.Run(ctx, w.role, now, group, c.Index, fmt.Sprintf("claim:%d:%d:%d", w.role.ID, now.UnixNano(), c.Index))
		if e == nil && r.CreatedMail && w.notifyBoostMail != nil {
			w.notifyBoostMail(w.role.ID)
		}
		return e
	}
	var rules *activity.RoutineRules
	switch c.Family {
	case featurebridge.Weekly:
		if w.weekly != nil {
			rules = &w.weekly.Rules
		}
	case featurebridge.Daily:
		if w.daily != nil {
			rules = &w.daily.Rules
		}
	case featurebridge.Online:
		if w.online != nil {
			rules = &w.online.Rules
		}
	}
	if rules == nil || c.Source != rules.SourceChecksum || c.Campaign != rules.CampaignKey()[:32] {
		return fmt.Errorf("activity source mismatch")
	}
	group := ""
	switch c.Group {
	case featurebridge.Period:
		if c.Family != featurebridge.Daily {
			group = "period"
		}
	case featurebridge.Day:
		if c.Family != featurebridge.Weekly {
			group = "day"
		}
	case featurebridge.Cumulative:
		if c.Family != featurebridge.Online {
			group = "cumulative"
		}
	}
	if group == "" {
		return fmt.Errorf("invalid activity claim group")
	}
	var e error
	switch c.Family {
	case featurebridge.Weekly:
		_, e = w.weeklyStatus(ctx, now, group, c.Index)
	case featurebridge.Daily:
		_, e = w.dailyStatus(ctx, now, group, c.Index)
	case featurebridge.Online:
		_, e = w.onlineStatus(ctx, now, group, c.Index)
	}
	return e
}
func (w *worldSession) membershipView(ctx context.Context, now time.Time) (featurebridge.CampaignView, error) {
	c := featurebridge.CampaignView{Family: featurebridge.Membership, State: 2, Title: "阿拉德拾光之证", Detail: "当前资源未配置拾光之证"}
	if w.membership == nil {
		return c, nil
	}
	rules := w.membership.Rules
	c.Source = rules.SourceChecksum
	c.Campaign = rules.CampaignKey()[:32]
	r, e := w.membership.Run(ctx, w.role, now, "", 0, fmt.Sprintf("view:%d:%d", w.role.ID, now.UnixNano()))
	if errors.Is(e, activity.ErrRoutineInactive) {
		c.State = 1
		c.Detail = "拾光之证活动尚未开始或已结束"
		return c, nil
	}
	if e != nil {
		return c, e
	}
	p := r.Progress
	c.State = 0
	c.Detail = fmt.Sprintf("当前等级：%s；累计消费积分 %d。金币消费不计分。满级后每%d积分可领取一次尊享礼遇。", membershipGradeLabel(rules.Grades[rules.Grade(p.Spent)].Name), p.Spent, rules.RepeatPoints)
	bundle := func(rs []activity.Reward) string {
		var names []string
		for _, x := range rs {
			name := rules.ItemNames[x.Item]
			if name == "" {
				name = fmt.Sprintf("材料%d", x.Item)
			}
			names = append(names, fmt.Sprintf("%s×%d", name, x.Count))
		}
		return strings.Join(names, "、")
	}
	for _, g := range rules.Grades {
		row := featurebridge.Row{Group: featurebridge.Period, Index: g.Index + 1, Item: g.Rewards[0].Item, Count: g.Rewards[0].Count, Progress: p.Spent, Goal: g.Minimum, Label: membershipGradeLabel(g.Name) + "：" + bundle(g.Rewards)}
		if p.Spent >= g.Minimum {
			row.State = featurebridge.Claimable
		}
		if p.Claimed[g.Index] {
			row.State = featurebridge.Claimed
		}
		c.Rows = append(c.Rows, row)
	}
	if p.RepeatClaimed < math.MaxUint32 {
		points := uint64(0)
		if p.Spent > rules.Cap() {
			points = p.Spent - rules.Cap() - p.RepeatClaimed*rules.RepeatPoints
		}
		row := featurebridge.Row{Group: featurebridge.Cumulative, Index: uint32(p.RepeatClaimed + 1), Item: rules.RepeatRewards[0].Item, Count: rules.RepeatRewards[0].Count, Progress: points, Goal: rules.RepeatPoints, Label: "尊享礼遇：" + bundle(rules.RepeatRewards)}
		if rules.RepeatAvailable(p) > p.RepeatClaimed {
			row.State = featurebridge.Claimable
		}
		c.Rows = append(c.Rows, row)
	}
	return c, nil
}
