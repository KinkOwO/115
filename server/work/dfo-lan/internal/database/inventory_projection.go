package database

import "encoding/json"

// Historical cash migrations only own the fields declared in their local
// projection. Overlay those fields, preserving every other inventory key:
// shield deck, expansion flags, pets, skins, and future additive save fields.
func mergeCashInventoryProjection(original json.RawMessage, projection any) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(original, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		fields = map[string]json.RawMessage{}
	}
	raw, err := json.Marshal(projection)
	if err != nil {
		return nil, err
	}
	var updates map[string]json.RawMessage
	if err = json.Unmarshal(raw, &updates); err != nil {
		return nil, err
	}
	for key, value := range updates {
		fields[key] = value
	}
	return json.Marshal(fields)
}
