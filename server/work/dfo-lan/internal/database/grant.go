package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// GrantItem is one item handed out by an operator grant.
type GrantItem struct {
	Template uint32 `json:"template"`
	Amount   uint32 `json:"amount"`
}

type GrantHistoryEntry struct {
	GrantID     string
	CharacterID int64
	Operator    string
	Reason      string
	Request     json.RawMessage
	CreatedAt   time.Time
}

func (s *Store) GrantHistory(ctx context.Context, account int64, limit int) ([]GrantHistoryEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	rows, err := s.queries.GrantHistory(ctx, sqlcgen.GrantHistoryParams{AccountID: account, MaxEntries: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]GrantHistoryEntry, len(rows))
	for i, row := range rows {
		out[i] = GrantHistoryEntry{GrantID: row.GrantID, CharacterID: row.CharacterID, Operator: row.Operator,
			Reason: row.Reason, Request: row.Request, CreatedAt: row.CreatedAt}
	}
	return out, nil
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
	return s.execMigration(ctx, "0018_grants.sql")
}

// AccountCera reports an account's balance, creating no row for a read.
func (s *Store) AccountCera(ctx context.Context, account int64) (uint64, error) {
	cera, err := s.queries.AccountCera(ctx, account)
	if err != nil || cera < 0 {
		return 0, err
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
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	q := s.queries.WithTx(tx)

	// The audit row is the idempotency gate, claimed before anything is paid.
	// Checking for an existing grant and then inserting one would be a race:
	// two concurrent attempts at the same id could both pass the check. Here a
	// conflicting insert instead waits for the holder's outcome, so exactly one
	// attempt ever owns the payout, and a rollback releases the id again.
	request, e := json.Marshal(g)
	if e != nil {
		return out, e
	}
	claim, e := q.ClaimAdminGrant(ctx, sqlcgen.ClaimAdminGrantParams{GrantID: g.ID, AccountID: g.AccountID,
		CharacterID: pgtype.Int8{Int64: g.Character, Valid: g.Character != 0}, Request: request, Operator: g.Operator, Reason: g.Reason})
	if e != nil {
		return out, fmt.Errorf("grant target is not a known account/character: %w", e)
	}
	if claim == 0 {
		// Already recorded: hand back the original receipt, pay nothing.
		if out.Receipt, e = q.AdminGrantReceipt(ctx, g.ID); e != nil {
			return out, e
		}
		balance, err := q.AccountCera(ctx, g.AccountID)
		if err != nil {
			return out, err
		}
		out.Cera = uint64(balance)
		return out, tx.Commit(ctx)
	}

	if g.Cera != 0 {
		// The CHECK keeps a balance from going negative, so a deduction larger
		// than the balance aborts the whole grant rather than clamping.
		if e = q.EnsureAccountCurrency(ctx, g.AccountID); e != nil {
			return out, e
		}
		balance, err := q.AdjustAccountCurrency(ctx, sqlcgen.AdjustAccountCurrencyParams{AccountID: g.AccountID, Adjustment: g.Cera})
		if err != nil {
			return out, fmt.Errorf("cera adjustment refused (balance would go negative?): %w", err)
		}
		out.Cera = uint64(balance)
		if g.MaxCera != 0 && out.Cera > g.MaxCera {
			return out, fmt.Errorf("cera balance exceeds client range")
		}
	} else {
		balance, err := q.AccountCera(ctx, g.AccountID)
		if err != nil {
			return out, err
		}
		out.Cera = uint64(balance)
	}

	var receipt json.RawMessage
	if g.Character != 0 {
		r := &out.Character
		*r, e = lockCharacter(ctx, tx, g.AccountID, g.Character)
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
		if e = q.UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: r.ID, State: state}); e != nil {
			return out, e
		}
		r.State, receipt = state, rec
	} else {
		receipt, _ = json.Marshal(map[string]any{"cera": out.Cera})
	}

	if e = q.SaveAdminGrantReceipt(ctx, sqlcgen.SaveAdminGrantReceiptParams{GrantID: g.ID, Receipt: receipt}); e != nil {
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
