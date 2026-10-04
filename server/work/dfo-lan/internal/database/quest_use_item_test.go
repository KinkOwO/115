package database

import "testing"

func TestUseItemObjectiveRequiresPostAcceptanceConsume(t *testing.T) {
	store, ctx := sqlcTestStore(t)
	_, err := testPool(t, store).Exec(ctx, `CREATE TABLE characters(id bigint,account_id bigint,deleted_at timestamptz);
 CREATE TABLE character_quests(character_id bigint,quest_id integer,status text,progress bigint,
 config_version text,progress_model text,accepted_at timestamptz);
 CREATE TABLE character_events(character_id bigint,event_key text,config_version text,
 outcome jsonb,created_at timestamptz);
 INSERT INTO characters VALUES(11,7,NULL);
 INSERT INTO character_quests VALUES(11,12122,'accepted',1,repeat('a',64),
 'single-use-item-remaining-v1','2026-09-26T10:00:00Z');
 INSERT INTO character_events VALUES
 (11,'consume:66:10312154:2',repeat('a',64),'{"template":10312154}','2026-09-26T09:59:59Z'),
 (11,'consume:67:10312155:3',repeat('a',64),'{"template":10312155}','2026-09-26T10:00:02Z'),
 (11,'consume:68:10312154:4',repeat('a',64),'{"template":10312154}','2026-09-26T10:00:02Z');`)
	if err != nil {
		t.Fatal(err)
	}
	version := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	for _, key := range []string{"consume:66:10312154:2", "consume:67:10312155:3"} {
		applied, err := store.CompleteQuestUseObjective(ctx, 7, 11, 12122, version, "single-use-item-remaining-v1", key, 10312154)
		if err != nil || applied {
			t.Fatalf("unqualified use %s advanced quest: %t %v", key, applied, err)
		}
	}
	if applied, err := store.CompleteQuestUseObjective(ctx, 8, 11, 12122, version, "single-use-item-remaining-v1", "consume:68:10312154:4", 10312154); err != nil || applied {
		t.Fatalf("wrong account advanced quest: %t %v", applied, err)
	}
	if applied, err := store.CompleteQuestUseObjective(ctx, 7, 11, 12122, version, "single-use-item-remaining-v1", "consume:68:10312154:4", 10312154); err != nil || !applied {
		t.Fatalf("qualified use failed: %t %v", applied, err)
	}
	if applied, err := store.CompleteQuestUseObjective(ctx, 7, 11, 12122, version, "single-use-item-remaining-v1", "consume:68:10312154:4", 10312154); err != nil || applied {
		t.Fatalf("duplicate use changed quest: %t %v", applied, err)
	}
}
