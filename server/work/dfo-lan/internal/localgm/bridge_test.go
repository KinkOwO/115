package localgm

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBridgeIdentityAuthenticationAndLifetime(t *testing.T) {
	db := filepath.Join(t.TempDir(), "save.db")
	os.WriteFile(db, []byte("fixture"), 0600)
	calls := 0
	stop, e := Serve(db, func(ctx context.Context, r Request) Result { calls++; return JSON(200, map[string]any{"path": r.Path}) })
	if e != nil {
		t.Fatal(e)
	}
	defer stop()
	d, e := Read(db)
	if e != nil {
		t.Fatal(e)
	}
	if !Ready(context.Background(), db) {
		t.Fatal("not ready")
	}
	wrong := d
	wrong.Token = strings.Repeat("0", 64)
	r, e := Call(context.Background(), wrong, &Request{Path: "/api/grant", Body: json.RawMessage(`{}`)})
	if e != nil || r.Status != http.StatusUnauthorized || calls != 0 {
		t.Fatal(r, e, calls)
	}
	r, e = Call(context.Background(), d, &Request{Path: "/api/grant", Body: json.RawMessage(`{}`)})
	if e != nil || r.Status != 200 || calls != 1 {
		t.Fatal(r, e, calls)
	}
	other := filepath.Join(t.TempDir(), "other.db")
	os.WriteFile(other, []byte("fixture"), 0600)
	b, _ := json.Marshal(d)
	os.WriteFile(other+".online-gm.json", b, 0600)
	if _, e = Read(other); e == nil {
		t.Fatal("cross-save descriptor accepted")
	}
	d.Endpoint = "http://192.0.2.1:1234"
	b, _ = json.Marshal(d)
	os.WriteFile(other+".online-gm.json", b, 0600)
	if _, e = Read(other); e == nil {
		t.Fatal("non-loopback accepted")
	}
	stop()
	if Ready(context.Background(), db) {
		t.Fatal("closed endpoint ready")
	}
	if _, e = os.Stat(db + ".online-gm.json"); !os.IsNotExist(e) {
		t.Fatal("descriptor survived shutdown")
	}
}
