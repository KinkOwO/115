package quest

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
)

type Service struct {
	Store       *storage.Store
	Catalog     catalog.QuestCatalog
	Professions catalog.Characters
	Progression *character.ProgressionService
	Inventory   *inventory.Awarder
	// Odyssey carries the quest tables of aradodyssey.etc; nil simply
	// disables the graduation mainline pass.
	Odyssey  *catalog.OdysseyGrowth
	Dungeons *catalog.DungeonCatalog
	index    *Index
}

func jobAllowed(jobs []string, job string) bool {
	// An omitted [job] condition in the source applies to every profession.
	if len(jobs) == 0 {
		return true
	}
	for _, j := range jobs {
		if j == "[all]" || j == job {
			return true
		}
	}
	return false
}

func prerequisitesMet(groups [][]uint32, status map[uint32]string) bool {
	if len(groups) == 0 {
		return true
	}
	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		complete := true
		for _, id := range group {
			if status[id] != "completed" {
				complete = false
				break
			}
		}
		if complete {
			return true
		}
	}
	return false
}

func (s *Service) Accept(ctx context.Context, role storage.Character, id uint16) (storage.QuestState, error) {
	d, sourceErr := s.Catalog.Definition(uint32(id))
	if sourceErr != nil {
		return storage.QuestState{}, sourceErr
	}
	if len(d.Pending) > 0 {
		return storage.QuestState{}, fmt.Errorf("quest data unresolved: %s", d.Pending[0])
	}
	job := s.Professions.Professions[role.Profession].Job
	if !jobAllowed(d.Jobs, job) {
		return storage.QuestState{}, errors.New("quest profession requirement not met")
	}
	var charState character.State
	if e := json.Unmarshal(role.State, &charState); e != nil {
		return storage.QuestState{}, e
	}
	targets, usable := targetCharacters(d.Script.Cells)
	if !usable || !targetCharacterAllowed(targets, job, charState.Advancement, charState.Awakening) {
		return storage.QuestState{}, errors.New("quest target character requirement not met")
	}
	for _, g := range cells(d.Script.Cells, "[grow type]") {
		if g.Type != 0 || g.Value >= 0 && g.Value != int32(charState.Advancement) {
			return storage.QuestState{}, errors.New("quest advancement requirement not met")
		}
	}
	// A [collision quest] branch may only be taken while none of its peers is
	// accepted or completed. Without this gate the client-side available list
	// alone decides the faction choice, and a forged accept could still open
	// several Silent City branches at once.
	if len(d.Collisions) > 0 {
		states, e := s.Store.Quests(ctx, role.AccountID, role.ID)
		if e != nil {
			return storage.QuestState{}, e
		}
		for _, q := range states {
			for _, c := range d.Collisions {
				if uint32(q.ID) == c && (q.Status == "accepted" || q.Status == "completed") {
					return storage.QuestState{}, fmt.Errorf("quest %d conflicts with accepted or completed quest %d", id, q.ID)
				}
			}
		}
	}
	initial, model, e := InitialProgress(d)
	if e != nil {
		return storage.QuestState{}, e
	}
	if template, ok := AdventureCollectionObjective(d); ok {
		registered, err := s.Store.AdventureEquipmentRegistered(ctx, role.AccountID, role.ID, template)
		if err != nil {
			return storage.QuestState{}, err
		}
		if registered {
			initial = 0
		}
	}
	groups := d.PrerequisiteGroups
	if len(groups) == 0 && len(d.Prerequisites) > 0 {
		groups = [][]uint32{d.Prerequisites}
	}
	return s.Store.AcceptQuestGroups(ctx, role.AccountID, role.ID, id, s.Catalog.Source.SaveIdentity(), d.MinimumLevel, d.MaximumLevel, groups, initial, model)
}
