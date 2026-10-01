package main

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// checkReport is called only after source preparation and before installing
// runtime globals, opening storage, creating capture files or listeners.
func (c pvfCoreCatalogs) checkReport(selection string) (map[string]any, error) {
	domains, err := parsePVFCatalogSelection(selection)
	if err != nil {
		return nil, err
	}
	checksum, err := hex.DecodeString(c.sourceChecksum)
	if err != nil || len(checksum) != 32 || len(domains) == 0 {
		return nil, fmt.Errorf("no prepared PVF source for catalog check")
	}
	ordered := make([]string, 0, len(domains))
	for _, domain := range strings.Split(pvfSupportedDomains, ",") {
		if domains[domain] {
			ordered = append(ordered, domain)
		}
	}
	r := map[string]any{"source": c.sourceChecksum, "domain_count": len(domains), "domains": ordered, "storage_accessed": false, "runtime_started": false}
	if c.characters != nil {
		r["professions"] = len(c.characters.Professions)
	}
	if c.quests != nil {
		r["quests"] = len(c.quests.Quests)
	}
	if c.items != nil {
		r["items"] = len(c.items.Items)
	}
	if c.equipment != nil {
		r["equipment_bindings"] = len(c.equipment.Records)
	}
	if c.selection != nil {
		r["equipment_selection"] = len(c.selection.Rows)
	}
	if c.dungeons != nil {
		r["dungeons"] = len(c.dungeons.Dungeons)
	}
	if c.cashshop != nil {
		r["cashshop_products"] = c.cashshop.EnabledCount()
	}
	if c.boxes != nil {
		r["boxes"] = c.boxes.TableCount()
	}
	if c.itemShops != nil {
		r["item_shops"] = len(c.itemShops.Shops)
	}
	return r, nil
}
