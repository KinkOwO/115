package database

import (
	"context"
	"fmt"
)

type characterCapacityQueries interface {
	CharacterCapacity(context.Context, int64) (int64, error)
	SaveCharacterCapacity(context.Context, int64, int64) error
}

func effectiveCharacterCapacity(ctx context.Context, q querySet, account int64, base int) (int, error) {
	if account < 1 || base < 1 || base > 65534 {
		return 0, fmt.Errorf("invalid character capacity")
	}
	stored, err := q.CharacterCapacity(ctx, account)
	if err != nil {
		return 0, err
	}
	if int64(base) < stored {
		return int(stored), nil
	}
	return base, nil
}

// AccountCharacterCapacity preserves the operator's base capacity for accounts
// with no purchase record, and retains bought capacity across policy changes.
func (s *Store) AccountCharacterCapacity(ctx context.Context, account int64, base int) (int, error) {
	return effectiveCharacterCapacity(ctx, s.queries, account, base)
}

// ExpandCharacterCapacity runs under the account lock held by the character
// event transaction. Its save, coupon decrement and receipt commit together.
func (tx *Tx) ExpandCharacterCapacity(ctx context.Context, base, amount, limit int) (int, error) {
	capacity, err := effectiveCharacterCapacity(ctx, tx.queries, tx.accountID, base)
	if err != nil {
		return 0, err
	}
	if amount < 1 || limit < 1 || limit > 255 || capacity > limit-amount {
		return 0, fmt.Errorf("角色栏已达到扩展上限")
	}
	allocation, err := tx.queries.CharacterAllocation(ctx, tx.accountID)
	if err != nil {
		return 0, err
	}
	if allocation.ActiveCount < int64(capacity) {
		return 0, fmt.Errorf("已有角色栏尚未占满，不能使用扩展券")
	}
	capacity += amount
	err = tx.queries.SaveCharacterCapacity(ctx, tx.accountID, int64(capacity))
	return capacity, err
}

func (q *sqliteQueries) CharacterCapacity(ctx context.Context, account int64) (int64, error) {
	var capacity int64
	err := q.queryRow(ctx, "SELECT COALESCE((SELECT capacity FROM account_character_slots WHERE account_id=?),0)", account).Scan(&capacity)
	return capacity, storageError(err)
}

func (q *sqliteQueries) SaveCharacterCapacity(ctx context.Context, account, capacity int64) error {
	_, err := q.exec(ctx, "INSERT INTO account_character_slots(account_id,capacity) VALUES(?,?) ON CONFLICT(account_id) DO UPDATE SET capacity=excluded.capacity", account, capacity)
	return storageError(err)
}
