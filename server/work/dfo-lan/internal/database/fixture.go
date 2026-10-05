package database

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"dfolan/internal/database/sqlcgen"
)

// TestFixture is a capability owned by a newly created disposable database file.
// It never reads runtime/storage/local.json and never opens a player's store:
// each fixture gets its own file under a private temporary directory which Close
// removes, so isolation comes from the file rather than from a server-side schema.
// Game and GM code must use Store, not this type.
// The fixture offers concrete seeds/assertion reads, not arbitrary SQL.
// An unexported embedded alias promotes Store's ordinary operations without
// allowing other packages to construct a fixture around a player's Store.
type fixtureStore = Store

type TestFixture struct {
	*fixtureStore
	config Config
	dir    string
}

// OpenTestFixture creates a disposable SQLite database in a private temporary
// directory and opens a Store against it.
//
// PostgreSQL support was removed on 2026-10-05 (owner decision, see AGENTS §0.6),
// so the fixture no longer needs DFO_TEST_POSTGRES_DSN: it always runs on the one
// engine the server ships, and the integration tests that use it run everywhere.
func OpenTestFixture(ctx context.Context) (*TestFixture, error) {
	dir, err := os.MkdirTemp("", "dfolan-fixture-")
	if err != nil {
		return nil, err
	}
	f := &TestFixture{dir: dir, config: Config{
		SQLitePath:     filepath.Join(dir, "fixture.sqlite3"),
		MaxConnections: 4,
	}}
	f.fixtureStore, err = Open(ctx, f.config)
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return f, nil
}

func (f *TestFixture) Reopen(ctx context.Context) (*Store, error) { return Open(ctx, f.config) }

func (f *TestFixture) Storage() *Store { return f.fixtureStore }

// Path is the fixture's database file, for helpers that need raw DDL.
func (f *TestFixture) Path() string { return f.config.SQLitePath }

func (f *TestFixture) Close() error {
	if f.fixtureStore != nil {
		f.fixtureStore.Close()
		f.fixtureStore = nil
	}
	var err error
	if f.dir != "" {
		err = os.RemoveAll(f.dir)
		f.dir = ""
	}
	return err
}

func (f *TestFixture) SeedCharacterState(ctx context.Context, id int64, state json.RawMessage) error {
	return f.queries.UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state})
}
func (f *TestFixture) DeleteBirthRecord(ctx context.Context, id int64) error {
	return f.queries.FixtureDeleteBirth(ctx, id)
}
func (f *TestFixture) ArchivedCharacter(ctx context.Context, id int64) (bool, json.RawMessage, error) {
	row, err := f.queries.FixtureArchivedCharacter(ctx, id)
	return row.Archived, row.State, err
}
func (f *TestFixture) EventCount(ctx context.Context, id int64) (int64, error) {
	return f.queries.FixtureEventCount(ctx, id)
}
func (f *TestFixture) SeedVaultItems(ctx context.Context, id int64, items json.RawMessage) error {
	return f.queries.FixtureVaultItems(ctx, sqlcgen.FixtureVaultItemsParams{CharacterID: id, Items: items})
}
func (f *TestFixture) SeedFatigueUsed(ctx context.Context, id int64, used int32) error {
	return f.queries.FixtureFatigueUsed(ctx, sqlcgen.FixtureFatigueUsedParams{CharacterID: id, Used: used})
}
func (f *TestFixture) SeedQuestProgress(ctx context.Context, id int64, quest int32, progress int64) error {
	return f.queries.FixtureQuestProgress(ctx, sqlcgen.FixtureQuestProgressParams{CharacterID: id, QuestID: quest, Progress: progress})
}
func (f *TestFixture) SeedLegacyQuest(ctx context.Context, id int64, quest int32) error {
	return f.queries.FixtureLegacyQuest(ctx, sqlcgen.FixtureLegacyQuestParams{CharacterID: id, QuestID: quest})
}
func (f *TestFixture) QuestRepairCount(ctx context.Context, id int64) (int64, error) {
	return f.queries.FixtureQuestRepairCount(ctx, id)
}
func (f *TestFixture) SeedCompletedQuest(ctx context.Context, id int64, quest int32, version, model string) error {
	return f.queries.FixtureCompletedQuest(ctx, sqlcgen.FixtureCompletedQuestParams{CharacterID: id, QuestID: quest, ConfigVersion: version, ProgressModel: model})
}
func (f *TestFixture) QuestRewardCount(ctx context.Context, id int64, quest int32) (int64, error) {
	return f.queries.FixtureQuestRewardCount(ctx, sqlcgen.FixtureQuestRewardCountParams{CharacterID: id, QuestID: quest})
}
func (f *TestFixture) MapClearCount(ctx context.Context, id int64, run string) (int64, error) {
	return f.queries.FixtureMapClearCount(ctx, sqlcgen.FixtureMapClearCountParams{CharacterID: id, RunID: run})
}
func (f *TestFixture) CharacterState(ctx context.Context, id int64) (json.RawMessage, error) {
	return f.queries.FixtureCharacterState(ctx, id)
}

func (f *TestFixture) SeedAccountCurrency(ctx context.Context, account, cera int64) error {
	return f.queries.FixtureAccountCurrency(ctx, sqlcgen.FixtureAccountCurrencyParams{AccountID: account, Cera: cera})
}

func (f *TestFixture) SeedCharacterSnapshot(ctx context.Context, id int64, state json.RawMessage, version string) error {
	return f.queries.FixtureCharacterSnapshot(ctx, sqlcgen.FixtureCharacterSnapshotParams{CharacterID: id, State: state, ConfigVersion: version})
}

func (f *TestFixture) FatigueRoomStats(ctx context.Context, id int64) (count, cost int64, err error) {
	row, err := f.queries.FixtureFatigueRoomStats(ctx, id)
	return row.Count, row.Cost, err
}

// RejectGraduation makes the fixture refuse any character write that carries
// odyssey_graduation_version, so a test can verify that quest changes and the
// character event roll back together.
//
// The PostgreSQL fixture installed a CHECK constraint on characters
// (`NOT(state?'odyssey_graduation_version')`). SQLite cannot add a CHECK to an
// existing table, so the equivalent here is a BEFORE UPDATE trigger raising
// ABORT on exactly the same condition — the rebuild that
// docs/sqlite-dual-engine-design.md §650 (R10) asked for.
func (f *TestFixture) RejectGraduation(ctx context.Context, reject bool) error {
	db, err := openSQLite(ctx, f.Path(), 1, 0)
	if err != nil {
		return err
	}
	defer db.Close()
	statement := `DROP TRIGGER IF EXISTS fixture_reject_graduation`
	if reject {
		statement = `CREATE TRIGGER IF NOT EXISTS fixture_reject_graduation
			BEFORE UPDATE ON characters
			FOR EACH ROW
			WHEN json_extract(NEW.state, '$.odyssey_graduation_version') IS NOT NULL
			BEGIN
				SELECT RAISE(ABORT, 'reject_graduation');
			END`
	}
	_, err = db.ExecContext(ctx, statement)
	return err
}

func (f *TestFixture) MonsterEventCount(ctx context.Context, id int64) (int64, error) {
	return f.queries.FixtureMonsterEventCount(ctx, id)
}
func (f *TestFixture) SeedInventoryItems(ctx context.Context, id int64, items json.RawMessage) error {
	return f.queries.FixtureInventoryItems(ctx, sqlcgen.FixtureInventoryItemsParams{CharacterID: id, Items: items})
}
func (f *TestFixture) SeedWalletGold(ctx context.Context, id int64, gold json.RawMessage) error {
	return f.queries.FixtureWalletGold(ctx, sqlcgen.FixtureWalletGoldParams{CharacterID: id, Gold: gold})
}
func (f *TestFixture) MergeCharacterFields(ctx context.Context, id int64, fields json.RawMessage) error {
	return f.queries.FixtureMergeCharacterFields(ctx, sqlcgen.FixtureMergeCharacterFieldsParams{CharacterID: id, Fields: fields})
}
func (f *TestFixture) CharacterSnapshot(ctx context.Context, id int64) (json.RawMessage, string, error) {
	row, err := f.queries.FixtureReadCharacterSnapshot(ctx, id)
	return row.State, row.ConfigVersion, err
}
func (f *TestFixture) CashOrderCount(ctx context.Context, account int64) (int64, error) {
	return f.queries.FixtureCashOrderCount(ctx, account)
}
func (f *TestFixture) CashInventoryCount(ctx context.Context, account int64, claimState int32, template int64, amount int32) (int64, error) {
	return f.queries.FixtureCashInventoryCount(ctx, sqlcgen.FixtureCashInventoryCountParams{AccountID: account, ClaimState: claimState, Template: template, Amount: amount})
}
func (f *TestFixture) CashOrderSource(ctx context.Context, account int64, key string) (string, error) {
	return f.queries.FixtureCashOrderSource(ctx, sqlcgen.FixtureCashOrderSourceParams{AccountID: account, OrderKey: key})
}
func (f *TestFixture) SeedQuestRecord(ctx context.Context, id int64, quest int32, status, version, model string) error {
	return f.queries.FixtureQuestRecordSeed(ctx, sqlcgen.FixtureQuestRecordSeedParams{CharacterID: id, QuestID: quest, Status: status, ConfigVersion: version, ProgressModel: model})
}
func (f *TestFixture) QuestCount(ctx context.Context, id int64) (int64, error) {
	return f.queries.FixtureQuestCount(ctx, id)
}
func (f *TestFixture) QuestRecord(ctx context.Context, id int64, quest int32) (string, string, error) {
	row, err := f.queries.FixtureQuestRecord(ctx, sqlcgen.FixtureQuestRecordParams{CharacterID: id, QuestID: quest})
	return row.Status, row.ProgressModel, err
}

func (f *TestFixture) SeedFatigueUsage(ctx context.Context, id int64, used, usedMax int32) error {
	return f.queries.FixtureFatigueUsage(ctx, sqlcgen.FixtureFatigueUsageParams{CharacterID: id, Used: used, UsedMax: usedMax})
}
