package inventory

import (
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func TestFullEquipmentCacheBoundsPayloadAndRejectsClosedCachedReads(t *testing.T) {
	c := &FullEquipmentCatalog{}
	held := EquipmentDefinition{ID: 1, Fields: map[string][]pvf.Token{"[name]": {{Type: 6, Text: "held"}}}}
	c.cacheDefinition(1, held)
	for id := uint32(2); id < 42; id++ {
		c.cache.Get(1)
		c.cacheDefinition(id, EquipmentDefinition{ID: id, Fields: map[string][]pvf.Token{"large": {{Text: strings.Repeat("x", 1024*1024)}}}})
	}
	if n, count := c.cache.Usage(); n > 32*1024*1024 || count > 2048 {
		t.Fatal(n, count)
	}
	if _, ok := c.cache.Get(1); !ok {
		t.Fatal("hot equipment discarded")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Definition(1); err == nil {
		t.Fatal("closed cached equipment allowed")
	}
	if held.Fields["[name]"][0].Text != "held" {
		t.Fatal("held definition changed")
	}
}
