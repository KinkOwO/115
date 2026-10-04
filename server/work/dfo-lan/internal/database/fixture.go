package database

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"dfolan/internal/database/sqlcgen"
	"github.com/jackc/pgx/v5"
)

// TestFixture is a capability owned by a newly created disposable schema.
// Its constructor requires an explicit dedicated test DSN and never reads
// runtime/storage/local.json. Game and GM code must use Store, not this type.
// The fixture offers concrete seeds/assertion reads, not arbitrary SQL.
// An unexported embedded alias promotes Store's ordinary operations without
// allowing other packages to construct a fixture around a player's Store.
type fixtureStore = Store

type TestFixture struct {
	*fixtureStore
	config Config
	admin  *Store
	schema string
}

func OpenTestFixture(ctx context.Context) (*TestFixture, error) {
	dsn := os.Getenv("DFO_TEST_POSTGRES_DSN")
	if dsn == "" {
		return nil, errors.New("DFO_TEST_POSTGRES_DSN must explicitly select a dedicated test database")
	}
	var token [8]byte
	if _, err := rand.Read(token[:]); err != nil {
		return nil, err
	}
	schema := "charactercheck_" + hex.EncodeToString(token[:])
	admin, err := Open(ctx, Config{PostgresDSN: dsn, MaxConnections: 1})
	if err != nil {
		return nil, err
	}
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.db.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		admin.Close()
		return nil, err
	}
	f := &TestFixture{admin: admin, schema: schema, config: Config{PostgresDSN: dsn, PostgresSchema: schema, MaxConnections: 4}}
	f.fixtureStore, err = Open(ctx, f.config)
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	actual, err := f.queries.FixtureSchema(ctx)
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	if actual != schema {
		return nil, errors.Join(fmt.Errorf("test isolation failed: %s", actual), f.Close())
	}
	return f, nil
}

func (f *TestFixture) Reopen(ctx context.Context) (*Store, error) { return Open(ctx, f.config) }

func (f *TestFixture) Storage() *Store { return f.fixtureStore }

func (f *TestFixture) Close() error {
	if f.fixtureStore != nil {
		f.fixtureStore.Close()
	}
	if f.admin == nil {
		return nil
	}
	defer f.admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := f.admin.db.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{f.schema}.Sanitize()+" CASCADE")
	f.admin = nil
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

//go:embed sql/postgres/fixtures/*.sql
var fixtureDDL embed.FS

// RejectGraduation injects a database CHECK failure in this fixture's schema
// to verify that quest changes and the character event roll back together.
func (f *TestFixture) RejectGraduation(ctx context.Context, reject bool) error {
	name := "allow_graduation.sql"
	if reject {
		name = "reject_graduation.sql"
	}
	body, err := fixtureDDL.ReadFile("sql/postgres/fixtures/" + name)
	if err != nil {
		return err
	}
	_, err = f.db.Exec(ctx, string(body))
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
