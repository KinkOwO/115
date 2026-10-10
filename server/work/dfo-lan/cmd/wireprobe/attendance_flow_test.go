package main

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAttendanceGatewayPersistsAndResetsWithGameCalendar(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "activity.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	raw, err := os.ReadFile("../../internal/activity/testdata/login.json")
	require.NoError(t, err)
	var tokens []pvf.Token
	require.NoError(t, json.Unmarshal(raw, &tokens))
	rules, err := activity.ParseLogin(tokens)
	require.NoError(t, err)
	version := strings.Repeat("a", 64)
	rules.SourceChecksum, rules.MailTextTitle, rules.MailTextMessage = version, "Attendance", "Source rewards"
	rules.CalendarUI = "live/event/kor/2026/0326_attendancedailyevent/cal.xui"
	index := catalog.ItemIndex{Source: pvf.ArchiveSnapshot{Checksum: version}, Items: map[uint32]catalog.ItemIndexEntry{}}
	for _, d := range rules.Days {
		for _, r := range d.Rewards {
			index.Items[r.Item] = catalog.ItemIndexEntry{ID: r.Item, Kind: "stackable", Path: "fixture.stk"}
		}
	}
	svc, err := workflow.NewAttendanceService(store, rules, index)
	require.NoError(t, err)
	account, err := store.DevelopmentAccount(ctx, "attendance-gateway")
	require.NoError(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "Attendance gateway", Request: []byte{}, ConfigVersion: index.Source.SaveIdentity(), State: json.RawMessage(`{"level":115}`)}, 100)
	require.NoError(t, err)
	location := time.FixedZone("HK", 8*3600)
	fatigue := &character.FatigueService{Rules: character.FatigueRules{ResetHour: 6}, Location: location}
	notified := 0
	w := &worldSession{attendance: svc, account: account, role: role, fatigue: fatigue, notifyBoostMail: func(id int64) { require.Equal(t, role.ID, id); notified++ }}
	now := time.Date(2026, 9, 2, 5, 59, 0, 0, location)
	p, err := w.attendanceStatus(ctx, now)
	require.NoError(t, err)
	require.EqualValues(t, 1, binary.LittleEndian.Uint32(p[1:]))
	require.Equal(t, "2026-09-01", w.attendanceDay)
	request := make([]byte, 8)
	binary.LittleEndian.PutUint32(request, 331)
	packets, err := w.claimAttendance(ctx, request, now)
	require.NoError(t, err)
	require.Len(t, packets, 2)
	require.EqualValues(t, 1379, packets[0].ID)
	require.EqualValues(t, 2, packets[0].Payload[5])
	require.EqualValues(t, 680, packets[1].ID)
	require.Equal(t, 1, notified)
	_, err = w.claimAttendance(ctx, request, now)
	require.NoError(t, err)
	require.Equal(t, 1, notified)
	p, err = w.attendanceStatus(ctx, now.Add(time.Minute))
	require.NoError(t, err)
	require.EqualValues(t, 2, binary.LittleEndian.Uint32(p[1:]))
	require.EqualValues(t, 2, p[5])
	require.EqualValues(t, 1, p[6])
	// A new world session restores the same account state, without a new day.
	w.attendanceDay = ""
	p, err = w.attendanceStatus(ctx, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.EqualValues(t, 2, binary.LittleEndian.Uint32(p[1:]))
	binary.LittleEndian.PutUint32(request[4:], 7)
	packets, err = w.claimAttendance(ctx, request, now.Add(2*time.Minute))
	require.Error(t, err)
	require.Equal(t, protocol.AttendanceDailyRefusal115(23), packets[0].Payload)
	require.Equal(t, 1, notified)
}

func TestAttendanceEntrySnapshotPreservesOtherEventsAndDispatch(t *testing.T) {
	s := &workflow.AttendanceService{Rules: activity.Login{Dates: activity.Dates{Start: "2026-10-08 06:00:00", End: "2026-11-12 06:00:00"}, MailTextTitle: "金秋8日丰收之旅", CalendarUI: "live/event/kor/2026/0326_attendancedailyevent/cal.xui"}}
	merged, err := attendanceOpening(s, townEventInfoTable, time.UTC)
	require.NoError(t, err)
	require.EqualValues(t, binary.LittleEndian.Uint16(townEventInfoTable)+1, binary.LittleEndian.Uint16(merged))
	require.Equal(t, townEventInfoTable[2:len(townEventInfoTable)-1], merged[2:len(townEventInfoTable)-1])
	plan := entryPayloads{ChannelEventInfo: merged, Attendance: []byte{0}, Complete: []byte{0}}.packets()
	completed := false
	for _, p := range plan {
		if p.ID == 108 {
			require.Equal(t, merged, p.Payload)
		}
		if p.ID == 124 {
			completed = true
		}
		if p.ID == 1379 {
			require.True(t, completed)
		}
	}
	client := &gameConnection{worldState: &worldSession{attendance: s}, bootstrapped: true}
	for _, id := range []uint32{662, 387, 389} {
		p := make([]byte, 8)
		binary.LittleEndian.PutUint32(p, id)
		require.Equal(t, dispatchNext, client.dispatchAttendance(&clientRequest{frame: wire.Frame{Type: 1, ID: 680}, verified: true, plaintext: p}))
	}
}
