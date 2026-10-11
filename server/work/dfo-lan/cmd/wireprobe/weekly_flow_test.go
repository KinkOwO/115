package main

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/gamedata"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func weeklyGatewayFixture(t *testing.T) (*worldSession, time.Time, *int) {
	t.Helper()
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "weekly.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	version := strings.Repeat("a", 64)
	reward := activity.Reward{Item: 10001, Count: 3, MailTitle: "Source weekly title", MailMessage: "Source weekly body"}
	rules := activity.RoutineRules{Kind: "weekly", Table: "native-bridge", SourceChecksum: version, MinimumLevel: 1, SecondsPerProperClear: 10, ClickCumulative: true, Dates: activity.Dates{Start: "2026-10-08 06:00:00", End: "2026-10-22 06:00:00"}}
	rules.Weekly = []activity.Milestone{{Index: 1, Required: 60, Reward: reward, Dates: activity.Dates{Start: rules.Dates.Start, End: "2026-10-15 06:00:00"}}, {Index: 2, Required: 60, Reward: reward, Dates: activity.Dates{Start: "2026-10-15 06:00:00", End: rules.Dates.End}}}
	rules.Cumulative = []activity.Milestone{{Index: 1, Required: 1, Reward: reward}, {Index: 2, Required: 2, Reward: reward}}
	items := catalog.ItemIndex{Source: pvf.ArchiveSnapshot{Checksum: version}, Items: map[uint32]catalog.ItemIndexEntry{10001: {ID: 10001, Kind: "stackable", Path: "fixture.stk"}}}
	svc, err := workflow.NewRoutineService(store, rules, items)
	require.NoError(t, err)
	account, err := store.DevelopmentAccount(ctx, "weekly-gateway")
	require.NoError(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "Weekly gateway", Request: []byte{}, ConfigVersion: items.Source.SaveIdentity(), State: json.RawMessage(`{"level":115}`)}, 100)
	require.NoError(t, err)
	loc := time.FixedZone("HK", 8*3600)
	notified := new(int)
	w := &worldSession{weekly: svc, account: account, role: role, fatigue: &character.FatigueService{Rules: character.FatigueRules{ResetHour: 6}, Location: loc}, notifyBoostMail: func(id int64) { require.Equal(t, role.ID, id); *notified++ }}
	return w, time.Date(2026, 10, 8, 6, 0, 0, 0, loc), notified
}

func weeklyRequest(action, index uint32, cumulative bool) []byte {
	p := make([]byte, 8)
	binary.LittleEndian.PutUint32(p, 387)
	binary.LittleEndian.PutUint32(p[4:], action)
	if action == 2 {
		p = append(p, 0, 0, 0, 0)
		binary.LittleEndian.PutUint32(p[8:], index)
		if cumulative {
			p = append(p, p[8:12]...)
		} else {
			// CMD680 uses eight-byte transport blocks; native period body is 12 bytes.
			p = append(p, 0, 0, 0, 0)
		}
	}
	return p
}

func TestWeeklyGatewayCapturedPaddedClaimSendsOneMail(t *testing.T) {
	w, now, notified := weeklyGatewayFixture(t)
	ctx := context.Background()
	_, err := w.weeklyStatus(ctx, now, "", 0)
	require.NoError(t, err)
	request, err := hex.DecodeString("83010000020000000100000000000000")
	require.NoError(t, err)
	for attempt := 0; attempt < 2; attempt++ {
		packets, err := w.claimWeekly(ctx, request, now.Add(time.Minute))
		require.NoError(t, err)
		require.Len(t, packets, 2)
		require.Equal(t, protocol.EventReply115(387, [10]uint32{1, 10001, 3}), packets[1].Payload)
	}
	box, err := w.weekly.Store.Mailbox(ctx, w.account, w.role.ID)
	require.NoError(t, err)
	require.Len(t, box, 1)
	require.Equal(t, 1, *notified)
}

func TestWeeklyGatewayRestoresClockAndDeduplicatesNativeClaims(t *testing.T) {
	w, now, notified := weeklyGatewayFixture(t)
	ctx := context.Background()
	plan, err := w.claimWeekly(ctx, weeklyRequest(1, 0, false), now)
	require.NoError(t, err)
	require.Len(t, plan, 1)
	require.EqualValues(t, 1753, plan[0].ID)
	plan, err = w.claimWeekly(ctx, weeklyRequest(2, 1, false), now.Add(59*time.Second))
	require.Error(t, err)
	require.Equal(t, protocol.WeeklyAttendanceRefusal115(23), plan[0].Payload)
	plan, err = w.claimWeekly(ctx, weeklyRequest(2, 1, false), now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, plan, 2)
	require.EqualValues(t, 2, binary.LittleEndian.Uint32(plan[0].Payload[9:]))
	require.Zero(t, binary.LittleEndian.Uint32(plan[0].Payload[13:]))
	require.Equal(t, protocol.EventReply115(387, [10]uint32{1, 10001, 3}), plan[1].Payload)
	require.Equal(t, 1, *notified)
	// ClickCumulative=true still uses base 100 with an empty time vector.
	plan, err = w.claimWeekly(ctx, weeklyRequest(2, 101, true), now.Add(61*time.Second))
	require.NoError(t, err)
	require.EqualValues(t, 2, binary.LittleEndian.Uint32(plan[0].Payload[13:]))
	require.Zero(t, binary.LittleEndian.Uint32(plan[0].Payload[17:]))
	require.Equal(t, 2, *notified)
	_, err = w.claimWeekly(ctx, weeklyRequest(2, 101, true), now.Add(62*time.Second))
	require.NoError(t, err)
	require.Equal(t, 2, *notified)
	w.weeklyClock = time.Time{}
	w.weeklyRole = 0
	p, err := w.weeklyStatus(ctx, now.Add(time.Hour), "", 0)
	require.NoError(t, err)
	require.EqualValues(t, 62, binary.LittleEndian.Uint32(p[5:]))
	for _, bad := range [][]byte{weeklyRequest(2, 201, true), weeklyRequest(2, 3, false), append(weeklyRequest(2, 101, true), 0)} {
		plan, err = w.claimWeekly(ctx, bad, now.Add(time.Hour))
		require.Error(t, err)
		require.Len(t, plan, 1)
	}
	box, err := w.weekly.Store.Mailbox(ctx, w.account, w.role.ID)
	require.NoError(t, err)
	require.Len(t, box, 2)
	// At a new source period, the account retains past claims and starts at zero.
	w.weeklyClock = time.Time{}
	w.weeklyRole = 0
	p, err = w.weeklyStatus(ctx, time.Date(2026, 10, 15, 6, 0, 0, 0, now.Location()), "", 0)
	require.NoError(t, err)
	require.EqualValues(t, 2, p[4])
	require.Zero(t, binary.LittleEndian.Uint32(p[5:]))
	require.EqualValues(t, 2, binary.LittleEndian.Uint32(p[9:]))
}

func TestWeeklyCommittedClearRejectsForeignReceiptAndDeduplicates(t *testing.T) {
	w, now, _ := weeklyGatewayFixture(t)
	ctx := context.Background()
	_, err := w.weeklyStatus(ctx, now, "", 0)
	require.NoError(t, err)
	run := strings.Repeat("1", 32)
	w.activeDungeon = &dungeon.Session{RunID: run, Definition: catalog.DungeonDefinition{ID: 1001}}
	r := character.ClearReceipt{Source: w.weekly.Items.Source.SaveIdentity(), Run: run, RecommendedDungeonClear: true, DungeonID: 1001, CharacterLevel: 115}
	require.NoError(t, w.weekly.Store.MigrateAdventure(ctx))
	require.NoError(t, w.weekly.BindCommittedClears(now.Location(), 6))
	r.CompletedAt = now.Add(time.Second)
	_, _, err = w.weekly.Store.CommitCharacterEvent(ctx, w.account, w.role.ID, w.role.ConfigVersion, "clear:"+run, "weekly-clear-fixture", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		raw, e := json.Marshal(r)
		return current.State, raw, e
	})
	require.NoError(t, err)
	p, err := w.weeklyCommittedClear(ctx, r, now.Add(time.Second))
	require.NoError(t, err)
	require.EqualValues(t, 11, binary.LittleEndian.Uint32(p[0].Payload[5:]))
	p, err = w.weeklyCommittedClear(ctx, r, now.Add(2*time.Second))
	require.NoError(t, err)
	require.EqualValues(t, 12, binary.LittleEndian.Uint32(p[0].Payload[5:]))
	r.Run = "foreign"
	_, err = w.weeklyCommittedClear(ctx, r, now.Add(3*time.Second))
	require.Error(t, err)
	r.RecommendedDungeonClear = false
	p, err = w.weeklyCommittedClear(ctx, r, now.Add(3*time.Second))
	require.NoError(t, err)
	require.Empty(t, p)
}

func TestWeeklyOpeningEntryAndVerifiedDispatcher(t *testing.T) {
	w, now, _ := weeklyGatewayFixture(t)
	merged, err := weeklyOpening(w.weekly, townEventInfoTable, now.Location())
	require.NoError(t, err)
	require.EqualValues(t, binary.LittleEndian.Uint16(townEventInfoTable)+1, binary.LittleEndian.Uint16(merged))
	require.Equal(t, townEventInfoTable[2:len(townEventInfoTable)-1], merged[2:len(townEventInfoTable)-1])
	seenComplete := false
	seenWeekly := false
	for _, p := range (entryPayloads{ChannelEventInfo: merged, Weekly: []byte{1}, Complete: []byte{0}}).packets() {
		if p.ID == 108 {
			require.Equal(t, merged, p.Payload)
		}
		if p.ID == 124 {
			seenComplete = true
		}
		if p.ID == 1753 {
			require.True(t, seenComplete)
			seenWeekly = true
		}
	}
	require.True(t, seenWeekly)
	client := &gameConnection{bootstrapped: true, worldState: w, selectedCharacterID: w.role.ID}
	request := &clientRequest{frame: wire.Frame{Type: 1, ID: 680}, plaintext: weeklyRequest(2, 1, false), verified: false}
	require.Equal(t, dispatchNext, client.dispatchWeekly(request))
	request.verified = true
	client.selectedCharacterID = 0
	require.Equal(t, dispatchNext, client.dispatchWeekly(request))
}

func TestWeeklyGatewayMaterialCandidateNative(t *testing.T) {
	archive := os.Getenv("US115_TEST_WEEKLY_PVF")
	if archive == "" {
		t.Skip("set US115_TEST_WEEKLY_PVF to a selected isolated candidate")
	}
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "items,attendance-weekly", ArchivePath: archive, DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	require.NoError(t, err)
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "native.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	s, err := workflow.NewRoutineService(store, *c.AttendanceWeekly, *c.Items)
	require.NoError(t, err)
	account, err := store.DevelopmentAccount(ctx, "native-weekly")
	require.NoError(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "Native weekly", Request: []byte{}, ConfigVersion: c.Items.Source.SaveIdentity(), State: json.RawMessage(`{"level":115}`)}, 100)
	require.NoError(t, err)
	loc := time.FixedZone("HK", 8*3600)
	w := &worldSession{weekly: s, account: account, role: role, fatigue: &character.FatigueService{Rules: character.FatigueRules{ResetHour: 6}, Location: loc}}
	_, err = weeklyOpening(s, townEventInfoTable, loc)
	require.NoError(t, err)
	claims := 0
	for _, row := range s.Rules.Weekly {
		start, err := time.ParseInLocation("2006-01-02 15:04:05", row.Dates.Start, loc)
		require.NoError(t, err)
		w.weeklyClock = time.Time{}
		w.weeklyRole = 0
		_, err = w.claimWeekly(ctx, weeklyRequest(1, 0, false), start)
		require.NoError(t, err)
		now := start.Add(time.Duration(row.Required) * time.Second)
		packets, err := w.claimWeekly(ctx, weeklyRequest(2, row.Index, false), now)
		require.NoError(t, err)
		require.Len(t, packets, 2)
		require.Equal(t, protocol.EventReply115(387, [10]uint32{row.Index, row.Reward.Item, row.Reward.Count}), packets[1].Payload)
		claims++
		for _, bonus := range s.Rules.Cumulative {
			if bonus.Required != row.Index {
				continue
			}
			packets, err = w.claimWeekly(ctx, weeklyRequest(2, 100+bonus.Index, true), now)
			require.NoError(t, err)
			require.Len(t, packets, 2)
			claims++
		}
		_, err = w.claimWeekly(ctx, weeklyRequest(2, row.Index, false), now)
		require.NoError(t, err)
	}
	box, err := store.Mailbox(ctx, account, role.ID)
	require.NoError(t, err)
	require.Len(t, box, claims)
	require.Equal(t, 9, claims)
	t.Log(fmt.Sprintf("source=%s periods=%d cumulative=%d native_claim_ack=%d mails=%d runtime_started=false storage=temporary-sqlite", s.Rules.SourceChecksum, len(s.Rules.Weekly), len(s.Rules.Cumulative), claims, len(box)))
}
