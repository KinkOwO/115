-- name: SendGMMail :one
INSERT INTO gm_mail(to_account_id,to_character_id,template,amount,title,body)
VALUES(sqlc.arg(account_id),sqlc.arg(character_id),sqlc.arg(template),sqlc.arg(amount),sqlc.arg(title),sqlc.arg(body)) RETURNING id;

-- name: ListGMMail :many
SELECT id,to_account_id,to_character_id,template,amount,title,status,created_at
FROM gm_mail
WHERE (sqlc.arg(account_id)::bigint <= 0 OR to_account_id=sqlc.arg(account_id))
AND (sqlc.arg(status)::text='' OR status=sqlc.arg(status))
ORDER BY id DESC LIMIT 200;

-- name: RevokeGMMail :one
UPDATE gm_mail SET status='revoked'
WHERE id=sqlc.arg(id) AND status IN ('unread','read') RETURNING id;
