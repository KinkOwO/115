// Package ranking adapts the native HTTP score-board contract. No score is
// inferred from character fame, level, or unrelated dungeon clear records.
package ranking

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"dfolan/internal/catalog/pvf"
)

const Script = "contents/system/systemranking/main.cos"
const LocalAddress = "127.0.0.1:17001"

type Rules struct{ Paths map[string]bool }

// Import uses the same archive as all other catalogs. Minimal archives without
// this client window have no HTTP ranking endpoint to publish.
func Import(a *pvf.Archive) (*Rules, error) {
	if _, ok := a.FindFile(Script); !ok {
		return nil, nil
	}
	text, err := a.ReadText(Script)
	if err != nil {
		return nil, err
	}
	return FromText(text)
}

var urlBlocks = regexp.MustCompile(`(?s)\[ranking url\](.*?)\[/ranking url\]`)
var quotedValues = regexp.MustCompile("`([^`]+)`")

func FromText(text string) (*Rules, error) {
	r := &Rules{Paths: map[string]bool{}}
	blocks := urlBlocks.FindAllStringSubmatch(text, -1)
	if len(blocks) != strings.Count(text, "[ranking url]") || len(blocks) != strings.Count(text, "[/ranking url]") {
		return nil, fmt.Errorf("ranking source has incomplete URL definitions")
	}
	for _, block := range blocks {
		for _, value := range quotedValues.FindAllStringSubmatch(block[1], -1) {
			if !strings.Contains(value[1], "://") {
				continue
			}
			u, err := url.Parse(value[1])
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.RawQuery != "" || u.Fragment != "" || !route.MatchString(u.Path) {
				return nil, fmt.Errorf("invalid ranking source URL %q", value[1])
			}
			r.Paths[u.Path] = true
		}
	}
	if len(r.Paths) == 0 {
		return nil, fmt.Errorf("ranking source has no complete URL definitions")
	}
	return r, nil
}

var route = regexp.MustCompile(`^/rank/(?:inspect/)?[a-z0-9-]+$`)

// Handler returns the native JSON array. These gameplay domains currently have
// no authoritative score writer; an empty array means no recorded scores. It
// deliberately does not publish made-up players, ranks, or personal results.
func (r *Rules) Handler(event func(map[string]any)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		record := func(status int, route string, personal bool, success bool) {
			if event != nil {
				event(map[string]any{"kind": "ranking_http_response", "status": status, "route": route, "personal": personal, "records": 0, "score_writer": "unavailable", "success": success})
			}
		}
		if req.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			record(http.StatusMethodNotAllowed, "", false, false)
			return
		}
		path := strings.TrimSuffix(req.URL.Path, "/")
		found := r.Paths[path]
		personal := false
		if !found {
			// The native client appends its 33-byte NOTI2631 user key for personal
			// views. The key is opaque, never echoed or logged by this read-only API.
			if at := strings.LastIndexByte(path, '/'); at > 0 {
				found = r.Paths[path[:at]]
				if found {
					path, personal = path[:at], true
				}
			}
		}
		if !found {
			http.NotFound(w, req)
			record(http.StatusNotFound, "", false, false)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Length", "2")
		_, err := w.Write([]byte("[]"))
		record(http.StatusOK, path, personal, err == nil)
	})
}

// Serve has a bounded HTTP lifecycle and binds only the local game address.
// It never proxies requests to official services or opens a player database.
func (r *Rules) Serve(address string, event func(map[string]any)) (*http.Server, net.Listener, error) {
	host, _, err := net.SplitHostPort(address)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return nil, nil, fmt.Errorf("ranking HTTP requires a loopback address")
	}
	l, err := net.Listen("tcp4", address)
	if err != nil {
		return nil, nil, err
	}
	s := &http.Server{Handler: r.Handler(event), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 8192}
	go func() { _ = s.Serve(l) }()
	return s, l, nil
}
