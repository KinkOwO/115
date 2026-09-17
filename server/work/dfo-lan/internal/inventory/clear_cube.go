package inventory

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"fmt"
	"os"
)

func WithClearCube(base catalog.LootCatalog, path string) (catalog.LootCatalog, error) {
	raw, e := os.ReadFile(path)
	if e != nil {
		return base, e
	}
	var item catalog.LootItem
	if e = json.Unmarshal(raw, &item); e != nil {
		return base, e
	}
	if base.Source.Checksum != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80" || item.ID != 3037 || item.Kind != "stackable" || item.StackableType != "[material]" || item.Script.SHA256 != "c6b47f4db2c1ba08b809aa54e6a512becba560199699c95cd9a264caf07077d5" {
		return base, fmt.Errorf("clear cube source mismatch")
	}
	out := base
	out.Items = make(map[uint32]catalog.LootItem, len(base.Items)+1)
	for id, v := range base.Items {
		out.Items[id] = v
	}
	out.Items[item.ID] = item
	return out, nil
}
