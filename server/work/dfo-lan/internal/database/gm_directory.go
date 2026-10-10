package database

import (
	"context"
	"database/sql"
	"dfolan/internal/database/sqlcgensqlite"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type GMDirectoryCharacter struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Level      uint16 `json:"level"`
	Profession int16  `json:"profession"`
}
type GMDirectoryAccount struct {
	ID         int64                  `json:"id"`
	Username   string                 `json:"username"`
	Characters []GMDirectoryCharacter `json:"characters"`
}

// This reader has no migrations, write lease, catalog preparation or writable
// Store. mode=ro includes the live WAL; immutable=1 would hide recent saves.
func openGMDirectory(cfg Config) (*sql.DB, error) {
	if driver, err := EngineForConfig(cfg); err != nil || driver != DriverSQLite {
		return nil, fmt.Errorf("player directory requires SQLite")
	}
	if !filepath.IsAbs(cfg.SQLitePath) {
		return nil, fmt.Errorf("player directory path must be absolute")
	}
	if info, err := os.Stat(cfg.SQLitePath); err != nil || info.IsDir() {
		return nil, fmt.Errorf("player directory database does not exist")
	}
	db, err := sql.Open("sqlite", sqliteFileURI(cfg.SQLitePath)+"?mode=ro&_pragma=query_only(1)&_busy_timeout=1500&_timezone=UTC&_inttotime=1&_time_integer_format=unix_micro&_dqs=0")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, nil
}

// ReadGMDirectory exposes only account/character display fields from one read
// transaction. Password hashes, raw saves and deleted characters never leave it.
func ReadGMDirectory(ctx context.Context, cfg Config) ([]GMDirectoryAccount, error) {
	db, err := openGMDirectory(cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	q := sqlcgensqlite.New(tx)
	accounts, err := q.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]GMDirectoryAccount, 0, len(accounts))
	for _, a := range accounts {
		row := GMDirectoryAccount{ID: a.ID, Username: a.Username, Characters: []GMDirectoryCharacter{}}
		roles, err := q.AdminCharacters(ctx, a.ID)
		if err != nil {
			return nil, err
		}
		for _, r := range roles {
			var state struct {
				Level uint16 `json:"level"`
			}
			if err = json.Unmarshal(r.State, &state); err != nil {
				return nil, fmt.Errorf("character %d level: %w", r.ID, err)
			}
			row.Characters = append(row.Characters, GMDirectoryCharacter{ID: r.ID, Name: r.Name, Level: state.Level, Profession: int16(r.Profession)})
		}
		out = append(out, row)
	}
	return out, nil
}
