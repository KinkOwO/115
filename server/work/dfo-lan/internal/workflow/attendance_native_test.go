package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/gamedata"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAttendanceEightDayMaterialCandidateNative(t *testing.T) {
	archive := os.Getenv("US115_TEST_WEEKLY_PVF")
	if archive == "" {
		t.Skip("explicit isolated paired activity PVF required")
	}
	catalogs, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "items,attendance-login", ArchivePath: archive, DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	require.NoError(t, err)
	require.NotNil(t, catalogs.AttendanceLogin)
	require.Len(t, catalogs.AttendanceLogin.Days, 8)
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "eightday.sqlite3")})
	require.NoError(t, err)
	defer store.Close()
	service, err := NewAttendanceService(store, *catalogs.AttendanceLogin, *catalogs.Items)
	require.NoError(t, err)
	account, err := store.DevelopmentAccount(ctx, "eightday-candidate")
	require.NoError(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "Eight day candidate", Request: []byte{}, ConfigVersion: catalogs.Items.Source.SaveIdentity(), State: json.RawMessage(`{"level":115}`)}, 24)
	require.NoError(t, err)
	loc := time.FixedZone("HK", 8*3600)
	start, err := time.ParseInLocation("2006-01-02 15:04:05", service.Rules.Dates.Start, loc)
	require.NoError(t, err)
	for index := range service.Rules.Days {
		now := start.AddDate(0, 0, index).Add(time.Hour)
		day := now.Add(-6 * time.Hour).Format("2006-01-02")
		observed, err := service.Observe(ctx, role, now, day)
		require.NoError(t, err)
		require.EqualValues(t, index+1, observed.Progress.Visits)
		reward, err := service.Claim(ctx, role, now, day, uint32(index))
		require.NoError(t, err)
		require.True(t, reward.CreatedMail)
		replay, err := service.Claim(ctx, role, now, day, uint32(index))
		require.NoError(t, err)
		require.False(t, replay.CreatedMail)
	}
	mailbox, err := store.Mailbox(ctx, account, role.ID)
	require.NoError(t, err)
	require.Len(t, mailbox, 8)
	for _, mail := range mailbox {
		require.NotEmpty(t, mail.SenderName)
		require.NotEmpty(t, mail.Text)
		require.NotEmpty(t, mail.Assets)
	}
}
