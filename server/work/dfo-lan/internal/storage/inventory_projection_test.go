package storage

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCashInventoryMigrationPreservesUnownedFields(t *testing.T) {
	original := json.RawMessage(`{"version":"ordinary-bag-v1","gold":12,"coin":3,"items":[{"Template":1,"Amount":7}],"knight_shield_deck":[113370003,113370008,0,0,0],"expand_equip_flags":63,"pet_items":[{"slot":376,"Template":6,"Amount":2}],"creature_experience":{"3":4},"creature_satiety":{"3":77},"creature_loyalty_fraction":{"3":120},"creature_loyalty_updated_at":13,"creature_loyalty_dungeon_key":2,"weapon_skins":[17],"future_inventory_key":{"keep":true}}`)
	projection := struct {
		Version string `json:"version"`
		Gold    uint32 `json:"gold"`
		Items   []any  `json:"items"`
	}{"ordinary-bag-v1", 12, []any{}}
	updated, err := mergeCashInventoryProjection(original, projection)
	if err != nil {
		t.Fatal(err)
	}
	var before, after map[string]any
	json.Unmarshal(original, &before)
	json.Unmarshal(updated, &after)
	for key, value := range before {
		if key != "items" && !reflect.DeepEqual(value, after[key]) {
			t.Fatalf("migration lost %s: %v", key, after[key])
		}
	}
	if len(after["items"].([]any)) != 0 {
		t.Fatal("projection failed to replace owned item field")
	}
}
