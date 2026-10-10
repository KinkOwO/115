package social

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"time"
)

const LocalAddress = "127.0.0.1:17002"
const LocalURL = "http://" + LocalAddress + "/api/community"

type Lookup func(context.Context, []string) ([]CharacterInfo, error)

// Handler implements only the native community metadata request. It neither
// accepts relationship writes nor treats the opaque api value as authentication.
func Handler(lookup Lookup, event func(map[string]any)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status, count := http.StatusOK, 0
		defer func() {
			if event != nil {
				event(map[string]any{"kind": "social_http_request", "status": status, "records": count})
			}
		}()
		fail := func(code int) { status = code; http.Error(w, http.StatusText(code), code) }
		if r.URL.Path != "/api/community" {
			fail(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			fail(http.StatusMethodNotAllowed)
			return
		}
		var request struct {
			API   string   `json:"api"`
			Type  int      `json:"req_type"`
			Names []string `json:"member_name"`
		}
		// Transport limits, not gameplay capacities. Never log the api or names.
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		if decoder.Decode(&request) != nil {
			fail(http.StatusBadRequest)
			return
		}
		var extra any
		if !errors.Is(decoder.Decode(&extra), io.EOF) {
			fail(http.StatusBadRequest)
			return
		}
		if request.Type != 6 {
			fail(http.StatusUnprocessableEntity)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		rows, err := lookup(ctx, request.Names)
		if err != nil {
			fail(http.StatusServiceUnavailable)
			return
		}
		if rows == nil {
			rows = []CharacterInfo{}
		}
		count = len(rows)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(struct {
			Rows []CharacterInfo `json:"character_info"`
		}{rows}); err != nil {
			status = http.StatusInternalServerError
		}
	})
}

func Serve(lookup Lookup, event func(map[string]any)) (*http.Server, error) {
	listener, err := net.Listen("tcp", LocalAddress)
	if err != nil {
		return nil, err
	}
	server := &http.Server{Handler: Handler(lookup, event), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) && event != nil {
			event(map[string]any{"kind": "social_http_error", "error": err.Error()})
		}
	}()
	return server, nil
}
