-- Run against an existing initialized server database, after stopping game
-- sessions and backing up characters. Audits the entire original state in the
-- existing character_events table; does not change schema/config_version.
BEGIN;
SET LOCAL lock_timeout = '5s';
LOCK TABLE characters IN SHARE ROW EXCLUSIVE MODE;

-- Preserve ordinary shortcuts instead of silently creating duplicate slots.
DO $$
BEGIN
 IF EXISTS (
  SELECT 1 FROM characters c,
   LATERAL jsonb_each(CASE WHEN jsonb_typeof(c.state->'skill_slots'->0)='object'
     THEN c.state->'skill_slots'->0 ELSE '{}'::jsonb END) kv
  WHERE c.profession=9 AND c.deleted_at IS NULL
   AND NOT EXISTS (SELECT 1 FROM character_events e WHERE e.character_id=c.id
     AND e.event_key='dark-knight-combo-slots-v1')
   AND kv.key NOT IN ('118','119','120','121','122','123')
   AND kv.value IN ('0'::jsonb,'1'::jsonb,'2'::jsonb,'3'::jsonb,'4'::jsonb,'5'::jsonb)
 ) THEN
  RAISE EXCEPTION 'Ordinary skills occupy ASDFGH; preserve and relocate them before retrying';
 END IF;
END $$;

WITH target AS MATERIALIZED (
 SELECT c.id,c.config_version,c.state AS before_state,
  jsonb_set(c.state,'{skill_slots,0}',
   (c.state->'skill_slots'->0) || '{"118":0,"119":1,"120":2,"121":3,"122":4,"123":5}'::jsonb
  ) AS after_state
 FROM characters c
 WHERE c.profession=9 AND c.deleted_at IS NULL
  AND jsonb_typeof(c.state->'skill_slots')='array'
  AND jsonb_typeof(c.state->'skill_slots'->0)='object'
  AND NOT ((c.state->'skill_slots'->0) @> '{"118":0,"119":1,"120":2,"121":3,"122":4,"123":5}'::jsonb)
  AND NOT EXISTS (SELECT 1 FROM character_events e WHERE e.character_id=c.id
    AND e.event_key='dark-knight-combo-slots-v1')
 FOR UPDATE OF c
), audit AS (
 INSERT INTO character_events(character_id,event_key,config_version,model,outcome)
 SELECT id,'dark-knight-combo-slots-v1',config_version,'dark-knight-combo-slots-v1',
  jsonb_build_object('before_state',before_state,'after_state',after_state)
 FROM target
 RETURNING character_id
)
UPDATE characters c SET state=t.after_state
FROM target t JOIN audit a ON a.character_id=t.id
WHERE c.id=t.id
RETURNING c.id,c.state->'skill_slots'->0 AS corrected_slots;

COMMIT;
