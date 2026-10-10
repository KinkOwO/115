package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/workflow"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

func (w *worldSession) attendanceStatus(ctx context.Context, now time.Time) ([]byte, error) {
	if w == nil || w.attendance == nil {
		return nil, nil
	}
	if w.attendanceClock() == nil || w.attendanceClock().Location == nil || w.role.AccountID != w.account {
		return nil, fmt.Errorf("attendance requires owned role and game calendar")
	}
	day := w.attendanceClock().Day(now)
	r, err := w.attendance.Observe(ctx, w.role, now.In(w.attendanceClock().Location), day)
	if errors.Is(err, workflow.ErrAttendanceInactive) {
		w.attendanceDay = day
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	states, err := w.attendance.Rules.DayStates(r.Progress)
	if err != nil {
		return nil, err
	}
	p, err := protocol.AttendanceDailyStatus115(0, r.Progress.Visits, states)
	if err == nil {
		w.attendanceDay = day
	}
	return p, err
}

func (w *worldSession) claimAttendance(ctx context.Context, body []byte, now time.Time) ([]outboundPacket, error) {
	day, err := protocol.DecodeAttendanceDailyClaim115(body)
	refuse := func(err error) ([]outboundPacket, error) {
		// Existing local execution uses 23 for a refused operation. This is
		// a compatibility error, not an invented official attendance mapping.
		return []outboundPacket{{"attendance_claim_refused", 1, 680, protocol.AttendanceDailyRefusal115(23)}}, err
	}
	if err != nil {
		return refuse(err)
	}
	if w == nil || w.attendance == nil || w.attendanceClock() == nil || w.attendanceClock().Location == nil || w.role.AccountID != w.account {
		return refuse(fmt.Errorf("attendance is unavailable"))
	}
	r, err := w.attendance.Claim(ctx, w.role, now.In(w.attendanceClock().Location), w.attendanceClock().Day(now), day)
	if err != nil {
		return refuse(err)
	}
	states, err := w.attendance.Rules.DayStates(r.Progress)
	if err != nil {
		return nil, err
	}
	status, err := protocol.AttendanceDailyStatus115(0, r.Progress.Visits, states)
	if err != nil {
		return nil, err
	}
	// Match the captured 331 order: absolute progress, successful CMD680,
	// then the existing mailbox notifier. A retry never generates new mail.
	plan := []outboundPacket{{"attendance_claim_status", 0, 1379, status}, {"attendance_claim_ack", 1, 680, protocol.EventReply115(protocol.AttendanceDailyEvent115, [10]uint32{})}}
	if r.CreatedMail && w.notifyBoostMail != nil {
		w.notifyBoostMail(w.role.ID)
	}
	return plan, nil
}

func (client *gameConnection) dispatchAttendance(request *clientRequest) dispatchAction {
	if request.frame.Type != 1 || request.frame.ID != 680 || !request.verified || !client.bootstrapped || len(request.plaintext) < 4 || binary.LittleEndian.Uint32(request.plaintext) != protocol.AttendanceDailyEvent115 {
		return dispatchNext
	}
	if client.worldState == nil || client.worldState.attendance == nil {
		return dispatchNext
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	plan, err := client.worldState.claimAttendance(ctx, request.plaintext, time.Now())
	if err != nil {
		client.event(map[string]any{"kind": "attendance_claim_error", "character_id": client.selectedCharacterID, "reason": err.Error()})
	}
	if client.sendPlan(plan, client.logWorldResponseBody) != nil {
		return dispatchClose
	}
	return dispatchHandled
}

func attendanceOpening(s *workflow.AttendanceService, base []byte, location *time.Location) ([]byte, error) {
	if s == nil || location == nil || s.Rules.MinimumLevel != 0 {
		return nil, fmt.Errorf("attendance native level-gate branch is not supported")
	}
	start, err := time.ParseInLocation("2006-01-02 15:04:05", s.Rules.Dates.Start, location)
	if err != nil {
		return nil, err
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", s.Rules.Dates.End, location)
	if err != nil || start.Unix() < 0 || end.Unix() > 4294967295 {
		return nil, fmt.Errorf("attendance calendar exceeds native timestamp")
	}
	return protocol.AppendAttendanceOpening115(base, s.Rules.MailTextTitle, s.Rules.CalendarUI, uint32(start.Unix()), uint32(end.Unix()))
}
