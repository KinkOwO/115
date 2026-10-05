package database

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Windows 编辑器（记事本 / PowerShell 的 `Set-Content -Encoding UTF8`）默认写 BOM。
// PostgreSQL 档必须手工改 local.json 填 DSN，正是最容易带上 BOM 的那一份；此前服务端
// 用 json.Unmarshal 直接吃原始字节，于是启动即死在 `invalid character 'ï'`，而启动器
// （Python 的 utf-8-sig / Go 的 stripBOM）都读得进去 —— 报给业主就是「pgsql 端无法登录」。
func TestLoadConfigAcceptsTheByteOrderMark(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "local.json")
	body := "\ufeff{\"postgres_dsn\":\"postgres://dfo_owner:pw@127.0.0.1:25438/dfo_lan?sslmode=disable\"," +
		"\"max_connections\":12,\"postgres_bin\":\"C:/tools/pg/pgsql/bin\"}"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig refused a byte-order-marked config: %v", err)
	}
	if driver, err := EngineForConfig(cfg); err != nil || driver != DriverPostgres {
		t.Fatalf("engine = %q (%v), want postgres", driver, err)
	}
	if cfg.MaxConnections != 12 || cfg.PostgresDSN == "" {
		t.Fatalf("config = %+v, want the file's values", cfg)
	}

	// 坏配置必须报出**文件**，不再只丢一句 JSON 语法错误。
	broken := filepath.Join(dir, "broken.json")
	if err := os.WriteFile(broken, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(broken); err == nil || !strings.Contains(err.Error(), broken) {
		t.Fatalf("broken config error = %v, want it to name %s", err, broken)
	}
}

// 引擎选择是「启动器启哪个库」与「服务端读哪个库」之间的唯一契约：两边读同一份
// local.json，必须得出同一个答案，否则玩家明明有 PostgreSQL 存档却登录失败，
// 而 PostgreSQL 进程一切正常（2026-10-05 业主转达的 pgsql 端无法登录）。
//
// 这个表驱动用例就是那条契约的机械判据；internal/launcher 与两个 Python 存档档
// （launch_local.py / stop_environment.py）各有同一张表的对照用例。
func TestEngineForConfigSelectsTheSameEngineEverywhere(t *testing.T) {
	cases := []struct {
		name   string
		cfg    Config
		driver string
		fails  bool
	}{
		{name: "explicit sqlite wins", cfg: Config{Driver: "sqlite"}, driver: "sqlite"},
		{name: "explicit postgres wins", cfg: Config{Driver: "postgres"}, driver: "postgres"},
		{
			name:   "explicit sqlite wins over a DSN",
			cfg:    Config{Driver: "sqlite", PostgresDSN: "postgres://u@127.0.0.1:25438/dfo_lan"},
			driver: "sqlite",
		},
		{name: "driver is case and space insensitive", cfg: Config{Driver: " SQLite "}, driver: "sqlite"},
		{name: "unknown driver is refused", cfg: Config{Driver: "mysql"}, fails: true},

		// 关键回归：DSN 在、sqlite_path 也在、没写 driver。此前选 SQLite —— 启动器
		// 却按 PostgreSQL 起库，于是服务端在一个空 SQLite 文件里找不到账号。
		{
			name:   "a DSN outranks a leftover sqlite_path",
			cfg:    Config{PostgresDSN: "postgres://u@127.0.0.1:25438/dfo_lan", SQLitePath: "C:/saves/leftover.sqlite3"},
			driver: "postgres",
		},
		{name: "DSN alone selects postgres", cfg: Config{PostgresDSN: "postgres://u@127.0.0.1:25438/dfo_lan"}, driver: "postgres"},
		{name: "sqlite_path alone selects sqlite", cfg: Config{SQLitePath: "C:/saves/dfolan.sqlite3"}, driver: "sqlite"},
		{name: "neither falls back to sqlite (2026-10-05 default)", cfg: Config{}, driver: "sqlite"},
		{name: "blank DSN does not count as a DSN", cfg: Config{PostgresDSN: "   ", SQLitePath: "C:/saves/dfolan.sqlite3"}, driver: "sqlite"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			driver, err := EngineForConfig(tc.cfg)
			if tc.fails {
				if err == nil {
					t.Fatalf("EngineForConfig(%+v) = %q, want an error", tc.cfg, driver)
				}
				return
			}
			if err != nil {
				t.Fatalf("EngineForConfig(%+v): %v", tc.cfg, err)
			}
			if driver != tc.driver {
				t.Fatalf("EngineForConfig(%+v) = %q, want %q", tc.cfg, driver, tc.driver)
			}
		})
	}
}

// Open 必须走 engineForConfig：一个带 DSN 的配置绝不能因为多出一个 sqlite_path
// 就悄悄打开另一个库。用真实 SQLite 建立「DSN 优先级更高」的反向证明：
// 该配置会尝试 PostgreSQL（而不是在临时目录里新建一个 SQLite 库）。
func TestOpenPrefersTheDSNOverASQLitePath(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{
		// Unreachable on purpose: PostgreSQL must be attempted, and the failure must
		// not be "sqlite store" silently succeeding on the leftover path.
		PostgresDSN: "postgres://nobody@127.0.0.1:1/dfo_lan?sslmode=disable&connect_timeout=1",
		SQLitePath:  dir + "/leftover.sqlite3",
	}
	if driver, err := EngineForConfig(cfg); err != nil || driver != DriverPostgres {
		t.Fatalf("EngineForConfig chose %q (%v), want postgres", driver, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if store, err := Open(ctx, cfg); err == nil {
		store.Close()
		t.Fatal("Open succeeded against an unreachable DSN; the sqlite_path was used instead of the DSN")
	}
	if _, err := os.Stat(cfg.SQLitePath); err == nil {
		t.Fatal("Open created the leftover sqlite_path; the DSN must win outright")
	}
}
