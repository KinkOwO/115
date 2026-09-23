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
	Odyssey *catalog.OdysseyGrowth
	index   *Index
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

func (s *Service) Accept(ctx context.Context, role storage.Character, id uint16) (storage.QuestState, error) {
	d, ok := s.Catalog.Quests[uint32(id)]
	if !ok {
		return storage.QuestState{}, errors.New("quest absent from source index")
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
	for _, g := range cells(d.Script.Cells, "[grow type]") {
		if g.Type != 0 || g.Value >= 0 && g.Value != int32(charState.Advancement) {
			return storage.QuestState{}, errors.New("quest advancement requirement not met")
		}
	}
	initial, model, e := InitialProgress(d)
	if e != nil {
		return storage.QuestState{}, e
	}
	return s.Store.AcceptQuest(ctx, role.AccountID, role.ID, id, s.Catalog.Source.Checksum, d.MinimumLevel, d.MaximumLevel, d.Prerequisites, initial, model)
}
