// Package localgm connects the local desktop to the running gateway. Only the
// gateway mutates an online save; requests execute on its owning session loop.
package localgm

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Descriptor struct {
	Endpoint string `json:"endpoint"`
	Token    string `json:"token"`
	Database string `json:"database"`
	PID      int    `json:"pid"`
	Version  int    `json:"version"`
}
type Request struct {
	Path string          `json:"path"`
	Body json.RawMessage `json:"body"`
}
type Result struct {
	Status int
	Body   json.RawMessage
}

func JSON(status int, v any) Result        { b, _ := json.Marshal(v); return Result{status, b} }
func Failure(status int, err error) Result { return JSON(status, map[string]any{"error": err.Error()}) }
func SameDatabase(a, b string) bool {
	x, e := os.Stat(a)
	y, f := os.Stat(b)
	return e == nil && f == nil && os.SameFile(x, y)
}
func Read(db string) (Descriptor, error) {
	var d Descriptor
	b, e := os.ReadFile(db + ".online-gm.json")
	if e != nil {
		return d, e
	}
	if e = json.Unmarshal(b, &d); e != nil {
		return d, e
	}
	u, e := url.Parse(d.Endpoint)
	if e != nil {
		return d, e
	}
	ip := net.ParseIP(u.Hostname())
	if d.Version != 1 || len(d.Token) != 64 || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || ip == nil || !ip.IsLoopback() || !SameDatabase(db, d.Database) {
		return d, fmt.Errorf("在线GM入口与当前存档不匹配")
	}
	return d, nil
}
func Call(ctx context.Context, d Descriptor, request *Request) (Result, error) {
	method, path := http.MethodGet, "/health"
	var body io.Reader
	if request != nil {
		method, path = http.MethodPost, "/execute"
		b, e := json.Marshal(request)
		if e != nil {
			return Result{}, e
		}
		body = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, d.Endpoint+path, body)
	if e != nil {
		return Result{}, e
	}
	req.Header.Set("X-Local-GM", d.Token)
	client := http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 65 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("redirect refused") }}
	defer client.CloseIdleConnections()
	res, e := client.Do(req)
	if e != nil {
		return Result{}, e
	}
	defer res.Body.Close()
	b, e := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	return Result{res.StatusCode, b}, e
}
func Ready(ctx context.Context, db string) bool {
	d, e := Read(db)
	if e != nil {
		return false
	}
	r, e := Call(ctx, d, nil)
	return e == nil && r.Status == 200
}
func Serve(db string, execute func(context.Context, Request) Result) (func(), error) {
	l, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		return nil, e
	}
	token := make([]byte, 32)
	if _, e = rand.Read(token); e != nil {
		l.Close()
		return nil, e
	}
	d := Descriptor{"http://" + l.Addr().String(), hex.EncodeToString(token), db, os.Getpid(), 1}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Local-GM")), []byte(d.Token)) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/health" {
			w.Write([]byte(`{"ok":true,"version":1}`))
			return
		}
		if r.Method != http.MethodPost || r.URL.Path != "/execute" {
			http.NotFound(w, r)
			return
		}
		var req Request
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536))
		if e := dec.Decode(&req); e != nil {
			http.Error(w, "invalid request", 400)
			return
		}
		result := execute(r.Context(), req)
		w.WriteHeader(result.Status)
		w.Write(result.Body)
	})
	srv := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 70 * time.Second, IdleTimeout: 10 * time.Second}
	b, _ := json.Marshal(d)
	path := db + ".online-gm.json"
	tmp := path + ".tmp"
	if e = os.WriteFile(tmp, b, 0600); e == nil {
		e = os.Rename(tmp, path)
	}
	if e != nil {
		l.Close()
		return nil, e
	}
	go srv.Serve(l)
	return func() {
		srv.Close()
		current, e := Read(db)
		if e == nil && current.Token == d.Token {
			os.Remove(path)
		}
	}, nil
}
