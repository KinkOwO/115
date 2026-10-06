package database

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func maxLevelRewardCatalogFixture() *catalog.MaxLevelReward {
	return &catalog.MaxLevelReward{
		Source:     strings.Repeat("a", 64),
		Definition: catalog.ScriptRecord{Path: catalog.MaxLevelRewardPath, SHA256: strings.Repeat("b", 64)},
		Template:   10362946,
		Count:      1,
		Title:      "Max Level Reward",
		Text:       "You are now reached the Level 86. Take this box and I will let you ignite.",
	}
}

// 本仓 2026-10-05 起存储只有 SQLite（AGENTS.md §0.6），交付包给的用例是 PostgreSQL 专用
// （PostgresDSN + CREATE SCHEMA + $1 占位符）⇒ 按 sqlite 引擎重写同一组判据：
// 源文案落库、并发重放只有一封、未达上限不发不收据、满邮箱整笔回滚且清位后可重试。
func TestMaxLevelRewardMailSQLite(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	store, err := Open(ctx, Config{SQLitePath: filepath.Join(t.TempDir(), "max-level-reward.db"), MaxConnections: 4})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer store.Close()
	db := store.engine.(*sqliteEngine).db

	account, err := store.DevelopmentAccount(ctx, "max-level-reward-test")
	if err != nil {
		t.Fatal(err)
	}
	progress := &character.ProgressionService{Store: store, MaxLevelReward: maxLevelRewardCatalogFixture(), Rules: character.GrowthRules{LevelCap: 115}}
	create := func(name, state string) Character {
		req := append([]byte{0, 4, 0, 0, 0}, []byte("test")...)
		req = append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0)
		for len(req)%8 != 0 {
			req = append(req, 0)
		}
		role, err := store.CreateCharacter(ctx, Character{AccountID: account, Name: name, Request: req, ConfigVersion: savecontract.Identity(), State: json.RawMessage(state)}, 24)
		if err != nil {
			t.Fatal(err)
		}
		return role
	}

	// 到上限：发一封信，发件人与正文逐字取自源，存档其它字段不动。
	role := create("CapReward", `{"level":115,"inventory":{"sentinel":"preserve"},"unknown":"keep"}`)
	next, applied, err := progress.MaxLevelRewardMail(ctx, role)
	if err != nil || !applied {
		t.Fatal(applied, err)
	}
	messages, err := store.Mailbox(ctx, account, role.ID)
	if err != nil || len(messages) != 1 || len(messages[0].Assets) != 1 {
		t.Fatal(messages, err)
	}
	if messages[0].SenderName != "Max Level Reward" || !strings.Contains(messages[0].Text, "Take this box") {
		t.Fatalf("mail wording is not the source's: %+v", messages[0])
	}
	var attachment inventory.MailItem
	if err = json.Unmarshal(messages[0].Assets[0].Item, &attachment); err != nil || attachment.Stack.Template != 10362946 || attachment.Stack.Amount != 1 {
		t.Fatal(attachment, err)
	}
	var doc map[string]any
	if err = json.Unmarshal(next.State, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["unknown"] != "keep" || doc["inventory"].(map[string]any)["sentinel"] != "preserve" {
		t.Fatalf("reward rewrote the save: %+v", doc)
	}

	// 登录、回城、副本结算都会重试同一个事件键：并发下也只有一封信。
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, applied, err := progress.MaxLevelRewardMail(ctx, role); err != nil || applied {
				t.Errorf("replay: %v %v", applied, err)
			}
		}()
	}
	wg.Wait()
	if messages, err = store.Mailbox(ctx, account, role.ID); err != nil || len(messages) != 1 {
		t.Fatal("duplicate mail", messages, err)
	}

	// 未达上限的角色不发信也不留收据。
	before := create("BelowCap", `{"level":114}`)
	if _, applied, err = progress.MaxLevelRewardMail(ctx, before); err != nil || applied {
		t.Fatal(applied, err)
	}
	var receipts int
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM character_events WHERE character_id=? AND event_key=?`, before.ID, MaxLevelRewardMailEvent).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatal(receipts, err)
	}

	// 满邮箱时整笔回滚：既不写信也不留下收据，下次登录仍可重试。
	full := create("InboxFull", `{"level":115}`)
	filled := false
	for i := 0; i < 400; i++ {
		if _, _, err = store.CommitSystemMail(ctx, account, full.ID, full.ConfigVersion,
			fmt.Sprintf("fill-%d", i), "fill-mail-v1", "GM", "fixture", nil); errors.Is(err, ErrMailFull) {
			filled = true
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	if !filled {
		t.Fatal("mailbox never reported full")
	}
	if _, applied, err = progress.MaxLevelRewardMail(ctx, full); !errors.Is(err, ErrMailFull) || applied {
		t.Fatal(applied, err)
	}
	if err = db.QueryRowContext(ctx, `SELECT count(*) FROM character_events WHERE character_id=? AND event_key=?`, full.ID, MaxLevelRewardMailEvent).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatal(receipts, err)
	}

	// 腾出一个位置后重试成功（收据与邮件同事务，回滚不留半成品）。
	if _, err = db.ExecContext(ctx,
		`UPDATE character_mail SET deleted_at=CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER) WHERE id=(SELECT min(id) FROM character_mail WHERE recipient_id=?)`,
		full.ID); err != nil {
		t.Fatal(err)
	}
	if _, applied, err = progress.MaxLevelRewardMail(ctx, full); err != nil || !applied {
		t.Fatal(applied, err)
	}
}
