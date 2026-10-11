package database

import (
	"context"
	"encoding/json"
	"fmt"
)

// Pure account credit callback, invoked once inside the owning purchase transaction.
// It must not start nested transactions or perform external I/O.
type CashAccountObserver func(Character, CashReceipt, *AccountAdventure) (bool, error)
type CharacterAccountTxObserver func(context.Context, *Tx, Character, string, json.RawMessage, *AccountAdventure) (bool, error)

func (s *Store) RegisterCashAccountObserver(observe CashAccountObserver) error {
	if s == nil || !s.adventureEnabled || observe == nil {
		return fmt.Errorf("cash observer requires account storage")
	}
	s.cashAccountObservers = append(s.cashAccountObservers, observe)
	return nil
}
func (s *Store) RegisterCharacterAccountTxObserver(observe CharacterAccountTxObserver) error {
	if s == nil || !s.adventureEnabled || observe == nil {
		return fmt.Errorf("character observer requires account storage")
	}
	s.characterAccountTxObservers = append(s.characterAccountTxObservers, observe)
	return nil
}
func (s *Store) observeCashAccount(ctx context.Context, tx txHandle, role Character, receipt CashReceipt) error {
	if len(s.cashAccountObservers) == 0 {
		return nil
	}
	profile, e := lockAdventure(ctx, tx, role)
	if e != nil {
		return e
	}
	changed := false
	for _, observe := range s.cashAccountObservers {
		c, e := observe(role, receipt, &profile)
		if e != nil {
			return e
		}
		changed = changed || c
	}
	if changed {
		return saveAdventure(ctx, tx, role.AccountID, profile)
	}
	return nil
}
