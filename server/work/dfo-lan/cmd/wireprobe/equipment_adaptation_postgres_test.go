package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"dfolan/internal/workflow"
	"github.com/stretchr/testify/require"
)

// Real PostgreSQL transactions verify wallet/vault conservation, rollback,
// account isolation and replay. Every case owns a disposable schema.
func TestEquipmentAdaptationVaultTransaction(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("requires dedicated PostgreSQL test database")
	}
	for _, tc := range []struct {
		name          string
		wallet, vault uint32
		refused       bool
	}{
		{"wallet_first", 40000, 100000, false},
		{"vault_shortfall", 10000, 100000, false},
		{"insufficient_total", 10000, 20000, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			fixture, err := database.OpenTestFixture(ctx)
			require.NoError(t, err)
			defer func() { require.NoError(t, fixture.Close()) }()
			store := fixture.Storage()
			for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateAccountMaterials, store.MigrateAccountVault} {
				require.NoError(t, migrate(ctx))
			}
			account, err := store.DevelopmentAccount(ctx, "equipment-adaptation")
			require.NoError(t, err)
			other, err := store.DevelopmentAccount(ctx, "other-account")
			require.NoError(t, err)
			version := strings.Repeat("a", 64)
			gear, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
				Source: pvf.ArchiveSnapshot{Checksum: version},
				Rows:   []inventory.EquipmentDefinition{primerDef(100401592, 2), primerDef(100401597, 6)},
			}, version)
			require.NoError(t, err)
			table, err := catalog.ParseEquipmentTransformSystem(primerTransformFlowTable)
			require.NoError(t, err)
			journal := catalog.EquipmentJournalRules{Maximum: 99}
			items := &inventory.ItemService{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: version}},
				Equipment: gear, Transform: &table, Journal: &journal, BagRules: inventory.BagRules{MissingStackLimit: 2147483647}}
			state, err := inventory.SaveBag(json.RawMessage(`{"level":115}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: tc.wallet,
				Worn: []inventory.BagEquipment{{Slot: 36, Template: 100401592}}})
			require.NoError(t, err)
			state, err = inventory.SaveEquipmentJournal(state, inventory.EquipmentJournal{Counts: map[uint32]uint32{100401597: 1}})
			require.NoError(t, err)
			role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "vault-test", Profession: 11,
				ConfigVersion: savecontract.Identity(), Request: []byte{0}, State: state}, 24)
			require.NoError(t, err)
			role.WireID = 17
			_, _, _, applied, err := store.CommitAccountVault(ctx, account, role.ID, savecontract.Identity(), "seed-vault", 0,
				func(current database.Character, materials json.RawMessage, vault database.AccountVaultState) (json.RawMessage, json.RawMessage, database.AccountVaultState, error) {
					vault.Slots, vault.Gold = 8, tc.vault
					return current.State, materials, vault, nil
				})
			require.NoError(t, err)
			require.True(t, applied)
			r, err := protocol.DecodePrimerTransformRequest(primerTransformFrame(t, 0, 100401597))
			require.NoError(t, err)
			// Invalid payment groups must not authorize any deductions.
			w := &worldSession{account: account, store: store, role: role, items: items}
			invalid := primerTransformFrame(t, 0, 100401597)
			binary.LittleEndian.PutUint32(invalid[13:17], 3)
			var refusedEvent string
			packets, err := w.primerTransform(invalid, func(event map[string]any) { refusedEvent, _ = event["kind"].(string) })
			require.NoError(t, err)
			require.Len(t, packets, 1)
			require.Equal(t, "primer_transform_invalid_payment", refusedEvent)
			require.JSONEq(t, string(state), string(w.role.State))
			service := &workflow.ItemService{Store: store, Items: items}
			// A forged account ID cannot access either this character or its vault.
			forged := role
			forged.AccountID = other
			_, _, _, err = service.TransformPrimers(ctx, forged, r, 1)
			require.Error(t, err)
			saved, receipt, applied, err := service.TransformPrimers(ctx, role, r, 1)
			if tc.refused {
				require.Error(t, err)
				require.False(t, applied)
			} else {
				require.NoError(t, err)
				require.True(t, applied)
				require.Equal(t, uint16(17), saved.WireID)
				fromWallet := min(tc.wallet, uint32(35000))
				require.Equal(t, uint32(35000)-fromWallet, receipt.VaultGold)
				// Same original role snapshot replays the same event key exactly once.
				replay, _, repeated, err := service.TransformPrimers(ctx, role, r, 1)
				require.NoError(t, err)
				require.False(t, repeated)
				require.JSONEq(t, string(saved.State), string(replay.State))
			}
			stored, err := store.AdminCharacter(ctx, role.ID)
			require.NoError(t, err)
			bag, err := inventory.ReadBag(stored.State)
			require.NoError(t, err)
			vault, err := store.LoadAccountVault(ctx, account, role.ID)
			require.NoError(t, err)
			if tc.refused {
				require.JSONEq(t, string(state), string(stored.State))
				require.Equal(t, tc.wallet, bag.Gold)
				require.Equal(t, tc.vault, vault.Gold)
			} else {
				require.Equal(t, tc.wallet+tc.vault-35000, bag.Gold+vault.Gold)
				ledger, err := inventory.ReadEquipmentJournal(stored.State)
				require.NoError(t, err)
				require.Equal(t, uint32(1), ledger.Counts[100401592])
				require.Zero(t, ledger.Counts[100401597])
			}
			materials, err := store.AccountMaterials(ctx, account)
			require.NoError(t, err)
			decoded, err := inventory.ReadAccountMaterials(materials)
			require.NoError(t, err)
			require.Empty(t, decoded.Counts)
		})
	}
}
