package elvenmereseason

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/loot"
	"dfolan/internal/savecontract"
	"dfolan/internal/workflow"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSeasonMaintenanceRefusesActiveServerAndLeaseThenPreservesAssets(t *testing.T) {
	ctx := context.Background()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	store := f.Storage()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateCharacterEvents(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "season-tool-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "SeasonTool", ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"keep":42}`), Request: []byte{0}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	svc := &workflow.LootService{Store: store, Loot: &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: savecontract.Identity()}}}, ElvenmereNow: func() time.Time { return time.Now().Add(time.Hour) }}
	operation := strings.Repeat("a", 32)
	if _, err := resetGuarded(ctx, svc, role, operation, "fixture announcement", func() bool { return true }); err == nil {
		t.Fatal("active server accepted")
	}
	release, err := store.HoldAdminGuard(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, guardErr := resetGuarded(ctx, svc, role, operation, "fixture announcement", func() bool { return false })
	release()
	if guardErr == nil {
		t.Fatal("other administrator lease accepted")
	}
	if n, err := f.EventCount(ctx, role.ID); err != nil || n != 0 {
		t.Fatal("refused operation wrote", n, err)
	}
	saved, err := resetGuarded(ctx, svc, role, operation, "fixture announcement", func() bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(saved.State, &fields); err != nil || string(fields["keep"]) != "42" {
		t.Fatal("lost existing state", err)
	}
	if n, err := f.EventCount(ctx, role.ID); err != nil || n != 1 {
		t.Fatal("missing audit", n, err)
	}
	release, err = store.HoldAdminGuard(ctx)
	if err != nil {
		t.Fatal("maintenance left guard held", err)
	}
	release()
}
