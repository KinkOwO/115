package character

import (
	"encoding/json"
	"testing"
)

func TestStateLevel(t *testing.T) {
	cases := []struct {
		name string
		raw  json.RawMessage
		want byte
	}{
		{"level", json.RawMessage(`{"level":42}`), 42},
		{"missing", json.RawMessage(`{}`), 0},
		{"nil", nil, 0},
		{"malformed", json.RawMessage(`{"level":`), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stateLevel(tc.raw); got != tc.want {
				t.Fatalf("stateLevel(%q) = %d, want %d", tc.raw, got, tc.want)
			}
		})
	}
}
