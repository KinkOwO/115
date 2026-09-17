package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/storage"
	"encoding/json"
	"testing"
)

func TestAutomaticSkillLevelRefresh(t *testing.T) {
	c, err := catalog.LoadCharacters("../../configs/characters.auto-skills-candidate.json")
	if err != nil {
		t.Fatal(err)
	}
	l, err := character.LoadLearningCatalog("../../configs/skills.awakening-candidate.json", c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	service := &character.Service{Catalog: c, Learning: l}
	st := character.State{Level: 15, Advancement: 2, AllJobsPilot: true, SourceSHA256: c.Professions[11].RawSHA256, InitialSkills: c.Professions[11].InitialSkills}
	raw, _ := json.Marshal(st)
	w := worldSession{characters: service, role: storage.Character{Profession: 11, ConfigVersion: c.Source.Checksum, State: raw}}
	plan, err := w.automaticSkillRefresh()
	if err != nil || len(plan) != 1 {
		t.Fatal(plan, err)
	}
	w.characters = nil
	plan, err = w.automaticSkillRefresh()
	if err != nil || len(plan) != 0 {
		t.Fatal("legacy world changed", err)
	}
}
