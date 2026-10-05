package database

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// sync_test.go 覆盖双端同步里**不需要活 PostgreSQL** 的那一半：迁移分节表、打码、
// SQLite 备份/还原往返、值转换与"备份失败就中止"的底线。跨引擎复制那一半由
// 实机验收（临时 root + 独立测试库）覆盖，理由见报告。

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

// TestSyncMigrationSectionsMatchEmbeddedFiles 钉住 rebuildSchemaInTx 的前提：
// 两份初始迁移文件的**分节名与顺序完全一致**。它一旦漂移，SQLite → PostgreSQL 复制
// 就会按错误的分节表建表（少建/错序），而那种错误只在实机上才看得出来。
func TestSyncMigrationSectionsMatchEmbeddedFiles(t *testing.T) {
	postgres := migrationSectionNames(t, migrationSQL, "sql/postgres/migrations/0001_initial.sql")
	sqlite := migrationSectionNames(t, sqliteMigrationSQL, "sql/sqlite/migrations/0001_initial.sql")

	if len(postgres) != len(sqlite) {
		t.Fatalf("两份初始迁移的分节数不同：postgres %d，sqlite %d", len(postgres), len(sqlite))
	}
	for i := range postgres {
		if postgres[i] != sqlite[i] {
			t.Fatalf("第 %d 节不一致：postgres %q，sqlite %q", i, postgres[i], sqlite[i])
		}
	}
	want := append([]string{"0000_migration_ledger.sql"}, sqliteMigrationSections...)
	if strings.Join(want, ",") != strings.Join(postgres, ",") {
		t.Fatalf("分节表与 sqliteMigrationSections 不一致：\n文件 %v\n列表 %v", postgres, want)
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

// TestMaskDSNHidesPassword：报告、日志、manifest 里都不许出现口令。
func TestMaskDSNHidesPassword(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
		want string
	}{
		{"URI 带口令", "postgres://dfo_owner:s3cret@127.0.0.1:25438/dfo_lan?sslmode=disable",
			"postgres://dfo_owner:***@127.0.0.1:25438/dfo_lan?sslmode=disable"},
		{"URI 无口令", "postgres://dfo_owner@127.0.0.1:25438/dfo_lan", "postgres://dfo_owner@127.0.0.1:25438/dfo_lan"},
		{"key=value", "host=127.0.0.1 port=25438 user=dfo_owner password=s3cret dbname=dfo_lan",
			"host=127.0.0.1 port=25438 user=dfo_owner password=*** dbname=dfo_lan"},
		{"空串", "", ""},
	}
	for _, item := range cases {
		if got := MaskDSN(item.dsn); got != item.want {
			t.Errorf("%s：MaskDSN = %q，期望 %q", item.name, got, item.want)
		}
		if strings.Contains(MaskDSN(item.dsn), "s3cret") {
			t.Errorf("%s：打码后仍然出现口令", item.name)
		}
	}
	if got := DescribeDSN("postgres://dfo_owner:s3cret@127.0.0.1:25438/dfo_lan?sslmode=disable"); got != "127.0.0.1:25438/dfo_lan" {
		t.Errorf("DescribeDSN = %q", got)
	}
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
	entry, err := Backup(ctx, target, backupsDir, Tools{}, nil)
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

	report, err := RestoreBackup(ctx, entry, target, backupsDir, Tools{}, nil)
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
	if _, err := autoBackupTarget(context.Background(), SQLiteTarget(badPath), filepath.Join(dir, BackupDirName), Tools{}, nil); err == nil {
		t.Fatal("目标端读不出来时仍然继续了：必须中止")
	}
	// 目标端还没有数据时不算失败（没有可覆盖的东西）。
	missing := SQLiteTarget(filepath.Join(dir, "missing.sqlite3"))
	entry, err := autoBackupTarget(context.Background(), missing, filepath.Join(dir, BackupDirName), Tools{}, nil)
	if err != nil || entry != nil {
		t.Fatalf("目标端不存在时应当跳过备份：entry=%v err=%v", entry, err)
	}
}

// TestCopyRefusesSameEngine：复制不是切档，同引擎之间没有可搬运的东西。
func TestCopyRefusesSameEngine(t *testing.T) {
	dir := t.TempDir()
	_, err := CopyBetweenEngines(context.Background(),
		SQLiteTarget(filepath.Join(dir, "a.sqlite3")),
		SQLiteTarget(filepath.Join(dir, "b.sqlite3")),
		filepath.Join(dir, BackupDirName), Tools{}, nil)
	if err == nil || !strings.Contains(err.Error(), "两端都是") {
		t.Fatalf("同引擎复制没有被拒绝：%v", err)
	}
}

// TestCanonicalJSONIgnoresOrderAndNumberForm：跨引擎比的是内容，不是字节。
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

// TestSQLiteValueToPostgres：单值转换的正反规则（与 sqliteconvert.go 的 convertValue 对应）。
func TestSQLiteValueToPostgres(t *testing.T) {
	cases := []struct {
		name   string
		column pgColumn
		value  any
		want   any
	}{
		{"布尔 0/1", pgColumn{DataType: "boolean"}, int64(1), true},
		{"布尔 0", pgColumn{DataType: "boolean"}, int64(0), false},
		{"整数", pgColumn{DataType: "bigint"}, int64(42), int64(42)},
		{"文本 BLOB", pgColumn{DataType: "text"}, []byte("hi"), "hi"},
		{"时间微秒", pgColumn{DataType: "timestamp with time zone"}, int64(1700000000000000),
			time.UnixMicro(1700000000000000).UTC()},
		{"DATE 文本", pgColumn{DataType: "date"}, "2026-10-05", time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)},
		{"大整数数组", pgColumn{DataType: "ARRAY", UDTName: "_int8"}, []byte("[1,2,3]"), []int64{1, 2, 3}},
		{"JSON 原样", pgColumn{DataType: "jsonb"}, []byte(`{"a":1}`), `{"a":1}`},
		{"NULL", pgColumn{DataType: "text"}, nil, nil},
	}
	for _, item := range cases {
		got, err := sqliteValueToPostgres("t", item.column, item.value)
		if err != nil {
			t.Errorf("%s：%v", item.name, err)
			continue
		}
		if !reflectDeepEqual(got, item.want) {
			t.Errorf("%s：得到 %#v，期望 %#v", item.name, got, item.want)
		}
	}
	// 目标端没有对应关系时原样传递，而不是丢数据。
	if got, err := sqliteValueToPostgres("t", pgColumn{DataType: "uuid"}, "abc"); err != nil || got != "abc" {
		t.Errorf("未知类型 = %#v, %v", got, err)
	}
	if _, err := sqliteValueToPostgres("t", pgColumn{DataType: "bigint"}, "not-a-number"); err == nil {
		t.Error("非数字文本被当成整数接受了")
	}
}

// reflectDeepEqual 只服务本文件的几个断言，避免为一个比较引入 reflect 之外的写法。
func reflectDeepEqual(a, b any) bool {
	left, errLeft := json.Marshal(a)
	right, errRight := json.Marshal(b)
	if errLeft != nil || errRight != nil {
		return false
	}
	return bytes.Equal(left, right)
}
