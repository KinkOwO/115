package database

import (
	"encoding/json"
	"fmt"
)

// CharacterEventAccountObserver updates source-driven account progress inside
// the owning character event transaction. Replay does not invoke observers.
// Observers must perform no external I/O or nested storage transactions.
type CharacterEventAccountObserver func(Character, string, json.RawMessage, *AccountAdventure) (bool, error)

// RegisterCharacterEventAccountObserver is startup configuration only, before
// serving connections. Disabled/default profiles register no observers.
func (s *Store) RegisterCharacterEventAccountObserver(observe CharacterEventAccountObserver) error {
	if s == nil || !s.adventureEnabled || observe == nil {
		return fmt.Errorf("account event observer requires prepared account storage")
	}
	s.characterEventObservers = append(s.characterEventObservers, observe)
	return nil
}
