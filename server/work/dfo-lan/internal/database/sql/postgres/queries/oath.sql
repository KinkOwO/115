-- name: EquippedOathSelection :one
SELECT selected_option FROM character_oath_options WHERE character_id=sqlc.arg(character_id) AND core_instance_key=sqlc.arg(core_instance_key);

-- name: SelectEquippedOathOption :exec
INSERT INTO character_oath_options(character_id,core_instance_key,selected_option,revision,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(core_instance_key),sqlc.arg(selected_option)::bigint,1,now())
ON CONFLICT(character_id,core_instance_key) DO UPDATE SET selected_option=EXCLUDED.selected_option,
revision=character_oath_options.revision+1,updated_at=now();

-- name: OathOption :one
SELECT character_id,core_instance_key,selected_option,revision FROM character_oath_options
WHERE character_id=sqlc.arg(character_id) AND core_instance_key=sqlc.arg(core_instance_key);

-- name: LockOathOptionRevision :one
SELECT revision FROM character_oath_options WHERE character_id=sqlc.arg(character_id) AND core_instance_key=sqlc.arg(core_instance_key) FOR UPDATE;

-- name: InsertOathOption :exec
INSERT INTO character_oath_options(character_id,core_instance_key,selected_option,revision,updated_at)
VALUES(sqlc.arg(character_id),sqlc.arg(core_instance_key),sqlc.arg(selected_option)::bigint,sqlc.arg(revision),now());

-- name: UpdateOathOption :exec
UPDATE character_oath_options SET selected_option=sqlc.arg(selected_option)::bigint,revision=sqlc.arg(revision),updated_at=now()
WHERE character_id=sqlc.arg(character_id) AND core_instance_key=sqlc.arg(core_instance_key);
