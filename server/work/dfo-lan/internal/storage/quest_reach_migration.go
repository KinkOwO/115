package storage

// The first 3281 candidate persisted its own model before the generic NPC
// reach decoder replaced it. Translate that historical storage identifier;
// objective decoding remains independent of quest IDs. Keep all player data,
// including progress and timestamps, and record the original row for audit.
const migrateReachNPCProgressSQL = `
 INSERT INTO character_quest_repairs(character_id,quest_id,reason,before_state,after_state)
 SELECT q.character_id,q.quest_id,'reach-npc-model-v1',to_jsonb(q),
        jsonb_set(to_jsonb(q),'{progress_model}','"alflyra-3252-reach-npc-remaining-v1"'::jsonb)
 FROM character_quests q
 WHERE q.quest_id=3281 AND q.status='accepted' AND q.progress BETWEEN 0 AND 1
   AND q.progress_model='stormpass-3281-reach-npc-remaining-v1'
 ON CONFLICT (character_id,quest_id,reason) DO NOTHING;

 UPDATE character_quests
 SET progress_model='alflyra-3252-reach-npc-remaining-v1'
 WHERE quest_id=3281 AND status='accepted' AND progress BETWEEN 0 AND 1
   AND progress_model='stormpass-3281-reach-npc-remaining-v1';
`
