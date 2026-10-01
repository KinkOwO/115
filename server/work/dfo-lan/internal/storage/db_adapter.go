package storage

import (
	"context"
	"dfolan/internal/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// pgxTx 把 pgx.Tx 适配成中立的 db.Tx，供领域在事务回调里使用，
// 避免 pgx 类型出现在领域声明的 Store 接口中。
type pgxTx struct{ tx pgx.Tx }

func (p pgxTx) Exec(ctx context.Context, sql string, args ...any) (db.Result, error) {
	tag, err := p.tx.Exec(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	return pgxResult{tag: tag}, nil
}

func (p pgxTx) QueryRow(ctx context.Context, sql string, args ...any) db.Row {
	return pgxRow{row: p.tx.QueryRow(ctx, sql, args...)}
}

type pgxResult struct{ tag pgconn.CommandTag }

func (r pgxResult) RowsAffected() int64 { return r.tag.RowsAffected() }

type pgxRow struct{ row pgx.Row }

func (r pgxRow) Scan(dest ...any) error { return r.row.Scan(dest...) }
