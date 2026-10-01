package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestJointEnhancementConsumerSkipsFallbackAndRejectsBrokenWeights(t *testing.T) {
	b := &EnhancementItemImport{c: &EnhancementCatalog{}}
	s := catalog.ItemScript{Item: catalog.ItemIndexEntry{ID: 7, Kind: "stackable", Path: "stackable/ticket.stk"}, Cells: []pvf.Token{{Type: 3, Text: "[amplification random value]"}, {Type: 0, Value: 5}}}
	if err := b.Consume(s); err != nil || len(b.c.Grimoires.Grimoires) != 0 {
		t.Fatal("fallback entered enhancement projection", err)
	}
	s.Exact = true
	if err := b.Consume(s); err == nil {
		t.Fatal("accepted incomplete weight pair")
	}
	s.Cells = append(s.Cells, pvf.Token{Type: 0, Value: 100})
	if err := b.Consume(s); err != nil {
		t.Fatal(err)
	}
	if len(b.c.Grimoires.Grimoires) != 1 || b.c.Grimoires.Grimoires[0].Random[0].Weight != 100 {
		t.Fatal(b.c.Grimoires)
	}
}
