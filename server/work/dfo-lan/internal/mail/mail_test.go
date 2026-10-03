package mail

import (
	"encoding/json"
	"testing"
)

func TestPostage(t *testing.T) {
	cases := []struct {
		name      string
		gold      uint32
		itemCount int
		want      uint64
	}{
		{"no gold no items", 0, 0, 100},
		{"one item", 0, 1, 1100},
		{"gold below divisor floors out", 19, 0, 100},
		{"gold at divisor adds one", 20, 0, 101},
		{"gold five percent floors", 39, 2, 2101},
		{"gold cap reached", 200000, 1, 11100},
		{"gold cap with items", 1000000, 3, 13100},
	}
	for _, tc := range cases {
		if got := Postage(tc.gold, tc.itemCount); got != tc.want {
			t.Errorf("%s: Postage(%d, %d) = %d, want %d", tc.name, tc.gold, tc.itemCount, got, tc.want)
		}
	}
}

func TestSendReceiptJSON(t *testing.T) {
	got, err := json.Marshal(SendReceipt{MessageID: 5, RecipientID: 9})
	if err != nil {
		t.Fatalf("marshal receipt: %v", err)
	}
	if want := `{"MessageID":5,"RecipientID":9}`; string(got) != want {
		t.Errorf("SendReceipt JSON = %s, want %s", got, want)
	}
}
