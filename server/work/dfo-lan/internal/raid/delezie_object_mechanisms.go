package raid

import (
	"dfolan/internal/catalog"
	"fmt"
)

// The future authoritative collision/mechanism owner chooses the exact source
// recipe, ordered live materials and destination. No client claim or inferred
// collision, material commutativity, health inheritance or counter is accepted.
type DelezieResolvedCombination struct {
	RecipeRow int
	Materials [2]uint32
	Location  uint32
}

type DelezieCombinationChange struct {
	RecipeRow int
	Consumed  [2]DelezieObjectInstance
	Created   DelezieObjectInstance
}

// CombineResolved consumes and replaces objects atomically, including fresh
// identity allocation. Result definitions/static HP are taken from this mode's
// source, not from the caller. It does not execute the complete synthesis event.
func (s *DelezieObjects) CombineResolved(requests []DelezieResolvedCombination) ([]DelezieCombinationChange, error) {
	staged := *s
	staged.instances = s.Snapshot()
	var changes []DelezieCombinationChange
	for _, request := range requests {
		var recipe catalog.DelezieObjectCombo
		found := false
		for _, candidate := range s.graph.Combos {
			if candidate.Row == request.RecipeRow {
				recipe, found = candidate, true
				break
			}
		}
		if !found || request.Materials[0] == request.Materials[1] {
			return nil, fmt.Errorf("%s row %d: missing recipe or duplicate material identity", s.path, request.RecipeRow)
		}
		var consumed [2]DelezieObjectInstance
		for i, key := range request.Materials {
			object, live := staged.instances[key]
			// All supplied material identities must exist at batch entry.
			_, existing := s.instances[key]
			if !live || !existing || object.Definition != recipe.Materials[i] {
				return nil, fmt.Errorf("%s row %d: invalid ordered live material %d", s.path, recipe.Row, key)
			}
			consumed[i] = object
		}
		created, err := staged.Spawn([]DelezieObjectSpawn{{Definition: recipe.Result, Location: request.Location}})
		if err != nil {
			return nil, fmt.Errorf("%s row %d: combination result: %w", s.path, recipe.Row, err)
		}
		if err := staged.Remove(request.Materials[:]); err != nil {
			return nil, err
		}
		changes = append(changes, DelezieCombinationChange{RecipeRow: recipe.Row, Consumed: consumed, Created: created[0]})
	}
	s.instances, s.next = staged.instances, staged.next
	return changes, nil
}

// AIProgram binds an existing object to this owned mode's native instruction
// sequence. Returning a copy prevents callers mutating future object decisions.
// First/Second retain positional values; no timing or movement semantics inferred.
func (s *DelezieObjects) AIProgram(key uint32) (catalog.DelezieAIRule, error) {
	object, live := s.instances[key]
	if !live {
		return catalog.DelezieAIRule{}, fmt.Errorf("%s: missing live object %d", s.path, key)
	}
	id, bound := s.graph.ObjectAIs[object.Definition]
	program, present := s.graph.AIs[id]
	if !bound || !present {
		return catalog.DelezieAIRule{}, fmt.Errorf("%s: object %d has no source AI", s.path, key)
	}
	program.StartResults = append([]catalog.DelezieAIInstruction(nil), program.StartResults...)
	program.Results = append([]catalog.DelezieAIInstruction(nil), program.Results...)
	return program, nil
}
