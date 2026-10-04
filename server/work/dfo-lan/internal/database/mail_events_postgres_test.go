package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func mailEventFixture(t *testing.T) (*Store, context.Context, int64, Character, Character) {
	t.Helper()
	s, ctx := sqlcTestStore(t)
	for _, migrate := range []func(context.Context) error{s.Migrate, s.MigrateCharacterEvents, s.MigrateMailbox} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "mail-events")
	if err != nil {
		t.Fatal(err)
	}
	roles := make([]Character, 2)
	for i := range roles {
		roles[i], err = s.CreateCharacter(ctx, Character{AccountID: account, Name: fmt.Sprintf("MailRole%d", i), ConfigVersion: strings.Repeat("a", 64), Request: []byte{0, 255}, State: json.RawMessage(`{"unknown":{"keep":true},"gold":100}`)}, 4)
		if err != nil {
			t.Fatal(err)
		}
	}
	return s, ctx, account, roles[0], roles[1]
}

func TestSQLCMailSendRollbackReplayAndSharedSequence(t *testing.T) {
	s, ctx, account, sender, receiver := mailEventFixture(t)
	if _, err := s.MailRecipient(ctx, "missing"); !errors.Is(err, ErrMailRecipient) {
		t.Fatal(err)
	}
	recipient, err := s.MailRecipient(ctx, strings.ToUpper(receiver.Name))
	if err != nil || recipient.ID != receiver.ID {
		t.Fatalf("recipient: %+v %v", recipient, err)
	}
	if inbox, err := s.Mailbox(ctx, account, receiver.ID); err != nil || inbox == nil || len(inbox) != 0 {
		t.Fatalf("empty inbox: %+v %v", inbox, err)
	}
	prepareCalls := 0
	prepare := func(c Character) (json.RawMessage, []MailAsset, error) {
		prepareCalls++
		return json.RawMessage(`{"unknown":{"keep":true},"gold":90}`), []MailAsset{{Gold: 5}, {Item: json.RawMessage(`{"unknown_item":{"binary":"00ff"}}`)}}, nil
	}
	var first MailSendReceipt
	for i := 0; i < 2; i++ {
		role, receipt, applied, err := s.SendMail(ctx, account, sender.ID, sender.ConfigVersion, "send", receiver.Name, "hello", prepare)
		if err != nil || applied != (i == 0) || role.ID != sender.ID || receipt.RecipientID != receiver.ID {
			t.Fatalf("send %d: %+v %v %v", i, receipt, applied, err)
		}
		if i == 0 {
			first = receipt
		} else if receipt != first {
			t.Fatalf("replay changed receipt: %+v %+v", first, receipt)
		}
	}
	if prepareCalls != 1 {
		t.Fatalf("prepare calls: %d", prepareCalls)
	}
	var raw json.RawMessage
	if err := s.db.QueryRow(ctx, `SELECT outcome FROM character_events WHERE character_id=$1 AND event_key='send'`, sender.ID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var receiptKeys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &receiptKeys); err != nil {
		t.Fatal(err)
	}
	if receiptKeys["MessageID"] == nil || receiptKeys["RecipientID"] == nil {
		t.Fatalf("historical receipt keys changed: %s", raw)
	}
	inbox, err := s.Mailbox(ctx, account, receiver.ID)
	if err != nil || len(inbox) != 1 || len(inbox[0].Assets) != 2 || inbox[0].SenderID != sender.ID {
		t.Fatalf("inbox: %+v %v", inbox, err)
	}
	if inbox[0].Assets[0].ID >= inbox[0].Assets[1].ID || inbox[0].Assets[1].ID >= inbox[0].ID || !sameJSON(t, inbox[0].Assets[1].Item, json.RawMessage(`{"unknown_item":{"binary":"00ff"}}`)) {
		t.Fatalf("shared IDs or item changed: %+v", inbox[0])
	}
	if latest, unread, err := s.MailboxDeliveryState(ctx, account, receiver.ID); err != nil || latest != first.MessageID || unread != 1 {
		t.Fatalf("delivery: %d %d %v", latest, unread, err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,'conflict',$2,'other-model','{}')`, sender.ID, sender.ConfigVersion); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := s.SendMail(ctx, account, sender.ID, sender.ConfigVersion, "conflict", receiver.Name, "rollback", func(c Character) (json.RawMessage, []MailAsset, error) {
		return json.RawMessage(`{"gold":0}`), []MailAsset{{Gold: 10}}, nil
	}); err == nil {
		t.Fatal("receipt conflict committed")
	}
	inbox, err = s.Mailbox(ctx, account, receiver.ID)
	if err != nil || len(inbox) != 1 {
		t.Fatalf("failed send persisted mail: %+v %v", inbox, err)
	}
	role, err := s.AdminCharacter(ctx, sender.ID)
	if err != nil || !sameJSON(t, role.State, json.RawMessage(`{"unknown":{"keep":true},"gold":90}`)) {
		t.Fatalf("failed send changed save: %s %v", role.State, err)
	}
	systemID, applied, err := s.CommitSystemMail(ctx, account, receiver.ID, receiver.ConfigVersion, "system", "reward-mail-v1", "System", "empty", nil)
	if err != nil || !applied || systemID <= first.MessageID {
		t.Fatalf("system mail: %d %v %v", systemID, applied, err)
	}
	inbox, err = s.Mailbox(ctx, account, receiver.ID)
	if err != nil || len(inbox) != 2 || inbox[1].SenderID != 0 || inbox[1].Assets == nil || len(inbox[1].Assets) != 0 {
		t.Fatalf("system sender/empty assets: %+v %v", inbox, err)
	}
	// Historical behavior returns no new message id on an idempotent system replay.
	if replayID, applied, err := s.CommitSystemMail(ctx, account, receiver.ID, receiver.ConfigVersion, "system", "reward-mail-v1", "System", "empty", nil); err != nil || applied || replayID != 0 {
		t.Fatalf("system replay: %d %v %v", replayID, applied, err)
	}
}

func TestSQLCMailSelectionSoftDeleteAndAtomicClaim(t *testing.T) {
	s, ctx, account, _, receiver := mailEventFixture(t)
	for i := 0; i < 3; i++ {
		if _, _, err := s.CommitSystemMail(ctx, account, receiver.ID, receiver.ConfigVersion, fmt.Sprintf("system%d", i), "reward-mail-v1", "System", "mail", []MailAsset{{Gold: 1}}); err != nil {
			t.Fatal(err)
		}
	}
	inbox, err := s.Mailbox(ctx, account, receiver.ID)
	if err != nil {
		t.Fatal(err)
	}
	selectCount := func(key, model string, ids []int64, want int) {
		t.Helper()
		_, _, applied, err := s.MutateMailbox(ctx, account, receiver.ID, receiver.ConfigVersion, key, model, ids, func(c Character, m []MailMessage) (json.RawMessage, []MailMessage, json.RawMessage, error) {
			if len(m) != want {
				return nil, nil, nil, fmt.Errorf("selected %d, want %d", len(m), want)
			}
			return c.State, nil, json.RawMessage(`{}`), nil
		})
		if err != nil || !applied {
			t.Fatalf("selection %s: %v %v", key, applied, err)
		}
	}
	selectCount("nil", "mail-claim-v1", nil, 3)
	selectCount("empty", "mail-claim-v1", []int64{}, 0)
	reject := errors.New("claim refused")
	if _, _, _, err := s.MutateMailbox(ctx, account, receiver.ID, receiver.ConfigVersion, "claim", "mail-claim-v1", []int64{inbox[0].ID}, func(c Character, m []MailMessage) (json.RawMessage, []MailMessage, json.RawMessage, error) {
		return nil, nil, nil, reject
	}); !errors.Is(err, reject) {
		t.Fatal(err)
	}
	if _, _, _, err := s.MutateMailbox(ctx, account, receiver.ID, receiver.ConfigVersion, "claim", "mail-claim-v1", []int64{inbox[0].ID}, func(c Character, m []MailMessage) (json.RawMessage, []MailMessage, json.RawMessage, error) {
		m[0].Assets[0].Claimed = true
		m = append(m, MailMessage{ID: -1, Status: 1, Assets: []MailAsset{}})
		return json.RawMessage(`{"gold":0}`), m, json.RawMessage(`{}`), nil
	}); err == nil {
		t.Fatal("partially invalid mail update committed")
	}
	unchanged, err := s.Mailbox(ctx, account, receiver.ID)
	if err != nil || len(unchanged) != 3 || unchanged[0].Assets[0].Claimed {
		t.Fatalf("failed second mail update did not roll back first: %+v %v", unchanged, err)
	}
	calls := 0
	apply := func(c Character, m []MailMessage) (json.RawMessage, []MailMessage, json.RawMessage, error) {
		calls++
		m[0].Assets[0].Claimed = true
		m[0].Deleted = true
		return json.RawMessage(`{"unknown":{"keep":true},"gold":101}`), m, json.RawMessage(`{"claimed":true}`), nil
	}
	for i := 0; i < 2; i++ {
		_, _, applied, err := s.MutateMailbox(ctx, account, receiver.ID, receiver.ConfigVersion, "claim", "mail-claim-v1", []int64{inbox[0].ID}, apply)
		if err != nil || applied != (i == 0) {
			t.Fatalf("claim replay %d: %v %v", i, applied, err)
		}
	}
	if calls != 1 {
		t.Fatalf("claim callbacks: %d", calls)
	}
	selectCount("after-delete", "mail-claim-v1", []int64{inbox[0].ID}, 0)
	selectCount("delete-status", "mail-status-v1", []int64{inbox[0].ID}, 1)
	if _, err := s.db.Exec(ctx, `UPDATE character_mail SET expires_at=now()-interval '1 day',status=CASE WHEN id=$1 THEN 3 ELSE 1 END WHERE recipient_id=$2`, inbox[1].ID, receiver.ID); err != nil {
		t.Fatal(err)
	}
	inbox, err = s.Mailbox(ctx, account, receiver.ID)
	if err != nil || len(inbox) != 1 || inbox[0].Status != 3 {
		t.Fatalf("expiry retained-status filter: %+v %v", inbox, err)
	}
	if latest, unread, err := s.MailboxDeliveryState(ctx, account, receiver.ID); err != nil || latest != 0 || unread != 0 {
		t.Fatalf("expired delivery: %d %d %v", latest, unread, err)
	}
}

func TestSQLCMailConcurrentCapacity(t *testing.T) {
	s, ctx, account, sender, receiver := mailEventFixture(t)
	if _, err := s.db.Exec(ctx, `INSERT INTO character_mail(recipient_id,sender_name,body,assets,expires_at) SELECT $1,'System','','[]',now()+interval '1 day' FROM generate_series(1,254)`, receiver.ID); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, _, err := s.SendMail(ctx, account, sender.ID, sender.ConfigVersion, fmt.Sprintf("capacity%d", i), receiver.Name, "capacity", func(c Character) (json.RawMessage, []MailAsset, error) { return c.State, nil, nil })
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrMailFull) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("capacity successes: %d", successes)
	}
	inbox, err := s.Mailbox(ctx, account, receiver.ID)
	if err != nil || len(inbox) != 255 {
		t.Fatalf("capacity: %d %v", len(inbox), err)
	}
}

func TestSQLCMailLegacyUnclaimedAssetCapacity(t *testing.T) {
	s, ctx, account, sender, receiver := mailEventFixture(t)
	assets := make([]MailAsset, 255)
	for i := range assets {
		assets[i] = MailAsset{ID: int64(i + 1), Gold: 1}
	}
	raw, err := json.Marshal(assets)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO character_mail(recipient_id,sender_name,body,assets,expires_at) VALUES($1,'Legacy','',$2,now()+interval '1 day')`, receiver.ID, raw); err != nil {
		t.Fatal(err)
	}
	prepare := func(c Character) (json.RawMessage, []MailAsset, error) { return c.State, []MailAsset{{Gold: 1}}, nil }
	if _, _, _, err := s.SendMail(ctx, account, sender.ID, sender.ConfigVersion, "asset-capacity", receiver.Name, "mail", prepare); !errors.Is(err, ErrMailFull) {
		t.Fatalf("legacy attachment capacity ignored: %v", err)
	}
	assets[0].Claimed = true
	raw, err = json.Marshal(assets)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `UPDATE character_mail SET assets=$1 WHERE recipient_id=$2`, raw, receiver.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, applied, err := s.SendMail(ctx, account, sender.ID, sender.ConfigVersion, "asset-capacity", receiver.Name, "mail", prepare); err != nil || !applied {
		t.Fatalf("claimed asset still consumed capacity or failed key persisted: %v %v", applied, err)
	}
}

func TestSQLCBlackPurgatoryQuotaRewardAndRecovery(t *testing.T) {
	s, ctx, account, role, other := mailEventFixture(t)
	day := time.Now().UTC().Truncate(24 * time.Hour)
	week := day.Add(-6 * 24 * time.Hour)
	run := strings.Repeat("1", 32)
	for i := 0; i < 2; i++ {
		quota, err := s.BlackPurgatoryQuota(ctx, account, role.ID, run, "enter", day, week)
		if err != nil || quota != (BlackPurgatoryQuota{0, 1}) {
			t.Fatalf("entry %d: %+v %v", i, quota, err)
		}
	}
	if _, err := s.FreezeBlackPurgatoryReward(ctx, account, role.ID, role.ConfigVersion, run, "black-purgatory-card-v1", func() (json.RawMessage, error) { return json.RawMessage(`bad`), nil }); err == nil {
		t.Fatal("invalid plan accepted")
	}
	calls := 0
	create := func() (json.RawMessage, error) { calls++; return json.RawMessage(`{"unknown_reward":[1,2]}`), nil }
	for i := 0; i < 2; i++ {
		raw, err := s.FreezeBlackPurgatoryReward(ctx, account, role.ID, role.ConfigVersion, run, "black-purgatory-card-v1", create)
		if err != nil || !sameJSON(t, raw, json.RawMessage(`{"unknown_reward":[1,2]}`)) {
			t.Fatalf("freeze: %s %v", raw, err)
		}
	}
	if calls != 1 {
		t.Fatalf("reward created %d times", calls)
	}
	if quota, err := s.BlackPurgatoryQuota(ctx, account, role.ID, run, "refund", day, week); err != nil || quota != (BlackPurgatoryQuota{0, 1}) {
		t.Fatalf("cleared refunded: %+v %v", quota, err)
	}
	if _, err := s.BlackPurgatoryQuota(ctx, account, role.ID, run, "enter", day, week); err == nil {
		t.Fatal("cleared run reused")
	}
	if _, err := s.BlackPurgatoryQuota(ctx, account, other.ID, run, "enter", day, week); err != nil {
		t.Fatal(err)
	}
	if quota, err := s.BlackPurgatoryQuota(ctx, account, other.ID, "", "recover", day, week); err != nil || quota != (BlackPurgatoryQuota{1, 2}) {
		t.Fatalf("recover: %+v %v", quota, err)
	}
	if _, err := s.BlackPurgatoryQuota(ctx, account, other.ID, run, "enter", day, week); err == nil {
		t.Fatal("refunded run reused")
	}
}

func TestSQLCBlackPendingPlansOrderLimitAndCallbackFailure(t *testing.T) {
	s, ctx, account, role, _ := mailEventFixture(t)
	for i := 0; i < 20; i++ {
		if _, err := s.db.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome,created_at) VALUES($1,$2,$3,'black-purgatory-card-v1',jsonb_build_object('ordinal',$4::integer),now()+$4*interval '1 second')`, role.ID, fmt.Sprintf("cardplan:%d", i), role.ConfigVersion, i); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,'black-purgatory-recovered:0',$2,'recovered','{}')`, role.ID, role.ConfigVersion); err != nil {
		t.Fatal(err)
	}
	count := 0
	reject := errors.New("decode stopped")
	err := s.ReadPendingBlackPurgatoryRewards(ctx, account, role.ID, "black-purgatory-card-v1", func(raw json.RawMessage) error {
		count++
		var p struct{ Ordinal int }
		if err := json.Unmarshal(raw, &p); err != nil {
			return err
		}
		if p.Ordinal != count {
			return fmt.Errorf("order: %d at %d", p.Ordinal, count)
		}
		if count == 2 {
			return reject
		}
		return nil
	})
	if !errors.Is(err, reject) || count != 2 {
		t.Fatalf("callback error: %d %v", count, err)
	}
	count = 0
	if err := s.ReadPendingBlackPurgatoryRewards(ctx, account, role.ID, "black-purgatory-card-v1", func(json.RawMessage) error { count++; return nil }); err != nil || count != 16 {
		t.Fatalf("pending limit: %d %v", count, err)
	}
	count = 0
	if err := s.ReadPendingBlackPurgatoryRewards(ctx, account+99, role.ID, "black-purgatory-card-v1", func(json.RawMessage) error { count++; return nil }); err != nil || count != 0 {
		t.Fatalf("owner filter: %d %v", count, err)
	}
}
