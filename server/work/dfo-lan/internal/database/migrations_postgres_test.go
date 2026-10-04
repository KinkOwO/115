package database

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestSQLCMigrationFailureDoesNotAdoptOrRecord(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	// Missing characters makes mailbox initialization fail after its sequence DDL.
	if err := s.MigrateMailbox(ctx); err == nil {
		t.Fatal("mailbox without prerequisites succeeded")
	}
	var untouched bool
	if err := testPool(t, s).QueryRow(ctx, `SELECT to_regclass('storage_migrations') IS NULL AND to_regclass('mailbox_id_seq') IS NULL`).Scan(&untouched); err != nil || !untouched {
		t.Fatalf("failed initialization left DDL or ledger: %v %v", untouched, err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateMailbox(ctx); err != nil {
		t.Fatal(err)
	}
	var entries int
	if err := testPool(t, s).QueryRow(ctx, `SELECT count(*) FROM storage_migrations`).Scan(&entries); err != nil || entries != 2 {
		t.Fatalf("recorded migrations: %d %v", entries, err)
	}
}

func TestSQLCMigrationConcurrentAdoptionAndChecksum(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.Migrate(ctx) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var checksum string
	var runs int64
	if err := testPool(t, s).QueryRow(ctx, `SELECT checksum,runs FROM storage_migrations WHERE name='0001_core.sql'`).Scan(&checksum, &runs); err != nil || runs != 1 {
		t.Fatalf("concurrent adoption: %d %v", runs, err)
	}
	if _, err := testPool(t, s).Exec(ctx, `UPDATE storage_migrations SET checksum=repeat('0',64) WHERE name='0001_core.sql'`); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err == nil || !strings.Contains(err.Error(), "checksum differs") {
		t.Fatalf("changed applied SQL accepted: %v", err)
	}
	if _, err := testPool(t, s).Exec(ctx, `UPDATE storage_migrations SET checksum=$1 WHERE name='0001_core.sql'`, checksum); err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DevelopmentAccount(ctx, "after-adoption"); err != nil {
		t.Fatal(err)
	}
}

func TestSQLCMigrationRepeatRepairsAndOptionalCapacity(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	for _, migrate := range []func() error{func() error { return s.Migrate(ctx) }, func() error { return s.MigrateVault(ctx) }, func() error { return s.MigrateQuests(ctx) }} {
		if err := migrate(); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "repeat-repairs")
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "RepeatRepairs", Request: []byte{1}, ConfigVersion: strings.Repeat("a", 64), State: json.RawMessage(`{"unknown":true}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	// Optional capacity repair is never enabled just because its SQL is embedded.
	var count int
	if err := testPool(t, s).QueryRow(ctx, `SELECT count(*) FROM storage_migrations WHERE name='0028_secondary_vault_upgrade.sql'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("optional repair auto-enabled: %d %v", count, err)
	}
	if err := s.UpgradeSecondaryVaultCapacity(ctx); err != nil {
		t.Fatal(err)
	}
	if v, err := s.LoadVault(ctx, account, role.ID, 8, role.ConfigVersion, 45); err != nil || v.Slots != 8 {
		t.Fatalf("new vault: %+v %v", v, err)
	}
	if err := s.UpgradeSecondaryVaultCapacity(ctx); err != nil {
		t.Fatal(err)
	}
	if v, err := s.LoadVault(ctx, account, role.ID, 8, role.ConfigVersion, 45); err != nil || v.Slots != 24 {
		t.Fatalf("repeat legacy capacity repair: %+v %v", v, err)
	}
	// A legacy save imported after the first schema adoption still gets its
	// precise old-model repair at the next explicit quest initialization.
	if _, err := testPool(t, s).Exec(ctx, `INSERT INTO character_quests(character_id,quest_id,status,progress,config_version,progress_model,accepted_at,completed_at) VALUES($1,3145,'completed',0,$2,'act-clear-v1',now(),now())`, role.ID, role.ConfigVersion); err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateQuests(ctx); err != nil {
		t.Fatal(err)
	}
	if err := testPool(t, s).QueryRow(ctx, `SELECT count(*) FROM character_quests WHERE character_id=$1`, role.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("late imported synthetic quest kept: %d %v", count, err)
	}
	if err := testPool(t, s).QueryRow(ctx, `SELECT count(*) FROM character_quest_repairs WHERE character_id=$1`, role.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("late repair audit: %d %v", count, err)
	}
	var runs int64
	if err := testPool(t, s).QueryRow(ctx, `SELECT runs FROM storage_migrations WHERE name='0028_secondary_vault_upgrade.sql'`).Scan(&runs); err != nil || runs != 2 {
		t.Fatalf("repeat ledger: %d %v", runs, err)
	}
	if err := s.MigrateTowerProgress(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := testPool(t, s).Exec(ctx, `INSERT INTO account_tower_grief_progress(account_id,highest_cleared,cleared_day,last_run_id) VALUES($1,37,'2026-10-03','legacy-late')`, account); err != nil {
		t.Fatal(err)
	}
	if err := s.MigrateTowerProgress(ctx); err != nil {
		t.Fatal(err)
	}
	var floor int
	if err := testPool(t, s).QueryRow(ctx, `SELECT highest_cleared FROM account_tower_progress WHERE account_id=$1 AND tower_key='grief'`, account).Scan(&floor); err != nil || floor != 37 {
		t.Fatalf("late tower save not adopted: %d %v", floor, err)
	}
}

// Frozen checksums from the 38 original files before consolidation. This proves
// that every existing ledger identity still selects the exact original SQL,
// independently of the new section loader's own checksum calculation.
func TestConsolidatedMigrationChecksums(t *testing.T) {
	expected := map[string]string{
		"0000_migration_ledger.sql":        "c616ec70d52f6544248a4f15752832d2ff8b033b9c58457cdfc310d8ac510d84",
		"0001_core.sql":                    "4726996b13eb61609cc1bf1072207b6b7871260837f6779f51f645bb906ae26f",
		"0002_character_events.sql":        "19782f6791d426ce5929eb0c7fa3abfe2f88c0106c99eb0472df75d37613f4b7",
		"0003_character_notices.sql":       "bf38cdf007f074a7cef62e67d1d8a571650dcdb8fb4be27bcf536613ed2ac5e5",
		"0004_tutorial.sql":                "1489c19df7f387b054d6a4adda1bb79ed95cee39c810029825004dfb370d0693",
		"0005_warp_favorites.sql":          "cd8779bb8d5d39676a3d9f370eba8d4b6a51664e25dea6a2de7dacaf327c4e0b",
		"0006_adventure.sql":               "ceee91f205cf5649ddf646d9654b801b452ac5dc0464be902ca0f86009ad4c9e",
		"0007_equipment_skill.sql":         "474f535088acf45509db8cf6bb1872afa41828cbc029561960300c2f9e62bd64",
		"0008_skill_locks.sql":             "fd5bc487bc0152d22c3cd0d7f1b8bde964d67d4654a4fe9bb86e79146344356c",
		"0009_profile_skins.sql":           "0e0a56686192d27b038c33024af4c5d70513c1dd4448f57aaad3b30ca896a751",
		"0010_gamepad.sql":                 "731df73c57f57d1370f5bf042145a3f171528ffc33617641a88c59df873cd86d",
		"0011_shop_purchases.sql":          "a145725122b1da8c0bedae9c360ef1c12f515d272d3050f0f534d85a5a7750cc",
		"0012_roster_backgrounds.sql":      "877726ab860a72c373babf9a09e8a227570ac918515151580f27833bcf2c61dd",
		"0013_skin_selection.sql":          "50cf7ace781aa2a4dc0205e894f7ebe02dcf48dd16c2664a0b9c2f0cd4ad0a24",
		"0014_skin_lists.sql":              "afc1ed906f0cf4d8b24da814485a9883f03404d002cee1dc008747aaf23d2d8a",
		"0015_skin_cargo.sql":              "97e3d109a6d7cdd46ef19365c51cd453cc0761bb3cc1987e4e232f51f68aa599",
		"0016_unified_options.sql":         "8e6b4740ca4311de5a98c6865316da04e7b52704c87dd5fc373d737e04ab4978",
		"0017_account_materials.sql":       "f5cbc0e8a224250c83c9b9354877f4200c265c5697a809d979faefa4efb29c46",
		"0018_grants.sql":                  "287738e7fb91e948a4ac7fc7ad1c74290d05a914335f6dac58575fcb3a53080c",
		"0019_world.sql":                   "2ac96e6f3eaba505074b0265976e6cf9f6294ec1136076ab1ba70e4fb1a77bae",
		"0020_birth.sql":                   "e97b77bcbf2fc3eeef3b3334b299e3775406cdee21c28b24180ec88890c5fa95",
		"0021_fatigue.sql":                 "b8d3aaa2cfb81861b8101d66fcf323e368d7c9add184c4f88ccd6f8a2feb8e68",
		"0022_premiums.sql":                "84cf2758e514a53cf151752dc790000d218735fec0b1a46b0d1664fed3e8b07e",
		"0023_quests.sql":                  "73e05aa54b737b3683cef041602c23dcc92a016fc16703cd3c92c18ee787a35b",
		"0024_quest_objectives.sql":        "2706bd012fe1343af1006eab666ea8dde715693675da61b598868eb30521acd0",
		"0025_quest_rewards.sql":           "39f23cecc7e0126b6794653c4eb7684b3ae827d0c182db92c61ea0b71af9f8f2",
		"0026_vaults.sql":                  "7f0d9373b8901176e2abdf6be3c0e6649205a7d3ce1371b274cb85f48a3399bd",
		"0027_account_vault.sql":           "b52cb9f97c470fa1bb1cc24a78d1aaf93989c09e06ab6526cd47febbea2a6b41",
		"0028_secondary_vault_upgrade.sql": "c5dc0adffa2008e36320202986ed54ab5f71444585bde34bd05c60b600a23e46",
		"0029_cash_shop.sql":               "7ea3d151584f7614326cfdd30427dfc77bcc6f26fde019a8dd2d61a6c5cd47e8",
		"0030_omen_state.sql":              "09b2862b1e3044edd7f4577bb3e78ef00e376aa99bd83bd84e570bd3088b0ddd",
		"0031_oath_progress.sql":           "a01cba794bb2faaa71ce450f40ab35cac8c86a560695e1733b6ff88465e3119d",
		"0032_oath_options.sql":            "e4daa797396a7ea8c47952ab6d50222de1635ad1e0bb13d1d2a393b121dd93d1",
		"0033_bleeding_mine.sql":           "9e2a63165645b0de704bf5a2b7c741abad57c2b2d6dc039daf6953f1cdd12c69",
		"0034_tower_grief.sql":             "e89ff5387646f7030661e92ba5a7b5e79378fd6c0d8c5b748d9ebd06efa3b6fd",
		"0035_tower_progress.sql":          "371ef9d1901f427b3feb0913872f622c32ef15e27ceefb7fa06ad6a7f5626776",
		"0036_mailbox.sql":                 "8501a243ecd9409dc1f2f8a8721c8b3fbbe5b848745612c46102d2e485521270",
		"0037_gm_mail.sql":                 "9769f2fd589f9794f5df519bf548058c8d4efb87cab5b0aede3a785068f2cbb6",
	}
	files, err := migrationSQL.ReadDir("sql/postgres/migrations")
	if err != nil {
		t.Fatal(err)
	}
	initialFound := false
	for _, file := range files {
		initialFound = initialFound || file.Name() == "0001_initial.sql"
		if _, oldFile := expected[file.Name()]; oldFile {
			t.Fatalf("original migration file still exists: %s", file.Name())
		}
	}
	if !initialFound {
		t.Fatal("consolidated initial migration missing")
	}
	for name, checksum := range expected {
		query, err := migrationQuery(name)
		if err != nil {
			t.Fatal(name, err)
		}
		actual := fmt.Sprintf("%x", sha256.Sum256(query))
		if actual != checksum {
			t.Errorf("historical migration %s changed: %s != %s", name, actual, checksum)
		}
	}
	if _, err := migrationQuery("0001_initial.sql"); err == nil {
		t.Fatal("complete schema execution would bypass module gates")
	}
	if _, err := migrationQuery("missing-migration.sql"); err == nil {
		t.Fatal("missing migration selected unrelated SQL")
	}
}

func TestConsolidatedMigrationAdoptsExistingLedger(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	// Simulate a database initialized by the former individual-file layout.
	for _, name := range []string{"0000_migration_ledger.sql", "0001_core.sql"} {
		query, err := migrationQuery(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := testPool(t, s).Exec(ctx, string(query)); err != nil {
			t.Fatal(err)
		}
	}
	const previousCoreChecksum = "4726996b13eb61609cc1bf1072207b6b7871260837f6779f51f645bb906ae26f"
	if _, err := testPool(t, s).Exec(ctx, `INSERT INTO storage_migrations(name,checksum,runs) VALUES('0001_core.sql',$1,7)`, previousCoreChecksum); err != nil {
		t.Fatal(err)
	}
	account, err := s.DevelopmentAccount(ctx, "consolidated-legacy")
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "PreservedLegacy",
		Request: []byte{0, 255}, ConfigVersion: strings.Repeat("a", 64),
		State: json.RawMessage(`{"inventory":{"gold":123},"unknown":9007199254740993}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	var runs int64
	if err := testPool(t, s).QueryRow(ctx, `SELECT runs FROM storage_migrations WHERE name='0001_core.sql'`).Scan(&runs); err != nil || runs != 7 {
		t.Fatalf("old migration record was replaced or replayed: %d %v", runs, err)
	}
	rows, err := s.Characters(ctx, account)
	if err != nil || len(rows) != 1 || rows[0].ID != role.ID || rows[0].WireID != role.WireID ||
		!sameJSON(t, rows[0].State, role.State) || string(rows[0].Request) != string(role.Request) {
		t.Fatalf("legacy save changed: %+v %v", rows, err)
	}
	var gated bool
	if err := testPool(t, s).QueryRow(ctx, `SELECT to_regclass('gm_mail') IS NULL AND to_regclass('character_quests') IS NULL AND to_regclass('account_adventure') IS NULL AND to_regclass('character_vaults') IS NULL`).Scan(&gated); err != nil || !gated {
		t.Fatalf("consolidation enabled unrelated module schemas: %v %v", gated, err)
	}
}
