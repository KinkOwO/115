package character

import (
	"context"
	"fmt"
)

// These presentation values come from storage outside the character JSON.
// Capture them before a character transaction: profile-frame restoration can
// initialize a row and therefore must never open a nested SQLite write.
type entryProjectionSnapshot struct {
	account, character int64
	version            string
	content            [28]byte
	growth             byte
	frames             [2]uint32
	adventure          uint32
}

func (v *entryProjectionSnapshot) check(role Character) error {
	if v.account != role.AccountID || v.character != role.ID || v.version != role.ConfigVersion {
		return fmt.Errorf("entry projection snapshot belongs to another character")
	}
	return nil
}

// WithEntryProjectionSnapshot returns a request-local service copy. Domain
// mutations still use the authoritative locked role and the original Store;
// only the external presentation lookups are frozen for packet preflight.
// The caller must not retain this copy across requests or character selections.
func (s *Service) WithEntryProjectionSnapshot(role Character) (*Service, error) {
	content, err := s.roleContentClearFlags(role)
	if err != nil {
		return nil, err
	}
	growth, err := s.roleGrowthEffectFlags(role)
	if err != nil {
		return nil, err
	}
	frames, err := s.profileFrames(role)
	if err != nil {
		return nil, err
	}
	view := *s
	var adventure uint32
	if s.Store != nil {
		adventure, err = s.Store.AdventureLevel(context.Background(), role.AccountID, role.ID)
		if err != nil {
			return nil, err
		}
	}
	view.entrySnapshot = &entryProjectionSnapshot{role.AccountID, role.ID, role.ConfigVersion, content, growth, frames, adventure}
	return &view, nil
}
