package ranking

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"dfolan/internal/catalog/pvf"
)

func TestNativeArchive(t *testing.T) {
	path := os.Getenv("DFO_RANKING_TEST_PVF")
	if path == "" {
		t.Skip("set DFO_RANKING_TEST_PVF to validate the current source")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	r, err := Import(a)
	if err != nil || r == nil || len(r.Paths) == 0 {
		t.Fatalf("native source: %+v %v", r, err)
	}
	text, err := a.ReadText(Script)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("DFO_RANKING_TEST_LOCAL") == "1" {
		for _, token := range quotedValues.FindAllStringSubmatch(text, -1) {
			if strings.Contains(token[1], "://") && !strings.HasPrefix(token[1], "http://"+LocalAddress+"/rank/") {
				t.Fatalf("ranking URL still external: %s", token[1])
			}
		}
	}
	for path := range r.Paths {
		w := httptest.NewRecorder()
		r.Handler(nil).ServeHTTP(w, httptest.NewRequest("GET", path+"/?season=0&jobId=-1&jobGrowId=-1", nil))
		if w.Code != http.StatusOK || w.Body.String() != "[]" {
			t.Fatalf("native route %s: %d %s", path, w.Code, w.Body)
		}
	}
	t.Logf("source %s: %d routes", a.Snapshot().Checksum, len(r.Paths))
}

func TestRejectIncompleteSource(t *testing.T) {
	for _, source := range []string{"https://example.invalid/outside", "https://example.invalid/rank/test?token=x", "file:///rank/test"} {
		_, err := FromText("[ranking url] `" + source + "` [/ranking url]")
		if err == nil {
			t.Fatalf("invalid source accepted: %s", source)
		}
	}
	if _, err := FromText("[ranking url] `https://example.invalid/rank/test`"); err == nil {
		t.Fatal("unclosed URL block accepted")
	}
}

func TestSourceRoutesAndNativeEmptyArray(t *testing.T) {
	text := "[ranking url] `real` `release` `https://example.invalid/rank/source-defined` [/ranking url]"
	r, err := FromText(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Paths) != 1 || !r.Paths["/rank/source-defined"] {
		t.Fatalf("source routes: %+v", r)
	}
	for _, path := range []string{"/rank/source-defined", "/rank/source-defined/", "/rank/source-defined/opaque-key?season=0&jobId=1&jobGrowId=2"} {
		w := httptest.NewRecorder()
		r.Handler(nil).ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 || w.Body.String() != "[]" || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
	}
	w := httptest.NewRecorder()
	r.Handler(nil).ServeHTTP(w, httptest.NewRequest("GET", "/rank/invented", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
	w = httptest.NewRecorder()
	r.Handler(nil).ServeHTTP(w, httptest.NewRequest("POST", "/rank/source-defined", nil))
	if w.Code != 405 {
		t.Fatal(w.Code)
	}
	changed, err := FromText(strings.ReplaceAll(text, "https://example.invalid/rank/source-defined", "http://127.0.0.1:17001/rank/changed-at-source"))
	if err != nil || !changed.Paths["/rank/changed-at-source"] || changed.Paths["/rank/source-defined"] {
		t.Fatalf("source mutation ignored: %+v %v", changed, err)
	}
}

func TestHTTPListenerLifecycle(t *testing.T) {
	r := &Rules{Paths: map[string]bool{"/rank/test": true}}
	if _, _, err := r.Serve("0.0.0.0:0", nil); err == nil {
		t.Fatal("wildcard ranking listener allowed")
	}
	s, l, err := r.Serve("127.0.0.1:0", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get("http://" + l.Addr().String() + "/rank/test")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || string(body) != "[]" {
		t.Fatalf("%s %v", body, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = http.Get("http://" + l.Addr().String() + "/rank/test"); err == nil {
		t.Fatal("closed endpoint still accepts requests")
	}
}

func TestPersonalRouteAndFailureDiagnosticsDoNotDiscloseKey(t *testing.T) {
	r := &Rules{Paths: map[string]bool{"/rank/test": true}}
	var events []map[string]any
	handler := r.Handler(func(event map[string]any) { events = append(events, event) })
	key, _ := UserKey(1)
	for _, request := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/rank/test/" + string(key[:32]) + "?season=0&jobId=16&jobGrowId=3", 200},
		{"GET", "/unknown/" + string(key[:32]), 404},
		{"POST", "/rank/test/" + string(key[:32]), 405},
	} {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(request.method, request.path, nil))
		if w.Code != request.status {
			t.Fatal("request lost its response", w.Code)
		}
		event := events[len(events)-1]
		if event["status"] != request.status {
			t.Fatal("failed request missing from diagnostics", event)
		}
		for _, value := range event {
			if text, ok := value.(string); ok && strings.Contains(text, string(key[:32])) {
				t.Fatal("opaque ranking identity leaked into diagnostics")
			}
		}
	}
	if events[0]["personal"] != true || events[0]["route"] != "/rank/test" {
		t.Fatal("personal loading path not distinguishable", events[0])
	}
}
