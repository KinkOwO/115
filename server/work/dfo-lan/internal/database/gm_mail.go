package database

import (
	"context"
	"errors"
	"math"
	"time"

	"dfolan/internal/database/sqlcgen"
)

// GMMail is the existing management queue, not an in-game mail delivery.
type GMMail struct {
	ID          int64     `json:"id"`
	AccountID   int64     `json:"to_account_id"`
	CharacterID int64     `json:"to_character_id"`
	Template    int64     `json:"template"`
	Amount      int64     `json:"amount"`
	Title       string    `json:"title"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

// Only the GM entry point initializes this optional administrative table.
func (s *Store) MigrateGMMail(ctx context.Context) error {
	return s.execMigration(ctx, "0037_gm_mail.sql")
}

func (s *Store) SendGMMail(ctx context.Context, account, character, template, amount int64, title, body string) (int64, error) {
	if account <= 0 || template <= 0 || amount <= 0 || amount > math.MaxUint32 {
		return 0, errors.New("to_account_id/template/amount 必须为正数，amount 不得超过 uint32")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	q := s.queries.WithTx(tx)
	if _, err = q.LockAccountState(ctx, account); err != nil {
		return 0, storageError(err)
	}
	if character > 0 {
		owned, err := q.CharacterOwned(ctx, sqlcgen.CharacterOwnedParams{AccountID: account, CharacterID: character})
		if err != nil {
			return 0, err
		}
		if !owned {
			return 0, errors.New("收件角色不存在或不属于收件账号")
		}
	}
	id, err := q.SendGMMail(ctx, sqlcgen.SendGMMailParams{AccountID: account, CharacterID: character, Template: template, Amount: amount, Title: title, Body: body})
	if err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) GMMails(ctx context.Context, account int64, status string) ([]GMMail, error) {
	rows, err := s.queries.ListGMMail(ctx, sqlcgen.ListGMMailParams{AccountID: account, Status: status})
	if err != nil {
		return nil, err
	}
	out := make([]GMMail, 0, len(rows))
	for _, r := range rows {
		out = append(out, GMMail{ID: r.ID, AccountID: r.ToAccountID, CharacterID: r.ToCharacterID, Template: r.Template, Amount: r.Amount, Title: r.Title, Status: r.Status, CreatedAt: r.CreatedAt})
	}
	return out, nil
}

func (s *Store) RevokeGMMail(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("缺少邮件 id")
	}
	_, err := s.queries.RevokeGMMail(ctx, id)
	return storageError(err)
}
