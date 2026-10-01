package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/quest"
	"dfolan/internal/storage"
	"fmt"
)

func prepareRolePVFDetails(ctx context.Context, characters *character.Service, quests *quest.Service, items *loot.Service, role storage.Character) error {
	if err := characters.PrepareRoleDetails(role); err != nil {
		return err
	}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return err
	}
	seen := map[uint32]bool{}
	warm := func(id uint32, gear bool) error {
		if id == 0 || seen[id] {
			return nil
		}
		seen[id] = true
		if gear {
			catalog := characters.Equipment
			if catalog == nil && items != nil {
				catalog = items.Equipment
			}
			if catalog != nil && catalog.Full != nil {
				_, err := catalog.Definition(id)
				return err
			}
			return nil
		}
		if items != nil && items.Catalog.HasRuntimeDetails() {
			_, err := items.Catalog.ItemScript(id)
			return err
		}
		return nil
	}
	for _, rows := range [][]inventory.BagEquipment{bag.Worn, bag.Equipment} {
		for _, r := range rows {
			if err := warm(r.Template, true); err != nil {
				return fmt.Errorf("prepare equipment %d: %w", r.Template, err)
			}
		}
	}
	for _, rows := range bag.Special {
		for _, r := range rows {
			if err := warm(r.Template, true); err != nil {
				return err
			}
		}
	}
	for _, rows := range [][]inventory.BagItem{bag.Items, bag.PetItems} {
		for _, r := range rows {
			if err := warm(r.Template, false); err != nil {
				return fmt.Errorf("prepare item %d: %w", r.Template, err)
			}
		}
	}
	for _, id := range bag.KnightShieldDeck {
		if err := warm(id, true); err != nil {
			return err
		}
	}
	if err := warm(bag.WeaponSkin, true); err != nil {
		return err
	}
	if quests != nil && quests.Store != nil {
		states, err := quests.Store.Quests(ctx, role.AccountID, role.ID)
		if err != nil {
			return err
		}
		for _, q := range states {
			if q.Status == "accepted" {
				if _, err := quests.Catalog.Definition(uint32(q.ID)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
