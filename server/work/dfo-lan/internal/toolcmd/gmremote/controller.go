// Package gmremote exposes the existing cloud GM through an authenticated TLS
// maintenance controller. It never implements another save writer or accepts
// shell commands from the network.
package gmremote

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"dfolan/internal/database"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Root        string `json:"root"`
	Listen      string `json:"listen"`
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"private_key"`
	TokenHash   string `json:"token_sha256"`
}

// Provision separates the server files from the owner's plaintext import file.
// The desktop application protects the latter with Windows DPAPI before use.
func Provision(directory, host string) error {
	if net.ParseIP(host) == nil {
		return fmt.Errorf("host must be an IP address")
	}
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	for _, n := range []string{"server.json", "client.json", "server-cert.pem", "server-key.pem"} {
		if _, e := os.Stat(filepath.Join(directory, n)); !os.IsNotExist(e) {
			return fmt.Errorf("refusing to overwrite %s", n)
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	now := time.Now()
	cert := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "115US Owner GM"}, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(3, 0, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP(host)}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		return err
	}
	kb, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return err
	}
	token := hex.EncodeToString(secret)
	digest := sha256.Sum256([]byte(token))
	pin := sha256.Sum256(der)
	cfg := Config{Root: `C:\DFO115-cloud-test`, Listen: "0.0.0.0:10443", Certificate: `C:\DFO115-cloud-test\gm-tool\remote\server-cert.pem`, PrivateKey: `C:\DFO115-cloud-test\gm-tool\remote\server-key.pem`, TokenHash: hex.EncodeToString(digest[:])}
	client := map[string]string{"endpoint": "https://" + net.JoinHostPort(host, "10443"), "certificate_sha256": hex.EncodeToString(pin[:]), "token": token}
	files := map[string][]byte{"server-cert.pem": pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), "server-key.pem": pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kb})}
	files["server.json"], err = json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	files["client.json"], err = json.MarshalIndent(client, "", "  ")
	if err != nil {
		return err
	}
	for n, b := range files {
		if err = os.WriteFile(filepath.Join(directory, n), b, 0600); err != nil {
			return err
		}
	}
	return nil
}

type Controller struct {
	cfg             Config
	mu              sync.RWMutex
	phase, detail   string
	backend         *exec.Cmd
	backendDone     chan struct{}
	endpoint, token string
	client          *http.Client
	inspect         func(context.Context) (bool, int, error)
	stopGame        func(context.Context) error
	startGame       func(context.Context) error
	launch          func(context.Context) (*exec.Cmd, string, string, error)
	directory       func(context.Context) ([]database.GMDirectoryAccount, error)
}

func Run(file string) error {
	b, e := os.ReadFile(file)
	if e != nil {
		return e
	}
	var cfg Config
	if e = json.Unmarshal(b, &cfg); e != nil {
		return e
	}
	if !filepath.IsAbs(cfg.Root) {
		return fmt.Errorf("root must be absolute")
	}
	digest, e := hex.DecodeString(cfg.TokenHash)
	if e != nil || len(digest) != sha256.Size {
		return fmt.Errorf("invalid owner credential digest")
	}
	if _, _, e = net.SplitHostPort(cfg.Listen); e != nil {
		return e
	}
	c := &Controller{cfg: cfg, phase: "idle", client: &http.Client{Timeout: 75 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
	c.inspect = c.inspectHost
	c.stopGame = c.stopHost
	c.startGame = c.startHost
	c.launch = c.launchGM
	c.directory = c.readDirectory
	pair, e := tls.LoadX509KeyPair(cfg.Certificate, cfg.PrivateKey)
	if e != nil {
		return e
	}
	server := &http.Server{Addr: cfg.Listen, Handler: c, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 100 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{pair}}}
	server.TLSNextProto = map[string]func(*http.Server, *tls.Conn, http.Handler){}
	fmt.Println("115US remote GM controller ready; encrypted owner access only")
	return server.ListenAndServeTLS("", "")
}

func response(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, code int, err error) {
	response(w, code, map[string]any{"error": err.Error()})
}

var allowed = map[string]string{
	"/api/gm/snapshot": "GET", "/api/gm/quests": "GET", "/api/gm/mailbox": "GET", "/api/gm/materials": "GET", "/api/gm/vp": "GET", "/api/types": "GET", "/api/filters": "GET", "/api/gm/command": "POST",
	"/api/gm/status": "GET", "/api/overview": "GET", "/api/characters": "GET", "/api/character": "GET", "/api/items": "GET", "/api/catalog-metadata": "GET", "/api/history": "GET", "/api/gm/backups": "GET",
	"/api/grant": "POST", "/api/gm/level": "POST", "/api/gm/backup": "POST",
}

func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.TLS == nil {
		failure(w, 400, fmt.Errorf("TLS required"))
		return
	}
	auth := r.Header.Get("Authorization")
	got := sha256.Sum256([]byte(strings.TrimPrefix(auth, "Bearer ")))
	want, _ := hex.DecodeString(c.cfg.TokenHash)
	if !strings.HasPrefix(auth, "Bearer ") || len(want) != 32 || subtle.ConstantTimeCompare(got[:], want) != 1 {
		failure(w, 401, fmt.Errorf("owner authorization required"))
		return
	}
	if len(r.URL.RawQuery) > 8192 || r.URL.RawPath != "" {
		failure(w, 400, fmt.Errorf("invalid request path"))
		return
	}
	switch r.URL.Path {
	case "/control/players":
		if r.Method != "GET" {
			failure(w, 405, fmt.Errorf("method not allowed"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		rows, e := c.directory(ctx)
		if e != nil {
			failure(w, 503, e)
			return
		}
		response(w, 200, map[string]any{"accounts": rows, "read_only": true, "edition": "cloud"})
		return
	case "/control/status":
		if r.Method != "GET" {
			failure(w, 405, fmt.Errorf("method not allowed"))
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		running, players, e := c.inspect(ctx)
		if e != nil {
			failure(w, 503, e)
			return
		}
		c.mu.RLock()
		defer c.mu.RUnlock()
		response(w, 200, map[string]any{"phase": c.phase, "detail": c.detail, "game_running": running, "connections": players, "gm_ready": c.phase == "ready", "version": "1.3.0", "edition": "cloud"})
		return
	case "/control/maintenance":
		if !controlRequest(w, r) {
			return
		}
		c.begin(w)
		return
	case "/control/resume":
		if !controlRequest(w, r) {
			return
		}
		c.resume(w)
		return
	}
	method, ok := allowed[r.URL.Path]
	if !ok {
		failure(w, 404, fmt.Errorf("unknown management action"))
		return
	}
	if r.Method != method {
		failure(w, 405, fmt.Errorf("method not allowed"))
		return
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.phase != "ready" {
		failure(w, 409, fmt.Errorf("请先进入GM维护模式，等待物品目录准备完成"))
		return
	}
	body, e := io.ReadAll(io.LimitReader(r.Body, (64<<10)+1))
	if e != nil || len(body) > 64<<10 {
		failure(w, 413, fmt.Errorf("request too large"))
		return
	}
	target := c.endpoint + r.URL.Path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	req, e := http.NewRequestWithContext(r.Context(), r.Method, target, strings.NewReader(string(body)))
	if e != nil {
		failure(w, 400, e)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GM-Token", c.token)
	res, e := c.client.Do(req)
	if e != nil {
		failure(w, 502, fmt.Errorf("GM维护进程无响应，请查看连接状态"))
		return
	}
	defer res.Body.Close()
	data, e := io.ReadAll(io.LimitReader(res.Body, (16<<20)+1))
	if e != nil || len(data) > 16<<20 {
		failure(w, 502, fmt.Errorf("invalid GM response"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(res.StatusCode)
	_, _ = w.Write(data)
}

func controlRequest(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != "POST" {
		failure(w, 405, fmt.Errorf("method not allowed"))
		return false
	}
	var body struct {
		Confirm bool `json:"confirm"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&body) != nil || !body.Confirm {
		failure(w, 400, fmt.Errorf("explicit maintenance confirmation required"))
		return false
	}
	return true
}
func (c *Controller) begin(w http.ResponseWriter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.phase == "ready" || c.phase == "preparing" {
		response(w, 200, map[string]any{"phase": c.phase})
		return
	}
	if c.phase == "resuming" {
		failure(w, 409, fmt.Errorf("正在恢复游戏服务"))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	_, players, e := c.inspect(ctx)
	if e != nil {
		failure(w, 503, e)
		return
	}
	if players > 0 {
		failure(w, 409, fmt.Errorf("仍有%d条游戏连接，请所有玩家退出后再进入维护", players))
		return
	}
	c.phase = "preparing"
	c.detail = "正在停止游戏服务并准备原生物品目录，约一分钟"
	go c.prepare()
	response(w, 202, map[string]any{"phase": "preparing"})
}
func (c *Controller) prepare() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if e := c.stopGame(ctx); e != nil {
		c.setError(e)
		return
	}
	cmd, endpoint, token, e := c.launch(ctx)
	if e != nil {
		c.setError(e)
		return
	}
	c.mu.Lock()
	c.backend = cmd
	done := make(chan struct{})
	c.backendDone = done
	c.endpoint = endpoint
	c.token = token
	c.phase = "ready"
	c.detail = "云端GM已就绪；修改前自动备份"
	c.mu.Unlock()
	if cmd != nil {
		_ = cmd.Wait()
		close(done)
		c.mu.Lock()
		if c.backend == cmd {
			c.backend = nil
			c.backendDone = nil
			c.endpoint = ""
			c.token = ""
			if c.phase == "ready" {
				c.phase = "error"
				c.detail = "GM进程已退出，可重新进入维护或恢复开服"
			}
		}
		c.mu.Unlock()
	}
}
func (c *Controller) setError(e error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.phase = "error"
	c.detail = e.Error()
}
func (c *Controller) resume(w http.ResponseWriter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.phase == "preparing" || c.phase == "resuming" {
		failure(w, 409, fmt.Errorf("当前操作尚未完成"))
		return
	}
	if c.phase == "idle" && c.backend == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		running, _, err := c.inspect(ctx)
		cancel()
		if err != nil {
			failure(w, 500, err)
			return
		}
		if running {
			response(w, 200, map[string]any{"phase": "idle", "detail": "游戏已经运行"})
			return
		}
	}
	if c.backend != nil && c.backend.Process != nil {
		if e := c.backend.Process.Kill(); e != nil && !strings.Contains(e.Error(), "finished") {
			failure(w, 500, e)
			return
		}
		// Wait owns process cleanup and closes this channel before taking mu.
		// All in-flight proxy writes have completed before we acquire mu here.
		if c.backendDone != nil {
			select {
			case <-c.backendDone:
			case <-time.After(10 * time.Second):
				failure(w, 500, fmt.Errorf("GM进程退出超时，游戏尚未恢复"))
				return
			}
		}
		c.backend = nil
		c.backendDone = nil
	}
	c.endpoint = ""
	c.token = ""
	c.phase = "resuming"
	c.detail = "正在恢复游戏服务"
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if e := c.startGame(ctx); e != nil {
			c.setError(e)
			return
		}
		c.mu.Lock()
		c.phase = "idle"
		c.detail = "已提交开服启动，请等待游戏目录准备"
		c.mu.Unlock()
	}()
	response(w, 202, map[string]any{"phase": "resuming"})
}
func (c *Controller) powershell(ctx context.Context, script string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Dir = c.cfg.Root
	b, e := cmd.CombinedOutput()
	if e != nil {
		return nil, fmt.Errorf("maintenance host command failed: %s", strings.TrimSpace(string(b)))
	}
	return b, nil
}
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }
func (c *Controller) readDirectory(ctx context.Context) ([]database.GMDirectoryAccount, error) {
	cfg, e := database.LoadConfig(filepath.Join(c.cfg.Root, "runtime", "storage", "local.json"))
	if e != nil {
		return nil, e
	}
	expected := filepath.Join(c.cfg.Root, "runtime", "storage", "cloud-test.sqlite3")
	if !strings.EqualFold(filepath.Clean(cfg.SQLitePath), filepath.Clean(expected)) {
		return nil, fmt.Errorf("cloud player directory storage mismatch")
	}
	return database.ReadGMDirectory(ctx, cfg)
}
func (c *Controller) inspectHost(ctx context.Context) (bool, int, error) {
	script := `$ErrorActionPreference='Stop'; $OutputEncoding=[Console]::OutputEncoding=New-Object Text.UTF8Encoding($false); $game=@(Get-CimInstance Win32_Process -Filter "Name='wireprobe-cloud.exe'" | Where-Object {$_.ExecutablePath -eq ` + quote(filepath.Join(c.cfg.Root, "bin", "wireprobe-cloud.exe")) + `}); $connections=@(Get-NetTCPConnection -State Established -ErrorAction SilentlyContinue | Where-Object {$_.LocalPort -eq 7001 -or ($_.LocalPort -ge 10006 -and $_.LocalPort -le 10262)}); @{running=($game.Count -gt 0);connections=$connections.Count} | ConvertTo-Json -Compress`
	b, e := c.powershell(ctx, script)
	if e != nil {
		return false, 0, e
	}
	var v struct {
		Running     bool `json:"running"`
		Connections int  `json:"connections"`
	}
	e = json.Unmarshal(b, &v)
	return v.Running, v.Connections, e
}
func (c *Controller) stopHost(ctx context.Context) error {
	// Recheck sockets immediately before using the existing stop entry. Reclaim
	// only a guard containing a dead PID, exactly as storage-route clear-guard.
	script := `$ErrorActionPreference='Stop'; $OutputEncoding=[Console]::OutputEncoding=New-Object Text.UTF8Encoding($false); $connections=@(Get-NetTCPConnection -State Established -ErrorAction SilentlyContinue | Where-Object {$_.LocalPort -eq 7001 -or ($_.LocalPort -ge 10006 -and $_.LocalPort -le 10262)}); if($connections.Count -gt 0){throw 'Players are connected; maintenance refused'}; & ` + quote(filepath.Join(c.cfg.Root, "scripts", "Stop-CloudTest.ps1")) + `; Start-Sleep -Milliseconds 500; $game=@(Get-CimInstance Win32_Process -Filter "Name='wireprobe-cloud.exe'" | Where-Object {$_.ExecutablePath -eq ` + quote(filepath.Join(c.cfg.Root, "bin", "wireprobe-cloud.exe")) + `}); if($game.Count){throw 'Game process did not stop'}; $guard=` + quote(filepath.Join(c.cfg.Root, "runtime", "storage", "cloud-test.sqlite3.admin-guard")) + `; if(Test-Path -LiteralPath $guard){$holder=(Get-Content -LiteralPath $guard -Raw).Trim(); if($holder -notmatch '^[1-9][0-9]*$'){throw 'Unknown guard owner; manual review required'}; if(Get-Process -Id ([int]$holder) -ErrorAction SilentlyContinue){throw 'Guard owner is still alive'}; Remove-Item -LiteralPath $guard}`
	_, e := c.powershell(ctx, script)
	return e
}
func (c *Controller) startHost(ctx context.Context) error {
	_, e := c.powershell(ctx, `$ErrorActionPreference='Stop'; Start-ScheduledTask -TaskName 'DFO115-Cloud-Test-20261007'`)
	return e
}

var backendURL = regexp.MustCompile(`http://127\.0\.0\.1:([0-9]+)/#([0-9a-f]{32})`)

func (c *Controller) launchGM(ctx context.Context) (*exec.Cmd, string, string, error) {
	exe := filepath.Join(c.cfg.Root, "gm-tool", "bin", "115US联网服主GM.exe")
	cmd := exec.Command(exe, "-root", c.cfg.Root, "-open=false")
	cmd.Dir = filepath.Join(c.cfg.Root, "gm-tool")
	pipe, e := cmd.StderrPipe()
	if e != nil {
		return nil, "", "", e
	}
	cmd.Stdout = io.Discard
	if e = cmd.Start(); e != nil {
		return nil, "", "", e
	}
	type found struct {
		endpoint, token string
		err             error
	}
	result := make(chan found, 1)
	go func() {
		scanner := bufio.NewScanner(pipe)
		last := ""
		sent := false
		for scanner.Scan() {
			line := scanner.Text()
			if m := backendURL.FindStringSubmatch(line); m != nil {
				if !sent {
					result <- found{endpoint: "http://127.0.0.1:" + m[1], token: m[2]}
					sent = true
				}
			} else {
				last = line
			}
		}
		if !sent {
			result <- found{err: fmt.Errorf("GM启动失败：%s", last)}
		}
	}()
	select {
	case f := <-result:
		if f.err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return nil, "", "", f.err
		}
		return cmd, f.endpoint, f.token, nil
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, "", "", fmt.Errorf("物品目录准备超时；游戏保持停止，请重试或恢复开服")
	}
}
