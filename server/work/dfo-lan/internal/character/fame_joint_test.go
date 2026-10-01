package character

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"testing"
)

func TestJointFameConsumerKeepsLastFieldHashAndExactBoundary(t *testing.T) {
	b := &FameItemImport{rules: &FameRules{Items: map[uint32]fameSourceValue{}, Sources: map[string]string{}}}
	s := catalog.ItemScript{Item: catalog.ItemIndexEntry{ID: 7, Kind: "stackable", Path: "stackable/fame.stk"}, Raw: []byte{1, 2, 3}, Cells: []pvf.Token{{Type: 3, Text: "[fame value]"}, {Type: 0, Value: 10}, {Type: 3, Text: "[fame value]"}, {Type: 0, Value: 20}}}
	if err := b.Consume(s); err == nil {
		t.Fatal("fame accepted fallback")
	}
	s.Exact = true
	if err := b.Consume(s); err != nil {
		t.Fatal(err)
	}
	if b.rules.Items[7].Value != 20 || b.rules.Sources[s.Item.Path] != fmt.Sprintf("%x", sha256.Sum256(s.Raw)) {
		t.Fatal(b.rules)
	}
	s.Cells = []pvf.Token{{Type: 3, Text: "[fame table]"}, {Type: 0, Value: 1}}
	if err := b.Consume(s); err == nil {
		t.Fatal("accepted malformed fame reference")
	}
}
