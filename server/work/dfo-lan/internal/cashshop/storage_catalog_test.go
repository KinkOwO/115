package cashshop

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

func TestShopStorageCatalogDepositsPurchasedKaleido(t *testing.T) {
	p := nativePilot(t, false)
	base := catalog.LootCatalog{Source: p.Config.Source, Items: map[uint32]catalog.LootItem{}}
	n := len(base.Items)
	merged, e := p.StorageCatalog(base)
	if e != nil {
		t.Fatal(e)
	}
	if len(base.Items) != n || merged.Items[15].Kind != "stackable" {
		t.Fatal("drop map modified or cash item missing")
	}
	rules, e := inventory.LoadVaultRules("../../configs/vault.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	bagRules, e := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if e != nil {
		t.Fatal(e)
	}
	s := inventory.VaultService{Rules: rules, Catalog: merged, BagRules: bagRules}
	role := inventory.Role{ConfigVersion: base.Source.SaveIdentity(), State: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":65,"Template":15,"Amount":10}]}}`)}
	v := inventory.VaultState{ConfigVersion: rules.SourceSHA256, Slots: 8, Items: json.RawMessage(`[]`)}
	_, items, e := s.TransferStacks(role, v, protocol.ItemMoveRequest{DestinationList: 2, SourceSlot: 65, SourceItem: 15, Count: 10, Selection: 0xffffffff})
	if e != nil {
		t.Fatal("real current PVF cash item deposit", e)
	}
	v.Items = items
	rows, e := inventory.ReadVaultBagItems(v)
	if e != nil || len(rows) != 1 || rows[0].Amount != 10 {
		t.Fatal(rows, e)
	}
}
