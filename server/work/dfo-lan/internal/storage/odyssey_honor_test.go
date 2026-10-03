package storage

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
)

func TestOdysseyHonorMailPostgres(t *testing.T) {
	dsn := os.Getenv("DFO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("dedicated PostgreSQL test database required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, err := Open(ctx, Config{PostgresDSN: dsn, MaxConnections: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("odyssey_honor_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	s, err := Open(ctx, Config{PostgresDSN: dsn, PostgresSchema: schema, MaxConnections: 4})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, migrate := range []func(context.Context) error{s.Migrate, s.MigrateCharacterEvents, s.MigrateMailbox} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "odyssey-honor-test")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := catalog.LoadOdysseyCompletionRewards("../../configs/odyssey-completion-rewards.json")
	if err != nil {
		t.Fatal(err)
	}
	progress := &character.ProgressionService{Store: s, CompletionRewards: rules, CompletionAwarder: &inventory.Awarder{Catalog: catalog.LootCatalog{Items: map[uint32]catalog.LootItem{10420561: {ID: 10420561, Kind: "stackable"}}}}}
	create := func(name string) Character {
		req := append([]byte{0, 4, 0, 0, 0}, []byte("test")...)
		req = append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0)
		for len(req)%8 != 0 {
			req = append(req, 0)
		}
		role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: name, Request: req, ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"level":115,"odyssey_graduated":true,"odyssey_graduation_reward_owed":10420561,"inventory":{"sentinel":"preserve"},"unknown":"keep"}`)}, 24)
		if err != nil {
			t.Fatal(err)
		}
		return role
	}
	role := create("HonorMail")
	next, applied, err := progress.OdysseyHonorMail(ctx, role)
	if err != nil || !applied {
		t.Fatal(applied, err)
	}
	messages, err := s.Mailbox(ctx, account, role.ID)
	if err != nil || len(messages) != 1 || len(messages[0].Assets) != 1 {
		t.Fatal(messages, err)
	}
	var attachment inventory.MailItem
	if err = json.Unmarshal(messages[0].Assets[0].Item, &attachment); err != nil || attachment.Stack.Template != 10420561 {
		t.Fatal(attachment, err)
	}
	var doc map[string]any
	json.Unmarshal(next.State, &doc)
	if doc["unknown"] != "keep" || doc["odyssey_graduation_reward_owed"] != float64(0) || doc["inventory"].(map[string]any)["sentinel"] != "preserve" {
		t.Fatal(doc)
	}
	// Concurrent login/clear delivery retries must all read the same receipt.
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, applied, err := progress.OdysseyHonorMail(ctx, role)
			if err != nil || applied {
				t.Errorf("replay: %v %v", applied, err)
			}
		}()
	}
	wg.Wait()
	messages, err = s.Mailbox(ctx, account, role.ID)
	if err != nil || len(messages) != 1 {
		t.Fatal("duplicate mail", messages, err)
	}
	// Claiming/deleting the letter must never restore eligibility.
	if _, err = s.DB.Exec(ctx, `UPDATE character_mail SET deleted_at=now() WHERE recipient_id=$1`, role.ID); err != nil {
		t.Fatal(err)
	}
	if _, applied, err = progress.OdysseyHonorMail(ctx, role); err != nil || applied {
		t.Fatal(applied, err)
	}
	// Legacy graduation boxes are already paid even if the mailbox is full.
	legacy := create("HonorOld")
	legacy, _, err = s.CommitCharacterEvent(ctx, account, legacy.ID, legacy.ConfigVersion, "odyssey-graduate-reward-v1", "odyssey-graduate-reward-v1", func(c Character) (json.RawMessage, json.RawMessage, error) {
		return c.State, json.RawMessage(`{}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, applied, err = progress.OdysseyHonorMail(ctx, legacy); err != nil || !applied {
		t.Fatal(applied, err)
	}
	messages, err = s.Mailbox(ctx, account, legacy.ID)
	if err != nil || len(messages) != 0 {
		t.Fatal("legacy duplicate", messages, err)
	}
	// Full inbox rolls back both debt clearing and payment receipt. Removing
	// one unrelated letter then permits exactly one delivery.
	full := create("HonorFull")
	if _, err = s.DB.Exec(ctx, `INSERT INTO character_mail(recipient_id,sender_name,body,assets,expires_at) SELECT $1,'GM','fixture','[]',now()+interval '15 days' FROM generate_series(1,255)`, full.ID); err != nil {
		t.Fatal(err)
	}
	if _, applied, err = progress.OdysseyHonorMail(ctx, full); !errors.Is(err, ErrMailFull) || applied {
		t.Fatal(applied, err)
	}
	var receipts int
	if err = s.DB.QueryRow(ctx, `SELECT count(*) FROM character_events WHERE character_id=$1 AND event_key=$2`, full.ID, OdysseyHonorMailEvent).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatal(receipts, err)
	}
	var owed uint32
	if err = s.DB.QueryRow(ctx, `SELECT (state->>'odyssey_graduation_reward_owed')::bigint FROM characters WHERE id=$1`, full.ID).Scan(&owed); err != nil || owed != 10420561 {
		t.Fatal(owed, err)
	}
	if _, err = s.DB.Exec(ctx, `UPDATE character_mail SET deleted_at=now() WHERE id=(SELECT min(id) FROM character_mail WHERE recipient_id=$1)`, full.ID); err != nil {
		t.Fatal(err)
	}
	if _, applied, err = progress.OdysseyHonorMail(ctx, full); err != nil || !applied {
		t.Fatal(applied, err)
	}
}
