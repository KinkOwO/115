package database

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"dfolan/internal/game/protocol"
)

func sqlcTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("DFO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("DFO_TEST_POSTGRES_DSN requires a dedicated PostgreSQL test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	admin, err := Open(ctx, Config{PostgresDSN: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := fmt.Sprintf("sqlc_%d", time.Now().UnixNano())
	if _, err := testPool(t, admin).Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		defer admin.Close()
		if _, err := testPool(t, admin).Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	s, err := Open(ctx, Config{PostgresDSN: dsn, PostgresSchema: schema, MaxConnections: 4})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s, ctx
}

// Integration tests must never silently fall back to a player's local.json.
func loadPostgresTestConfig() (Config, error) {
	dsn := os.Getenv("DFO_TEST_POSTGRES_DSN")
	if dsn == "" {
		return Config{}, errors.New("DFO_TEST_POSTGRES_DSN requires a dedicated test database")
	}
	return Config{PostgresDSN: dsn, MaxConnections: 4}, nil
}

func TestSQLCCoreUpgradeAndAccountSerialization(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	// Pre-roster schema with a real historical save: initialize through the new
	// migration files without resetting identity counters, bytea or unknown JSON.
	_, err := testPool(t, s).Exec(ctx, `CREATE TABLE accounts (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, username text NOT NULL UNIQUE,
 password_hash text, development_only boolean NOT NULL DEFAULT true,
 created_at timestamptz NOT NULL DEFAULT now(), CHECK(development_only OR password_hash IS NOT NULL));
 CREATE TABLE characters (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, account_id bigint NOT NULL REFERENCES accounts(id),
 wire_id integer NOT NULL CHECK(wire_id BETWEEN 1 AND 65534), name text NOT NULL,
 profession integer NOT NULL CHECK(profession BETWEEN 0 AND 255), create_request bytea NOT NULL,
 config_version text NOT NULL,state jsonb NOT NULL,created_at timestamptz NOT NULL DEFAULT now(), UNIQUE(account_id,wire_id));
 INSERT INTO accounts(username) VALUES('historical');
 INSERT INTO characters(account_id,wire_id,name,profession,create_request,config_version,state)
 VALUES(1,17,'Historical',255,decode('00ff','hex'),repeat('a',64),'{"unknown":{"binary":"00ff"},"level":1}');`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := s.Migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "historical")
	if err != nil || account != 1 {
		t.Fatalf("legacy account changed: %d %v", account, err)
	}
	roles, err := s.Characters(ctx, account)
	if err != nil || len(roles) != 1 || roles[0].ID != 1 || roles[0].WireID != 17 || roles[0].Profession != 255 ||
		!reflect.DeepEqual(roles[0].Request, []byte{0, 255}) || !sameJSON(t, roles[0].State, json.RawMessage(`{"unknown":{"binary":"00ff"},"level":1}`)) {
		t.Fatalf("historical save changed: %+v %v", roles, err)
	}
	if exists, err := s.NameExists(ctx, "hISTORICAL"); err != nil || !exists {
		t.Fatalf("case-insensitive name lookup: %v %v", exists, err)
	}
	// Confirm aggregates retain bigint semantics rather than truncating to int32.
	const highOrder = int64(1) << 33
	if _, err := testPool(t, s).Exec(ctx, `UPDATE characters SET roster_order=$1 WHERE id=1`, highOrder); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, name := range []string{"ConcurrentA", "ConcurrentB"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			_, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: name, Profession: 1,
				Request: []byte{1}, ConfigVersion: strings.Repeat("a", 64), State: json.RawMessage(`{}`)}, 2)
			results <- err
		}(name)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if err.Error() != "character slots full" {
			t.Error(err)
		}
	}
	var wire int
	var order int64
	if err := testPool(t, s).QueryRow(ctx, `SELECT wire_id,roster_order FROM characters WHERE id<>1`).Scan(&wire, &order); err != nil {
		t.Fatal(err)
	}
	if success != 1 || wire != 18 || order != highOrder+1 {
		t.Fatalf("allocation: success=%d wire=%d order=%d", success, wire, order)
	}
	if err := s.ChangeCharacterSlots(ctx, account, CharacterSlotChange{From: 0, To: 1, Swap: true}, 24); err != nil {
		t.Fatal(err)
	}
	roles, err = s.Characters(ctx, account)
	if err != nil || len(roles) != 2 || roles[1].ID != 1 || roles[1].WireID != 17 {
		t.Fatalf("roster reorder changed identities: %+v %v", roles, err)
	}
	if err := s.execMigration(ctx, "0006_adventure.sql"); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool(t, s).Exec(ctx, `INSERT INTO account_adventures(account_id,name,data) VALUES($1,'Fixture','{"season_level":{"future":123}}')`, account); err != nil {
		t.Fatal(err)
	}
	s.adventureEnabled = true
	roles, err = s.Characters(ctx, account)
	if err != nil || len(roles) != 2 {
		t.Fatalf("adventure roster query: %+v %v", roles, err)
	}
	var state map[string]json.RawMessage
	if err := json.Unmarshal(roles[1].State, &state); err != nil || !sameJSON(t, state["season_level"], json.RawMessage(`{"future":123}`)) ||
		!sameJSON(t, state["unknown"], json.RawMessage(`{"binary":"00ff"}`)) {
		t.Fatalf("adventure projection changed existing save: %s %v", roles[1].State, err)
	}
	if _, err := testPool(t, s).Exec(ctx, `INSERT INTO accounts(username,development_only,password_hash) VALUES('protected',false,'hash')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DevelopmentAccount(ctx, "protected"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("development access changed protected account: %v", err)
	}
}

func TestSQLCBinarySettingsKeepIndependentSnapshots(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	for _, migrate := range []func(context.Context) error{s.Migrate, s.MigrateEquipmentSkill, s.MigrateGamepad} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "binary-settings")
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "Binary", Request: []byte{0},
		ConfigVersion: strings.Repeat("a", 64), State: json.RawMessage(`{}`)}, 24)
	if err != nil {
		t.Fatal(err)
	}
	sk, cm, err := s.EquipmentSkillSnapshots(ctx, account, role.ID)
	if err != nil || sk != nil || cm != nil {
		t.Fatalf("unset snapshots should stay NULL: %x %x %v", sk, cm, err)
	}
	skills, commands := bytes.Repeat([]byte{0, 255}, 40), bytes.Repeat([]byte{255, 0}, 40)
	if err := s.SaveEquipmentSkillSnapshot(ctx, account, role.ID, EquipmentSkillSnapshotColumn, skills); err != nil {
		t.Fatal(err)
	}
	sk, cm, err = s.EquipmentSkillSnapshots(ctx, account, role.ID)
	if err != nil || !bytes.Equal(sk, skills) || cm != nil {
		t.Fatalf("skills write changed commands: %x %x %v", sk, cm, err)
	}
	if err := s.SaveEquipmentSkillSnapshot(ctx, account, role.ID, EquipmentCommandSnapshotColumn, commands); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveEquipmentSkillSnapshot(ctx, account+1, role.ID, EquipmentSkillSnapshotColumn, commands); err == nil {
		t.Fatal("wrong owner overwrote skills")
	}
	sk, cm, err = s.EquipmentSkillSnapshots(ctx, account, role.ID)
	if err != nil || !bytes.Equal(sk, skills) || !bytes.Equal(cm, commands) {
		t.Fatalf("snapshots no longer independent: %x %x %v", sk, cm, err)
	}
	if err := s.ClearEquipmentSkill(ctx, account, role.ID); err != nil {
		t.Fatal(err)
	}
	sk, cm, err = s.EquipmentSkillSnapshots(ctx, account, role.ID)
	if err != nil || sk != nil || cm != nil {
		t.Fatalf("clear did not restore unset state: %x %x %v", sk, cm, err)
	}
	if payload, err := s.ResolveGamepadPayload(ctx, account, role.ID); err != nil || payload != nil {
		t.Fatalf("unset gamepad: %v %v", payload, err)
	}
	options := []byte{0, 255, 1, 0, 2, 0, 3, 0, 4, 0}
	if err := s.SaveAccountGamepadKeys(ctx, account, []byte("account\x00\x00")); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAccountGamepadOptions(ctx, account, options); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCharacterGamepadKeys(ctx, account, role.ID, []byte("character\x00")); err != nil {
		t.Fatal(err)
	}
	if payload, err := s.ResolveGamepadPayload(ctx, account, role.ID); err != nil || !bytes.Equal(payload, BuildGamepadPayload([]byte("character"), options)) {
		t.Fatalf("partial fallback lost independent account options: %v", err)
	}
	if err := s.ClearAccountCharacterGamepadSettings(ctx, account); err != nil {
		t.Fatal(err)
	}
	if payload, err := s.ResolveGamepadPayload(ctx, account, role.ID); err != nil || !bytes.Equal(payload, BuildGamepadPayload([]byte("account"), options)) {
		t.Fatalf("cleared character did not fall back to account: %v", err)
	}
}

func TestSQLCSettingsPersistenceAndRollback(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	for _, migrate := range []func(context.Context) error{s.Migrate, s.MigrateCharacterNotices, s.MigrateTutorial, s.migrateWarpFavorites} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "settings")
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "Settings", Request: []byte{0},
		ConfigVersion: strings.Repeat("a", 64), State: json.RawMessage(`{}`)}, 24)
	if err != nil {
		t.Fatal(err)
	}
	if seen, err := s.CharacterNoticeSeen(ctx, account, role.ID, 2); err != nil || seen == nil || len(seen) != 0 {
		t.Fatalf("notice empty-list semantics: %v %v", seen, err)
	}
	for _, id := range []uint16{math.MaxInt16, 62, 1, 62} {
		if err := s.MarkCharacterNotice(ctx, account, role.ID, 2, id, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.MarkCharacterNotice(ctx, account, role.ID, 2, math.MaxUint16, true); err == nil {
		t.Fatal("out-of-range notice silently truncated")
	}
	if err := s.MarkCharacterNotice(ctx, account, role.ID, 2, 62, false); err != nil {
		t.Fatal(err)
	}
	if seen, err := s.CharacterNoticeSeen(ctx, account, role.ID, 2); err != nil || !reflect.DeepEqual(seen, []uint16{1, math.MaxInt16}) {
		t.Fatalf("notice persistence/order: %v %v", seen, err)
	}
	if flags, err := s.TutorialFlags(ctx, account, role.ID); err != nil || flags == nil || len(flags) != 0 {
		t.Fatalf("tutorial empty-list semantics: %v %v", flags, err)
	}
	if err := s.SaveTutorialFlag(ctx, account+1, role.ID, 100, true); err == nil {
		t.Fatal("tutorial accepted wrong account")
	}
	for _, completed := range []bool{true, false, true} {
		if err := s.SaveTutorialFlag(ctx, account, role.ID, 100, completed); err != nil {
			t.Fatal(err)
		}
	}
	if flags, err := s.TutorialFlags(ctx, account, role.ID); err != nil || !reflect.DeepEqual(flags, []byte{100}) {
		t.Fatalf("tutorial persistence: %v %v", flags, err)
	}
	if favorites, err := s.AccountWarpFavorites(ctx, account); err != nil || favorites != nil {
		t.Fatalf("warp empty-list semantics: %v %v", favorites, err)
	}
	entries := []protocol.WarpFavoriteEntry{{Position: 9, Value: [12]byte{1, 0, 255}}, {Position: 0}}
	if err := s.SaveAccountWarpFavorites(ctx, account, entries); err != nil {
		t.Fatal(err)
	}
	favorites, err := s.AccountWarpFavorites(ctx, account)
	if err != nil || !reflect.DeepEqual(favorites, []protocol.WarpFavoriteEntry{entries[1], entries[0]}) {
		t.Fatalf("explicit empty slot or bytes lost: %+v %v", favorites, err)
	}
	// Fail the second row after updating the first, proving WithTx does not
	// accidentally execute writes on the pool outside the transaction.
	if _, err := testPool(t, s).Exec(ctx, `CREATE FUNCTION reject_warp() RETURNS trigger LANGUAGE plpgsql AS $$
 BEGIN IF get_byte(NEW.value,0)=255 THEN RAISE EXCEPTION 'fixture rejection'; END IF; RETURN NEW; END $$;
 CREATE TRIGGER reject_warp BEFORE INSERT OR UPDATE ON account_warp_favorites FOR EACH ROW EXECUTE FUNCTION reject_warp();`); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveAccountWarpFavorites(ctx, account, []protocol.WarpFavoriteEntry{{Position: 0, Value: [12]byte{7}}, {Position: 1, Value: [12]byte{255}}}); err == nil {
		t.Fatal("expected failure in second row")
	}
	if after, err := s.AccountWarpFavorites(ctx, account); err != nil || !reflect.DeepEqual(after, favorites) {
		t.Fatalf("failed upload partially committed: %+v %v", after, err)
	}
}
