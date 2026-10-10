package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSymbolSocketTransactionReplayAndReload(t *testing.T) {
	for _, driver := range []string{database.DriverSQLite, database.DriverPostgres} {
		t.Run(driver, func(t *testing.T) {
			ctx := context.Background()
			var store *database.Store
			var err error
			if driver == database.DriverPostgres {
				if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
					t.Skip("dedicated PostgreSQL test DSN required")
				}
				fixture, e := database.OpenTestFixture(ctx)
				require.NoError(t, e)
				t.Cleanup(func() { require.NoError(t, fixture.Close()) })
				store = fixture.Storage()
			} else {
				store, err = database.Open(ctx, database.Config{Driver: driver, SQLitePath: filepath.Join(t.TempDir(), "symbol.sqlite")})
				require.NoError(t, err)
				t.Cleanup(store.Close)
			}
			require.NoError(t, store.Migrate(ctx))
			require.NoError(t, store.MigrateCharacterEvents(ctx))
			source := strings.Repeat("a", 64)
			catalog := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: source}, Items: map[uint32]catalog.LootItem{20001: {ID: 20001, Kind: "stackable", StackableType: "[avatar emblem]", StackLimit: 100}}}
			layout := &inventory.SymbolSocketLayout{Source: source, Sockets: map[string]inventory.SymbolSocketGroup{"red_symbol": {Count: 4, Mask: 1}}}
			service := &ItemService{Store: store, Items: &inventory.ItemService{Catalog: catalog, BagRules: inventory.BagRules{Source: source, MissingStackLimit: 100, Slots: map[string][2]uint16{"[avatar emblem]": {289, 290}}}, EmblemInlay: &inventory.EmblemInlayRules{Source: source, Masks: map[uint32]uint16{20001: 1}}}}
			account, e := store.DevelopmentAccount(ctx, "symbol-socket-fixture")
			require.NoError(t, e)
			initial := json.RawMessage(`{"level":115,"future_role":{"keep":true},"inventory":{"version":"ordinary-bag-v1","gold":777,"future_inventory":{"keep":true},"items":[{"slot":289,"Template":20001,"Amount":2}]}}`)
			role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "symbol-fixture", ConfigVersion: catalog.Source.SaveIdentity(), State: initial, Request: []byte{}}, 24)
			require.NoError(t, e)
			req := inventory.SymbolSocketRequest{Source: source, Group: "red_symbol", BagSlot: 289, Template: 20001}
			saved, applied, e := service.ExchangeSymbolSocket(ctx, role, layout, req, time.Unix(100, 0))
			require.NoError(t, e)
			require.True(t, applied)
			// A reply can be lost; both old and refreshed role objects must replay.
			replay, applied, e := service.ExchangeSymbolSocket(ctx, saved, layout, req, time.Unix(101, 0))
			require.NoError(t, e)
			require.False(t, applied)
			require.JSONEq(t, string(saved.State), string(replay.State))
			fresh, e := store.AdminCharacter(ctx, role.ID)
			require.NoError(t, e)
			slots, e := inventory.ReadSymbolSockets(fresh.State, layout)
			require.NoError(t, e)
			require.Equal(t, uint64(1), slots.Sequence)
			require.Equal(t, uint32(20001), slots.Groups["red_symbol"][0].Template)
			stale := req
			stale.Socket = 1
			_, _, e = service.ExchangeSymbolSocket(ctx, fresh, layout, stale, time.Unix(102, 0))
			require.ErrorContains(t, e, "stale")
			req.Sequence = 1
			req.ExpectedTemplate = 20001
			req.Template = 0
			req.BagSlot = 0
			final, applied, e := service.ExchangeSymbolSocket(ctx, fresh, layout, req, time.Unix(103, 0))
			require.NoError(t, e)
			require.True(t, applied)
			var before, after map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(initial, &before))
			require.NoError(t, json.Unmarshal(final.State, &after))
			require.JSONEq(t, string(before["inventory"]), string(after["inventory"]))
			require.JSONEq(t, string(before["future_role"]), string(after["future_role"]))
		})
	}
}
