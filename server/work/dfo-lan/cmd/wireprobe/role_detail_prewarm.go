package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/quest"
	"fmt"
)

func prepareRolePVFDetails(ctx context.Context, characters *character.Service, quests *quest.Service, items *loot.Service, role database.Character) error {
	if err := characters.PrepareRoleDetails(role); err != nil {
		return err
	}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return err
	}
	seen := map[uint32]bool{}
	equipment := characters.Equipment
	if equipment == nil && items != nil {
		equipment = items.Equipment
	}
	warm := func(id uint32) error {
		if id == 0 || seen[id] {
			return nil
		}
		seen[id] = true
		// Persisted bag locations are not native template classifications.
		// Prefetch only known definitions; historical unknown entries retain
		// the existing login behavior and their usual use-time validation.
		if items != nil && items.Catalog.Items[id].Kind == "stackable" && items.Catalog.HasRuntimeDetails() {
			_, err := items.Catalog.ItemScript(id)
			return err
		}
		if equipment != nil && equipment.Full != nil && equipment.Full.HasDefinition(id) {
			_, err := equipment.Definition(id)
			return err
		}
		return nil
	}
	for _, rows := range [][]inventory.BagEquipment{bag.Worn, bag.Equipment} {
		for _, r := range rows {
			if err := warm(r.Template); err != nil {
				return fmt.Errorf("prepare equipment %d: %w", r.Template, err)
			}
		}
	}
	for _, rows := range bag.Special {
		for _, r := range rows {
			if err := warm(r.Template); err != nil {
				return err
			}
		}
	}
	for _, rows := range [][]inventory.BagItem{bag.Items, bag.PetItems} {
		for _, r := range rows {
			if err := warm(r.Template); err != nil {
				return fmt.Errorf("prepare item %d: %w", r.Template, err)
			}
		}
	}
	for _, id := range bag.KnightShieldDeck {
		if err := warm(id); err != nil {
			return err
		}
	}
	if err := warm(bag.WeaponSkin); err != nil {
		return err
	}
	if quests != nil && quests.Store != nil {
		states, err := quests.Store.Quests(ctx, role.AccountID, role.ID)
		if err != nil {
			return err
		}
		for _, q := range states {
			if _, known := quests.Catalog.Quests[uint32(q.ID)]; q.Status == "accepted" && known {
				if _, err := quests.Catalog.Definition(uint32(q.ID)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
