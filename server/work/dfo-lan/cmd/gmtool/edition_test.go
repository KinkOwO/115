package main

import (
	"bytes"
	"context"
	"dfolan/internal/admin"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEditionLoopbackOnly(t *testing.T) {
	for _, address := range []string{"127.0.0.1:0", "[::1]:28080"} {
		if err := validateLoopback(address); err != nil {
			t.Fatal(err)
		}
	}
	for _, address := range []string{"0.0.0.0:28080", ":28080", "134.175.57.52:28080", "localhost:28080"} {
		if validateLoopback(address) == nil {
			t.Fatalf("accepted %s", address)
		}
	}
}

func TestEditionStorageIsolationAndReorganisation(t *testing.T) {
	previous := defaultEdition
	defer func() { defaultEdition = previous }()
	root := t.TempDir()
	base := filepath.Join(root, "02_服务端", "server", "work", "dfo-lan")
	if err := os.MkdirAll(filepath.Join(base, "runtime", "storage"), 0755); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(base, "runtime", "storage", "dfolan.sqlite3")
	if err := os.WriteFile(local, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	defaultEdition = "local"
	p := editionPaths(root, "local")
	if filepath.Base(p.bagRules) != "inventory.current37.json" {
		t.Fatal("GM drifted from current launcher bag policy")
	}
	cfg := database.Config{Driver: "sqlite", SQLitePath: filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage", "dfolan.sqlite3")}
	e, err := newEditionManager(p, cfg, catalog.Progression{})
	if err != nil || e.config.SQLitePath != local {
		t.Fatalf("historical mapping: %+v %v", e, err)
	}
	// Existing directory junctions/symlinks and hard links are aliases, not a
	// second edition. The exact legacy path must still reject a different file.
	legacy := cfg.SQLitePath
	if err := os.MkdirAll(filepath.Dir(legacy), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(local, legacy); err != nil {
		t.Fatal(err)
	}
	e, err = newEditionManager(p, cfg, catalog.Progression{})
	if err != nil || e.config.SQLitePath != local {
		t.Fatalf("same-file legacy alias: %+v %v", e, err)
	}
	if err := os.Remove(legacy); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newEditionManager(p, cfg, catalog.Progression{}); err == nil {
		t.Fatal("accepted a distinct legacy DB with identical contents")
	}
	cfg.SQLitePath = filepath.Join(root, "runtime", "storage", "cloud-test.sqlite3")
	if _, err := newEditionManager(p, cfg, catalog.Progression{}); err == nil {
		t.Fatal("local accepted cloud DB")
	}
	defaultEdition = "cloud"
	cloud := filepath.Join(root, "runtime", "storage", "cloud-test.sqlite3")
	if err := os.MkdirAll(filepath.Dir(cloud), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cloud, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	p = editionPaths(root, "cloud")
	cfg.SQLitePath = "runtime/storage/cloud-test.sqlite3"
	e, err = newEditionManager(p, cfg, catalog.Progression{})
	if err != nil || e.config.SQLitePath != cloud || !e.config.DurableWrites {
		t.Fatalf("cloud resolution: %+v %v", e, err)
	}
	cfg.SQLitePath = local
	if _, err := newEditionManager(p, cfg, catalog.Progression{}); err == nil {
		t.Fatal("cloud accepted local DB")
	}
	cfg.SQLitePath = cloud
	if err := os.Remove(cloud); err != nil {
		t.Fatal(err)
	}
	if _, err := newEditionManager(p, cfg, catalog.Progression{}); err == nil {
		t.Fatal("accepted missing DB")
	}
	if _, err := os.Stat(cloud); !os.IsNotExist(err) {
		t.Fatal("created missing DB")
	}
}

func TestGMIndexDoesNotDiscloseToken(t *testing.T) {
	s := &server{token: "private-test-token"}
	w := httptest.NewRecorder()
	s.handleIndex(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 401 || strings.Contains(w.Body.String(), s.token) {
		t.Fatalf("anonymous legacy root: %d %s", w.Code, w.Body)
	}
	s.edition = &editionManager{kind: "cloud"}
	w = httptest.NewRecorder()
	s.handleIndex(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), s.token) || !strings.Contains(w.Body.String(), "联网服主 GM") {
		t.Fatal("edition root leaked secret or label missing")
	}
}

func TestEditionAuditedMutationsAndLease(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, m := range []func(context.Context) error{f.Migrate, f.MigrateGrants} {
		if err = m(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := f.DevelopmentAccount(ctx, "gm-test")
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.DevelopmentAccount(ctx, "other")
	if err != nil {
		t.Fatal(err)
	}
	state := json.RawMessage(`{"level":2,"experience":100,"inventory":{"gold":123,"items":[]},"unknown":{"value":9007199254740993}}`)
	role, err := f.CreateCharacter(ctx, database.Character{AccountID: account, Name: "GM临时角色", Request: []byte{0}, ConfigVersion: strings.Repeat("a", 64), State: state}, 2)
	if err != nil {
		t.Fatal(err)
	}
	e := &editionManager{kind: "local", config: database.Config{Driver: "sqlite", SQLitePath: f.Path()}, backups: filepath.Join(t.TempDir(), "backups"), progression: catalog.Progression{Thresholds: []uint64{100, 300, 600}}}
	s := &server{store: f.Storage(), admin: &admin.Service{Store: f.Storage()}, token: "test-gm-token", edition: e}
	mux := http.NewServeMux()
	s.registerEditionRoutes(mux)
	mux.HandleFunc("/api/grant", s.auth(s.handleGrant))
	call := func(path string, body any, token string) *httptest.ResponseRecorder {
		raw, _ := json.Marshal(body)
		r := httptest.NewRequest("POST", path, bytes.NewReader(raw))
		r.Header.Set("X-GM-Token", token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	request := map[string]any{"account": account, "character": role.ID, "level": 4, "grant_id": "level-test-0001", "reason": "integration"}
	if w := call("/api/gm/level", request, "wrong"); w.Code != 401 {
		t.Fatalf("unauthorized: %d", w.Code)
	}
	release, err := f.Storage().HoldAdminGuard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if w := call("/api/gm/level", request, s.token); w.Code != 409 {
		release()
		t.Fatalf("active game lease: %d %s", w.Code, w.Body)
	}
	release()
	before, err := f.CharacterState(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, state) {
		t.Fatal("refusal changed state")
	}
	request["account"] = other
	if w := call("/api/gm/level", request, s.token); w.Code != 400 {
		t.Fatalf("foreign role: %d %s", w.Code, w.Body)
	}
	request["account"] = account
	for i := 0; i < 2; i++ {
		w := call("/api/gm/level", request, s.token)
		if w.Code != 200 {
			t.Fatalf("level: %d %s", w.Code, w.Body)
		}
		var out struct {
			Applied bool `json:"applied"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		if out.Applied != (i == 0) {
			t.Fatalf("idempotency: %s", w.Body)
		}
	}
	after, err := f.CharacterState(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err = json.Unmarshal(after, &doc); err != nil {
		t.Fatal(err)
	}
	if string(doc["level"]) != "4" || string(doc["experience"]) != "600" || !bytes.Equal(doc["inventory"], json.RawMessage(`{"gold":123,"items":[]}`)) || !strings.Contains(string(doc["unknown"]), "9007199254740993") {
		t.Fatalf("lost state fields: %s", after)
	}
	cera := map[string]any{"account": account, "cera": 100, "grant_id": "cera-test-0001", "reason": "integration"}
	for i := 0; i < 2; i++ {
		if w := call("/api/grant", cera, s.token); w.Code != 200 {
			t.Fatalf("cera: %s", w.Body)
		}
	}
	balance, err := f.AccountCera(ctx, account)
	if err != nil || balance != 100 {
		t.Fatalf("cera duplicate: %d %v", balance, err)
	}
	backups, err := database.ListBackups(e.backups)
	if err != nil || len(backups) < 4 {
		t.Fatalf("backups: %d %v", len(backups), err)
	}
	for _, b := range backups {
		if err = database.VerifyBackup(b); err != nil {
			t.Fatal(err)
		}
	}
	// Backup failure cancels a new, otherwise valid mutation before it reaches storage.
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err = os.WriteFile(blocked, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	e.backups = blocked
	request["grant_id"] = "level-test-0002"
	request["level"] = 3
	if w := call("/api/gm/level", request, s.token); w.Code != 500 {
		t.Fatalf("backup failure: %d %s", w.Code, w.Body)
	}
	final, err := f.CharacterState(ctx, role.ID)
	if err != nil || !bytes.Equal(final, after) {
		t.Fatalf("backup failure mutated character: %s %v", final, err)
	}
	if _, err = os.Stat(f.Path() + ".admin-guard"); !os.IsNotExist(err) {
		t.Fatal("lease leaked after request")
	}
}
