package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

func preparePVFLayerRevisits(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["layer-revisits"] {
		return nil
	}
	if c.dungeons == nil {
		return fmt.Errorf("native layer revisits require native dungeons")
	}
	f, err := os.Open(i.layerRevisitPolicyPath)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	var policy catalog.LayerRevisitPolicy
	if err = decoder.Decode(&policy); err != nil {
		return err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("layer revisit policy has trailing data")
	}
	direct, err := s.LayerRevisits(*c.dungeons, policy)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old := clonePVFDungeons(*c.dungeons)
		if err = catalog.AttachLayerRevisits(&old, filepath.Join(filepath.Dir(fullDungeonAuditPath(i)), "dungeons.layer-revisits.json")); err != nil {
			return err
		}
		if err = verifyPVFCatalog(old.LayerRevisits, direct.Scenes); err != nil {
			return fmt.Errorf("layer revisits: %w", err)
		}
	}
	c.layerRevisits = &direct
	s.ReleaseReadCaches()
	log.Printf("PVF layer revisits prepared: %d native final-layer/map/ACT/CMT bindings; witnessed records and base cache restoration retained", len(direct.Scenes))
	return nil
}
func (c pvfCoreCatalogs) attachLayerRevisits(d *catalog.DungeonCatalog, path string) error {
	if c.layerRevisits != nil {
		return catalog.ApplyLayerRevisits(d, *c.layerRevisits)
	}
	return catalog.AttachLayerRevisits(d, path)
}
