package inventory

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"fmt"
	"strings"
)

// Character progress, shared by every equipped oath. Options use the native
// zero-based option index, distinct from the core's 1..3 selection ledger.
type OathTruthState struct {
	Level   byte     `json:"level"`
	Options [10]byte `json:"options"`
}

func ReadOathTruth(raw json.RawMessage) (OathTruthState, error) {
	var fields map[string]json.RawMessage
	var out OathTruthState
	if err := json.Unmarshal(raw, &fields); err != nil {
		return out, err
	}
	if v := fields["oath_truth"]; len(v) != 0 {
		var shape struct {
			Options []byte `json:"options"`
		}
		if json.Unmarshal(v, &shape) != nil || len(shape.Options) != 10 {
			return out, fmt.Errorf("invalid saved truth option array")
		}
		if err := json.Unmarshal(v, &out); err != nil {
			return out, err
		}
	}
	if out.Level > 9 {
		return out, fmt.Errorf("invalid saved truth level")
	}
	return out, nil
}
func SaveOathTruth(raw json.RawMessage, truth OathTruthState) (json.RawMessage, error) {
	if truth.Level > 9 {
		return nil, fmt.Errorf("invalid truth level")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("invalid character JSON")
	}
	var values map[string]json.RawMessage
	if v := fields["oath_truth"]; len(v) != 0 {
		if err := json.Unmarshal(v, &values); err != nil {
			return nil, err
		}
	}
	if values == nil {
		values = map[string]json.RawMessage{}
	}
	values["level"], _ = json.Marshal(truth.Level)
	values["options"], _ = json.Marshal(truth.Options)
	fields["oath_truth"], _ = json.Marshal(values)
	return json.Marshal(fields)
}
func ValidateOathTruth(t OathTruthState, r *catalog.OathTruthRules) error {
	if r == nil || int(t.Level) > r.MaxLevel {
		return fmt.Errorf("truth progress exceeds source maximum")
	}
	for n := 1; n <= int(t.Level); n++ {
		if int(t.Options[n]) >= len(r.Stages[n].Options) {
			return fmt.Errorf("invalid saved truth option")
		}
	}
	if t.Options[0] != 0 {
		return fmt.Errorf("reserved truth option is not zero")
	}
	for n := int(t.Level) + 1; n < len(t.Options); n++ {
		if t.Options[n] != 0 {
			return fmt.Errorf("option in locked truth stage")
		}
	}
	return nil
}

// Costs are resolved before this call from the prepared PVF. All modifications
// remain local until the surrounding SQLite transaction commits them together.
func UpgradeOathTruth(raw json.RawMessage, shared AccountVaultState, r *catalog.OathTruthRules, next byte, payment uint32) (json.RawMessage, AccountVaultState, error) {
	state, vault, _, err := UpgradeOathTruthWithVaults(raw, shared, [2]VaultState{}, r, next, payment)
	return state, vault, err
}
func UpgradeOathTruthWithVaults(raw json.RawMessage, shared AccountVaultState, personal [2]VaultState, r *catalog.OathTruthRules, next byte, payment uint32) (json.RawMessage, AccountVaultState, [2]VaultState, error) {
	fail := func(err error) (json.RawMessage, AccountVaultState, [2]VaultState, error) {
		return nil, shared, personal, err
	}
	t, err := ReadOathTruth(raw)
	if err != nil {
		return fail(err)
	}
	if err = ValidateOathTruth(t, r); err != nil {
		return fail(err)
	}
	if next != t.Level+1 || int(next) > r.MaxLevel {
		return fail(fmt.Errorf("stale or locked truth upgrade"))
	}
	var character struct {
		Level int `json:"level"`
	}
	if json.Unmarshal(raw, &character) != nil || character.Level < 115 {
		return fail(fmt.Errorf("truth requires level 115"))
	}
	stage := r.Stages[int(next)]
	// The confirmation callback snapshots window+6a0 (zero-based) + 1,
	// then 1405324d0 forwards that one-based PVF group unchanged.
	if payment == 0 || payment > uint32(len(stage.Costs)) {
		return fail(fmt.Errorf("invalid truth payment index"))
	}
	bag, err := ReadBag(raw)
	if err != nil {
		return fail(err)
	}
	for _, cost := range stage.Costs[int(payment)] {
		if cost.Template == 0 {
			taken := min(cost.Amount, bag.Gold)
			remaining := cost.Amount - taken
			if shared.Gold < remaining {
				return fail(fmt.Errorf("truth gold insufficient"))
			}
			bag.Gold -= taken
			shared.Gold -= remaining
			continue
		}
		remaining := cost.Amount
		for _, template := range append([]uint32{cost.Template}, r.Alternatives[cost.Template]...) {
			before := remaining
			for _, row := range bag.Items {
				if row.Template == template {
					remaining -= min(remaining, row.Amount)
				}
			}
			if remaining < before {
				bag, _, err = consumeBagTemplate(bag, template, before-remaining)
				if err != nil {
					return fail(err)
				}
			}
			// The native writer carries no warehouse selector. The checkbox gates
			// client eligibility; issued requests consume bag, personal, then shared.
			for i := range personal {
				personal[i].Items, remaining, err = spendTruthVault(personal[i].Items, template, remaining)
				if err != nil {
					return fail(err)
				}
			}
			shared.Items, remaining, err = spendTruthVault(shared.Items, template, remaining)
			if err != nil {
				return fail(err)
			}
			if remaining == 0 {
				break
			}
		}
		if remaining != 0 {
			return fail(fmt.Errorf("truth material %d insufficient", cost.Template))
		}
	}
	t.Options[next] = 0
	t.Level = next
	state, err := saveTruthBag(raw, bag)
	if err != nil {
		return fail(err)
	}
	state, err = SaveOathTruth(state, t)
	if err != nil {
		return fail(err)
	}
	return state, shared, personal, nil
}

// Only wallet and stack quantities change. Retain every opaque item property,
// worn record and future inventory field rather than reserializing them all.
func saveTruthBag(raw json.RawMessage, bag Bag) (json.RawMessage, error) {
	state, err := SaveBag(raw, bag)
	if err != nil {
		return nil, err
	}
	var before, after map[string]json.RawMessage
	if json.Unmarshal(raw, &before) != nil || json.Unmarshal(state, &after) != nil {
		return nil, fmt.Errorf("invalid truth inventory document")
	}
	var oldInventory, nextInventory map[string]json.RawMessage
	if old := before["inventory"]; len(old) > 0 {
		if err = json.Unmarshal(old, &oldInventory); err != nil {
			return nil, err
		}
	}
	if err = json.Unmarshal(after["inventory"], &nextInventory); err != nil {
		return nil, err
	}
	for key, value := range oldInventory {
		if !strings.EqualFold(key, "items") && !strings.EqualFold(key, "gold") {
			nextInventory[key] = value
		}
	}
	if old := oldInventory["items"]; len(old) > 0 {
		var rows []json.RawMessage
		if err = json.Unmarshal(old, &rows); err != nil {
			return nil, err
		}
		remaining := map[uint16]BagItem{}
		for _, item := range bag.Items {
			remaining[item.Slot] = item
		}
		next := make([]json.RawMessage, 0, len(bag.Items))
		for _, row := range rows {
			var item BagItem
			if err = json.Unmarshal(row, &item); err != nil {
				return nil, err
			}
			current, ok := remaining[item.Slot]
			if !ok {
				continue
			}
			if item.Amount != current.Amount {
				var fields map[string]json.RawMessage
				if err = json.Unmarshal(row, &fields); err != nil {
					return nil, err
				}
				for key := range fields {
					if strings.EqualFold(key, "amount") {
						delete(fields, key)
					}
				}
				fields["Amount"], _ = json.Marshal(current.Amount)
				row, err = json.Marshal(fields)
				if err != nil {
					return nil, err
				}
			}
			next = append(next, row)
		}
		nextInventory["items"], err = json.Marshal(next)
		if err != nil {
			return nil, err
		}
	}
	after["inventory"], err = json.Marshal(nextInventory)
	if err != nil {
		return nil, err
	}
	return json.Marshal(after)
}
func spendTruthVault(raw json.RawMessage, template, amount uint32) (json.RawMessage, uint32, error) {
	if amount == 0 || len(raw) == 0 {
		return raw, amount, nil
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(raw, &rows); err != nil || rows == nil {
		return nil, amount, fmt.Errorf("invalid truth material vault")
	}
	next := make([]json.RawMessage, 0, len(rows))
	seen := map[uint16]bool{}
	for _, rawRow := range rows {
		var row VaultItem
		var fields map[string]json.RawMessage
		if json.Unmarshal(rawRow, &row) != nil || json.Unmarshal(rawRow, &fields) != nil || fields == nil || seen[row.Slot] {
			return nil, amount, fmt.Errorf("invalid truth vault row")
		}
		seen[row.Slot] = true
		if amount > 0 && row.Template == template && !row.IsEquip && row.Equipment == nil {
			take := min(amount, row.Amount)
			amount -= take
			if take == row.Amount {
				continue
			}
			fields["amount"], _ = json.Marshal(row.Amount - take)
			rawRow, _ = json.Marshal(fields)
		}
		next = append(next, rawRow)
	}
	encoded, err := json.Marshal(next)
	return encoded, amount, err
}
func SelectOathTruth(raw json.RawMessage, r *catalog.OathTruthRules, slot, option byte) (json.RawMessage, error) {
	var character struct {
		Level int `json:"level"`
	}
	if json.Unmarshal(raw, &character) != nil || character.Level < 115 {
		return nil, fmt.Errorf("truth requires level 115")
	}
	t, err := ReadOathTruth(raw)
	if err != nil {
		return nil, err
	}
	if err = ValidateOathTruth(t, r); err != nil {
		return nil, err
	}
	if slot == 0 || slot > t.Level || int(option) >= len(r.Stages[int(slot)].Options) {
		return nil, fmt.Errorf("locked truth slot or invalid option")
	}
	t.Options[slot] = option
	return SaveOathTruth(raw, t)
}
