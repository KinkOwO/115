package database

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"os"
)

type Config struct {
	PostgresDSN    string `json:"postgres_dsn"`
	MaxConnections int32  `json:"max_connections"`
	PostgresSchema string `json:"postgres_schema,omitempty"`
}

func LoadConfig(path string) (Config, error) {
	var c Config
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	return c, e
}

type Store struct {
	db               *pgxpool.Pool
	queries          *sqlcgen.Queries
	adventureEnabled bool
}

// ErrNotFound keeps optional persistence results independent of the SQL driver.
var ErrNotFound = errors.New("stored record not found")

func storageError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func Open(ctx context.Context, c Config) (*Store, error) {
	if c.PostgresDSN == "" {
		return nil, errors.New("storage configuration incomplete")
	}
	cfg, e := pgxpool.ParseConfig(c.PostgresDSN)
	if e != nil {
		return nil, errors.New("invalid PostgreSQL configuration")
	}
	if c.MaxConnections > 0 {
		cfg.MaxConns = c.MaxConnections
	}
	if c.PostgresSchema != "" {
		cfg.ConnConfig.RuntimeParams["search_path"] = c.PostgresSchema
	}
	db, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		return nil, e
	}
	s := &Store{db: db, queries: sqlcgen.New(db)}
	if e = db.Ping(ctx); e != nil {
		s.Close()
		return nil, e
	}
	return s, nil
}

func (s *Store) NameExists(ctx context.Context, name string) (bool, error) {
	return s.queries.NameExists(ctx, name)
}
func (s *Store) Close() { s.db.Close() }
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
	tx, e := s.db.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return c, e
	}
	defer tx.Rollback(ctx)
	queries := s.queries.WithTx(tx)
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
	if e = tx.Commit(ctx); e != nil {
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
func lockCharacter(ctx context.Context, tx pgx.Tx, account, id int64) (Character, error) {
	row, err := sqlcgen.New(tx).LockCharacter(ctx, sqlcgen.LockCharacterParams{AccountID: account, CharacterID: id})
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
