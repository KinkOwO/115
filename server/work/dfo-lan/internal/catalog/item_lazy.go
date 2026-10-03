package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// Global stack limits and classifications remain compact typed projections.
// Details do not alter drop-pool membership or permit unknown native IDs.
func (c *LootCatalog) EnableRuntimeDetails(a *pvf.Archive, index ItemIndex) error {
	if c.details != nil {
		return nil
	}
	if c.Source.Checksum != index.Source.Checksum {
		return fmt.Errorf("item detail source mismatch")
	}
	refs := map[uint32]ScriptRecord{}
	for id, r := range index.Items {
		if r.Kind == "stackable" {
			refs[id] = ScriptRecord{Path: ResolveScriptPath(a, r.Path)}
		}
	}
	d, err := NewScriptDetails(a, refs, func(_ uint32, s ScriptRecord) ScriptRecord { return s }, ScriptBytes)
	if err != nil {
		return err
	}
	c.details = d
	return nil
}
func (c LootCatalog) ItemScript(id uint32) (ScriptRecord, error) {
	item, ok := c.Items[id]
	if !ok {
		return ScriptRecord{}, fmt.Errorf("item absent from runtime index: %d", id)
	}
	if c.details == nil || item.Kind != "stackable" {
		return item.Script, nil
	}
	// A small explicit overlay not present in native LISTs retains its original
	// definition. This does not widen the native query scope.
	if _, ok := c.details.refs[id]; !ok {
		return item.Script, nil
	}
	return c.details.Get(id)
}
func (c LootCatalog) HasRuntimeDetails() bool { return c.details != nil }
func (c LootCatalog) CloseDetails() error {
	if c.details == nil {
		return nil
	}
	return c.details.Close()
}
