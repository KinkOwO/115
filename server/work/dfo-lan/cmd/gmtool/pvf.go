package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/managementdata"
	"fmt"
	"net/http"
	"sort"
)

type preparedGMData struct {
	awarder *inventory.Awarder
	index   *ItemIndex
}

func prepareNativeGMData(p paths, f managementdata.Flags) (*preparedGMData, error) {
	s, err := f.Open()
	if err != nil {
		return nil, err
	}
	if s == nil {
		return nil, fmt.Errorf("native GM data requires catalog-source=pvf")
	}
	a, err := managementdata.Awarder(s, f.DropPolicy, p.bagRules)
	if err != nil {
		return nil, err
	}
	ok := false
	defer func() {
		if !ok {
			a.Equipment.Full.Close()
		}
	}()
	bindings, err := s.ItemIndex("")
	if err != nil {
		return nil, err
	}
	s.ReleaseReadCaches()
	// Current native PVF texts differ from the historical Chinese translation.
	// These files remain optional external display overlays, never source IDs,
	// stats, prices, equipment slots, grant validation or archive identity.
	ix := &ItemIndex{byID: map[uint32]int{}, namesClient: loadNames(p.namesClient), namesZH: loadNames(p.namesZH), namesEN: NameTable{}, source: "pvf", path: f.ArchivePath, count: len(bindings.Items)}
	err = s.VisitItemDisplay(bindings, func(d catalog.ItemDisplay) error {
		kind := d.Kind
		if kind == "avatar" {
			kind = "equipment"
		}
		ix.push(IndexItem{ID: d.ID, Name: d.Name, NameKey: d.NameKey, NativeName: true, Kind: kind, Grade: d.Grade, Rarity: d.Rarity, StackType: d.StackableType, StackLimit: d.StackLimit})
		entry := &ix.items[len(ix.items)-1]
		if kind == "equipment" {
			ix.applySlot(entry, d.EquipmentType, d.MinimumLevel)
		}
		// Gold is a native source entry but cannot be granted as an item.
		entry.Grantable = d.ID != 0
		return nil
	})
	if err != nil {
		return nil, err
	}
	ix.finish()
	ok = true
	return &preparedGMData{awarder: a, index: ix}, nil
}

// The proxy gets classification and stackability from the prepared backend.
// It does not need its own exported item, equipment or loot JSON copies.
func (s *server) handleCatalogMetadata(w http.ResponseWriter, r *http.Request) {
	rows := make(map[uint32][2]string, len(s.index.items))
	for _, it := range s.index.items {
		rows[it.ID] = [2]string{it.Kind, it.Type}
	}
	stackables := make([]uint32, 0)
	for id, it := range s.loot.Items {
		if id != 0 && it.Kind == "stackable" {
			stackables = append(stackables, id)
		}
	}
	sort.Slice(stackables, func(i, j int) bool { return stackables[i] < stackables[j] })
	writeJSON(w, map[string]any{"source": s.loot.Source.Checksum, "items": rows, "stackables": stackables})
}
