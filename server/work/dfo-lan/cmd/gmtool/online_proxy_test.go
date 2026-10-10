package main

import (
	"bytes"
	"context"
	"dfolan/internal/database"
	"dfolan/internal/localgm"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalOnlineProxyNeverFallsBackToOfflineWrites(t *testing.T) {
	db := filepath.Join(t.TempDir(), "save.db")
	os.WriteFile(db, []byte("test"), 0600)
	os.WriteFile(db+".admin-guard", []byte("live"), 0600)
	s := &server{token: "test", edition: &editionManager{kind: "local", config: database.Config{SQLitePath: db}}}
	offline := 0
	handler := s.auth(func(w http.ResponseWriter, r *http.Request) { offline++; writeJSON(w, map[string]any{"offline": true}) })
	call := func() *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/grant", bytes.NewBufferString(`{"gold":1}`))
		r.Header.Set("X-GM-Token", "test")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	if r := call(); r.Code != 409 || offline != 0 {
		t.Fatal("missing gateway bypassed guard", r.Code, offline)
	}
	stop, e := localgm.Serve(db, func(_ context.Context, r localgm.Request) localgm.Result {
		if r.Path != "/api/grant" || string(r.Body) != `{"gold":1}` {
			t.Error(r)
		}
		return localgm.JSON(200, map[string]any{"online": true})
	})
	if e != nil {
		t.Fatal(e)
	}
	defer stop()
	r := call()
	var body map[string]any
	json.Unmarshal(r.Body.Bytes(), &body)
	if r.Code != 200 || body["online"] != true || offline != 0 {
		t.Fatal(r.Code, r.Body, offline)
	}
	stop()
	r = call()
	if r.Code != 409 || offline != 0 {
		t.Fatal("stopped gateway fell back to write", r.Code, offline)
	}
}
