package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/mail"
	"dfolan/internal/savecontract"
	"encoding/json"
	"testing"
	"time"
)

func TestMailWireTimestampPreservesStoredDeadline(t *testing.T) {
	now := time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)
	retention := time.Duration(mail.RetentionDays) * 24 * time.Hour
	for _, age := range []time.Duration{0, 3 * 24 * time.Hour, retention - time.Second} {
		sent := now.Add(-age)
		deadline := sent.Add(retention)
		got := mailWireSentAt(database.MailMessage{Status: 2, ExpiresAt: deadline})
		// Native list view: wire + 0x13c680 - UnixNow (145FD5135..145FD5151).
		displaySeconds := int64(got) + int64(retention/time.Second) - now.Unix()
		if got != uint32(sent.Unix()) || displaySeconds != int64(deadline.Sub(now)/time.Second) {
			t.Fatalf("age=%s stamp=%d remaining=%d", age, got, displaySeconds)
		}
	}
	if n := mailWireSentAt(database.MailMessage{Status: 3}); n != 0 {
		t.Fatalf("saved timestamp=%d", n)
	}
	if n := mailWireSentAt(database.MailMessage{Status: 1, ExpiresAt: time.Unix(0, 0)}); n != 0 {
		t.Fatalf("pre-epoch wrapped=%d", n)
	}
}

func TestMailboxExistingChineseMailUsesCorrectedWireOnly(t *testing.T) {
	deadline := time.Now().Add(10 * 24 * time.Hour).Truncate(time.Second)
	item := json.RawMessage(`{"stack":{"slot":65,"Template":100,"Amount":1}}`)
	messages := []database.MailMessage{
		{ID: 209, SenderName: "奇迹加速", Text: "奇迹加速等级奖励已送达，请注意查收。", Status: 2, ExpiresAt: deadline, Assets: []database.MailAsset{{ID: 208, Item: item}}},
		{ID: 211, SenderName: "奇迹加速", Text: "奇迹加速升级奖励已送达，请注意查收。", Status: 1, ExpiresAt: deadline},
		{ID: 207, SenderName: "最高等级奖励", Text: "您现在已经达到86级。", Status: 2, ExpiresAt: deadline},
		{ID: 201, SenderName: "GM", Text: "expired", Status: 1, ExpiresAt: time.Now().Add(-time.Hour)},
		{ID: 203, SenderName: "GM", Text: "saved", Status: 3, ExpiresAt: time.Now().Add(-time.Hour)},
	}
	before, e := json.Marshal(messages)
	if e != nil {
		t.Fatal(e)
	}
	packets, e := mailboxSnapshot(messages)
	if e != nil || len(packets) != 1 || packets[0].ID != 97 {
		t.Fatalf("packets=%v error=%v", packets, e)
	}
	for _, m := range []database.MailMessage{messages[0], messages[1], messages[2], messages[4]} {
		if !bytes.Contains(packets[0].Payload, []byte(m.Text)) {
			t.Fatalf("mail %d text missing", m.ID)
		}
	}
	if bytes.Contains(packets[0].Payload, []byte("expired")) {
		t.Fatal("expired mail shown")
	}
	expected, e := protocol.MailboxList([]protocol.MailAttachmentView{{ID: 208, MessageID: 209, Sender: messages[0].SenderName, SentAt: mailWireSentAt(messages[0])}}, nil, 0)
	if e != nil {
		t.Fatal(e)
	}
	if expected[0] != 1 || packets[0].Payload[0] != 1 {
		t.Fatal("unclaimed attachment lost")
	}
	after, e := json.Marshal(messages)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("snapshot rewrote stored mail or asset state")
	}
}

func TestMaxLevelRewardExistingAndFutureMailPresentation(t *testing.T) {
	ctx := context.Background()
	fixture, e := database.OpenTestFixture(ctx)
	must115(t, e)
	defer fixture.Close()
	store := fixture.Storage()
	for _, m := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents, store.MigrateMailbox} {
		must115(t, m(ctx))
	}
	account, e := store.DevelopmentAccount(ctx, "mail-display-fixture")
	must115(t, e)
	rule := &catalog.MaxLevelReward{Title: "最高等级奖励", Text: "您现在已经达到86级。拿着这个盒子。", Template: 100, Count: 1}
	svc := &character.ProgressionService{Store: store, MaxLevelReward: rule, Rules: character.GrowthRules{LevelCap: 115}}
	create := func(name string) database.Character {
		r, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, ConfigVersion: savecontract.Identity(), Request: []byte{0}, State: json.RawMessage(`{"level":115,"unchanged":42}`)}, 24)
		must115(t, e)
		return r
	}
	fresh := create("freshmail")
	fresh, applied, e := svc.MaxLevelRewardMail(ctx, fresh)
	must115(t, e)
	if !applied {
		t.Fatal("future mail not sent")
	}
	mails, e := store.Mailbox(ctx, account, fresh.ID)
	must115(t, e)
	if len(mails) != 1 || mails[0].Text != rule.TextForLevelCap(115) {
		t.Fatalf("new mails=%v", mails)
	}
	_, applied, e = svc.MaxLevelRewardMail(ctx, fresh)
	must115(t, e)
	if applied {
		t.Fatal("reward sent twice")
	}
	legacy := create("legacymail")
	legacy, applied, e = store.CommitMaxLevelRewardMail(ctx, account, legacy.ID, legacy.ConfigVersion, rule.Title, rule.Text, svc.ApplyMaxLevelRewardMail)
	must115(t, e)
	if !applied {
		t.Fatal("legacy fixture missing")
	}
	before, e := store.Mailbox(ctx, account, legacy.ID)
	must115(t, e)
	storedBefore, e := json.Marshal(before)
	must115(t, e)
	w := &worldSession{account: account, role: legacy, store: store, characters: &character.Service{Store: store}, progression: svc}
	packets, _, e := w.handleMailbox(ctx, legacy.ID, 96, make([]byte, 16), nil, nil, "mail-display-test")
	must115(t, e)
	if len(packets) != 2 || packets[0].ID != 97 || packets[1].ID != 96 || !bytes.Contains(packets[0].Payload, []byte("达到115级")) {
		t.Fatalf("old mail not corrected: %+v", packets)
	}
	after, e := store.Mailbox(ctx, account, legacy.ID)
	must115(t, e)
	storedAfter, e := json.Marshal(after)
	must115(t, e)
	if !bytes.Equal(storedBefore, storedAfter) || len(after) != 1 || len(after[0].Assets) != 1 || after[0].Assets[0].Claimed {
		t.Fatal("display changed mail/attachments or redelivered reward")
	}
	// Identical wording without the matching durable system-mail ID stays intact.
	originals := []database.MailMessage{{ID: 1, Text: rule.Text}, {ID: 2, Text: rule.Text, SenderID: 10}}
	got := maxLevelRewardMailDisplay(originals, 1, rule, 120)
	if got[0].Text != rule.TextForLevelCap(120) || got[1].Text != rule.Text || originals[0].Text != rule.Text {
		t.Fatal("wrong message or backing slice changed")
	}
	if got := maxLevelRewardMailDisplay(originals, 2, rule, 120); got[1].Text != rule.Text {
		t.Fatal("player mail rewritten")
	}
}
