package character

import (
	"encoding/json"
	"testing"
)

func TestWornCreatureExtraction(t *testing.T) {
	raw := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","worn":[{"slot":26,"template":63000}]}}`)
	id, name := wornCreature(raw)
	if id != 63000 || name != "Faras" {
		t.Fatalf("expected 63000 Faras, got %d %q", id, name)
	}
}
