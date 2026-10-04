-- SQLite query fork of sql/postgres/queries/mail.sql (dual-engine, S3).
--
-- The PostgreSQL file stays untouched and remains authoritative for the PG path.
--
-- Dialect rules applied (docs/sqlite-query-port-guide.md, D22-D29):
--   * FOR UPDATE is deleted from LockMailCharacters / LockMailbox; SQLite has no
--     row locks and the engine serializes writers with BEGIN IMMEDIATE
--     (_txlock=immediate). Names and predicates are unchanged.
--   * id=ANY(sqlc.arg(ids)::bigint[]) -> id IN (SELECT value FROM
--     json_each(CAST(sqlc.arg(ids) AS TEXT))): the caller passes the id set as a
--     JSON array text (guide section 1, D9). The same mapping is used for the
--     optional sqlc.narg(message_ids) set; CAST is required there so the
--     parameter is a string instead of interface{}.
--   * now() -> (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)),
--     integer microseconds, matching the DDL defaults and the driver's
--     _inttotime=1 + _time_integer_format=unix_micro representation. Every
--     expires_at comparison therefore compares integers.
--   * (x IS NOT NULL)::boolean -> CAST((x IS NOT NULL) AS BOOLEAN) so the
--     projected column stays a Go bool.
--   * jsonb_array_elements(m.assets) -> json_each(m.assets) (JSON1 accepts the
--     BLOB storage class); the element payload is read with
--     json_extract(a.value,'$.claimed'), and NOT coalesce(<that>,0) reproduces
--     "NOT coalesce((a->>'claimed')::boolean,false)" for the JSON booleans the
--     server writes.
--   * Projections and parameters bound to a column are bare, so they keep the
--     schema widths (m.status SMALLINT -> int16, m.assets json -> json.RawMessage,
--     coalesce(m.sender_id,0) -> int64 from sender_id BIGINT). CAST is used only
--     where sqlc would infer interface{}: coalesce(max(m.id),0) and
--     coalesce(sum(...),0) (both bigint in PostgreSQL).
--
-- NOT PORTED: NextMailID (:one) ran nextval('mailbox_id_seq')::bigint. The target
-- schema has no sequence: character_mail.id is INTEGER PRIMARY KEY AUTOINCREMENT
-- (sqlite/migrations/0001_initial.sql, section 0036_mailbox; the PostgreSQL
-- sequence is gone). AUTOINCREMENT allocates the id at INSERT time, so there is no
-- portable statement that pre-allocates it, and a max(id)+1 stand-in would change
-- the "attachment ids are allocated before the message id and share one number
-- space" invariant (design doc D3 / section 6.2). Omitted rather than guessed.
--
-- DEVIATION, reported: InsertPlayerMail / InsertSystemMail compute
-- now()+interval '15 days'. SQLite has no interval type and datetime('now','+15
-- days') would produce TEXT, which cannot live in an integer-microsecond TIMESTAMP
-- column. The interval is therefore expressed as exact integer-microsecond
-- arithmetic (15 days = 1296000000000 us) on top of the same now() expression the
-- rest of the tree uses. If the port wants the expiry computed in Go instead, the
-- term is the only thing to change.

-- name: MailRecipient :one
SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,
0 AS fixed_slot FROM characters WHERE lower(name)=lower(CAST(sqlc.arg(name) AS TEXT)) AND deleted_at IS NULL;

-- name: MailRecipientID :one
SELECT id FROM characters WHERE lower(name)=lower(CAST(sqlc.arg(name) AS TEXT)) AND deleted_at IS NULL;

-- name: LockMailCharacters :many
-- Lock query; FOR UPDATE removed, engine-wide write serialization instead.
SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,
0 AS fixed_slot FROM characters WHERE id IN (SELECT value FROM json_each(CAST(sqlc.arg(character_ids) AS TEXT)))
AND deleted_at IS NULL ORDER BY id;

-- name: MailSendReceipt :one
SELECT outcome FROM character_events WHERE character_id=sqlc.arg(character_id)
AND event_key=sqlc.arg(event_key) AND model='mail-send-v1';

-- name: MailboxDeliveryState :one
SELECT CAST(coalesce(max(m.id),0) AS INTEGER) AS latest_id,
count(*) FILTER (WHERE m.status=1) AS unread
FROM character_mail m JOIN characters c ON c.id=m.recipient_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL
AND m.deleted_at IS NULL AND m.expires_at>(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER));

-- name: Mailbox :many
SELECT m.id,coalesce(m.sender_id,0) AS sender_id,m.recipient_id,m.sender_name,
m.body,m.status,m.assets,m.expires_at,CAST((m.deleted_at IS NOT NULL) AS BOOLEAN) AS deleted
FROM character_mail m JOIN characters c ON c.id=m.recipient_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL
AND m.deleted_at IS NULL AND (m.expires_at>(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) OR m.status=3) ORDER BY m.id;

-- name: LockMailbox :many
-- Lock query; FOR UPDATE removed, engine-wide write serialization instead.
SELECT m.id,coalesce(m.sender_id,0) AS sender_id,m.recipient_id,m.sender_name,
m.body,m.status,m.assets,m.expires_at,CAST((m.deleted_at IS NOT NULL) AS BOOLEAN) AS deleted
FROM character_mail m WHERE recipient_id=sqlc.arg(recipient_id)
AND (CAST(sqlc.arg(include_deleted) AS BOOLEAN) OR (deleted_at IS NULL AND (expires_at>(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) OR status=3)))
AND (CAST(sqlc.narg(message_ids) AS TEXT) IS NULL OR id IN (SELECT value FROM json_each(CAST(sqlc.narg(message_ids) AS TEXT)))) ORDER BY id;

-- name: MailboxCapacity :one
SELECT count(*) AS messages,
CAST(coalesce(sum((SELECT count(*) FROM json_each(m.assets) a
WHERE NOT coalesce(json_extract(a.value,'$.claimed'),0))),0) AS INTEGER) AS unclaimed_assets
FROM character_mail m WHERE recipient_id=sqlc.arg(recipient_id) AND deleted_at IS NULL
AND (expires_at>(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) OR status=3);

-- name: InsertPlayerMail :one
INSERT INTO character_mail(sender_id,recipient_id,sender_name,body,assets,expires_at)
VALUES(sqlc.arg(sender_id),sqlc.arg(recipient_id),sqlc.arg(sender_name),sqlc.arg(body),sqlc.arg(assets),
(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER) + 1296000000000)) RETURNING id;

-- name: InsertSystemMail :one
INSERT INTO character_mail(recipient_id,sender_name,body,assets,expires_at)
VALUES(sqlc.arg(recipient_id),sqlc.arg(sender_name),sqlc.arg(body),sqlc.arg(assets),
(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER) + 1296000000000)) RETURNING id;

-- name: UpdateMail :execrows
UPDATE character_mail SET assets=sqlc.arg(assets),status=sqlc.arg(status),
deleted_at=CASE WHEN CAST(sqlc.arg(deleted) AS BOOLEAN) THEN (CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER)) ELSE NULL END
WHERE id=sqlc.arg(mail_id) AND recipient_id=sqlc.arg(recipient_id) AND deleted_at IS NULL;
