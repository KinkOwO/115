package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"fmt"
	"log"
	"path/filepath"
)

func preparePVFSelectionBoxes(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["selection-boxes"] {
		return nil
	}
	if c.items == nil {
		return fmt.Errorf("native selection boxes require native item index")
	}
	policy, err := catalog.ReadSelectionBoxPolicy(i.selectionPolicyPath)
	if err != nil {
		return err
	}
	direct, err := s.SelectionBoxes(*c.items, policy)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		path := i.selectionBoxesPath
		if path == "" {
			path = filepath.Join(filepath.Dir(i.indexPath), "selection-boxes-candidate.json")
		}
		old, err := catalog.LoadSelectionBoxes(path)
		if err != nil {
			return err
		}
		if old.Source.Checksum != direct.Source.Checksum {
			return fmt.Errorf("selection boxes baseline source mismatch")
		}
		if err := verifyPVFCatalog(old, direct); err != nil {
			return fmt.Errorf("selection boxes: %w", err)
		}
	}
	c.selectionBoxes = direct
	s.ReleaseReadCaches()
	log.Printf("PVF selection boxes prepared: boxes=%d fixed=%d unparsed=%d; bounded server selection retained", len(direct.Boxes), len(direct.Fixed), len(direct.Unparsed))
	return nil
}

func (c pvfCoreCatalogs) loadSelectionBoxes(path string) (*catalog.SelectionBoxes, error) {
	if c.selectionBoxes != nil {
		return c.selectionBoxes, nil
	}
	return catalog.LoadSelectionBoxes(path)
}
