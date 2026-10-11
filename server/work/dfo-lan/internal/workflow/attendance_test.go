package workflow

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/mail"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func attendanceFixture(t *testing.T) (*AttendanceService, database.Character, database.Character) {
	t.Helper()
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "attendance.sqlite3"), MaxConnections: 4})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	raw, err := os.ReadFile("../activity/testdata/login.json")
	require.NoError(t, err)
	var tokens []pvf.Token
	require.NoError(t, json.Unmarshal(raw, &tokens))
	rules, err := activity.ParseLogin(tokens)
	require.NoError(t, err)
	version := strings.Repeat("a", 64)
	rules.SourceChecksum, rules.MailTextTitle, rules.MailTextMessage = version, "Attendance", "Source reward"
	items := catalog.ItemIndex{Source: pvf.ArchiveSnapshot{Checksum: version}, Items: map[uint32]catalog.ItemIndexEntry{}}
	for _, day := range rules.Days {
		for _, r := range day.Rewards {
			items.Items[r.Item] = catalog.ItemIndexEntry{ID: r.Item, Path: fmt.Sprintf("fixture/%d.stk", r.Item), Kind: "stackable", StackLimit: 1000}
		}
	}
	svc := &AttendanceService{Store: store, Rules: rules, Items: items}
	account, err := store.DevelopmentAccount(ctx, "attendance")
	require.NoError(t, err)
	create := func(name string) database.Character {
		r, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, Profession: 0, Request: []byte{}, ConfigVersion: items.Source.SaveIdentity(), State: json.RawMessage(`{"level":115,"future_character":{"keep":true}}`)}, 100)
		require.NoError(t, err)
		return r
	}
	return svc, create("Attendance-A"), create("Attendance-B")
}

func TestAttendanceAccountSharingReplayAndSourceRewards(t *testing.T) {
	s, a, b := attendanceFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	r, err := s.Observe(ctx, a, now, "2026-09-01")
	require.NoError(t, err)
	require.EqualValues(t, 1, r.Progress.Visits)
	r, err = s.Observe(ctx, b, now, "2026-09-01")
	require.NoError(t, err)
	require.EqualValues(t, 1, r.Progress.Visits)
	// A real account writer runs between login and claim; activity state survives.
	_, _, _, err = s.Store.CommitAdventure(ctx, a.AccountID, a.ID, "settings-fixture", func(role database.Character, p *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		p.Data.SymbolID = 7
		return role.State, json.RawMessage(`{}`), nil
	})
	require.NoError(t, err)
	s.Rules.Days[0].Rewards[1].Count = 17 // grant follows source projection, not old constants
	r, err = s.Claim(ctx, a, now, "2026-09-01", 0)
	require.NoError(t, err)
	require.True(t, r.CreatedMail)
	first := r.MailID
	r, err = s.Claim(ctx, a, now, "2026-09-01", 0)
	require.NoError(t, err)
	require.False(t, r.CreatedMail)
	require.Equal(t, first, r.MailID)
	r, err = s.Claim(ctx, b, now, "2026-09-01", 0)
	require.NoError(t, err)
	require.False(t, r.CreatedMail)
	require.True(t, r.Progress.Claimed[0])
	box, err := s.Store.Mailbox(ctx, a.AccountID, a.ID)
	require.NoError(t, err)
	require.Len(t, box, 1)
	require.Len(t, box[0].Assets, 2)
	var item inventory.MailItem
	require.NoError(t, json.Unmarshal(box[0].Assets[1].Item, &item))
	require.EqualValues(t, 17, item.Stack.Amount)
	other, err := s.Store.Mailbox(ctx, b.AccountID, b.ID)
	require.NoError(t, err)
	require.Empty(t, other)
	characters, err := s.Store.Characters(ctx, a.AccountID)
	require.NoError(t, err)
	for _, current := range characters {
		require.JSONEq(t, string(a.State), string(current.State))
	}
	// One later observed login unlocks one more row, not every offline day.
	r, err = s.Observe(ctx, b, now.AddDate(0, 0, 3), "2026-09-04")
	require.NoError(t, err)
	require.EqualValues(t, 2, r.Progress.Visits)
	_, err = s.Claim(ctx, b, now.AddDate(0, 0, 3), "2026-09-04", 2)
	require.Error(t, err)
	r, err = s.Claim(ctx, b, now.AddDate(0, 0, 3), "2026-09-04", 1)
	require.NoError(t, err)
	require.True(t, r.CreatedMail)
}

func TestAttendanceConcurrentCharactersCreateOneMail(t *testing.T) {
	s, a, b := attendanceFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	var wg sync.WaitGroup
	results := make(chan AttendanceResult, 2)
	errs := make(chan error, 2)
	for _, role := range []database.Character{a, b} {
		wg.Add(1)
		go func(role database.Character) {
			defer wg.Done()
			r, err := s.Claim(ctx, role, now, "2026-09-01", 0)
			results <- r
			errs <- err
		}(role)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	created := 0
	for r := range results {
		if r.CreatedMail {
			created++
		}
	}
	require.Equal(t, 1, created)
	ma, err := s.Store.Mailbox(ctx, a.AccountID, a.ID)
	require.NoError(t, err)
	mb, err := s.Store.Mailbox(ctx, b.AccountID, b.ID)
	require.NoError(t, err)
	require.Equal(t, 1, len(ma)+len(mb))
}

func TestAttendanceFullMailboxRollsBackAndRetrySucceeds(t *testing.T) {
	s, a, _ := attendanceFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	for i := 0; i < mail.MaxMessages; i++ {
		_, _, err := s.Store.CommitSystemMail(ctx, a.AccountID, a.ID, a.ConfigVersion, fmt.Sprintf("fill:%d", i), "fixture", "Fill", "Fill", nil)
		require.NoError(t, err)
	}
	_, err := s.Claim(ctx, a, now, "2026-09-01", 0)
	require.True(t, errors.Is(err, database.ErrMailFull), err)
	p, err := s.Store.LoadAdventure(ctx, a.AccountID, a.ID, a.Name)
	require.NoError(t, err)
	require.Empty(t, p.Data.Attendance)
	box, err := s.Store.Mailbox(ctx, a.AccountID, a.ID)
	require.NoError(t, err)
	require.Len(t, box, mail.MaxMessages)
	_, _, _, err = s.Store.MutateMailbox(ctx, a.AccountID, a.ID, a.ConfigVersion, "delete-fill", "mail-status-v1", []int64{box[0].ID}, func(role database.Character, messages []database.MailMessage) (json.RawMessage, []database.MailMessage, json.RawMessage, error) {
		messages[0].Deleted = true
		return role.State, messages, json.RawMessage(`{}`), nil
	})
	require.NoError(t, err)
	r, err := s.Claim(ctx, a, now, "2026-09-01", 0)
	require.NoError(t, err)
	require.True(t, r.CreatedMail)
	require.True(t, r.Progress.Claimed[0])
	require.EqualValues(t, 1, r.Progress.Visits)
}
