package database

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sync_test.go 覆盖单引擎（SQLite）同步里不需要数据库服务的那一半：迁移分节表、
// SQLite 备份/还原往返、JSON 归一化与"备份失败就中止"的底线。
//
// 跨引擎复制（--copy-to）与 DSN 打码的用例随 PostgreSQL 支持一起移除
// （2026-10-05 业主口径，见根 AGENTS.md §0.6）。

// newSyncTestStore 造一个空库并跑完全部 SQLite 分节，返回库文件路径。
func newSyncTestStore(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "dfolan.sqlite3")
	store, err := openSQLiteStore(context.Background(), Config{Driver: DriverSQLite, SQLitePath: path})
	if err != nil {
		t.Fatalf("创建测试库失败：%v", err)
	}
	store.Close()
	return path
}

// execOnSQLite 在测试库上跑一条语句（走引擎的连接串，与真实链路一致）。
func execOnSQLite(t *testing.T, path, statement string) {
	t.Helper()
	db, err := openSQLite(context.Background(), path, 1, 5000)
	if err != nil {
		t.Fatalf("打开测试库失败：%v", err)
	}
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), statement); err != nil {
		t.Fatalf("执行 %q 失败：%v", statement, err)
	}
}

func countAccounts(t *testing.T, path string) int64 {
	t.Helper()
	db, err := openSQLite(context.Background(), path, 1, 5000)
	if err != nil {
		t.Fatalf("打开测试库失败：%v", err)
	}
	defer db.Close()
	var rows int64
	if err := db.QueryRowContext(context.Background(), "SELECT count(*) FROM accounts").Scan(&rows); err != nil {
		t.Fatalf("统计 accounts 失败：%v", err)
	}
	return rows
}

// TestSyncMigrationSectionsMatchEmbeddedFiles 钉住 openSQLiteStore 的前提：SQLite 初始
// 迁移文件里的分节名与顺序，必须与 sqliteMigrationSections 列表逐字一致。它一旦漂移，
// 建库就会少建/错序建表，而那种错误只在实机上才看得出来。
//
// 原先这条还比对 PostgreSQL 的初始迁移文件，随 PG 支持一起移除（2026-10-05 业主口径，
// 见根 AGENTS.md §0.6）。
func TestSyncMigrationSectionsMatchEmbeddedFiles(t *testing.T) {
	sqlite := migrationSectionNames(t, sqliteMigrationSQL, "sql/sqlite/migrations/0001_initial.sql")
	want := append([]string{"0000_migration_ledger.sql"}, sqliteMigrationSections...)
	if strings.Join(want, ",") != strings.Join(sqlite, ",") {
		t.Fatalf("分节表与 sqliteMigrationSections 不一致：\n文件 %v\n列表 %v", sqlite, want)
	}
}

func migrationSectionNames(t *testing.T, fs interface {
	ReadFile(string) ([]byte, error)
}, name string) []string {
	t.Helper()
	body, err := fs.ReadFile(name)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", name, err)
	}
	var names []string
	for _, line := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, "-- migration: ") {
			names = append(names, strings.TrimSpace(strings.TrimPrefix(line, "-- migration: ")))
		}
	}
	return names
}

// TestBackupAndRestoreSQLiteRoundTrip：备份落盘（副本 + manifest + 行数）→ 改库 →
// 还原覆盖（被改的数据回来、还原前的状态另存一份）。
func TestBackupAndRestoreSQLiteRoundTrip(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := newSyncTestStore(t, dir)
	backupsDir := filepath.Join(dir, BackupDirName)
	execOnSQLite(t, dbPath, "INSERT INTO accounts(username) VALUES('first')")

	target := SQLiteTarget(dbPath)
	entry, err := Backup(ctx, target, backupsDir, nil)
	if err != nil {
		t.Fatalf("备份失败：%v", err)
	}
	if entry.Engine != DriverSQLite {
		t.Fatalf("备份引擎 = %q", entry.Engine)
	}
	if entry.TotalRows != 1 {
		t.Fatalf("备份行数 = %d，期望 1", entry.TotalRows)
	}
	if len(entry.Files) != 1 || filepath.Ext(entry.Files[0].Name) != ".sqlite3" {
		t.Fatalf("备份数据文件不对：%+v", entry.Files)
	}
	for _, name := range []string{entry.Files[0].Name, manifestName} {
		if _, err := os.Stat(filepath.Join(entry.Directory, name)); err != nil {
			t.Fatalf("备份目录缺少 %s：%v", name, err)
		}
	}
	if entry.Files[0].Size == 0 || len(entry.Files[0].SHA256) != 64 {
		t.Fatalf("备份文件没有大小/哈希：%+v", entry.Files[0])
	}
	if !strings.Contains(entry.Source, "SQLite") {
		t.Fatalf("备份来源说明 = %q", entry.Source)
	}
	if err := VerifyBackup(entry); err != nil {
		t.Fatalf("刚写好的备份没通过校验：%v", err)
	}

	listed, err := ListBackups(backupsDir)
	if err != nil || len(listed) != 1 {
		t.Fatalf("ListBackups = %v, %v", listed, err)
	}
	if listed[0].Engine != DriverSQLite || listed[0].SizeBytes == 0 {
		t.Fatalf("列表项不完整：%+v", listed[0])
	}
	found, err := FindBackup(backupsDir, entry.Name)
	if err != nil || found.Name != entry.Name {
		t.Fatalf("FindBackup = %+v, %v", found, err)
	}
	if _, err := FindBackup(backupsDir, "../evil"); err == nil {
		t.Fatal("备份名里的路径穿越没有被拒绝")
	}

	execOnSQLite(t, dbPath, "INSERT INTO accounts(username) VALUES('second')")
	if rows := countAccounts(t, dbPath); rows != 2 {
		t.Fatalf("改动后行数 = %d，期望 2", rows)
	}

	report, err := RestoreBackup(ctx, entry, target, backupsDir, nil)
	if err != nil {
		t.Fatalf("还原失败：%v", err)
	}
	if report.TargetBackup == nil {
		t.Fatal("还原前没有自动备份被覆盖端")
	}
	if !report.Verified {
		t.Fatalf("还原后没有通过行数复核：%s", report.Note)
	}
	if rows := countAccounts(t, dbPath); rows != 1 {
		t.Fatalf("还原后行数 = %d，期望 1（被改的数据应当消失）", rows)
	}
	// 覆盖前的状态必须还在（那份自动备份是唯一的回退手段）。
	after, err := ListBackups(backupsDir)
	if err != nil || len(after) != 2 {
		t.Fatalf("还原后备份数 = %d, %v，期望 2", len(after), err)
	}
}

// TestAutoBackupTargetAbortsWhenBackupFails：目标端备份失败时**必须中止**，
// 不允许"没备份成功也照覆盖"。
func TestAutoBackupTargetAbortsWhenBackupFails(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "not-a-database.sqlite3")
	if err := os.MkdirAll(badPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := autoBackupTarget(context.Background(), SQLiteTarget(badPath), filepath.Join(dir, BackupDirName), nil); err == nil {
		t.Fatal("目标端读不出来时仍然继续了：必须中止")
	}
	// 目标端还没有数据时不算失败（没有可覆盖的东西）。
	missing := SQLiteTarget(filepath.Join(dir, "missing.sqlite3"))
	entry, err := autoBackupTarget(context.Background(), missing, filepath.Join(dir, BackupDirName), nil)
	if err != nil || entry != nil {
		t.Fatalf("目标端不存在时应当跳过备份：entry=%v err=%v", entry, err)
	}
}

// TestCanonicalJSONIgnoresOrderAndNumberForm：比的是内容，不是字节。
func TestCanonicalJSONIgnoresOrderAndNumberForm(t *testing.T) {
	left, err := canonicalJSON([]byte(`{"b":1,"a":2.0,"id":12345678901234567890}`))
	if err != nil {
		t.Fatal(err)
	}
	right, err := canonicalJSON(`{"a": 2, "id": 12345678901234567890, "b": 1.00}`)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(left, right) {
		t.Fatalf("同一份 JSON 归一后仍不同：\n%s\n%s", left, right)
	}
	if left, _ := canonicalJSON([]byte(`[1,2]`)); !bytes.Equal(left, []byte(`["1","2"]`)) {
		t.Fatalf("数组归一结果 = %s", left)
	}
	if _, err := canonicalJSON([]byte(`{oops`)); err == nil {
		t.Fatal("坏 JSON 没有被拒绝")
	}
	// 摘要是与行序无关的：两端的物理行序本来就不同。
	if jsonDigest([][]byte{[]byte("a"), []byte("b")}) != jsonDigest([][]byte{[]byte("b"), []byte("a")}) {
		t.Fatal("jsonDigest 依赖行序")
	}
	if jsonDigest([][]byte{[]byte("a")}) == jsonDigest([][]byte{[]byte("a"), []byte("a")}) {
		t.Fatal("jsonDigest 把重复行弄丢了")
	}
}
