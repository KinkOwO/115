package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Explicit archive only, disposable SQLite only. Exercise all source rows via
// the public catalog facade, persistence, and real mail attachment construction.
func TestRoutineWeeklyMaterialCandidateNative(t *testing.T) {
	archive := os.Getenv("US115_TEST_WEEKLY_PVF")
	if archive == "" {
		t.Skip("set US115_TEST_WEEKLY_PVF to an isolated weekly material candidate")
	}
	catalogs, e := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "items,attendance-weekly", ArchivePath: archive, DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	require.NoError(t, e)
	rules := catalogs.AttendanceWeekly
	require.NotNil(t, rules)
	require.Len(t, rules.Weekly, 5)
	require.Len(t, rules.Cumulative, 4)
	ctx := context.Background()
	store, e := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "weekly.sqlite3")})
	require.NoError(t, e)
	defer store.Close()
	service, e := NewRoutineService(store, *rules, *catalogs.Items)
	require.NoError(t, e)
	account, e := store.DevelopmentAccount(ctx, "weekly-candidate")
	require.NoError(t, e)
	role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "WeeklyCandidate", Request: []byte{}, ConfigVersion: catalogs.Items.Source.SaveIdentity(), State: json.RawMessage(`{"level":115}`)}, 24)
	require.NoError(t, e)
	loc := time.FixedZone("HK", 8*3600)
	expected := map[uint32]uint32{}
	for i, row := range rules.Weekly {
		start, e := time.ParseInLocation("2006-01-02 15:04:05", row.Dates.Start, loc)
		require.NoError(t, e)
		now := start.Add(time.Duration(row.Required) * time.Second)
		action := RoutineAction{From: start, Now: now, GameDay: now.Format("2006-01-02"), ResetHour: 6, ObservationKey: fmt.Sprintf("period:%d", i), ClaimKind: "period", ClaimIndex: row.Index}
		result, e := service.Run(ctx, role, action)
		require.NoError(t, e)
		require.True(t, result.CreatedMail)
		expected[row.Reward.Item] += row.Reward.Count
		result, e = service.Run(ctx, role, action)
		require.NoError(t, e)
		require.False(t, result.CreatedMail)
		for _, bonus := range rules.Cumulative {
			if bonus.Required != uint32(i+1) {
				continue
			}
			action.From, action.ClaimKind, action.ClaimIndex, action.ObservationKey = now, "cumulative", bonus.Index, fmt.Sprintf("bonus:%d", bonus.Index)
			result, e = service.Run(ctx, role, action)
			require.NoError(t, e)
			require.True(t, result.CreatedMail)
			expected[bonus.Reward.Item] += bonus.Reward.Count
		}
	}
	box, e := store.Mailbox(ctx, account, role.ID)
	require.NoError(t, e)
	require.Len(t, box, 9)
	actual := map[uint32]uint32{}
	for _, message := range box {
		require.NotEmpty(t, message.SenderName)
		require.NotEmpty(t, message.Text)
		for _, asset := range message.Assets {
			var item inventory.MailItem
			require.NoError(t, json.Unmarshal(asset.Item, &item))
			require.Equal(t, "[material]", catalogs.Items.Items[item.Stack.Template].StackableType)
			actual[item.Stack.Template] += item.Stack.Amount
		}
	}
	require.Equal(t, expected, actual)
	p, e := store.LoadAdventure(ctx, account, role.ID, role.Name)
	require.NoError(t, e)
	progress, e := routineProgress(p, rules.CampaignKey())
	require.NoError(t, e)
	require.Len(t, progress.Claimed, 9)
	t.Logf("source=%s weekly=5 cumulative=4 mails=%d material_types=%d storage=temporary-sqlite gateway_connected=false", rules.SourceChecksum, len(box), len(actual))
}
