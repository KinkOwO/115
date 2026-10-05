package main

// storagesync_test.go：storage-sync 里**不碰数据库**的那一半 —— 拒绝条件（管理租约）、
// 档位解析、引擎名收敛、端口解析。真正的数据搬运由 internal/database 的测试与实机验收覆盖。

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
	stores, active, err := resolveStorageSyncStores(root)
	if err != nil {
		t.Fatalf("解析档位失败：%v", err)
	}
	if active != "" {
		t.Fatalf("没有活动档（只有模板）时 active=%q", active)
	}
	if _, ok := stores[database.DriverSQLite]; !ok {
		t.Fatalf("没有从 local.sqlite.json 识别出 SQLite 档：%+v", stores)
	}
	blocked := storageSyncBlocked(stores)
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
	if blocked := storageSyncBlocked(stores); strings.Contains(blocked, "租约") {
		t.Fatalf("废弃的租约仍然挡着：%s", blocked)
	}
}

// TestResolveStorageSyncStoresUsesActiveProfile：活动档优先，模板兜底；相对路径的
// sqlite_path 不能拖垮另一端。
func TestResolveStorageSyncStoresUsesActiveProfile(t *testing.T) {
	root, dbPath := storageRootWithSQLite(t)
	storage := storageSyncDir(root)
	body, err := json.Marshal(map[string]any{"driver": "sqlite", "sqlite_path": dbPath})
	if err != nil {
		t.Fatal(err)
	}
	writeStorageFile(t, filepath.Join(storage, "local.json"), string(body))
	stores, active, err := resolveStorageSyncStores(root)
	if err != nil {
		t.Fatalf("解析档位失败：%v", err)
	}
	if active != database.DriverSQLite {
		t.Fatalf("active = %q，期望 sqlite", active)
	}
	if got := filepath.Base(stores[database.DriverSQLite].origin); got != "local.json" {
		t.Fatalf("没有用活动档：origin 的 basename = %s", got)
	}
	if _, ok := stores[database.DriverPostgres]; ok {
		t.Fatal("没有 PostgreSQL 档却给出了 PostgreSQL 目标端")
	}

	writeStorageFile(t, filepath.Join(storage, "local.json"),
		`{"driver":"sqlite","sqlite_path":"runtime/storage/x.sqlite3"}`)
	// 活动档自己写坏了（相对路径）就直说：服务端启动同样会拒绝它，不能悄悄退回模板装作没事。
	if _, _, err := resolveStorageSyncStores(root); err == nil || !strings.Contains(err.Error(), "绝对路径") {
		t.Fatalf("活动档里的相对 sqlite_path 没有被拒绝：%v", err)
	}
	// 坏掉的是**模板**时，只丢这一端，不能拖垮整条命令（另一端照样能备份/还原）。
	writeStorageFile(t, filepath.Join(storage, "local.json"), string(body))
	writeStorageFile(t, filepath.Join(storage, "local.postgres.json"), `{"driver":"postgres"}`)
	stores, active, err = resolveStorageSyncStores(root)
	if err != nil {
		t.Fatalf("坏的模板不该让整条命令失败：%v", err)
	}
	if active != database.DriverSQLite {
		t.Fatalf("active = %q，期望 sqlite", active)
	}
	if _, ok := stores[database.DriverPostgres]; ok {
		t.Fatal("缺 postgres_dsn 的模板被当成了可用目标端")
	}
}

// TestProfileEngineAndNormalize：档位判据与取值收敛。空档**不能**兜底成 SQLite ——
// 那会让"备份当前档"去读一个并不存在的文件。
func TestProfileEngineAndNormalize(t *testing.T) {
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
	for _, item := range []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"SQLite", database.DriverSQLite, false},
		{" postgres ", database.DriverPostgres, false},
		{"mysql", "", true},
		{"", "", true},
	} {
		got, err := normalizeSyncEngine(item.in)
		if (err != nil) != item.wantErr || got != item.want {
			t.Errorf("normalize(%q) = %q, %v", item.in, got, err)
		}
	}
	if otherSyncEngine(database.DriverSQLite) != database.DriverPostgres ||
		otherSyncEngine(database.DriverPostgres) != database.DriverSQLite {
		t.Error("另一端推导错误")
	}
}

// TestDsnPort：端口从 DSN 里取，取不到才退回项目默认的 25438。
func TestDsnPort(t *testing.T) {
	if got := dsnPort("postgres://u:p@127.0.0.1:25438/dfo_lan?sslmode=disable"); got != 25438 {
		t.Errorf("URI 端口 = %d", got)
	}
	if got := dsnPort("host=127.0.0.1 port=5555 dbname=x"); got != 5555 {
		t.Errorf("key=value 端口 = %d", got)
	}
	if got := dsnPort("postgres://u:p@127.0.0.1/dfo_lan"); got != launcherDefaultPort {
		t.Errorf("缺端口时应当退回 %d，得到 %d", launcherDefaultPort, got)
	}
}

// launcherDefaultPort 只在测试里用来说明"缺端口时的期望值"。
const launcherDefaultPort = 25438
