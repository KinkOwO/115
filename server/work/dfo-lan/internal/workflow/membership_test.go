package workflow

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func membershipServiceFixture(t *testing.T) (*MembershipService, database.Character, database.Character, time.Time) {
	base, a, b := attendanceFixture(t)
	ctx := context.Background()
	require.NoError(t, base.Store.MigrateAdventure(ctx))
	require.NoError(t, base.Store.MigrateCashShop(ctx))
	reward := base.Rules.Days[0].Rewards[0]
	reward.MailTitle = "Membership"
	reward.MailMessage = "Source material"
	rules := activity.MembershipRules{SourceChecksum: base.Rules.SourceChecksum, Table: "membership", Dates: activity.Dates{Start: "2026-10-08 06:00:00", End: "2026-12-10 06:00:00"}, Grades: []activity.MembershipGrade{{Index: 0, Name: "Basic", Minimum: 0, Maximum: 9, Rewards: []activity.Reward{reward}}, {Index: 1, Name: "Rare", Minimum: 10, Maximum: 19, Rewards: []activity.Reward{reward}}, {Index: 2, Name: "Epic", Minimum: 20, Maximum: 20, Rewards: []activity.Reward{reward}}}, RepeatPoints: 5, RepeatRewards: []activity.Reward{reward}, Paybacks: []activity.MembershipPayback{{Dungeon: 1001, Grades: map[uint32]activity.Reward{1: reward, 2: reward}}}}
	s, e := NewMembershipService(base.Store, rules, base.Items, time.UTC)
	require.NoError(t, e)
	now := time.Date(2026, 10, 8, 8, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	require.NoError(t, s.Bind())
	_, e = s.Store.ApplyGrant(ctx, database.Grant{ID: "membership-fund", AccountID: a.AccountID, Cera: 1000, Operator: "fixture", Reason: "isolated test"}, nil)
	require.NoError(t, e)
	return s, a, b, now
}
func TestMembershipPurchaseRollbackReplayGoldAndCrossCharacterClaims(t *testing.T) {
	s, a, b, now := membershipServiceFixture(t)
	ctx := context.Background()
	order := database.CashOrder{Key: "membership-order-001", Account: a.AccountID, Character: a.ID, Source: a.ConfigVersion, Lines: []database.CashOrderLine{{Product: 1, Template: 100, Quantity: 1, Units: 1, UnitPrice: 12}}}
	broken := true
	require.NoError(t, s.Store.RegisterCashAccountObserver(func(_ database.Character, _ database.CashReceipt, _ *database.AccountAdventure) (bool, error) {
		if broken {
			return false, fmt.Errorf("failure after membership credit")
		}
		return false, nil
	}))
	_, _, e := s.Store.PurchaseCash(ctx, order)
	require.Error(t, e)
	balance, e := s.Store.AccountCera(ctx, a.AccountID)
	require.NoError(t, e)
	require.EqualValues(t, 1000, balance)
	profile, e := s.Store.LoadAdventure(ctx, a.AccountID, a.ID, a.Name)
	require.NoError(t, e)
	require.Empty(t, profile.Data.Attendance)
	broken = false
	for i := 0; i < 2; i++ {
		_, applied, e := s.Store.PurchaseCash(ctx, order)
		require.NoError(t, e)
		require.Equal(t, i == 0, applied)
	}
	r, e := s.Run(ctx, b, now, "", 0, "view")
	require.NoError(t, e)
	require.EqualValues(t, 12, r.Progress.Spent)
	order.Key = "membership-gold-001"
	order.Character = b.ID
	order.Lines[0].UnitPrice = 0
	order.Lines[0].GoldUnitPrice = 5
	_, _, e = s.Store.PurchaseCashToBag(ctx, order, func(raw json.RawMessage) (json.RawMessage, error) { return raw, nil })
	require.NoError(t, e)
	r, e = s.Run(ctx, a, now, "", 0, "gold-view")
	require.NoError(t, e)
	require.EqualValues(t, 12, r.Progress.Spent)
	for i, role := range []database.Character{a, b} {
		r, e = s.Run(ctx, role, now, "period", 2, fmt.Sprint("claim", i))
		require.NoError(t, e)
		require.Equal(t, i == 0, r.CreatedMail)
	}
	ma, e := s.Store.Mailbox(ctx, a.AccountID, a.ID)
	require.NoError(t, e)
	mb, e := s.Store.Mailbox(ctx, b.AccountID, b.ID)
	require.NoError(t, e)
	require.Equal(t, 1, len(ma)+len(mb))
}
func TestMembershipClearPaybackAtomicAndPreservesFutureFields(t *testing.T) {
	s, a, _, now := membershipServiceFixture(t)
	ctx := context.Background()
	order := database.CashOrder{Key: "membership-payback-01", Account: a.AccountID, Character: a.ID, Source: a.ConfigVersion, Lines: []database.CashOrderLine{{Product: 1, Template: 100, Quantity: 1, Units: 1, UnitPrice: 12}}}
	_, _, e := s.Store.PurchaseCash(ctx, order)
	require.NoError(t, e)
	_, _, _, e = s.Store.CommitAdventure(ctx, a.AccountID, a.ID, "membership-future", func(role database.Character, p *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		var f map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(p.Data.Attendance[s.Rules.CampaignKey()], &f))
		f["future"] = json.RawMessage(`{"keep":true}`)
		raw, e := json.Marshal(f)
		p.Data.Attendance[s.Rules.CampaignKey()] = raw
		return role.State, json.RawMessage(`{}`), e
	})
	require.NoError(t, e)
	broken := true
	require.NoError(t, s.Store.RegisterCharacterAccountTxObserver(func(_ context.Context, _ *database.Tx, _ database.Character, _ string, _ json.RawMessage, _ *database.AccountAdventure) (bool, error) {
		if broken {
			return false, fmt.Errorf("failure after mail")
		}
		return false, nil
	}))
	run := "membership-clear"
	receipt := character.ClearReceipt{Source: a.ConfigVersion, Run: run, DungeonID: 1001, CharacterLevel: 115, CompletedAt: now.Add(-time.Second)}
	commit := func() (bool, error) {
		_, applied, e := s.Store.CommitCharacterEvent(ctx, a.AccountID, a.ID, a.ConfigVersion, "clear:"+run, "test", func(role database.Character) (json.RawMessage, json.RawMessage, error) {
			raw, e := json.Marshal(receipt)
			return role.State, raw, e
		})
		return applied, e
	}
	_, e = commit()
	require.Error(t, e)
	mail, e := s.Store.Mailbox(ctx, a.AccountID, a.ID)
	require.NoError(t, e)
	require.Empty(t, mail)
	broken = false
	for i := 0; i < 2; i++ {
		applied, e := commit()
		require.NoError(t, e)
		require.Equal(t, i == 0, applied)
	}
	mail, e = s.Store.Mailbox(ctx, a.AccountID, a.ID)
	require.NoError(t, e)
	require.Len(t, mail, 1)
	_, e = s.Run(ctx, a, now, "period", 1, "basic")
	require.NoError(t, e)
	profile, e := s.Store.LoadAdventure(ctx, a.AccountID, a.ID, a.Name)
	require.NoError(t, e)
	require.Contains(t, string(profile.Data.Attendance[s.Rules.CampaignKey()]), `"future":{"keep":true}`)
}
