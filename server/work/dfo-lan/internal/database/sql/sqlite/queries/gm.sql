-- SQLite query fork of sql/postgres/queries/gm.sql (dual-engine, S3).
--
-- The PostgreSQL file stays untouched and remains authoritative for the PG path.
--
-- Dialect rules applied:
--   * Parameters bound to a column are left bare so they inherit the column width
--     (gm_mail.to_account_id BIGINT -> int64, to_character_id INTEGER -> int32,
--     template/amount BIGINT -> int64). CAST(... AS INTEGER) is deliberately not
--     added: it would always produce int64 and lose the schema widths.
--   * ListGMMail's two sentinels (account_id <= 0 and status = '') carry an
--     explicit CAST: a parameter compared only with a literal infers interface{}
--     (measured), so CAST(... AS INTEGER) / CAST(... AS TEXT) reproduce the
--     PostgreSQL ::bigint / ::text widths.
--   * INSERT ... RETURNING and UPDATE ... RETURNING are supported (>= 3.35).
--   * No locking clause in this file.

-- name: SendGMMail :one
INSERT INTO gm_mail(to_account_id,to_character_id,template,amount,title,body)
VALUES(sqlc.arg(account_id),sqlc.arg(character_id),sqlc.arg(template),sqlc.arg(amount),sqlc.arg(title),sqlc.arg(body)) RETURNING id;

-- name: ListGMMail :many
-- CAST is required on both sentinels: a parameter compared only with the literal
-- 0 (or with '') infers interface{} (measured).
SELECT id,to_account_id,to_character_id,template,amount,title,status,created_at
FROM gm_mail
WHERE (CAST(sqlc.arg(account_id) AS INTEGER) <= 0 OR to_account_id=CAST(sqlc.arg(account_id) AS INTEGER))
AND (CAST(sqlc.arg(status) AS TEXT)='' OR status=CAST(sqlc.arg(status) AS TEXT))
ORDER BY id DESC LIMIT 200;

-- name: RevokeGMMail :one
UPDATE gm_mail SET status='revoked'
WHERE id=sqlc.arg(id) AND status IN ('unread','read') RETURNING id;
