package database

// sync.go 实现「双端同步」：备份、跨引擎覆盖式复制、还原（业主 2026-10-05 需求）。
//
// 边界（写在最前面，因为它决定了下面每一处的取舍）：
//
//   - **复制 ≠ 切档**。复制只改写目标端的数据（SQLite 文件 / PostgreSQL 库），
//     **不动** runtime/storage/local.json，也不动两个档位模板；这次跑哪一档仍然由
//     启动器「启动环境」决定。这样"搬家"和"选档"是两件互不牵连的事。
//   - 目标端在**被覆盖之前一定先自动备份**，备份失败就中止：不存在"没备份成功也照覆盖"。
//   - 备份目录：<storage>/backups/<时间戳>-<engine>/，里面有数据文件（SQLite 副本或
//     pg_dump -Fc 归档）与 manifest.json（引擎、时间、打码后的来源、文件 sha256、每张表行数）。
//     备份自带引擎身份，还原时靠它知道自己该回哪一端。
//   - 报告与日志里的 DSN 一律打码（MaskDSN）：manifest 也会落到磁盘上，同样不许出现口令。
//   - "服务端不在跑"由调用方（cmd/dfolauncher）守卫：本文件只做数据搬运，不判断端口。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// BackupDirName 是备份根目录名，位于 runtime/storage 之下。
	BackupDirName = "backups"
	// manifestName 是每份备份的清单文件名。它自己不进 Files：清单不能包含自己的哈希。
	manifestName = "manifest.json"
	// backupStampLayout 是备份目录名里的时间戳（可读、可排序：20261005-140312）。
	backupStampLayout = "20060102-150405"
)

// EngineTarget 描述同步要读写的那一端：用哪个引擎、配置是什么、报告里怎么称呼它。
type EngineTarget struct {
	Engine string
	Config Config
	// Label 是打码后的位置说明，会进 manifest、日志与界面。
	Label string
}

// SQLiteTarget 用一条库文件路径构造目标端。
func SQLiteTarget(path string) EngineTarget {
	path = strings.TrimSpace(path)
	return EngineTarget{
		Engine: DriverSQLite,
		Config: Config{Driver: DriverSQLite, SQLitePath: path},
		Label:  "SQLite（" + path + "）",
	}
}



// BackupFile 是备份目录里的一个数据文件及其哈希。
type BackupFile struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// TableRows 是一张表的行数，备份清单与复制报告共用。
type TableRows struct {
	Name string `json:"name"`
	Rows int64  `json:"rows"`
}

// BackupEntry 是一份备份的清单。它同时是 manifest.json 的内容：落盘与回读走同一个结构，
// 所以"写进去的字段"与"渲染出来的字段"不会漂移。
type BackupEntry struct {
	Name      string       `json:"name"`
	Engine    string       `json:"engine"`
	Directory string       `json:"directory"`
	CreatedAt string       `json:"created_at"`
	Source    string       `json:"source"`
	Files     []BackupFile `json:"files"`
	Tables    []TableRows  `json:"tables,omitempty"`
	TotalRows int64        `json:"total_rows"`
	// SizeBytes 是整个备份目录的大小（含 manifest.json 自己），列表与 UI 用。
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Note      string `json:"note,omitempty"`
}


// RestoreReport 是一次还原的结论。
type RestoreReport struct {
	Backup       BackupEntry  `json:"backup"`
	Engine       string       `json:"engine"`
	Target       string       `json:"target"`
	TargetBackup *BackupEntry `json:"target_backup,omitempty"`
	Tables       []TableRows  `json:"tables,omitempty"`
	TotalRows    int64        `json:"total_rows"`
	Verified     bool         `json:"verified"`
	Note         string       `json:"note,omitempty"`
}

// ---------------------------------------------------------------------------
// 打码：报告、日志、manifest 只允许出现打码后的 DSN
// ---------------------------------------------------------------------------


// ---------------------------------------------------------------------------
// 备份
// ---------------------------------------------------------------------------

// Backup 把 target 的数据完整备份到 backupsDir 下的 <时间戳>-<engine> 目录。
//
// 一致性做法：
//   - SQLite 用 VACUUM INTO（SQLite 自己的在线备份语义）：一个读事务里把整库
//     （含 WAL 中已提交的部分）写成一份**一致**的新文件，不需要先 checkpoint，
//     也不会留下半个文件。副本写完还要只读打开、逐表计数，能读出来才算备份可用。
func Backup(ctx context.Context, target EngineTarget, backupsDir string, log func(string, ...any)) (BackupEntry, error) {
	if log == nil {
		log = func(string, ...any) {}
	}
	if strings.TrimSpace(backupsDir) == "" {
		return BackupEntry{}, errors.New("未指定备份目录")
	}
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return BackupEntry{}, fmt.Errorf("创建备份目录失败：%w", err)
	}
	dir := uniquePath(filepath.Join(backupsDir, time.Now().Format(backupStampLayout)+"-"+target.Engine))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return BackupEntry{}, fmt.Errorf("创建备份目录失败：%w", err)
	}

	entry := BackupEntry{
		Name:      filepath.Base(dir),
		Engine:    target.Engine,
		Directory: dir,
		CreatedAt: time.Now().Format(time.RFC3339),
		Source:    target.Label,
	}
	var err error
	switch target.Engine {
	case DriverSQLite:
		err = backupSQLite(ctx, target, dir, &entry, log)
	default:
		err = fmt.Errorf("未知的存储引擎 %q", target.Engine)
	}
	if err != nil {
		// 失败不留半个目录：留着它会被 --list 当成一份"真备份"，比没有更糟。
		_ = os.RemoveAll(dir)
		return BackupEntry{}, err
	}

	body, marshalErr := json.MarshalIndent(entry, "", "  ")
	if marshalErr != nil {
		return BackupEntry{}, marshalErr
	}
	if writeErr := os.WriteFile(filepath.Join(dir, manifestName), append(body, '\n'), 0o644); writeErr != nil {
		return BackupEntry{}, fmt.Errorf("写入 manifest.json 失败：%w", writeErr)
	}
	entry.SizeBytes = dirSize(dir)
	log("已备份 %s → %s（%d 张表 / %d 行）", target.Label, dir, len(entry.Tables), entry.TotalRows)
	return entry, nil
}

// backupSQLite 用 VACUUM INTO 生成副本，并回读副本做行数统计。
func backupSQLite(ctx context.Context, target EngineTarget, dir string, entry *BackupEntry, log func(string, ...any)) error {
	path := strings.TrimSpace(target.Config.SQLitePath)
	if path == "" {
		return errors.New("SQLite 档缺少 sqlite_path，无法备份")
	}
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("SQLite 存档 %s 不可读：%w", path, err)
	}
	if st.IsDir() {
		return fmt.Errorf("SQLite 存档 %s 是目录，不是库文件", path)
	}
	dest := filepath.Join(dir, filepath.Base(path))
	db, err := openSQLite(ctx, path, 1, 10000)
	if err != nil {
		return fmt.Errorf("打开 SQLite 存档失败：%w", err)
	}
	defer db.Close()
	// VACUUM INTO 要求目标文件不存在 —— 这正是我们刚建的目录。
	// 路径转成斜杠并在单引号里转义，免得 Windows 反斜杠与引号把语句改味。
	if _, err := db.ExecContext(ctx, "VACUUM INTO "+quoteLiteral(filepath.ToSlash(dest))); err != nil {
		return fmt.Errorf("VACUUM INTO %s 失败：%w", dest, err)
	}
	file, err := describeFile(dest)
	if err != nil {
		return err
	}
	entry.Files = append(entry.Files, file)

	// 副本必须**能读出来**才算备份：只读 + immutable 打开，不写 -shm/-wal，也不动副本本身。
	tables, total, err := sqliteRowCounts(ctx, dest)
	if err != nil {
		return fmt.Errorf("备份副本 %s 读不出来（备份不可用）：%w", dest, err)
	}
	entry.Tables, entry.TotalRows = tables, total
	log("SQLite 在线备份完成：%s（%d 字节，%d 张表）", filepath.Base(dest), file.Size, len(tables))
	return nil
}


// ListBackups 列出备份目录下的所有备份，最新的排在最前面。
//
// manifest.json 读不出来的目录**照样列出来**（引擎留空、带一句说明）：它占着空间也需要
// 人去处理，静默隐藏只会让人以为磁盘里的东西不见了。
func ListBackups(backupsDir string) ([]BackupEntry, error) {
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []BackupEntry
	for _, item := range entries {
		if !item.IsDir() {
			continue
		}
		dir := filepath.Join(backupsDir, item.Name())
		entry := BackupEntry{Name: item.Name(), Directory: dir}
		body, readErr := os.ReadFile(filepath.Join(dir, manifestName))
		if readErr == nil {
			if jsonErr := json.Unmarshal(body, &entry); jsonErr != nil {
				entry = BackupEntry{Name: item.Name(), Directory: dir,
					Note: "manifest.json 解析失败：" + jsonErr.Error()}
			}
		} else {
			entry.Note = "缺少 manifest.json：" + readErr.Error()
		}
		entry.Name = item.Name()
		entry.Directory = dir
		entry.SizeBytes = dirSize(dir)
		out = append(out, entry)
	}
	// 目录名以时间戳开头，倒序即"最新在前"。
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out, nil
}

// FindBackup 在备份目录里按名字取一份备份。名字只允许是单层目录名。
func FindBackup(backupsDir, name string) (BackupEntry, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return BackupEntry{}, errors.New("未指定备份名")
	}
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return BackupEntry{}, fmt.Errorf("备份名 %q 只能是 backups 下的一个目录名", name)
	}
	dir := filepath.Join(backupsDir, name)
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return BackupEntry{}, fmt.Errorf("备份 %s 不存在", name)
	}
	entry := BackupEntry{Name: name, Directory: dir}
	body, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return BackupEntry{}, fmt.Errorf("备份 %s 缺少 manifest.json，无法确认它属于哪一端", name)
	}
	if err := json.Unmarshal(body, &entry); err != nil {
		return BackupEntry{}, fmt.Errorf("备份 %s 的 manifest.json 解析失败：%w", name, err)
	}
	entry.Name = name
	entry.Directory = dir
	entry.SizeBytes = dirSize(dir)
	// PostgreSQL 支持已于 2026-10-05 移除（业主口径，见根 AGENTS.md §0.6）。一份历史
	// PostgreSQL 备份仍然会被识别出来并**明确拒绝**，而不是被当成 SQLite 备份去覆盖现有库：
	// 要读它请回退到移除前的构建并用 dfo-tool sqliteconvert 搬运（见 docs/sqlite-operations.md）。
	if entry.Engine != DriverSQLite {
		return BackupEntry{}, fmt.Errorf(
			"备份 %s 是 %s 的：PostgreSQL 支持已移除，只能还原 sqlite 备份", name, entry.Engine)
	}
	return entry, nil
}

// VerifyBackup 重算备份里每个文件的 sha256，与 manifest 对比。
//
// 还原之前必须先过这一关：从一份损坏的备份覆盖掉现有数据，是这套功能里唯一
// "两个方向都救不回来"的操作。
func VerifyBackup(entry BackupEntry) error {
	if len(entry.Files) == 0 {
		return fmt.Errorf("备份 %s 的 manifest.json 里没有任何数据文件", entry.Name)
	}
	for _, file := range entry.Files {
		path := filepath.Join(entry.Directory, file.Name)
		got, err := describeFile(path)
		if err != nil {
			return fmt.Errorf("备份文件 %s 读不出来：%w", file.Name, err)
		}
		if got.Size != file.Size || !strings.EqualFold(got.SHA256, file.SHA256) {
			return fmt.Errorf("备份文件 %s 与 manifest 不一致（大小 %d/%d，sha256 %s/%s）：备份已损坏，拒绝还原",
				file.Name, got.Size, file.Size, got.SHA256, file.SHA256)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 复制：覆盖式写入另一端
// ---------------------------------------------------------------------------


// autoBackupTarget 在覆盖目标端之前自动备份它。目标端还没有数据时如实说明并跳过
// （没有可覆盖的东西，就不存在"备份失败还照覆盖"）。
func autoBackupTarget(ctx context.Context, target EngineTarget, backupsDir string, log func(string, ...any)) (*BackupEntry, error) {
	if log == nil {
		// 本函数既被 RestoreBackup（已兜底）调用，也会被测试直接调用。
		log = func(string, ...any) {}
	}
	if target.Engine == DriverSQLite {
		path := strings.TrimSpace(target.Config.SQLitePath)
		if path == "" {
			return nil, errors.New("SQLite 目标端缺少 sqlite_path")
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			log("目标端 %s 还没有数据（文件不存在），无需备份", target.Label)
			return nil, nil
		}
	}
	entry, err := Backup(ctx, target, backupsDir, log)
	if err != nil {
		return nil, fmt.Errorf("覆盖前自动备份目标端失败，已中止：%w", err)
	}
	log("目标端已自动备份：%s", entry.Directory)
	return &entry, nil
}


// ---------------------------------------------------------------------------
// 还原：把一份备份写回它自己的引擎（覆盖）
// ---------------------------------------------------------------------------

// RestoreBackup 把 entry 还原到 target（target 必须是 entry 记录的引擎）。
//
// 与复制同一条底线：先校验备份自身的 sha256，再自动备份被覆盖端，最后才写。
func RestoreBackup(ctx context.Context, entry BackupEntry, target EngineTarget, backupsDir string, log func(string, ...any)) (RestoreReport, error) {
	report := RestoreReport{Backup: entry, Engine: entry.Engine, Target: target.Label}
	if log == nil {
		log = func(string, ...any) {}
	}
	if entry.Engine != target.Engine {
		return report, fmt.Errorf("备份 %s 属于 %s，不能还原到 %s", entry.Name, entry.Engine, target.Engine)
	}
	if err := VerifyBackup(entry); err != nil {
		return report, err
	}
	backup, err := autoBackupTarget(ctx, target, backupsDir, log)
	if err != nil {
		return report, err
	}
	report.TargetBackup = backup

	switch entry.Engine {
	case DriverSQLite:
		err = restoreSQLite(ctx, entry, target, log)
	default:
		err = fmt.Errorf("未知的存储引擎 %q", entry.Engine)
	}
	if err != nil {
		return report, err
	}

	// 还原后按 manifest 里的行数复核。这一步不改数据，只如实报告：还原本身已经完成，
	// 而"少了几行"必须让人看见（覆盖前的备份路径就在上面，可以回退）。
	tables, total, verifyErr := currentRowCounts(ctx, target)
	if verifyErr != nil {
		report.Note = "还原后无法复核行数：" + verifyErr.Error()
		log("注意：%s", report.Note)
		return report, nil
	}
	report.Tables, report.TotalRows = tables, total
	if mismatch := rowCountMismatch(entry.Tables, tables); mismatch != "" {
		report.Note = "还原完成，但行数与备份清单不一致：" + mismatch
		log("警告：%s", report.Note)
		return report, nil
	}
	report.Verified = true
	log("已还原 %s → %s（%d 张表 / %d 行，与备份清单一致）", entry.Name, target.Label, len(tables), total)
	return report, nil
}

// restoreSQLite 用备份副本替换目标库文件。
func restoreSQLite(ctx context.Context, entry BackupEntry, target EngineTarget, log func(string, ...any)) error {
	dest := strings.TrimSpace(target.Config.SQLitePath)
	if dest == "" {
		return errors.New("SQLite 目标端缺少 sqlite_path")
	}
	if !filepath.IsAbs(dest) {
		return fmt.Errorf("sqlite_path 必须是绝对路径：%q", dest)
	}
	src := filepath.Join(entry.Directory, entry.Files[0].Name)
	if err := checkSQLiteFile(src); err != nil {
		return err
	}
	tmp := dest + ".sync-tmp"
	if err := copyFile(src, tmp); err != nil {
		return fmt.Errorf("准备还原副本失败：%w", err)
	}
	if err := replaceSQLiteFile(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	log("SQLite 已还原：%s（来自 %s）", dest, entry.Name)
	return nil
}


// toInt64 把 SQLite 可能给出的几种表示收敛成 int64。
func toInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case float64:
		return int64(typed), nil
	case bool:
		if typed {
			return 1, nil
		}
		return 0, nil
	case []byte:
		return parseIntString(string(typed))
	case string:
		return parseIntString(typed)
	case time.Time:
		return typed.UTC().UnixMicro(), nil
	}
	return 0, fmt.Errorf("无法把 %T 当成整数", value)
}

func parseIntString(text string) (int64, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, errors.New("空字符串不是整数")
	}
	var out int64
	if _, err := fmt.Sscanf(text, "%d", &out); err != nil {
		return 0, fmt.Errorf("%q 不是整数", text)
	}
	return out, nil
}

func parseBoolString(text string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "1", "t", "true", "yes", "on":
		return true, nil
	case "0", "f", "false", "no", "off", "":
		return false, nil
	}
	return false, fmt.Errorf("%q 不是布尔值", text)
}

// toString 把 SQLite 的 TEXT/BLOB 表示收敛成 string。
func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case nil:
		return ""
	}
	return fmt.Sprint(value)
}

// toTime 把 SQLite 的时间表示收敛成 time.Time（库里存的是整数微秒）。
func toTime(value any) (time.Time, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC(), nil
	case int64:
		return time.UnixMicro(typed).UTC(), nil
	case int:
		return time.UnixMicro(int64(typed)).UTC(), nil
	case float64:
		return time.UnixMicro(int64(typed)).UTC(), nil
	case []byte:
		return parseTimeString(string(typed))
	case string:
		return parseTimeString(typed)
	}
	return time.Time{}, fmt.Errorf("无法把 %T 当成时间", value)
}

func parseTimeString(text string) (time.Time, error) {
	text = strings.TrimSpace(text)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999-07", "2006-01-02 15:04:05"} {
		if moment, err := time.Parse(layout, text); err == nil {
			return moment.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("%q 不是可识别的时间", text)
}

// toIntSlice 把 SQLite 里的 JSON 数组文本转成 PostgreSQL 数组入参（本 schema 只有 bigint[]）。
func toIntSlice(value any) ([]int64, error) {
	text := strings.TrimSpace(toString(value))
	if text == "" {
		return nil, nil
	}
	var numbers []int64
	if err := json.Unmarshal([]byte(text), &numbers); err != nil {
		return nil, fmt.Errorf("数组文本 %q 解析失败：%w", text, err)
	}
	return numbers, nil
}

// ---------------------------------------------------------------------------
// JSON 校验：跨引擎比较内容而不是字节
// ---------------------------------------------------------------------------

// canonicalJSON 把一段 JSON 归一成"语义相同则字符串相同"的形式：
// 对象键排序、数字按精确有理数归一（1.0 与 1 相等）。
//
// 为什么不比字节：SQLite 存的是文本，PostgreSQL 的 jsonb 会在存储时重新归一
// （键序、空白、数字写法都可能变），字节比较必然误报。也不比 float64：
// 大整数会在往返里丢精度，同样误报。json.Number + big.Rat 两个坑一起避开。
func canonicalJSON(raw any) ([]byte, error) {
	var decoded any
	switch typed := raw.(type) {
	case []byte:
		if len(bytes.TrimSpace(typed)) == 0 {
			return []byte("null"), nil
		}
		if err := decodeNumbers(typed, &decoded); err != nil {
			return nil, err
		}
	case string:
		if strings.TrimSpace(typed) == "" {
			return []byte("null"), nil
		}
		if err := decodeNumbers([]byte(typed), &decoded); err != nil {
			return nil, err
		}
	default:
		return json.Marshal(typed)
	}
	return json.Marshal(canonicalizeNumbers(decoded))
}

func decodeNumbers(raw []byte, out *any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(out)
}

func canonicalizeNumbers(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, sub := range typed {
			typed[key] = canonicalizeNumbers(sub)
		}
		return typed
	case []any:
		for i, sub := range typed {
			typed[i] = canonicalizeNumbers(sub)
		}
		return typed
	case json.Number:
		if rat, ok := new(big.Rat).SetString(typed.String()); ok {
			return rat.RatString()
		}
		return typed.String()
	}
	return value
}

// jsonDigest 对一列 JSON 值给出与行序无关的摘要：逐行哈希、排序后再折一次。
//
// 与行序无关是必须的：SQLite 与 PostgreSQL 的物理行序本来就不同，而"内容一致"
// 才是要证明的事。重复行不会互相抵消（每一行都单独进摘要）。
func jsonDigest(values [][]byte) string {
	hashes := make([]string, 0, len(values))
	for _, value := range values {
		sum := sha256Sum(string(value))
		hashes = append(hashes, hex.EncodeToString(sum))
	}
	sort.Strings(hashes)
	digest := sha256.New()
	for _, hash := range hashes {
		io.WriteString(digest, hash)
		io.WriteString(digest, "\n")
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func sha256Sum(text string) []byte {
	sum := sha256.Sum256([]byte(text))
	return sum[:]
}

// ---------------------------------------------------------------------------
// 通用小工具
// ---------------------------------------------------------------------------

// currentRowCounts 按引擎读出目标端当前的每表行数（还原后复核用）。
func currentRowCounts(ctx context.Context, target EngineTarget) ([]TableRows, int64, error) {
	switch target.Engine {
	case DriverSQLite:
		return sqliteRowCounts(ctx, target.Config.SQLitePath)
	}
	return nil, 0, fmt.Errorf("未知的存储引擎 %q", target.Engine)
}

// rowCountMismatch 比较两份行数清单，一致时返回空串。
func rowCountMismatch(want, got []TableRows) string {
	wanted := map[string]int64{}
	for _, table := range want {
		wanted[table.Name] = table.Rows
	}
	var problems []string
	for _, table := range got {
		if expected, ok := wanted[table.Name]; !ok {
			problems = append(problems, fmt.Sprintf("%s 多出 %d 行", table.Name, table.Rows))
		} else if expected != table.Rows {
			problems = append(problems, fmt.Sprintf("%s 期望 %d 行、实际 %d 行", table.Name, expected, table.Rows))
		}
		delete(wanted, table.Name)
	}
	for name, rows := range wanted {
		problems = append(problems, fmt.Sprintf("%s 缺 %d 行", name, rows))
	}
	sort.Strings(problems)
	return strings.Join(problems, "；")
}

// sqliteRowCounts 数一份 SQLite 库每张表的行数（只读 + immutable，不动文件）。
func sqliteRowCounts(ctx context.Context, path string) ([]TableRows, int64, error) {
	db, err := openSQLiteImmutable(ctx, path)
	if err != nil {
		return nil, 0, err
	}
	defer db.Close()
	names, err := sqliteTableNames(ctx, db)
	if err != nil {
		return nil, 0, err
	}
	var out []TableRows
	var total int64
	for _, name := range names {
		if engineLocalTables[name] {
			continue
		}
		var rows int64
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+quoteIdent(name)).Scan(&rows); err != nil {
			return nil, 0, fmt.Errorf("统计表 %s 行数失败：%w", name, err)
		}
		out = append(out, TableRows{Name: name, Rows: rows})
		total += rows
	}
	return out, total, nil
}


// sqliteColumnNames 读一张 SQLite 表的列名（按声明顺序）。
func sqliteColumnNames(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+quoteIdent(table)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var cid int64
		var name, columnType string
		var notNull, primaryKey int64
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// openSQLiteImmutable 以只读 + 不可变方式打开一份**副本**（备份产物）。
//
// 与引擎的连接串刻意不同：不给副本建立 -shm/-wal，也不加锁，所以"读一遍备份"不会
// 改动备份目录里的任何字节（否则清单里的 sha256 会在校验之后失效）。
func openSQLiteImmutable(ctx context.Context, path string) (*sql.DB, error) {
	slashed := filepath.ToSlash(path)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed // C:/x -> /C:/x，与 sqliteDSN 同一处理
	}
	db, err := sql.Open("sqlite", "file://"+slashed+"?mode=ro&immutable=1&_timezone=UTC&_dqs=0")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// replaceSQLiteFile 用 src（已校验完的新库）替换 dest。
//
// 替换之前必须先删掉 dest 的 -wal / -shm / -journal：它们是**旧库**的附属文件，
// 留着会让 SQLite 下次打开时按旧 WAL 恢复，表现为"覆盖成功但数据没变"，甚至损坏。
func replaceSQLiteFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("创建目标端目录失败：%w", err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err := os.Remove(dest + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("删除旧库附属文件 %s 失败：%w", dest+suffix, err)
		}
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	// Windows 上目标文件被占用/已存在时 Rename 可能失败：删掉再试一次，仍失败就如实报错。
	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("替换 %s 失败：%w", dest, err)
	}
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("替换 %s 失败：%w", dest, err)
	}
	return nil
}

// checkSQLiteFile 确认一个文件确实是 SQLite 库（文件头魔数）。
//
// 光比对 sha256 只能证明"文件没被改过"，证明不了"它是一份 SQLite 存档"：
// manifest 是人可读的 JSON，指错文件时用它覆盖目标端会直接毁掉目标库。
func checkSQLiteFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, 16)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("%s 读不出 SQLite 文件头：%w", path, err)
	}
	if string(header) != "SQLite format 3\x00" {
		return fmt.Errorf("%s 不是 SQLite 存档（文件头不符），拒绝用它覆盖目标端", path)
	}
	return nil
}

// copyFile 复制文件（先写 .partial 再改名，避免留下半个文件被当成真备份）。
func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".partial"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// describeFile 给出一个文件的路径、大小与 sha256。
func describeFile(path string) (BackupFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return BackupFile{}, err
	}
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	if err != nil {
		return BackupFile{}, err
	}
	return BackupFile{Name: filepath.Base(path), Size: size, SHA256: hex.EncodeToString(digest.Sum(nil))}, nil
}

// dirSize 递归统计目录大小（失败时返回已知部分，不因此让整个列表失败）。
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// uniquePath 在路径已存在时追加 -2、-3……（同一秒内对同一引擎备份两次不会互相覆盖）。
func uniquePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", path, i)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", path, time.Now().UnixNano())
}

// quoteLiteral 把一个字符串放进 SQL 单引号里（SQLite 的 VACUUM INTO 不接受绑定参数）。
func quoteLiteral(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
}

// tailLines 取文本末尾若干行，用于把子进程（pg_dump/pg_restore）的报错缩到可读。
func tailLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\r\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// listHas 判断字符串切片里有没有某个值（本包最小的集合判断，别引入新依赖）。
func listHas(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func containsInt(list []int, want int) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

