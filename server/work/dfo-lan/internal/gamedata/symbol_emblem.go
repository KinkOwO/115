package gamedata

import "dfolan/internal/inventory"

// SymbolEmblems reads native CTPs through the selected source. It intentionally
// does not advertise a ready gameplay domain: the current US client stat
// consumer and native equipment-tab controller have not been verified.
func (s *Source) SymbolEmblems() (*inventory.SymbolEmblemRules, error) {
	if s == nil {
		return inventory.ImportSymbolEmblemRules(nil)
	}
	return inventory.ImportSymbolEmblemRules(s.archive)
}
