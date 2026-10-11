package main

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/workflow"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

func weeklyOpening(s *workflow.RoutineService, base []byte, location *time.Location) ([]byte, error) {
	if s == nil || location == nil || s.Rules.Kind != "weekly" || len(s.Rules.Weekly) == 0 {
		return nil, fmt.Errorf("weekly source unavailable")
	}
	if err := s.Rules.Validate(); err != nil {
		return nil, err
	}
	start, err := time.ParseInLocation("2006-01-02 15:04:05", s.Rules.Dates.Start, location)
	if err != nil {
		return nil, err
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", s.Rules.Dates.End, location)
	if err != nil {
		return nil, err
	}
	if start.Unix() < 0 || end.Unix() > 0x7fffffff {
		return nil, fmt.Errorf("weekly source timestamp exceeds signed client time")
	}
	return protocol.AppendWeeklyOpening115(base, s.Rules.Weekly[0].Reward.MailTitle, uint32(start.Unix()), uint32(end.Unix()))
}

// Only intervals observed in this selected-role session reach the account
// transaction. Reconnect and roster screens cannot add offline time.
func (w *worldSession) weeklyStatus(ctx context.Context, now time.Time, kind string, index uint32) ([]byte, error) {
	if w == nil || w.weekly == nil {
		return nil, nil
	}
	if w.attendanceClock() == nil || w.attendanceClock().Location == nil || w.role.ID <= 0 || w.role.AccountID != w.account {
		return nil, fmt.Errorf("weekly requires owned role and calendar")
	}
	now = now.In(w.attendanceClock().Location)
	from := w.weeklyClock
	if from.IsZero() || w.weeklyRole != w.role.ID {
		from = now
	}
	action := workflow.RoutineAction{From: from, Now: now, GameDay: w.attendanceClock().Day(now), ResetHour: w.attendanceClock().Rules.ResetHour, ClaimKind: kind, ClaimIndex: index, ObservationKey: fmt.Sprintf("native:%d:%d:%d:%s:%d", w.role.ID, from.UnixNano(), now.UnixNano(), kind, index)}
	r, err := w.weekly.Run(ctx, w.role, action)
	if errors.Is(err, activity.ErrRoutineInactive) && kind == "" {
		w.weeklyClock, w.weeklyRole = now, w.role.ID
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	w.weeklyClock, w.weeklyRole = now, w.role.ID

	v, err := w.weekly.Rules.WeeklyNativeView(r.Progress, now, w.attendanceClock().Rules.ResetHour)
	if err != nil {
		return nil, err
	}
	status, err := (protocol.WeeklyAttendanceStatus115{EligiblePeriods: v.EligiblePeriods, CurrentPeriod: v.CurrentPeriod, OnlineSeconds: v.Seconds, ClaimedPeriods: v.ClaimedPeriods, ClaimedCumulative: v.ClaimedCumulative}).Bytes()
	if err == nil && r.CreatedMail && w.notifyBoostMail != nil {
		w.notifyBoostMail(w.role.ID)
	}
	return status, err
}

func (client *gameConnection) sendWeeklyStatus(now time.Time) error {
	if client.worldState == nil || client.worldState.weekly == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p, err := client.worldState.weeklyStatus(ctx, now, "", 0)
	if err != nil {
		client.event(map[string]any{"kind": "weekly_tick_error", "reason": err.Error()})
		return nil
	}
	if p != nil {
		return client.output.send(0, 1753, p)
	}
	return nil
}

func (client *gameConnection) flushWeeklyClock() {
	defer client.flushOnlineClock()
	if client.worldState == nil || client.worldState.weeklyClock.IsZero() {
		return
	}
	w := client.worldState
	defer func() { w.weeklyClock = time.Time{}; w.weeklyRole = 0 }()
	if w.weeklyRole != w.role.ID {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := w.weeklyStatus(ctx, time.Now(), "", 0); err != nil && client.event != nil {
		client.event(map[string]any{"kind": "weekly_disconnect_save_error", "reason": err.Error()})
	}
}

func (w *worldSession) claimWeekly(ctx context.Context, body []byte, now time.Time) ([]outboundPacket, error) {
	refuse := func(err error) ([]outboundPacket, error) {
		return []outboundPacket{{"weekly_request_refused", 1, 680, protocol.WeeklyAttendanceRefusal115(23)}}, err
	}
	if w == nil || w.weekly == nil {
		return refuse(fmt.Errorf("weekly unavailable"))
	}
	// Prepared weekly rules reject [accu period reward info] explicitly.
	req, err := protocol.DecodeWeeklyAttendanceRequest115(body, false)
	if err != nil {
		return refuse(err)
	}
	kind := ""
	var reward activity.Reward
	if !req.Sync {
		kind = "period"
		rows := w.weekly.Rules.Weekly
		if req.Cumulative {
			kind = "cumulative"
			rows = w.weekly.Rules.Cumulative
		}
		found := false
		for _, row := range rows {
			if row.Index == req.Index {
				found = true
				reward = row.Reward
				break
			}
		}
		if !found {
			return refuse(fmt.Errorf("weekly claim absent from selected source"))
		}
	}
	p, err := w.weeklyStatus(ctx, now, kind, req.Index)
	if err != nil {
		return refuse(err)
	}
	if p == nil {
		return refuse(activity.ErrRoutineInactive)
	}
	plan := []outboundPacket{{"weekly_status", 0, 1753, p}}
	if !req.Sync {
		plan = append(plan, outboundPacket{"weekly_claim_ack", 1, 680, protocol.EventReply115(protocol.WeeklyAttendanceEvent115, [10]uint32{req.NativeID, reward.Item, reward.Count})})
	}
	return plan, nil
}

func (client *gameConnection) dispatchWeekly(request *clientRequest) dispatchAction {
	if request.frame.Type != 1 || request.frame.ID != 680 || !request.verified || !client.bootstrapped || len(request.plaintext) < 4 || binary.LittleEndian.Uint32(request.plaintext) != protocol.WeeklyAttendanceEvent115 || client.worldState == nil || client.worldState.weekly == nil || client.selectedCharacterID == 0 {
		return dispatchNext
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	plan, err := client.worldState.claimWeekly(ctx, request.plaintext, time.Now())
	if err != nil {
		client.event(map[string]any{"kind": "weekly_request_error", "reason": err.Error()})
	}
	if client.sendPlan(plan, client.logWorldResponseBody) != nil {
		return dispatchClose
	}
	return dispatchHandled
}

// Use the existing committed clear receipt's native recommended-dungeon
// predicate and run identity. Never award on an uncommitted death/room packet.
func (w *worldSession) weeklyCommittedClear(ctx context.Context, receipt character.ClearReceipt, now time.Time) ([]outboundPacket, error) {
	if w.weekly == nil || !receipt.RecommendedDungeonClear {
		return nil, nil
	}
	if w.activeDungeon == nil || receipt.Run == "" || receipt.Run != w.activeDungeon.RunID || receipt.DungeonID != w.activeDungeon.Definition.ID || receipt.Source != w.weekly.Items.Source.SaveIdentity() {
		return nil, fmt.Errorf("weekly clear receipt identity mismatch")
	}
	if uint32(receipt.CharacterLevel) < w.weekly.Rules.MinimumLevel {
		return nil, nil
	}
	// The clear transaction already saved the source bonus. This only refreshes UI.
	p, err := w.weeklyStatus(ctx, now, "", 0)
	if err != nil || p == nil {
		return nil, err
	}
	return []outboundPacket{{"weekly_committed_clear", 0, 1753, p}}, nil
}
