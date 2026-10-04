-- name: MailRecipient :one
SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,
0::smallint AS fixed_slot FROM characters WHERE lower(name)=lower(sqlc.arg(name)::text) AND deleted_at IS NULL;

-- name: MailRecipientID :one
SELECT id FROM characters WHERE lower(name)=lower(sqlc.arg(name)::text) AND deleted_at IS NULL;

-- name: LockMailCharacters :many
SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at,
0::smallint AS fixed_slot FROM characters WHERE id=ANY(sqlc.arg(character_ids)::bigint[])
AND deleted_at IS NULL ORDER BY id FOR UPDATE;

-- name: MailSendReceipt :one
SELECT outcome FROM character_events WHERE character_id=sqlc.arg(character_id)
AND event_key=sqlc.arg(event_key) AND model='mail-send-v1';

-- name: MailboxDeliveryState :one
SELECT coalesce(max(m.id),0)::bigint AS latest_id,count(*) FILTER (WHERE m.status=1) AS unread
FROM character_mail m JOIN characters c ON c.id=m.recipient_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL
AND m.deleted_at IS NULL AND m.expires_at>now();

-- name: Mailbox :many
SELECT m.id,coalesce(m.sender_id,0)::bigint AS sender_id,m.recipient_id,m.sender_name,
m.body,m.status,m.assets,m.expires_at,(m.deleted_at IS NOT NULL)::boolean AS deleted
FROM character_mail m JOIN characters c ON c.id=m.recipient_id
WHERE c.account_id=sqlc.arg(account_id) AND c.id=sqlc.arg(character_id) AND c.deleted_at IS NULL
AND m.deleted_at IS NULL AND (m.expires_at>now() OR m.status=3) ORDER BY m.id;

-- name: LockMailbox :many
SELECT m.id,coalesce(m.sender_id,0)::bigint AS sender_id,m.recipient_id,m.sender_name,
m.body,m.status,m.assets,m.expires_at,(m.deleted_at IS NOT NULL)::boolean AS deleted
FROM character_mail m WHERE recipient_id=sqlc.arg(recipient_id)
AND (sqlc.arg(include_deleted)::boolean OR (deleted_at IS NULL AND (expires_at>now() OR status=3)))
AND (sqlc.narg(message_ids)::bigint[] IS NULL OR id=ANY(sqlc.narg(message_ids)::bigint[])) ORDER BY id FOR UPDATE;

-- name: MailboxCapacity :one
SELECT count(*) AS messages,coalesce(sum((SELECT count(*) FROM jsonb_array_elements(m.assets) a
WHERE NOT coalesce((a->>'claimed')::boolean,false))),0)::bigint AS unclaimed_assets
FROM character_mail m WHERE recipient_id=sqlc.arg(recipient_id) AND deleted_at IS NULL AND (expires_at>now() OR status=3);

-- name: NextMailID :one
SELECT nextval('mailbox_id_seq')::bigint AS id;

-- name: InsertPlayerMail :one
INSERT INTO character_mail(sender_id,recipient_id,sender_name,body,assets,expires_at)
VALUES(sqlc.arg(sender_id),sqlc.arg(recipient_id),sqlc.arg(sender_name),sqlc.arg(body),sqlc.arg(assets),now()+interval '15 days') RETURNING id;

-- name: InsertSystemMail :one
INSERT INTO character_mail(recipient_id,sender_name,body,assets,expires_at)
VALUES(sqlc.arg(recipient_id),sqlc.arg(sender_name),sqlc.arg(body),sqlc.arg(assets),now()+interval '15 days') RETURNING id;

-- name: UpdateMail :execrows
UPDATE character_mail SET assets=sqlc.arg(assets),status=sqlc.arg(status)::integer,
deleted_at=CASE WHEN sqlc.arg(deleted)::boolean THEN now() ELSE NULL END
WHERE id=sqlc.arg(mail_id) AND recipient_id=sqlc.arg(recipient_id) AND deleted_at IS NULL;
