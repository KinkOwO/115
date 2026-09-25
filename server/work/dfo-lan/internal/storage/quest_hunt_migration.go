package storage

// The first single-target hunt candidate used a quest-specific model name.
// Preserve every accepted save while switching to the source-shape model.
const migrateSingleHuntProgressSQL = `
 INSERT INTO character_quest_repairs(character_id,quest_id,reason,before_state,after_state)
 SELECT q.character_id,q.quest_id,'single-hunt-model-v1',to_jsonb(q),
        jsonb_set(to_jsonb(q),'{progress_model}','"single-hunt-enemy-remaining-v1"'::jsonb)
 FROM character_quests q
 WHERE q.status='accepted' AND q.progress BETWEEN 0 AND 1
   AND q.progress_model='antber-3526-hunt-enemy-remaining-v1'
 ON CONFLICT (character_id,quest_id,reason) DO NOTHING;

 UPDATE character_quests
 SET progress_model='single-hunt-enemy-remaining-v1'
 WHERE status='accepted' AND progress BETWEEN 0 AND 1
   AND progress_model='antber-3526-hunt-enemy-remaining-v1';
`
