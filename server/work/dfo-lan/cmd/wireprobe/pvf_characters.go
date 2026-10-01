package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

func readPVFCharacterPolicy(path string) (catalog.CharacterRuntimePolicy, error) {
	var policy catalog.CharacterRuntimePolicy
	f, err := os.Open(path)
	if err != nil {
		return policy, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err = d.Decode(&policy); err != nil {
		return policy, err
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return policy, fmt.Errorf("character runtime policy has trailing data")
	}
	return policy, nil
}
func preparePVFCharacters(c *pvfCoreCatalogs, s *gamedata.Source, policy catalog.CharacterRuntimePolicy, path string, i pvfItemInputs) error {
	raw, err := s.Characters("")
	if err != nil {
		return err
	}
	direct, err := catalog.ProjectCharacterRuntime(raw, policy)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := catalog.LoadCharacters(path)
		if err != nil {
			return err
		}
		if old.Source.Checksum != direct.Source.Checksum {
			return fmt.Errorf("character baseline source identity changed")
		}
		if err = verifyPVFCatalog(old, direct); err != nil {
			return fmt.Errorf("characters: %w", err)
		}
	}
	c.sourceCharacters, c.characters = &raw, &direct
	s.ReleaseReadCaches()
	log.Printf("PVF characters prepared: %d professions; raw growtype views retained; existing shortcut/default-command policy and save source identity preserved", len(direct.Professions))
	return nil
}
func (c pvfCoreCatalogs) loadCharacters(path string) (catalog.Characters, error) {
	if c.characters != nil {
		return *c.characters, nil
	}
	return catalog.LoadCharacters(path)
}
