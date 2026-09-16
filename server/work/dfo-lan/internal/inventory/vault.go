package inventory

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
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
	Store *storage.Store
	Rules VaultRules
}

func (s *VaultService) Bootstrap(ctx context.Context, role storage.Character) ([]byte, error) {
	v, e := s.Store.LoadVault(ctx, role.AccountID, role.ID, s.Rules.InitialSlots, s.Rules.SourceSHA256)
	if e != nil {
		return nil, e
	}
	if v.ConfigVersion != s.Rules.SourceSHA256 || !s.Rules.allows(v.Slots) {
		return nil, fmt.Errorf("vault requires configuration migration")
	}
	var items []json.RawMessage
	if e = json.Unmarshal(v.Items, &items); e != nil {
		return nil, e
	}
	if len(items) != 0 {
		return nil, fmt.Errorf("nonempty vault item encoding is pending; stored items preserved")
	}
	return protocol.EmptyPersonalVault(v.Slots)
}
