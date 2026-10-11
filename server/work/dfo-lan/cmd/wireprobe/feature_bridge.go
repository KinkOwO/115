package main

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/featurebridge"
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"time"
)

func (client *gameConnection) featureCall(call featurebridge.Call, now time.Time) {
	reply := featurebridge.Frame{Kind: call.Request.Kind | featurebridge.Response, ID: call.Request.ID}
	defer func() { call.Reply <- reply }()
	if !client.bootstrapped || client.selectedCharacterID == 0 || client.worldState == nil || client.worldState.role.ID != client.selectedCharacterID || client.worldState.role.AccountID != client.worldState.account {
		reply.Status = featurebridge.StatusNotReady
		return
	}
	if call.Request.Kind == featurebridge.SymbolSnapshot || call.Request.Kind == featurebridge.SymbolExchange || call.Request.Kind == featurebridge.SymbolRefresh {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		client.symbolFeature(ctx, call.Request, &reply, now)
		return
	}
	w := client.worldState
	if w.attendanceClock() == nil || w.attendanceClock().Location == nil {
		reply.Status = featurebridge.StatusNotReady
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	switch call.Request.Kind {
	case featurebridge.Snapshot:
		if len(call.Request.Body) != 0 {
			reply.Status = featurebridge.StatusRefused
			return
		}
	case featurebridge.Claim:
		c, e := featurebridge.DecodeClaim(call.Request.Body)
		if e != nil {
			reply.Status = featurebridge.StatusRefused
			return
		}
		if e := w.claimFeature(ctx, now, c); e != nil {
			reply.Status = featurebridge.StatusRefused
			return
		}
	default:
		reply.Status = featurebridge.StatusUnsupported
		return
	}
	weekly, e := client.weeklyFeatureView(ctx, now)
	if e != nil {
		reply.Status = featurebridge.StatusInternal
		return
	}
	daily, e := w.dailyView(ctx, now)
	if e != nil {
		reply.Status = featurebridge.StatusInternal
		return
	}
	online, e := w.onlineView(ctx, now.In(w.attendanceClock().Location))
	if e != nil {
		reply.Status = featurebridge.StatusInternal
		return
	}
	membership, e := w.membershipView(ctx, now)
	if e != nil {
		reply.Status = featurebridge.StatusInternal
		return
	}
	view := featurebridge.View{Role: uint64(w.role.ID), Campaigns: []featurebridge.CampaignView{weekly, daily, online,
		membership,
		{Family: featurebridge.Emblem, State: 2, Title: "徽章与星徽", Detail: "徽章与星徽请使用原生背包入口"}}}
	reply.Body, e = view.Bytes()
	if e != nil {
		reply.Body = nil
		reply.Status = featurebridge.StatusInternal
	}
}
func (client *gameConnection) weeklyFeatureView(ctx context.Context, now time.Time) (featurebridge.CampaignView, error) {
	c := featurebridge.CampaignView{Family: featurebridge.Weekly, State: 2, Title: "周常活动", Detail: "当前资源未配置周常活动"}
	w := client.worldState
	if w.weekly == nil {
		return c, nil
	}
	p, e := w.weeklyStatus(ctx, now, "", 0)
	if errors.Is(e, activity.ErrRoutineInactive) {
		c.State = 1
		c.Detail = "活动未开始、已结束或角色等级未达要求"
		return c, nil
	}
	if e != nil {
		return c, e
	}
	if len(p) != 21 {
		return c, fmt.Errorf("invalid weekly projection")
	}
	if client.output != nil {
		if e = client.output.send(0, 1753, p); e != nil {
			return c, e
		}
	}
	rules := w.weekly.Rules
	c = featurebridge.CampaignView{Family: featurebridge.Weekly, Title: rules.Weekly[0].Reward.MailTitle, Detail: rules.Dates.Start + " ~ " + rules.Dates.End, Source: rules.SourceChecksum, Campaign: rules.CampaignKey()[:32]}
	eligible := binary.LittleEndian.Uint32(p)
	claimed := binary.LittleEndian.Uint32(p[9:])
	cum := binary.LittleEndian.Uint32(p[13:])
	n := uint64(bits.OnesCount32(claimed))
	for _, r := range rules.Weekly {
		row := featurebridge.Row{Group: featurebridge.Period, Index: r.Index, Item: r.Reward.Item, Count: r.Reward.Count, Goal: uint64(r.Required), Label: routineRewardLabel(rules, fmt.Sprintf("第%d期 · %s", r.Index, r.Dates.Start), r.Reward)}
		if p[4] == byte(r.Index) {
			row.Progress = uint64(binary.LittleEndian.Uint32(p[5:]))
		}
		if eligible&(1<<r.Index) != 0 {
			row.State = featurebridge.Claimable
			if row.Progress < row.Goal {
				row.Progress = row.Goal
			}
		}
		if claimed&(1<<r.Index) != 0 {
			row.State = featurebridge.Claimed
		}
		c.Rows = append(c.Rows, row)
	}
	for _, r := range rules.Cumulative {
		row := featurebridge.Row{Group: featurebridge.Cumulative, Index: r.Index, Item: r.Reward.Item, Count: r.Reward.Count, Goal: uint64(r.Required), Progress: n, Label: routineRewardLabel(rules, fmt.Sprintf("累计领取%d期", r.Required), r.Reward)}
		if n >= row.Goal {
			row.State = featurebridge.Claimable
		}
		if cum&(1<<r.Index) != 0 {
			row.State = featurebridge.Claimed
		}
		c.Rows = append(c.Rows, row)
	}
	return c, nil
}
