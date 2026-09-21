package character

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Client CMD507 is sent only for fatigue potions: the slot's stackable must be
// an expert town potion (or a nostrum recovery) to be consumed as fatigue fuel.
func (s *FatigueService) RecoverPotion(ctx context.Context, role storage.Character, c catalog.LootCatalog, slot uint16, now time.Time) (storage.Character, storage.FatigueState, error) {
	var fp storage.FatigueState
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return role, fp, e
	}
	var template uint32
	for _, it := range b.Items {
		if it.Slot == slot {
			template = it.Template
			break
		}
	}
	if template == 0 {
		return role, fp, fmt.Errorf("fatigue potion slot empty")
	}
	item, ok := c.Items[template]
	if !ok || c.Source.Checksum != role.ConfigVersion {
		return role, fp, fmt.Errorf("fatigue potion source missing")
	}
	if item.StackableType != "[expert town potion]" && !strings.Contains(item.Script.Path, "nostrum_recovery") {
		return role, fp, fmt.Errorf("fatigue potion policy mismatch")
	}
	amount := uint16(30)
	values := map[string]int32{}
	for i, t := range item.Script.Cells {
		if i+1 < len(item.Script.Cells) && item.Script.Cells[i+1].Type == 0 {
			values[t.Text] = item.Script.Cells[i+1].Value
		}
	}
	if v, ok := values["[add fatigue]"]; ok && v > 0 {
		amount = uint16(v)
	}
	r := storage.FatigueRecovery{Day: s.day(now), Limit: s.Rules.DailyLimit, Amount: amount, Template: template, DailyUses: 1, Cooldown: 10 * time.Second, Now: now}
	return s.Store.RecoverFatigue(ctx, role.AccountID, role.ID, role.ConfigVersion, r, func(current storage.Character) (json.RawMessage, error) {
		b, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, e
		}
		b, _, e = b.Consume(c, slot, template)
		if e != nil {
			return nil, e
		}
		return inventory.SaveBag(current.State, b)
	})
}
