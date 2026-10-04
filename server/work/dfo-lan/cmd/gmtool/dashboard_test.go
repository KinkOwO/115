package main

import (
	"bytes"
	"context"
	"dfolan/internal/database"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDashboardMailAndConcurrentVaultDelivery(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("requires dedicated DFO_TEST_POSTGRES_DSN")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, migrate := range []func(context.Context) error{f.Migrate, f.MigrateVault, f.MigrateGMMail} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := f.DevelopmentAccount(ctx, "dashboard")
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.DevelopmentAccount(ctx, "other-dashboard")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("a", 64)
	role, err := f.CreateCharacter(ctx, database.Character{AccountID: account, Name: "Dashboard", Request: []byte{0}, ConfigVersion: version, State: []byte(`{"unknown":9007199254740993}`)}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.LoadVault(ctx, account, role.ID, 1, version); err != nil {
		t.Fatal(err)
	}
	if err = f.SeedVaultItems(ctx, role.ID, []byte(`[{"slot":0,"template":55,"amount":3,"unknown":{"number":9007199254740993}}]`)); err != nil {
		t.Fatal(err)
	}
	s := &server{store: f.Storage(), token: "test-token"}
	mux := http.NewServeMux()
	s.registerDashboardRoutes(mux)
	call := func(path, method string, payload any, token string) *httptest.ResponseRecorder {
		raw, e := json.Marshal(payload)
		if e != nil {
			t.Error(e)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		r.Header.Set("X-GM-Token", token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	request := map[string]any{"account": account, "character": role.ID, "template": 55, "amount": 1}
	if w := call("/api/vault/send", "POST", request, "wrong"); w.Code != 401 {
		t.Fatalf("unauthenticated write: %d", w.Code)
	}
	foreign := map[string]any{"account": other, "character": role.ID, "template": 55, "amount": 1}
	if w := call("/api/vault/send", "POST", foreign, s.token); w.Code != 400 {
		t.Fatalf("foreign write: %s", w.Body)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if w := call("/api/vault/send", "POST", request, s.token); w.Code != 200 {
				t.Errorf("vault write: %d %s", w.Code, w.Body)
			}
		}()
	}
	wg.Wait()
	vault, err := f.LoadVault(ctx, account, role.ID, 1, version)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]json.RawMessage
	if err = json.Unmarshal(vault.Items, &rows); err != nil || len(rows) != 1 || string(rows[0]["Amount"]) != "15" || !bytes.Contains(rows[0]["unknown"], []byte("9007199254740993")) {
		t.Fatalf("lost update/unknown field: %s %v", vault.Items, err)
	}
	for _, bad := range []map[string]any{
		{"account": account, "character": role.ID, "template": 66, "amount": 1},
		{"account": account, "character": role.ID, "template": 55, "amount": math.MaxUint32},
	} {
		if w := call("/api/vault/send", "POST", bad, s.token); w.Code != 400 {
			t.Fatalf("full/overflow accepted: %s", w.Body)
		}
	}
	after, err := f.LoadVault(ctx, account, role.ID, 1, version)
	if err != nil || !bytes.Equal(after.Items, vault.Items) {
		t.Fatalf("failed write changed vault: %s %v", after.Items, err)
	}
	state, err := f.CharacterState(ctx, role.ID)
	if err != nil || !bytes.Contains(state, []byte("9007199254740993")) {
		t.Fatalf("character state lost: %s %v", state, err)
	}
	title := "中文|O'Brien\n第二行"
	w := call("/api/mail/send", "POST", map[string]any{"to_account_id": account, "to_character_id": role.ID, "template": 55, "amount": 1, "title": title}, s.token)
	if w.Code != 200 {
		t.Fatalf("mail send: %s", w.Body)
	}
	var receipt struct {
		OK bool  `json:"ok"`
		ID int64 `json:"mail_id"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &receipt); err != nil || !receipt.OK || receipt.ID == 0 {
		t.Fatalf("mail receipt: %s %v", w.Body, err)
	}
	w = call("/api/mail/list?status=unread", "GET", nil, s.token)
	var result struct {
		OK    bool              `json:"ok"`
		Count int               `json:"count"`
		Mails []database.GMMail `json:"mails"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &result); err != nil || !result.OK || result.Count != 1 || result.Mails[0].Title != title {
		t.Fatalf("mail list: %s %v", w.Body, err)
	}
	if w = call("/api/mail/revoke", "POST", map[string]any{"id": receipt.ID}, s.token); w.Code != 200 {
		t.Fatalf("revoke: %s", w.Body)
	}
	if w = call("/api/mail/revoke", "POST", map[string]any{"id": receipt.ID}, s.token); w.Code != 400 {
		t.Fatalf("double revoke: %s", w.Body)
	}
	if w = call("/api/mail/send", "GET", nil, s.token); w.Code != 405 {
		t.Fatal("write accepted GET")
	}
	if w = call("/api/mail/send", "POST", map[string]any{"amount": "invalid"}, s.token); w.Code != 400 {
		t.Fatal("bad JSON value accepted")
	}
}
