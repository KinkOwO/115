package inventory

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type VaultRules struct {
	SourceSHA256  string   `json:"source_sha256"`
	InitialSlots  uint16   `json:"initial_slots"`
	VerifiedSlots []uint16 `json:"verified_slots"`
}

func LoadVaultRules(path string) (VaultRules, error) {
	var r VaultRules
	b, e := os.ReadFile(path)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	if len(r.SourceSHA256) != 64 || !r.allows(r.InitialSlots) {
		return r, fmt.Errorf("invalid source vault configuration")
	}
	return r, nil
}
func (r VaultRules) allows(n uint16) bool {
	for _, v := range r.VerifiedSlots {
		if n != 0 && n == v {
			return true
		}
	}
	return false
}

type VaultService struct {
	Store    *storage.Store
	Rules    VaultRules
	Catalog  catalog.LootCatalog
	BagRules BagRules
}

func (s *VaultService) Bootstrap(ctx context.Context, role storage.Character) ([]byte, error) {
	v, e := s.Store.LoadVault(ctx, role.AccountID, role.ID, s.Rules.InitialSlots, s.Rules.SourceSHA256)
	if e != nil {
		return nil, e
	}
	if v.ConfigVersion != s.Rules.SourceSHA256 || !s.Rules.allows(v.Slots) {
		return nil, fmt.Errorf("vault requires configuration migration")
	}
	return VaultPayload(v)
}

// The initial persistent [] remains valid. Unknown fields and malformed rows
// are rejected rather than silently erasing metadata during a later transfer.
func ReadVault(v storage.VaultState) ([]BagItem, error) {
	var items []BagItem
	d := json.NewDecoder(bytes.NewReader(v.Items))
	d.DisallowUnknownFields()
	if e := d.Decode(&items); e != nil {
		return nil, e
	}
	var tail any
	if e := d.Decode(&tail); e != io.EOF {
		return nil, fmt.Errorf("trailing vault data")
	}
	if items == nil || v.Slots == 0 {
		return nil, fmt.Errorf("invalid vault array/capacity")
	}
	seen := map[uint16]bool{}
	for _, i := range items {
		if i.Slot >= v.Slots || i.Template == 0 || i.Amount == 0 || seen[i.Slot] {
			return nil, fmt.Errorf("invalid vault item")
		}
		seen[i.Slot] = true
	}
	return items, nil
}

func VaultPayload(v storage.VaultState) ([]byte, error) {
	items, e := ReadExtendedVault(v)
	if e != nil {
		return nil, e
	}
	return protocol.PersonalVault(v.Slots, items.Rows())
}
