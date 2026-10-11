package inventory

import "encoding/json"

// Register the removed server-owned instance in the same character transaction.
// Eligibility and caps share the native PVF journal rules used by disjointing.
func (s *WearService) registerDestroyedEquipment(state json.RawMessage, gear BagEquipment) (json.RawMessage, []JournalRegistration, []JournalSkip, error) {
	if s.Journal == nil || s.Catalog == nil {
		return state, nil, nil, nil
	}
	ledger, err := ReadEquipmentJournal(state)
	if err != nil {
		return nil, nil, nil, err
	}
	ledger, added, skipped, err := journalRegistrations(ledger, map[uint16]uint32{gear.Slot: gear.Template}, []uint16{gear.Slot}, s.Catalog, s.Journal)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(added) == 0 {
		return state, added, skipped, nil
	}
	next, err := SaveEquipmentJournal(state, ledger)
	return next, added, skipped, err
}
