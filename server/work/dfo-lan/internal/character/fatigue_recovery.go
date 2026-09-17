package character

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"time"
)

// This source-backed daily potion is the one observed in the shop capture.
// Other action types and unlimited-use products need their own replay contract.
func (s *FatigueService) RecoverPotion(ctx context.Context, role storage.Character, c catalog.LootCatalog, slot uint16, now time.Time) (storage.Character, storage.FatigueState, error) {
	var fp storage.FatigueState
	item, ok := c.Items[10000541]
	if !ok || item.Script.Path != "stackable/event/vendingmachine/nostrum_recovery_10000541.stk" || c.Source.Checksum != role.ConfigVersion {
		return role, fp, fmt.Errorf("fatigue potion source missing")
	}
	values := map[string]int32{}
	for i, t := range item.Script.Cells {
		if i+1 < len(item.Script.Cells) && item.Script.Cells[i+1].Type == 0 {
			values[t.Text] = item.Script.Cells[i+1].Value
		}
	}
	if values["[add fatigue]"] != 30 || values["[total usable count]"] != 1 || values["[cool time]"] != 10000 || values["[use action packet]"] != 1 {
		return role, fp, fmt.Errorf("fatigue potion policy mismatch")
	}
	r := storage.FatigueRecovery{Day: s.day(now), Limit: s.Rules.DailyLimit, Amount: 30, Template: 10000541, DailyUses: 1, Cooldown: 10 * time.Second, Now: now}
	return s.Store.RecoverFatigue(ctx, role.AccountID, role.ID, role.ConfigVersion, r, func(current storage.Character) (json.RawMessage, error) {
		b, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, e
		}
		b, _, e = b.Consume(c, slot, r.Template)
		if e != nil {
			return nil, e
		}
		return inventory.SaveBag(current.State, b)
	})
}
