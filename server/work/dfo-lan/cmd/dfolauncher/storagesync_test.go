package main

// storagesync_test.go：storage-sync 里**不碰数据库**的那一半 —— 拒绝条件（管理租约）、
// 档位解析、引擎判定。真正的数据搬运由 internal/database 的测试与实机验收覆盖。
//
// 跨引擎复制（--copy-to / --restore-to）随 PostgreSQL 一起移除（2026-10-05 业主口径，
// 见根 AGENTS.md §0.6），所以这里只剩单档的解析与拒绝断言。

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"dfolan/internal/database"
)

const testSQLiteHeader = "SQLite format 3\x00"

func writeStorageFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// storageRootWithSQLite 造一个最小包根：只有 SQLite 档的模板与一个（占位的）库文件。
func storageRootWithSQLite(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	storage := storageSyncDir(root)
	dbPath := filepath.Join(storage, "dfolan.sqlite3")
	if err := os.MkdirAll(storage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dbPath, []byte(testSQLiteHeader), 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(map[string]any{"driver": "sqlite", "sqlite_path": dbPath})
	if err != nil {
		t.Fatal(err)
	}
	writeStorageFile(t, filepath.Join(storage, "local.sqlite.json"), string(body))
	return root, dbPath
}

// TestStorageSyncBlockedByAdminLease：SQLite 管理租约还在（持有者是我们这个活着的进程）时
// 必须拒绝动手 —— 这是"绝不拷一个正在写的库"的第一层；第二层是 7001。
func TestStorageSyncBlockedByAdminLease(t *testing.T) {
	root, dbPath := storageRootWithSQLite(t)
	lease := dbPath + ".admin-guard"
	if err := os.WriteFile(lease, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
		t.Fatal(err)
	}
	store, active, err := resolveStorageSyncStore(root)
	if err != nil {
		t.Fatalf("解析档位失败：%v", err)
	}
	if active != "" {
		t.Fatalf("没有活动档（只有模板）时 active=%q", active)
	}
	if !store.usable {
		t.Fatal("没有从 local.sqlite.json 识别出可用的 SQLite 档")
	}
	blocked := storageSyncBlocked(store)
	if blocked == "" {
		t.Fatal("管理租约还在时没有被拒绝")
	}
	if strings.Contains(blocked, "7001") {
		t.Skipf("本机 7001 正在监听（有会话在跑），跳过租约断言：%s", blocked)
	}
	if !strings.Contains(blocked, "租约") {
		t.Fatalf("拒绝原因不是租约：%s", blocked)
	}

	// 持有者已不存在 + 超过续租窗口 = 废弃租约：不该再挡着（否则崩溃一次就永远同步不了）。
	if err := os.WriteFile(lease, []byte("999999"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(lease, old, old); err != nil {
		t.Fatal(err)
	}
	if blocked := storageSyncBlocked(store); strings.Contains(blocked, "租约") {
		t.Fatalf("废弃的租约仍然挡着：%s", blocked)
	}
}

// TestResolveStorageSyncStoreUsesActiveProfile：活动档优先、模板兜底；活动档里写坏
// （相对路径）要直说，模板写坏只丢这一端。
func TestResolveStorageSyncStoreUsesActiveProfile(t *testing.T) {
	root, dbPath := storageRootWithSQLite(t)
	storage := storageSyncDir(root)
	body, err := json.Marshal(map[string]any{"driver": "sqlite", "sqlite_path": dbPath})
	if err != nil {
		t.Fatal(err)
	}
	writeStorageFile(t, filepath.Join(storage, "local.json"), string(body))
	store, active, err := resolveStorageSyncStore(root)
	if err != nil {
		t.Fatalf("解析档位失败：%v", err)
	}
	if active != database.DriverSQLite {
		t.Fatalf("active = %q，期望 sqlite", active)
	}
	if !store.usable {
		t.Fatal("活动档是可用的 SQLite 档，却没被认成可用")
	}
	if got := filepath.Base(store.origin); got != "local.json" {
		t.Fatalf("没有用活动档：origin 的 basename = %s", got)
	}

	// 活动档自己写坏了（相对路径）就直说：服务端启动同样会拒绝它，不能悄悄退回模板装作没事。
	writeStorageFile(t, filepath.Join(storage, "local.json"),
		`{"driver":"sqlite","sqlite_path":"runtime/storage/x.sqlite3"}`)
	if _, _, err := resolveStorageSyncStore(root); err == nil || !strings.Contains(err.Error(), "绝对路径") {
		t.Fatalf("活动档里的相对 sqlite_path 没有被拒绝：%v", err)
	}

	// 坏掉的是**模板**时，只丢这一端，不能拖垮整条命令（--list 照样能列备份）。
	writeStorageFile(t, filepath.Join(storage, "local.json"),
		`{"driver":"sqlite","sqlite_path":"runtime/storage/x.sqlite3"}`)
	writeStorageFile(t, filepath.Join(storage, "local.sqlite.json"),
		`{"driver":"sqlite","sqlite_path":"relative/x.sqlite3"}`)
	if _, _, err := resolveStorageSyncStore(root); err == nil || !strings.Contains(err.Error(), "绝对路径") {
		t.Fatalf("活动档坏档时必须报错，得到：%v", err)
	}

	// 活动档是 PostgreSQL 档（已不再支持）：不能当成 SQLite 去备份一个不存在的文件。
	writeStorageFile(t, filepath.Join(storage, "local.json"), `{"driver":"postgres"}`)
	store, active, err = resolveStorageSyncStore(root)
	if err != nil {
		t.Fatalf("PostgreSQL 档不该让解析报错（应当被识别为不可用）：%v", err)
	}
	if active != database.DriverPostgres {
		t.Fatalf("active = %q，期望 postgres（用于给出「该引擎已移除」的说明）", active)
	}
	if store.usable {
		t.Fatal("PostgreSQL 档被当成了可用的 SQLite 档")
	}
}

// TestProfileEngine：档位判据。空档**不能**兜底成 SQLite —— 那会让"备份当前档"去读一个
// 并不存在的文件；写了 postgres 的要原样返回，好让上层明确报"该引擎已移除"。
func TestProfileEngine(t *testing.T) {
	if got := profileEngine(map[string]any{"driver": "sqlite"}); got != database.DriverSQLite {
		t.Errorf("driver=sqlite → %q", got)
	}
	if got := profileEngine(map[string]any{"postgres_dsn": "postgres://x"}); got != database.DriverPostgres {
		t.Errorf("postgres_dsn → %q", got)
	}
	if got := profileEngine(map[string]any{"sqlite_path": "/x.sqlite3"}); got != database.DriverSQLite {
		t.Errorf("sqlite_path → %q", got)
	}
	if got := profileEngine(map[string]any{}); got != "" {
		t.Errorf("空档 → %q，期望空串", got)
	}
	if got := profileEngine(map[string]any{"driver": "MySQL"}); got != "" {
		t.Errorf("未知 driver → %q", got)
	}
}
