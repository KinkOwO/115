package gmremote

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"dfolan/internal/database"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func fixture(t *testing.T) *Controller {
	t.Helper()
	sum := sha256.Sum256([]byte("owner-test"))
	c := &Controller{cfg: Config{TokenHash: hex.EncodeToString(sum[:])}, phase: "idle", client: &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	c.inspect = func(context.Context) (bool, int, error) { return true, 0, nil }
	c.stopGame = func(context.Context) error { return nil }
	c.startGame = func(context.Context) error { return nil }
	return c
}
func request(c *Controller, method, path, body, auth string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://gm"+path, strings.NewReader(body))
	r.TLS = &tls.ConnectionState{}
	if auth != "" {
		r.Header.Set("Authorization", "Bearer "+auth)
	}
	w := httptest.NewRecorder()
	c.ServeHTTP(w, r)
	return w
}

func TestPlayerDirectoryAvailableWithoutMaintenanceAndStillOwnerOnly(t *testing.T) {
	c := fixture(t)
	calls := 0
	c.directory = func(context.Context) ([]database.GMDirectoryAccount, error) {
		calls++
		return []database.GMDirectoryAccount{{ID: 7, Username: "alice", Characters: []database.GMDirectoryCharacter{{ID: 9, Name: "Role", Level: 115}}}}, nil
	}
	if w := request(c, "GET", "/control/players", "", ""); w.Code != 401 || calls != 0 {
		t.Fatal("unauthenticated directory accessible")
	}
	if w := request(c, "POST", "/control/players", "{}", "owner-test"); w.Code != 405 || calls != 0 {
		t.Fatal("directory accepts mutation")
	}
	w := request(c, "GET", "/control/players", "", "owner-test")
	if w.Code != 200 || calls != 1 || !strings.Contains(w.Body.String(), `"read_only":true`) || !strings.Contains(w.Body.String(), `"username":"alice"`) {
		t.Fatalf("directory response: %d %s", w.Code, w.Body.String())
	}
	if c.phase != "idle" || c.backend != nil {
		t.Fatal("reading players started maintenance")
	}
	if w := request(c, "POST", "/api/gm/command", "{}", "owner-test"); w.Code != 409 {
		t.Fatal("directory access unlocked GM writes")
	}
}
func eventually(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("state transition timeout")
}

func TestProvisionSeparatesOwnerSecretAndPinsCertificate(t *testing.T) {
	dir := t.TempDir()
	if e := Provision(dir, "127.0.0.1"); e != nil {
		t.Fatal(e)
	}
	if e := Provision(dir, "127.0.0.1"); e == nil {
		t.Fatal("overwrote credentials")
	}
	owner, _ := os.ReadFile(filepath.Join(dir, "client.json"))
	server, _ := os.ReadFile(filepath.Join(dir, "server.json"))
	var client map[string]string
	_ = json.Unmarshal(owner, &client)
	var cfg Config
	_ = json.Unmarshal(server, &cfg)
	if len(client["token"]) != 64 || strings.Contains(string(server), client["token"]) {
		t.Fatal("owner secret leaked into server configuration")
	}
	sum := sha256.Sum256([]byte(client["token"]))
	if hex.EncodeToString(sum[:]) != cfg.TokenHash {
		t.Fatal("credential digest mismatch")
	}
	certPEM, _ := os.ReadFile(filepath.Join(dir, "server-cert.pem"))
	block, _ := pem.Decode(certPEM)
	cert, e := x509.ParseCertificate(block.Bytes)
	if e != nil {
		t.Fatal(e)
	}
	pin := sha256.Sum256(cert.Raw)
	if hex.EncodeToString(pin[:]) != client["certificate_sha256"] {
		t.Fatal("certificate pin mismatch")
	}
	if e = cert.VerifyHostname("127.0.0.1"); e != nil {
		t.Fatal(e)
	}
}
func TestUnauthorizedRequestsNeverReachHost(t *testing.T) {
	c := fixture(t)
	c.inspect = func(context.Context) (bool, int, error) {
		t.Fatal("unauthorized host inspection")
		return false, 0, nil
	}
	for _, path := range []string{"/control/status", "/control/maintenance", "/api/grant", "/arbitrary"} {
		for _, auth := range []string{"", "wrong"} {
			if w := request(c, "POST", path, `{"confirm":true}`, auth); w.Code != 401 {
				t.Fatalf("%s status %d", path, w.Code)
			}
		}
	}
	r := httptest.NewRequest("GET", "http://gm/control/status", nil)
	r.Header.Set("Authorization", "Bearer owner-test")
	w := httptest.NewRecorder()
	c.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("plaintext accepted")
	}
}
func TestAllowedRoutesMethodsAndReadinessFailClosed(t *testing.T) {
	c := fixture(t)
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{{"GET", "/api/grant", "", 405}, {"POST", "/api/gm/level", "{}", 409}, {"POST", "/api/skin/unlock", "{}", 404}, {"POST", "/control/maintenance", "{}", 400}, {"GET", "/control/resume", "", 405}, {"POST", "/control/resume", `{"confirm":true,"command":"delete"}`, 400}} {
		if w := request(c, tc.method, tc.path, tc.body, "owner-test"); w.Code != tc.status {
			t.Fatalf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}
func TestPlayersRefuseMaintenanceWithoutStopping(t *testing.T) {
	c := fixture(t)
	c.inspect = func(context.Context) (bool, int, error) { return true, 2, nil }
	c.stopGame = func(context.Context) error { t.Error("stopped live game"); return nil }
	w := request(c, "POST", "/control/maintenance", `{"confirm":true}`, "owner-test")
	if w.Code != 409 {
		t.Fatal(w.Code)
	}
	if c.phase != "idle" {
		t.Fatal("maintenance state changed")
	}
}
func TestMaintenanceSingleLaunchAndHostFailure(t *testing.T) {
	c := fixture(t)
	var stops atomic.Int32
	release := make(chan struct{})
	c.stopGame = func(context.Context) error { stops.Add(1); return nil }
	c.launch = func(context.Context) (*exec.Cmd, string, string, error) {
		<-release
		return nil, "http://127.0.0.1:9999", "internal-secret", nil
	}
	for i := 0; i < 2; i++ {
		w := request(c, "POST", "/control/maintenance", `{"confirm":true}`, "owner-test")
		if w.Code != 202 && w.Code != 200 {
			t.Fatal(w.Code)
		}
	}
	close(release)
	eventually(t, func() bool { c.mu.RLock(); defer c.mu.RUnlock(); return c.phase == "ready" })
	if stops.Load() != 1 {
		t.Fatal("duplicate stop/launch")
	}
	d := fixture(t)
	d.stopGame = func(context.Context) error { return fmt.Errorf("players arrived") }
	d.launch = func(context.Context) (*exec.Cmd, string, string, error) {
		t.Error("launched despite stop rejection")
		return nil, "", "", nil
	}
	request(d, "POST", "/control/maintenance", `{"confirm":true}`, "owner-test")
	eventually(t, func() bool { d.mu.RLock(); defer d.mu.RUnlock(); return d.phase == "error" })
}
func TestProxyOwnTokenBoundedBodyAndNoRedirect(t *testing.T) {
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-GM-Token") != "internal-secret" || r.Header.Get("Authorization") != "" {
			t.Error("wrong authorization forwarded")
		}
		if r.URL.Path == "/api/items" {
			http.Redirect(w, r, "http://example.invalid", 302)
			return
		}
		b, _ := io.ReadAll(r.Body)
		response(w, 200, map[string]any{"ok": true, "body": string(b)})
	}))
	defer internal.Close()
	c := fixture(t)
	c.phase = "ready"
	c.endpoint = internal.URL
	c.token = "internal-secret"
	w := request(c, "POST", "/api/grant", `{"grant_id":"fixture"}`, "owner-test")
	if w.Code != 200 || strings.Contains(w.Body.String(), "internal-secret") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request(c, "POST", "/api/grant", strings.Repeat("x", (64<<10)+1), "owner-test")
	if w.Code != 413 {
		t.Fatal("unbounded request body")
	}
	w = request(c, "GET", "/api/items", "", "owner-test")
	if w.Code != 302 {
		t.Fatal("redirect followed")
	}
	w = request(c, "GET", "/control/status", "", "owner-test")
	if strings.Contains(w.Body.String(), "internal-secret") || strings.Contains(w.Body.String(), "owner-test") {
		t.Fatal("status leaked credentials")
	}
}
func TestResumeWaitsForInflightWrite(t *testing.T) {
	started := make(chan struct{})
	finish := make(chan struct{})
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-finish
		response(w, 200, map[string]bool{"applied": true})
	}))
	defer internal.Close()
	c := fixture(t)
	c.phase = "ready"
	c.endpoint = internal.URL
	c.token = "internal-secret"
	resumed := make(chan struct{})
	c.startGame = func(context.Context) error { close(resumed); return nil }
	done := make(chan struct{})
	go func() { request(c, "POST", "/api/grant", "{}", "owner-test"); close(done) }()
	<-started
	resumeDone := make(chan struct{})
	go func() { request(c, "POST", "/control/resume", `{"confirm":true}`, "owner-test"); close(resumeDone) }()
	select {
	case <-resumed:
		t.Fatal("resumed during a write")
	case <-time.After(50 * time.Millisecond):
	}
	close(finish)
	<-done
	<-resumeDone
	select {
	case <-resumed:
	case <-time.After(time.Second):
		t.Fatal("did not resume")
	}
}
