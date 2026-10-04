package database

import (
	"errors"
	"math"
	"strings"
	"sync"
	"testing"
)

func TestGMMailAdoptsLegacyQueueAndChecksRecipients(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	// Install the exact former Python DDL before adopting it into the ledger.
	ddl, err := migrationQuery("0037_gm_mail.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, string(ddl)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(ctx, `INSERT INTO gm_mail(to_account_id,template,amount,title,body,status) VALUES(888,55,7,'legacy','preserve body','claimed')`); err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateGMMail(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := s.DevelopmentAccount(ctx, "gm-mail")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.DevelopmentAccount(ctx, "other-gm-mail")
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "GMMail", Request: []byte{0}, ConfigVersion: strings.Repeat("a", 64), State: []byte(`{"unknown":9007199254740993}`)}, 2)
	if err != nil {
		t.Fatal(err)
	}
	title := "中文|O'Brien\n第二行"
	id, err := s.SendGMMail(ctx, account, role.ID, 55, math.MaxUint32, title, "body 'quoted'")
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct{ account, character, amount int64 }{{other, role.ID, 1}, {math.MaxInt64, 0, 1}, {account, 0, math.MaxUint32 + 1}} {
		if _, err := s.SendGMMail(ctx, bad.account, bad.character, 55, bad.amount, "bad", ""); err == nil {
			t.Fatal("invalid mail inserted")
		}
	}
	rows, err := s.GMMails(ctx, account, "unread")
	if err != nil || len(rows) != 1 || rows[0].ID != id || rows[0].Title != title || rows[0].Amount != math.MaxUint32 {
		t.Fatalf("mail text/amount/filter lost: %+v %v", rows, err)
	}
	if err := s.RevokeGMMail(ctx, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("claimed legacy mail revoked: %v", err)
	}
	results := make(chan error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- s.RevokeGMMail(ctx, id) }()
	}
	wg.Wait()
	close(results)
	applied := 0
	for err := range results {
		if err == nil {
			applied++
		} else if !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}
	if applied != 1 {
		t.Fatalf("revocation applications=%d", applied)
	}
	rows, err = s.GMMails(ctx, 0, "")
	if err != nil || len(rows) != 2 || rows[0].Status != "revoked" || rows[1].Status != "claimed" {
		t.Fatalf("existing queue changed: %+v %v", rows, err)
	}
	var body string
	if err = s.db.QueryRow(ctx, `SELECT body FROM gm_mail WHERE id=1`).Scan(&body); err != nil || body != "preserve body" {
		t.Fatalf("legacy body lost: %q %v", body, err)
	}
}
