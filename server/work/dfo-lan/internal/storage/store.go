package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"os"
	"time"
)

type Config struct {
	PostgresDSN    string `json:"postgres_dsn"`
	RedisAddress   string `json:"redis_address"`
	RedisPassword  string `json:"redis_password"`
	RedisPrefix    string `json:"redis_prefix"`
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
	DB               *pgxpool.Pool
	Cache            *redis.Client
	prefix           string
	adventureEnabled bool
}

func Open(ctx context.Context, c Config) (*Store, error) {
	if c.PostgresDSN == "" || c.RedisAddress == "" || c.RedisPassword == "" || c.RedisPrefix == "" {
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
	cache := redis.NewClient(&redis.Options{Addr: c.RedisAddress, Password: c.RedisPassword, DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second})
	s := &Store{DB: db, Cache: cache, prefix: c.RedisPrefix}
	if e = db.Ping(ctx); e != nil {
		s.Close()
		return nil, e
	}
	if e = cache.Ping(ctx).Err(); e != nil {
		s.Close()
		return nil, e
	}
	return s, nil
}

func (s *Store) NameExists(ctx context.Context, name string) (bool, error) {
	var exists bool
	err := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM characters WHERE lower(name)=lower($1))`, name).Scan(&exists)
	return exists, err
}
func (s *Store) Close() { s.Cache.Close(); s.DB.Close() }
func (s *Store) Migrate(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS accounts (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 username text NOT NULL UNIQUE,
 password_hash text,
 development_only boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(development_only OR password_hash IS NOT NULL)
 );
 CREATE TABLE IF NOT EXISTS characters (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 account_id bigint NOT NULL REFERENCES accounts(id),
 wire_id integer NOT NULL CHECK(wire_id BETWEEN 1 AND 65534),
 name text NOT NULL,
 profession integer NOT NULL CHECK(profession BETWEEN 0 AND 255),
 create_request bytea NOT NULL,
 config_version text NOT NULL,
 state jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(account_id,wire_id));
 ALTER TABLE characters ADD COLUMN IF NOT EXISTS deleted_at timestamptz;
 ALTER TABLE characters ADD COLUMN IF NOT EXISTS max_fame integer NOT NULL DEFAULT 0 CHECK(max_fame >= 0);
 ALTER TABLE characters ADD COLUMN IF NOT EXISTS roster_order bigint CHECK(roster_order > 0);
 ALTER TABLE characters ADD COLUMN IF NOT EXISTS fixed_slot smallint NOT NULL DEFAULT 0 CHECK(fixed_slot BETWEEN 0 AND 255);
 CREATE UNIQUE INDEX IF NOT EXISTS characters_name_unique ON characters(lower(name));
 CREATE TABLE IF NOT EXISTS npc_favor (
 character_id bigint NOT NULL REFERENCES characters(id),
 npc_id bigint NOT NULL,
 point bigint NOT NULL DEFAULT 0,
 daily_count integer NOT NULL DEFAULT 0,
 last_gift_day date,
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,npc_id));`)
	return e
}
func (s *Store) DevelopmentAccount(ctx context.Context, name string) (int64, error) {
	var id int64
	e := s.DB.QueryRow(ctx, `INSERT INTO accounts(username,development_only) VALUES($1,true) ON CONFLICT(username) DO UPDATE SET username=EXCLUDED.username WHERE accounts.development_only RETURNING id`, name).Scan(&id)
	return id, e
}

type Character struct {
	ID            int64
	AccountID     int64
	WireID        uint16
	FixedSlot     byte
	Name          string
	Profession    byte
	Request       []byte
	ConfigVersion string
	State         json.RawMessage
	CreatedAt     time.Time
}

func (s *Store) CreateCharacter(ctx context.Context, c Character, maxCharacters int) (Character, error) {
	if maxCharacters < 1 || maxCharacters > 65534 || c.Name == "" || c.ConfigVersion == "" || !json.Valid(c.State) {
		return c, errors.New("invalid character or missing initial configuration")
	}
	tx, e := s.DB.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return c, e
	}
	defer tx.Rollback(ctx)
	var account int64
	if e = tx.QueryRow(ctx, `SELECT id FROM accounts WHERE id=$1 FOR UPDATE`, c.AccountID).Scan(&account); e != nil {
		return c, e
	}
	var n, next int
	var order int64
	if e = tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE deleted_at IS NULL),coalesce(max(wire_id),0)+1,coalesce(max(coalesce(roster_order,wire_id)),0)+1 FROM characters WHERE account_id=$1`, account).Scan(&n, &next, &order); e != nil {
		return c, e
	}
	if n >= maxCharacters || next > 65534 {
		return c, errors.New("character slots full")
	}
	c.WireID = uint16(next)
	c.FixedSlot = 0
	e = tx.QueryRow(ctx, `INSERT INTO characters(account_id,wire_id,name,profession,create_request,config_version,state,roster_order,fixed_slot) VALUES($1,$2,$3,$4,$5,$6,$7,$8,0) RETURNING id,created_at`, account, next, c.Name, c.Profession, c.Request, c.ConfigVersion, c.State, order).Scan(&c.ID, &c.CreatedAt)
	if e != nil {
		return c, e
	}
	if e = tx.Commit(ctx); e != nil {
		return c, e
	}
	// Cache is disposable. A cache failure after commit cannot undo a saved role.
	s.Cache.Del(ctx, fmt.Sprintf("%scharacters:%d", s.prefix, account))
	return c, nil
}
func (s *Store) Characters(ctx context.Context, account int64) ([]Character, error) {
	query := `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,fixed_slot FROM characters WHERE account_id=$1 AND deleted_at IS NULL ORDER BY coalesce(roster_order,wire_id),wire_id`
	if s.adventureEnabled {
		// 角色列表和重新选角使用最新账号迷雾阶段；历史角色快照不是进度真源。
		query = `SELECT c.id,c.account_id,c.wire_id,c.name,c.profession,c.create_request,c.config_version,
 jsonb_set(c.state,'{season_level}',COALESCE(a.data->'season_level','{}'::jsonb),true),c.created_at,c.fixed_slot
 FROM characters c LEFT JOIN account_adventures a ON a.account_id=c.account_id
 WHERE c.account_id=$1 AND c.deleted_at IS NULL ORDER BY coalesce(c.roster_order,c.wire_id),c.wire_id`
	}
	rows, e := s.DB.Query(ctx, query, account)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Character{}
	for rows.Next() {
		var c Character
		if e = rows.Scan(&c.ID, &c.AccountID, &c.WireID, &c.Name, &c.Profession, &c.Request, &c.ConfigVersion, &c.State, &c.CreatedAt, &c.FixedSlot); e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
