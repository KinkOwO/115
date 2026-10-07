package database

import (
	"bytes"
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"os"
	"path/filepath"
)

type Config struct {
	// Driver names the storage engine. SQLite is the only engine since 2026-10-05 (owner
	// decision, see root AGENTS.md §0.6); the field is still accepted so a profile that
	// spells out "sqlite" keeps working unchanged. A profile that asks for the removed
	// engine is refused by EngineForConfig rather than silently opened as SQLite.
	Driver         string `json:"driver,omitempty"`
	MaxConnections int32  `json:"max_connections"`
	// SQLitePath and SQLiteBusyTimeoutMS apply to the SQLite engine.
	SQLitePath          string `json:"sqlite_path,omitempty"`
	SQLiteBusyTimeoutMS int32  `json:"sqlite_busy_timeout_ms,omitempty"`
	// BusyTimeoutMS and MaxReadConnections are the same two settings under the names the
	// 20261004 upgrade package writes. Accepting both keeps one local.json usable by
	// either implementation, which matters because the two are cross-validated against
	// each other rather than being one code path.
	BusyTimeoutMS      int32 `json:"busy_timeout_ms,omitempty"`
	MaxReadConnections int32 `json:"max_read_connections,omitempty"`
}

// LoadConfig reads a storage configuration.
//
// The byte-order mark is stripped on purpose. Windows editors write it by default, and a
// PostgreSQL profile has to be hand-edited (its DSN carries the credentials), so that is
// exactly the file most likely to carry one. Everything else in the chain already tolerates
// it - launch_local.py reads with utf-8-sig, internal/launcher strips it - so refusing it
// here meant the launcher started PostgreSQL while the gateway died before touching it with
// `invalid character 'ï' looking for beginning of value`: the report reaches the owner as
// "pgsql 端无法登录" (2026-10-05).
func LoadConfig(path string) (Config, error) {
	var c Config
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	b = bytes.TrimPrefix(b, utf8BOM)
	if e = json.Unmarshal(b, &c); e != nil {
		return c, fmt.Errorf("storage config %s: %w", path, e)
	}
	return c, nil
}

// utf8BOM is the byte-order mark Windows editors put in front of UTF-8 configuration.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type Store struct {
	// engine owns the connection and transactions; queries is the engine's query
	// surface, kept as a field so the ~200 existing s.queries.X(...) call sites stay
	// untouched. Nothing above this type knows which engine is underneath.
	engine           engine
	queries          querySet
	adventureEnabled bool
}

// ErrNotFound keeps optional persistence results independent of the SQL driver.
var ErrNotFound = errors.New("stored record not found")

// storageError maps "the query found no row" onto the package's ErrNotFound.
//
// Both sentinels are recognised because the two engines report absence differently: pgx
// returns pgx.ErrNoRows while database/sql returns sql.ErrNoRows. Recognising only the
// former made every "look the row up; if it is absent, carry on" path fail hard on SQLite -
// which is exactly how equipping gear and accepting quests stopped working.
//
// Callers must not test the driver sentinels themselves: by the time an error leaves the
// adapter it is ErrNotFound on SQLite and pgx.ErrNoRows on PostgreSQL. Ask isNoRows.
func storageError(err error) error {
	if err == nil {
		return nil
	}
	if driverNoRows(err) {
		return ErrNotFound
	}
	return err
}

// Driver names accepted by Config.Driver.
const (
	// DriverPostgres is the engine that was removed on 2026-10-05 (owner decision, see root
	// AGENTS.md §0.6). The name is kept only so a configuration or profile that still asks
	// for it is *recognised and refused with a clear message* instead of being silently
	// opened as SQLite - which would point the server at a different database and reach the
	// player as "my save is gone".
	DriverPostgres = "postgres"
	DriverSQLite   = "sqlite"
)

// EngineForConfig decides which engine a configuration selects. It is the single rule for
// this repository, and the launchers mirror it (internal/launcher.StorageConfig.DriverName
// and the storage profiles under scripts/): the launcher and the server must never disagree,
// or a player's save looks empty and their login fails.
//
// SQLite is the only engine since 2026-10-05, so:
//  1. an explicit driver wins - "sqlite" is accepted, "postgres" is refused with the removal
//     notice, anything else is an unknown driver;
//  2. a configuration that names no driver falls back to **SQLite** (2026-10-05 业主口径
//     「默认 sqlite」，与启动器 internal/config.StorageDriver 的兜底一致)；SQLite 那条打开
//     路径会以 "sqlite storage configuration incomplete" 明确报错，不会凭空发明一个库。
//
// A leftover postgres_dsn in an old profile is ignored (the field is gone), so an existing
// local.json still loads; that is deliberately different from a profile that *asks* for the
// removed engine, which is refused rather than quietly downgraded.
func EngineForConfig(c Config) (string, error) {
	if driver := strings.ToLower(strings.TrimSpace(c.Driver)); driver != "" {
		switch driver {
		case DriverSQLite:
			return driver, nil
		case DriverPostgres:
			return "", fmt.Errorf("PostgreSQL support was removed (2026-10-05, see root AGENTS.md §0.6); " +
				"the storage configuration must name sqlite_path")
		default:
			return "", fmt.Errorf("unknown storage driver %q", c.Driver)
		}
	}
	return DriverSQLite, nil
}

// Open builds the Store for the configured engine (SQLite is the only one).
//
// The SQLite path applies every schema section up front, which is why the per-domain
// Migrate* entry points have nothing left to do for that engine (see execMigration).
func Open(ctx context.Context, c Config) (*Store, error) {
	driver, err := EngineForConfig(c)
	if err != nil {
		return nil, err
	}
	if driver != DriverSQLite {
		return nil, fmt.Errorf("unsupported storage engine %q", driver)
	}
	return openSQLiteStore(ctx, c)
}

// openSQLiteStore opens (creating if needed) a SQLite database, applies every schema
// section and wraps it in the SQLite engine. Migrations run here rather than at each
// Migrate* call site because SQLite has no per-domain ordered walk: the ledger makes
// the whole application idempotent, and the startup path's calls become no-ops.
func openSQLiteStore(ctx context.Context, c Config) (*Store, error) {
	if strings.TrimSpace(c.SQLitePath) == "" {
		return nil, errors.New("sqlite storage configuration incomplete")
	}
	// The path must be absolute. A relative one would resolve against whatever working
	// directory the server happened to inherit (the launcher uses server/), so the save
	// would land somewhere other than the path suggests - silently, because nothing errors.
	if !filepath.IsAbs(strings.TrimSpace(c.SQLitePath)) {
		return nil, fmt.Errorf(
			"sqlite_path must be an absolute path; %q would resolve against the server's working directory",
			c.SQLitePath)
	}
	// Either naming pair is honoured, so one local.json serves both implementations.
	maxConns := int(c.MaxReadConnections)
	if maxConns <= 0 {
		maxConns = int(c.MaxConnections)
	}
	if maxConns <= 0 {
		maxConns = 4
	}
	busyTimeout := int(c.SQLiteBusyTimeoutMS)
	if busyTimeout <= 0 {
		busyTimeout = int(c.BusyTimeoutMS)
	}
	if busyTimeout <= 0 {
		busyTimeout = 5000
	}
	db, err := openSQLite(ctx, c.SQLitePath, maxConns, busyTimeout)
	if err != nil {
		return nil, err
	}
	// Stamp the same save identity the 20261004 backend requires, so a database this
	// implementation creates is accepted by that one as a DFO save rather than rejected as
	// a foreign file. The reverse direction needs no work: an existing save is opened
	// whether or not it carries the stamp.
	if err := stampSQLiteSaveIdentity(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateSQLiteAll(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	eng := newSQLiteEngine(db)
	return &Store{engine: eng, queries: eng.queries()}, nil
}

func (s *Store) NameExists(ctx context.Context, name string) (bool, error) {
	return s.queries.NameExists(ctx, name)
}
func (s *Store) Close() { s.engine.close() }
func (s *Store) Migrate(ctx context.Context) error {
	return s.execMigration(ctx, "0001_core.sql")
}
func (s *Store) DevelopmentAccount(ctx context.Context, name string) (int64, error) {
	id, err := s.queries.DevelopmentAccount(ctx, name)
	return id, storageError(err)
}

// Character aliases the character-owned aggregate during migration.
type Character = character.Character

func (s *Store) CreateCharacter(ctx context.Context, c Character, maxCharacters int) (Character, error) {
	if maxCharacters < 1 || maxCharacters > 65534 || c.Name == "" || c.ConfigVersion == "" || !json.Valid(c.State) {
		return c, errors.New("invalid character or missing initial configuration")
	}
	tx, e := s.engine.begin(ctx)
	if e != nil {
		return c, e
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	account, e := queries.LockAccount(ctx, c.AccountID)
	if e != nil {
		return c, e
	}
	allocation, e := queries.CharacterAllocation(ctx, account)
	if e != nil {
		return c, e
	}
	if allocation.ActiveCount >= int64(maxCharacters) || allocation.NextWireID > 65534 {
		return c, errors.New("character slots full")
	}
	c.WireID = uint16(allocation.NextWireID)
	c.FixedSlot = 0
	created, e := queries.CreateCharacter(ctx, sqlcgen.CreateCharacterParams{
		AccountID: account, WireID: allocation.NextWireID, Name: c.Name, Profession: int32(c.Profession),
		CreateRequest: c.Request, ConfigVersion: c.ConfigVersion, State: c.State, RosterOrder: allocation.NextRosterOrder,
	})
	if e != nil {
		return c, e
	}
	c.ID, c.CreatedAt = created.ID, created.CreatedAt
	if e = tx.commit(ctx); e != nil {
		return c, e
	}
	return c, nil
}
func (s *Store) Characters(ctx context.Context, account int64) ([]Character, error) {
	out := []Character{}
	if s.adventureEnabled {
		// 角色列表和重新选角使用最新账号迷雾阶段；历史角色快照不是进度真源。
		rows, err := s.queries.CharactersWithAdventure(ctx, account)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, storedCharacter(sqlcgen.CharactersRow(row)))
		}
		return out, nil
	}
	rows, e := s.queries.Characters(ctx, account)
	if e != nil {
		return nil, e
	}
	for _, row := range rows {
		out = append(out, storedCharacter(row))
	}
	return out, nil
}

// Keep generated types and database integer widths within storage. The schema
// constrains wire_id, profession and fixed_slot to their domain ranges.
func storedCharacter(row sqlcgen.CharactersRow) Character {
	return Character{
		ID: row.ID, AccountID: row.AccountID, WireID: uint16(row.WireID), Name: row.Name,
		Profession: byte(row.Profession), Request: row.CreateRequest, ConfigVersion: row.ConfigVersion,
		State: row.State, CreatedAt: row.CreatedAt, FixedSlot: byte(row.FixedSlot),
	}
}

// This shared projection maps database widths to the domain aggregate. Driver
// and generated row types stay within storage; the supplied transaction owns
// the character lock until the caller commits or rolls back.
func lockCharacter(ctx context.Context, tx txHandle, account, id int64) (Character, error) {
	row, err := tx.queries().LockCharacter(ctx, sqlcgen.LockCharacterParams{AccountID: account, CharacterID: id})
	return storedCharacter(sqlcgen.CharactersRow(row)), err
}

type Account struct {
	ID       int64
	Username string
}

func (s *Store) Accounts(ctx context.Context) ([]Account, error) {
	rows, err := s.queries.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	accounts := make([]Account, len(rows))
	for i, row := range rows {
		accounts[i] = Account{ID: row.ID, Username: row.Username}
	}
	return accounts, nil
}

func (s *Store) DatabaseName(ctx context.Context) (string, error) {
	return s.queries.DatabaseName(ctx)
}

// AdminCharacters preserves the GM view's wire-ID ordering and raw save. It
// deliberately does not apply the gameplay roster's adventure projection.
func (s *Store) AdminCharacters(ctx context.Context, account int64) ([]Character, error) {
	rows, err := s.queries.AdminCharacters(ctx, account)
	if err != nil {
		return nil, err
	}
	roles := make([]Character, len(rows))
	for i, row := range rows {
		roles[i] = storedCharacter(sqlcgen.CharactersRow(row))
	}
	return roles, nil
}

func (s *Store) AdminCharacter(ctx context.Context, id int64) (Character, error) {
	row, err := s.queries.AdminCharacter(ctx, id)
	return storedCharacter(sqlcgen.CharactersRow(row)), storageError(err)
}

func (s *Store) DevelopmentCharacterAccount(ctx context.Context, id int64) (int64, error) {
	account, err := s.queries.DevelopmentCharacterAccount(ctx, id)
	return account, storageError(err)
}
