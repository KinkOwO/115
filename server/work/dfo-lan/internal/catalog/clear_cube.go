package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// ImportClearCube preserves the minimal storage overlay used by disjoint and
// skills. Grade, Rarity and random Weight are intentionally unprojected zeros;
// their native source values remain in the complete ScriptRecord.
func ImportClearCube(a *pvf.Archive, index ItemIndex) (LootItem, error) {
	var item LootItem
	if a == nil || a.Snapshot().Checksum != OdysseySource || index.Source.Checksum != OdysseySource {
		return item, fmt.Errorf("clear cube source mismatch")
	}
	entry, ok := index.Items[3037]
	if !ok || entry.Kind != "stackable" || entry.StackableType != "[material]" {
		return item, fmt.Errorf("clear cube template missing source index")
	}
	script, err := ReadScript(a, entry.Path)
	if err != nil {
		return item, err
	}
	return LootItem{ID: entry.ID, Kind: entry.Kind, StackableType: entry.StackableType, Script: script}, nil
}
