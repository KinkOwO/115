UPDATE characters
SET state = jsonb_set(
  state,
  '{inventory,equipment}',
  (SELECT COALESCE(jsonb_agg(elem), '[]'::jsonb)
   FROM jsonb_array_elements(state->'inventory'->'equipment') elem
   WHERE elem->>'template' <> '63000'),
  false
)
WHERE id = 1;
SELECT jsonb_array_length(state->'inventory'->'equipment') AS equip_rows FROM characters WHERE id = 1;
