package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/gamedata"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

func preparePVFScriptWarps(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["script-warps"] {
		return nil
	}
	if c.dungeons == nil {
		return fmt.Errorf("native script warps require native dungeons")
	}
	f, err := os.Open(i.scriptWarpPolicyPath)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	var policy catalog.ScriptWarpPolicy
	if err = decoder.Decode(&policy); err != nil {
		return err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("script warp policy has trailing data")
	}
	direct, err := s.ScriptWarpRoutes(*c.dungeons, policy)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := dungeon.EmbeddedScriptWarpRoutes()
		if err != nil {
			return err
		}
		if len(old) != len(direct) {
			return fmt.Errorf("script warp scope changed")
		}
		for n := range old {
			if old[n].Source != direct[n].Source {
				return fmt.Errorf("script warp source identity changed")
			}
		}
		if err = verifyPVFCatalog(old, direct); err != nil {
			return fmt.Errorf("script warps: %w", err)
		}
	}
	c.scriptWarps = direct
	s.ReleaseReadCaches()
	log.Printf("PVF script warps prepared: %d native map/CMT/object/custom-action/maze bindings; witnessed transition records and key-room admission retained", len(direct))
	return nil
}
func (c pvfCoreCatalogs) installScriptWarps() (func(), error) {
	if c.scriptWarps == nil {
		return func() {}, nil
	}
	return dungeon.InstallScriptWarpRoutes(c.scriptWarps)
}
