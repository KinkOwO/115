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
// 手工改 local.json 是最容易带上 BOM 的那一步；此前服务端用 json.Unmarshal 直接吃
// 原始字节，于是启动即死在 `invalid character 'ï'`，而启动器（Python 的 utf-8-sig /
// Go 的 stripBOM）都读得进去 —— 报给业主就是「登录不上」。
func TestLoadConfigAcceptsTheByteOrderMark(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "local.json")
	body := "\ufeff{\"driver\":\"sqlite\",\"sqlite_path\":\"C:/Game/saves/dfolan.sqlite3\"," +
		"\"max_connections\":12}"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig refused a byte-order-marked config: %v", err)
	}
	if driver, err := EngineForConfig(cfg); err != nil || driver != DriverSQLite {
		t.Fatalf("engine = %q (%v), want sqlite", driver, err)
	}
	if cfg.MaxConnections != 12 || cfg.SQLitePath == "" {
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
// local.json，必须得出同一个答案，否则玩家明明有存档却登录失败。
//
// SQLite 是唯一引擎（2026-10-05 业主口径，见根 AGENTS.md §0.6），所以这张表只剩三种
// 结局：sqlite、明确拒绝已移除的 postgres、明确拒绝未知 driver。internal/launcher 有
// 同一张表的镜像用例，两侧必须逐字一致。
func TestEngineForConfigSelectsTheSameEngineEverywhere(t *testing.T) {
	cases := []struct {
		name   string
		cfg    Config
		driver string
		fails  bool
	}{
		{name: "explicit sqlite wins", cfg: Config{Driver: "sqlite"}, driver: "sqlite"},
		{name: "driver is case and space insensitive", cfg: Config{Driver: " SQLite "}, driver: "sqlite"},
		{name: "sqlite_path alone selects sqlite", cfg: Config{SQLitePath: "C:/saves/dfolan.sqlite3"}, driver: "sqlite"},
		{name: "neither falls back to sqlite (2026-10-05 default)", cfg: Config{}, driver: "sqlite"},
		// 已移除的引擎必须被**明确拒绝**，绝不能悄悄降级成 SQLite：那会把服务端指向
		// 另一个库，而玩家看到的是「存档像丢了」。
		{name: "the removed postgres driver is refused", cfg: Config{Driver: "postgres"}, fails: true},
		{name: "unknown driver is refused", cfg: Config{Driver: "mysql"}, fails: true},
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

// Open 必须走 EngineForConfig：一份明确写了 postgres 的配置绝不能被当成 SQLite 打开，
// 即使同一份文件里还留着一个 sqlite_path。反向证明是那个文件必须**没有被建出来**。
func TestOpenRefusesTheRemovedEngineEvenWithASQLitePath(t *testing.T) {
	dir := t.TempDir()
	cfg := Config{Driver: "postgres", SQLitePath: filepath.Join(dir, "leftover.sqlite3")}
	if driver, err := EngineForConfig(cfg); err == nil {
		t.Fatalf("EngineForConfig accepted the removed engine as %q", driver)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if store, err := Open(ctx, cfg); err == nil {
		store.Close()
		t.Fatal("Open succeeded on a profile that asks for the removed engine")
	}
	if _, err := os.Stat(cfg.SQLitePath); err == nil {
		t.Fatal("Open created the leftover sqlite_path instead of refusing the profile")
	}
}
