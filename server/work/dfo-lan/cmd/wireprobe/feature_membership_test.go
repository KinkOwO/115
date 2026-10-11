package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/featurebridge"
	"dfolan/internal/gamedata"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFeatureMembershipMaterialSourceAllGradesRepeatAndPaybacks(t *testing.T) {
	archive := os.Getenv("US115_FEATURE_MEMBERSHIP_PVF")
	if archive == "" {
		t.Skip("explicit isolated candidate required")
	}
	c, e := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "items,membership", ArchivePath: archive, DerivedCacheDir: "-"}, gamedata.CatalogAdapters{})
	require.NoError(t, e)
	ctx := context.Background()
	store, e := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "membership.sqlite3")})
	require.NoError(t, e)
	defer store.Close()
	require.NoError(t, store.MigrateAdventure(ctx))
	require.NoError(t, store.MigrateCashShop(ctx))
	account, e := store.DevelopmentAccount(ctx, "membership-source")
	require.NoError(t, e)
	role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "Membership", Request: []byte{}, ConfigVersion: c.Items.Source.SaveIdentity(), State: json.RawMessage(`{"level":115}`)}, 100)
	require.NoError(t, e)
	loc := time.FixedZone("HK", 8*3600)
	svc, e := workflow.NewMembershipService(store, *c.Membership, *c.Items, loc)
	require.NoError(t, e)
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, loc)
	svc.Now = func() time.Time { return now }
	require.NoError(t, svc.Bind())
	credit := svc.Rules.Cap() + 2*svc.Rules.RepeatPoints + 1
	_, e = store.ApplyGrant(ctx, database.Grant{ID: "source-membership-fund", AccountID: account, Cera: int64(credit), Operator: "fixture", Reason: "isolated integration"}, nil)
	require.NoError(t, e)
	_, _, e = store.PurchaseCash(ctx, database.CashOrder{Key: "source-membership-purchase", Account: account, Character: role.ID, Source: role.ConfigVersion, Lines: []database.CashOrderLine{{Product: 1, Template: 10361515, Quantity: 1, Units: 1, UnitPrice: uint32(credit)}}})
	require.NoError(t, e)
	n := 0
	w := &worldSession{membership: svc, account: account, role: role, fatigue: &character.FatigueService{Location: loc}, notifyBoostMail: func(int64) { n++ }}
	client := &gameConnection{bootstrapped: true, selectedCharacterID: role.ID, worldState: w}
	claim := func(group byte, index uint32) {
		body := make([]byte, 104)
		body[0] = featurebridge.Membership
		body[1] = group
		binary.LittleEndian.PutUint32(body[4:], index)
		copy(body[8:], svc.Rules.SourceChecksum)
		copy(body[72:], svc.Rules.CampaignKey()[:32])
		call := featurebridge.Call{Request: featurebridge.Frame{Kind: featurebridge.Claim, ID: 1, Body: body}, Reply: make(chan featurebridge.Frame, 1)}
		client.featureCall(call, now)
		require.EqualValues(t, 0, (<-call.Reply).Status)
	}
	for _, g := range svc.Rules.Grades {
		claim(featurebridge.Period, g.Index+1)
		claim(featurebridge.Period, g.Index+1)
	}
	for i := uint32(1); i <= 2; i++ {
		claim(featurebridge.Cumulative, i)
		claim(featurebridge.Cumulative, i)
	}
	require.Equal(t, len(svc.Rules.Grades)+2, n)
	for _, b := range svc.Rules.Paybacks {
		run := fmt.Sprintf("membership-source-%d", b.Dungeon)
		receipt := character.ClearReceipt{Source: role.ConfigVersion, Run: run, DungeonID: b.Dungeon, CharacterLevel: 115, CompletedAt: now}
		for i := 0; i < 2; i++ {
			_, applied, e := store.CommitCharacterEvent(ctx, account, role.ID, role.ConfigVersion, "clear:"+run, "source", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
				raw, e := json.Marshal(receipt)
				return current.State, raw, e
			})
			require.NoError(t, e)
			require.Equal(t, i == 0, applied)
		}
	}
	mail, e := store.Mailbox(ctx, account, role.ID)
	require.NoError(t, e)
	require.Len(t, mail, len(svc.Rules.Grades)+2+len(svc.Rules.Paybacks))
	t.Logf("grades=%d repeat=2 paybacks=%d mails=%d", len(svc.Rules.Grades), len(svc.Rules.Paybacks), len(mail))
	view, e := w.membershipView(ctx, now)
	require.NoError(t, e)
	require.Equal(t, featurebridge.Locked, view.Rows[len(view.Rows)-1].State)
}
