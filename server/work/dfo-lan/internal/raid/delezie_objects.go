package raid

import (
	"dfolan/internal/catalog"
	"fmt"
	"math"
)

// DelezieObjects owns map-object identity and static health independently of
// monster combat HP. Calls belong to a serialized raid owner. It deliberately
// does not interpret unresolved SUMMON operands, choose random locations, or
// emit an incomplete 2788/2789 notification.
type DelezieObjects struct {
	path        string
	definitions map[uint32]catalog.DelezieObjectRule
	presets     map[uint32]catalog.DelezieObjectPreset
	graph       *catalog.DelezieObjectGraph
	summons     map[int]catalog.DelezieSummonAction
	summonPath  string
	locations   map[uint32]bool
	instances   map[uint32]DelezieObjectInstance
	next        uint32
}

type DelezieObjectInstance struct {
	Key, Definition, Dungeon, Location uint32
	HP, MaxHP                          int64
}

type DelezieObjectSpawn struct {
	Definition, Location uint32
}

type DeleziePresetSpawn struct {
	Preset    uint32
	Locations []uint32
}

func NewDelezieObjects(rules *catalog.DelezieRaidRules, mode *catalog.DelezieModeRules, phase int) (*DelezieObjects, error) {
	definitions, err := rules.ObjectProgram(mode)
	if err != nil {
		return nil, err
	}
	graph, err := rules.ObjectGraphProgram(mode)
	if err != nil {
		return nil, err
	}
	if phase < 0 || phase >= len(mode.Phases) {
		return nil, fmt.Errorf("%s: missing source phase %d", mode.Objects.Table.Path, phase)
	}
	s := &DelezieObjects{path: mode.Objects.Table.Path, definitions: definitions, presets: graph.Presets, graph: graph, locations: map[uint32]bool{}, instances: map[uint32]DelezieObjectInstance{}}
	for id := range mode.Phases[phase].Locations {
		s.locations[id] = true
	}
	if len(s.locations) == 0 {
		return nil, fmt.Errorf("%s: empty source phase locations", s.path)
	}
	if mode.Phase != nil {
		s.summons, err = rules.SummonProgram(mode)
		if err != nil {
			return nil, err
		}
		s.summonPath = mode.Phase.Table.Path
	}
	return s, nil
}

// SummonOperands resolves the exact native paired operands without creating
// objects. In particular, -1 is preserved, not converted into a guessed
// random destination. The battle action consumer still needs a verified
// contract before feeding these operands into Spawn.
func (s *DelezieObjects) SummonOperands(row int, values map[string]int32) ([][2]int32, error) {
	action, ok := s.summons[row]
	if !ok {
		return nil, fmt.Errorf("%s row %d: missing source summon action", s.summonPath, row)
	}
	resolve := func(operand catalog.DelezieScalarOperand) (int32, error) {
		if !operand.IsSymbol {
			return operand.Value, nil
		}
		value, ok := values[operand.Symbol]
		if !ok {
			return 0, fmt.Errorf("%s row %d: uninitialized summon symbol %q", s.summonPath, row, operand.Symbol)
		}
		return value, nil
	}
	out := make([][2]int32, 0, len(action.Pairs))
	for _, pair := range action.Pairs {
		first, err := resolve(pair.First)
		if err != nil {
			return nil, err
		}
		second, err := resolve(pair.Second)
		if err != nil {
			return nil, err
		}
		out = append(out, [2]int32{first, second})
	}
	return out, nil
}

func (s *DelezieObjects) Snapshot() map[uint32]DelezieObjectInstance {
	out := make(map[uint32]DelezieObjectInstance, len(s.instances))
	for key, object := range s.instances {
		out[key] = object
	}
	return out
}

// Spawn commits an explicit validated group or nothing. Missing dynamic HP
// initialization is an error, not a content-specific 10000/default fallback.
// Spawn does not infer occupancy restrictions for synthesis/moving objects.
func (s *DelezieObjects) Spawn(requests []DelezieObjectSpawn) ([]DelezieObjectInstance, error) {
	next := s.next
	var created []DelezieObjectInstance
	for _, request := range requests {
		definition, ok := s.definitions[request.Definition]
		if !ok {
			return nil, fmt.Errorf("%s: undeclared mode object %d", s.path, request.Definition)
		}
		if !s.locations[request.Location] {
			return nil, fmt.Errorf("%s row %d: undeclared phase location %d", s.path, definition.Row, request.Location)
		}
		allowed := false
		for _, location := range definition.CreatableLocations {
			if location == request.Location {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("%s row %d: object %d cannot spawn at location %d", s.path, definition.Row, request.Definition, request.Location)
		}
		if !definition.HasStaticHP || definition.StaticHP < 0 {
			return nil, fmt.Errorf("%s row %d: object %d requires unresolved dynamic health", s.path, definition.Row, request.Definition)
		}
		if next >= math.MaxInt32 {
			return nil, fmt.Errorf("%s: object identity exhausted", s.path)
		}
		next++
		created = append(created, DelezieObjectInstance{Key: next, Definition: definition.ID, Dungeon: definition.Dungeon, Location: request.Location, HP: definition.StaticHP, MaxHP: definition.StaticHP})
	}
	for _, object := range created {
		s.instances[object.Key] = object
	}
	s.next = next
	return created, nil
}

// SpawnPresets expands source lists in order, preserving repeated objects.
// The battle owner supplies resolved destinations; random locations and the
// source instruction's unresolved optional operands are not inferred here.
func (s *DelezieObjects) SpawnPresets(presets []DeleziePresetSpawn) ([]DelezieObjectInstance, error) {
	var requests []DelezieObjectSpawn
	for _, request := range presets {
		preset, owned := s.presets[request.Preset]
		if !owned || len(preset.Objects) == 0 || len(request.Locations) != len(preset.Objects) {
			return nil, fmt.Errorf("%s: missing preset or destination count mismatch for %d", s.path, request.Preset)
		}
		for i, definition := range preset.Objects {
			requests = append(requests, DelezieObjectSpawn{Definition: definition, Location: request.Locations[i]})
		}
	}
	return s.Spawn(requests)
}

// SetHealth changes map-object HP, not a monster's absolute combat HP. The
// future battle adapter must establish its report conversion/authority first.
func (s *DelezieObjects) SetHealth(key uint32, hp int64) (DelezieObjectInstance, error) {
	object, ok := s.instances[key]
	if !ok || hp < 0 || hp > object.MaxHP {
		return DelezieObjectInstance{}, fmt.Errorf("%s: invalid object health %d/%d", s.path, key, hp)
	}
	object.HP = hp
	s.instances[key] = object
	return object, nil
}

// Remove validates all identities before removing any. Keys are never reused
// after removal, so stale client reports cannot address a new object.
func (s *DelezieObjects) Remove(keys []uint32) error {
	seen := map[uint32]bool{}
	for _, key := range keys {
		if _, ok := s.instances[key]; !ok || seen[key] {
			return fmt.Errorf("%s: missing/duplicate object removal %d", s.path, key)
		}
		seen[key] = true
	}
	for _, key := range keys {
		delete(s.instances, key)
	}
	return nil
}
