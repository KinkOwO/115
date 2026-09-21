package protocol

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func loadSortItemVector(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("testdata/native_sort_item20.json")
	if e != nil {
		t.Fatal(e)
	}
	var doc struct {
		PayloadHex string `json:"payload_hex"`
	}
	if e = json.Unmarshal(b, &doc); e != nil {
		t.Fatal(e)
	}
	p, e := hex.DecodeString(doc.PayloadHex)
	if e != nil {
		t.Fatal(e)
	}
	return p
}

// The captured request is a total permutation over the client's 380 inventory
// slots; adopting it means slot i ends up holding the item of slot perm[i].
func TestDecodeCapturedSortItem(t *testing.T) {
	r, e := DecodeSortItem(loadSortItemVector(t))
	if e != nil {
		t.Fatal(e)
	}
	if r.List != 0 {
		t.Fatalf("list = %d, want 0", r.List)
	}
	if len(r.Slots) != 380 {
		t.Fatalf("slot count = %d, want 380", len(r.Slots))
	}
	seen := map[uint16]bool{}
	for _, v := range r.Slots {
		seen[v] = true
	}
	if len(seen) != 380 {
		t.Fatalf("table is not a permutation: %d distinct values", len(seen))
	}
	cycle := map[int]uint16{18: 21, 19: 22, 21: 18, 22: 19}
	for i, v := range r.Slots {
		if w, ok := cycle[i]; ok {
			if v != w {
				t.Fatalf("slot %d -> %d, want %d", i, v, w)
			}
			continue
		}
		if v != uint16(i) {
			t.Fatalf("slot %d -> %d, want identity", i, v)
		}
	}
}

func TestDecodeSortItemRejectsMalformedRequests(t *testing.T) {
	good := loadSortItemVector(t)
	if _, e := DecodeSortItem(good[:100]); e == nil {
		t.Fatal("truncated request accepted")
	}
	duplicate := append([]byte(nil), good...)
	duplicate[6] = duplicate[4]
	if _, e := DecodeSortItem(duplicate); e == nil {
		t.Fatal("table with a duplicate entry accepted")
	}
	padded := append([]byte(nil), good...)
	padded[len(padded)-1] = 7
	if _, e := DecodeSortItem(padded); e == nil {
		t.Fatal("request with nonzero padding accepted")
	}
	if _, e := DecodeSortItem(make([]byte, 4)); e == nil {
		t.Fatal("short request accepted")
	}
}
