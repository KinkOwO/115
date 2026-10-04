package database

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database/sqlcgen"
	"errors"
)

type CharacterSlotChange = character.CharacterSlotChange

// changeCharacterSlots follows the native roster maps: pinning into an empty
// grid cell preserves the roster index; swapping exchanges occupants at the
// two existing indices; insertion moves the source before/after the target.
// Database IDs, actor wire IDs and all saved character state travel together.
func changeCharacterSlots(roles []Character, r CharacterSlotChange, capacity int) ([]Character, error) {
	resolve := func(fixed bool, slot uint32) int {
		if !fixed {
			if uint64(slot) < uint64(len(roles)) && roles[slot].FixedSlot == 0 {
				return int(slot)
			}
			return -1
		}
		for i, role := range roles {
			if slot != 0 && uint32(role.FixedSlot) == slot {
				return i
			}
		}
		return -1
	}
	validCell := func(fixed bool, slot uint32) bool {
		return !fixed || (slot > 0 && slot <= 255 && uint64(slot) <= uint64(capacity))
	}
	if capacity < 1 || len(roles) > capacity || !validCell(r.FromFixed, r.From) || !validCell(r.ToFixed, r.To) {
		return nil, errors.New("character grid cell out of range")
	}
	from, to := resolve(r.FromFixed, r.From), resolve(r.ToFixed, r.To)
	if from < 0 {
		return nil, errors.New("source character slot is absent")
	}
	out := append([]Character(nil), roles...)
	if r.Swap {
		if to < 0 {
			return nil, errors.New("swap target character is absent")
		}
		out[from], out[to] = out[to], out[from]
		out[from].FixedSlot, out[to].FixedSlot = roles[from].FixedSlot, roles[to].FixedSlot
		return out, nil
	}
	if r.ToFixed {
		if to >= 0 && to != from {
			return nil, errors.New("target character grid cell is occupied")
		}
		out[from].FixedSlot = byte(r.To)
		return out, nil
	}
	if to < 0 {
		// 0x140225eda: unpin into an entirely empty normal list, index 0.
		if !r.FromFixed || r.To != 0 || !r.Before {
			return nil, errors.New("target character slot is absent")
		}
		for _, role := range roles {
			if role.FixedSlot == 0 {
				return nil, errors.New("target character slot is absent")
			}
		}
		to = 0
	} else {
		if from == to {
			return out, nil
		}
		if !r.Before {
			to++
		}
		if from < to {
			to--
		}
	}
	moved := out[from]
	moved.FixedSlot = 0
	out = append(out[:from], out[from+1:]...)
	out = append(out, Character{})
	copy(out[to+1:], out[to:len(out)-1])
	out[to] = moved
	return out, nil
}

// ChangeCharacterSlots shares the account lock with creation and archival.
// It commits the whole permutation atomically without rewriting any save data.
func (s *Store) ChangeCharacterSlots(ctx context.Context, account int64, r CharacterSlotChange, capacity int) error {
	tx, err := s.engine.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	if _, err = queries.LockAccount(ctx, account); err != nil {
		return err
	}
	rows, err := queries.LockCharacterRoster(ctx, account)
	if err != nil {
		return err
	}
	var roles []Character
	for _, row := range rows {
		roles = append(roles, Character{ID: row.ID, FixedSlot: byte(row.FixedSlot)})
	}
	ordered, err := changeCharacterSlots(roles, r, capacity)
	if err != nil {
		return err
	}
	for i, role := range ordered {
		if err = queries.SaveCharacterSlot(ctx, sqlcgen.SaveCharacterSlotParams{
			AccountID: account, CharacterID: role.ID, RosterOrder: int64(i + 1), FixedSlot: int16(role.FixedSlot),
		}); err != nil {
			return err
		}
	}
	if err = tx.commit(ctx); err != nil {
		return err
	}
	return nil
}
