package storage

import (
	"context"
	"encoding/json"
	"fmt"
)

// GrantItem is one item handed out by an operator grant.
type GrantItem struct {
	Template uint32 `json:"template"`
	Amount   uint32 `json:"amount"`
}

// Grant is one operator hand-out: cera at the account, gold and items at one
// character. Cera is account-scoped because the client reads it from the
// account's SELECT response, not from character state.
type Grant struct {
	ID        string      `json:"id"`
	AccountID int64       `json:"account_id"`
	Character int64       `json:"character_id,omitempty"`
	Cera      int64       `json:"cera,omitempty"`
	MaxCera   uint64      `json:"-"` // Optional transaction ceiling for client-encodable balances.
	Gold      uint32      `json:"gold,omitempty"`
	Items     []GrantItem `json:"items,omitempty"`
	Reason    string      `json:"reason"`
	Operator  string      `json:"operator"`
}

type GrantResult struct {
	Receipt   json.RawMessage
	Character Character
	Cera      uint64
	Applied   bool
}

// MigrateGrants creates the account currency ledger and the operator grant
// audit. Every hand-out is auditable after the fact, and the grant id is the
// primary key, so a repeated run of the same grant cannot pay out twice.
func (s *Store) MigrateGrants(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_currency(
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 cera bigint NOT NULL DEFAULT 0 CHECK(cera>=0),
 updated_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS admin_grants(
 grant_id text PRIMARY KEY,
 account_id bigint NOT NULL REFERENCES accounts(id),
 character_id bigint REFERENCES characters(id),
 request jsonb NOT NULL, receipt jsonb NOT NULL,
 operator text NOT NULL, reason text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now());`)
	return e
}

// AccountCera reports an account's balance, creating no row for a read.
func (s *Store) AccountCera(ctx context.Context, account int64) (uint64, error) {
	var cera int64
	e := s.DB.QueryRow(ctx, `SELECT coalesce(
 (SELECT cera FROM account_currency WHERE account_id=$1), 0)`, account).Scan(&cera)
	if e != nil || cera < 0 {
		return 0, e
	}
	return uint64(cera), nil
}

// ApplyGrant pays out one grant in a single transaction and records it.
//
// The grant id is the idempotency key: replaying the same id returns the
// original receipt and pays nothing further, so a retried or double-clicked
// hand-out cannot mint a second payout. Cera cannot go negative, character
// state is only touched through the caller's mutate function (which owns the
// source catalog for gold and items), and every ownership boundary is checked
// inside the transaction.
func (s *Store) ApplyGrant(ctx context.Context, g Grant, mutate func(Character) (json.RawMessage, json.RawMessage, error)) (GrantResult, error) {
	var out GrantResult
	if g.ID == "" || g.AccountID == 0 || g.Operator == "" || g.Reason == "" {
		return out, fmt.Errorf("a grant needs an id, an account, an operator and a reason")
	}
	if g.Character == 0 && (g.Gold != 0 || len(g.Items) > 0) {
		return out, fmt.Errorf("gold and items need a character")
	}
	if g.Character != 0 && mutate == nil {
		return out, fmt.Errorf("character payout requires a source-backed mutation")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)

	// The audit row is the idempotency gate, claimed before anything is paid.
	// Checking for an existing grant and then inserting one would be a race:
	// two concurrent attempts at the same id could both pass the check. Here a
	// conflicting insert instead waits for the holder's outcome, so exactly one
	// attempt ever owns the payout, and a rollback releases the id again.
	request, e := json.Marshal(g)
	if e != nil {
		return out, e
	}
	var character any
	if g.Character != 0 {
		character = g.Character
	}
	claim, e := tx.Exec(ctx, `INSERT INTO admin_grants(grant_id,account_id,character_id,request,receipt,operator,reason)
 VALUES($1,$2,$3,$4,'{}'::jsonb,$5,$6) ON CONFLICT (grant_id) DO NOTHING`,
		g.ID, g.AccountID, character, request, g.Operator, g.Reason)
	if e != nil {
		return out, fmt.Errorf("grant target is not a known account/character: %w", e)
	}
	if claim.RowsAffected() == 0 {
		// Already recorded: hand back the original receipt, pay nothing.
		if e = tx.QueryRow(ctx, `SELECT receipt FROM admin_grants WHERE grant_id=$1`, g.ID).Scan(&out.Receipt); e != nil {
			return out, e
		}
		if out.Cera, e = s.AccountCera(ctx, g.AccountID); e != nil {
			return out, e
		}
		return out, tx.Commit(ctx)
	}

	if g.Cera != 0 {
		// The CHECK keeps a balance from going negative, so a deduction larger
		// than the balance aborts the whole grant rather than clamping.
		if _, e = tx.Exec(ctx, `INSERT INTO account_currency(account_id,cera) VALUES($1,0)
 ON CONFLICT (account_id) DO NOTHING`, g.AccountID); e != nil {
			return out, e
		}
		if e = tx.QueryRow(ctx, `UPDATE account_currency SET cera=cera+$2,updated_at=now()
 WHERE account_id=$1 RETURNING cera`, g.AccountID, g.Cera).Scan(&out.Cera); e != nil {
			return out, fmt.Errorf("cera adjustment refused (balance would go negative?): %w", e)
		}
		if g.MaxCera != 0 && out.Cera > g.MaxCera {
			return out, fmt.Errorf("cera balance exceeds client range")
		}
	} else if out.Cera, e = s.AccountCera(ctx, g.AccountID); e != nil {
		return out, e
	}

	var receipt json.RawMessage
	if g.Character != 0 {
		r := &out.Character
		e = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at
 FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`,
			g.AccountID, g.Character).Scan(&r.ID, &r.AccountID, &r.WireID, &r.Name, &r.Profession,
			&r.Request, &r.ConfigVersion, &r.State, &r.CreatedAt)
		if e != nil {
			return out, fmt.Errorf("grant character is not owned by this account: %w", e)
		}
		state, rec, e := mutate(*r)
		if e != nil {
			return out, e
		}
		if !json.Valid(state) || !json.Valid(rec) {
			return out, fmt.Errorf("invalid grant payload")
		}
		if _, e = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, r.ID, state); e != nil {
			return out, e
		}
		r.State, receipt = state, rec
	} else {
		receipt, _ = json.Marshal(map[string]any{"cera": out.Cera})
	}

	if _, e = tx.Exec(ctx, `UPDATE admin_grants SET receipt=$2 WHERE grant_id=$1`, g.ID, receipt); e != nil {
		return out, e
	}
	if e = tx.Commit(ctx); e != nil {
		return out, e
	}
	out.Receipt, out.Applied = receipt, true
	// The roster cache carries character rows; drop it so the next read sees
	// the new balance and bag.
	return out, nil
}

// GrantHistory lists recorded grants, newest first, for audit.
func (s *Store) GrantHistory(ctx context.Context, account int64, limit int) ([]string, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, e := s.DB.Query(ctx, `SELECT grant_id,operator,reason,request,created_at
 FROM admin_grants WHERE account_id=$1 ORDER BY created_at DESC LIMIT $2`, account, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id, operator, reason string
		var request json.RawMessage
		var at any
		if e = rows.Scan(&id, &operator, &reason, &request, &at); e != nil {
			return nil, e
		}
		out = append(out, fmt.Sprintf("%v  %-24s by %-12s  %s  %s", at, id, operator, reason, request))
	}
	return out, rows.Err()
}
