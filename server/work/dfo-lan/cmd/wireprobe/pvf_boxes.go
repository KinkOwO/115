package main

import (
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

func readPVFBoxPolicy(path string) (inventory.BoxSourcePolicy, error) {
	var p inventory.BoxSourcePolicy
	f, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err = d.Decode(&p); err != nil {
		return p, err
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("box policy has trailing data")
	}
	return p, nil
}
func preparePVFBoxes(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["boxes"] {
		return nil
	}
	if c.items == nil {
		return fmt.Errorf("native boxes require native item index")
	}
	p, err := readPVFBoxPolicy(i.boxPolicyPath)
	if err != nil {
		return err
	}
	direct, err := s.Boxes(*c.items, p)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		path := i.boxesPath
		if path == "" {
			path = filepath.Join(filepath.Dir(i.indexPath), "boxes.json")
		}
		old, err := inventory.LoadBoxes(path)
		if err != nil {
			return err
		}
		if err = auditPVFBoxes(old, direct); err != nil {
			return err
		}
	}
	c.boxes = direct
	s.ReleaseReadCaches()
	log.Printf("PVF boxes prepared: tables=%d rewards=%d raw sources=%d; unique native COS material bindings and existing point/grant rules retained", direct.TableCount(), direct.RewardCount(), len(direct.Sources))
	return nil
}
func auditPVFBoxes(old, direct *inventory.BoxCatalog) error {
	if old == nil || direct == nil || direct.Source != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80" || (old.Source != "inner Script.pvf .cos content scripts" && old.Source != direct.Source) {
		return fmt.Errorf("unknown box source provenance")
	}
	// The old artifact omitted raw hashes. Audit every exported source field;
	// the native catalog additionally records exact archive/file identities.
	baseline, native := *old, *direct
	baseline.Sources = nil
	native.Sources = nil
	if err := verifyPVFCatalog(baseline, native); err != nil {
		return fmt.Errorf("boxes: %w", err)
	}
	return nil
}
func (c pvfCoreCatalogs) loadBoxes(path, source string) (*inventory.BoxCatalog, error) {
	if c.boxes != nil {
		if c.boxes.Source != source {
			return nil, fmt.Errorf("native box source differs from save catalog")
		}
		return c.boxes, nil
	}
	return inventory.LoadBoxes(path)
}
