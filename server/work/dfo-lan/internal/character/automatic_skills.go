package character

import (
	"dfolan/internal/storage"
	"fmt"
)

// Free ranks are a source-derived floor, not purchases. Computing them from
// persisted level/advancement also repairs older roles without SP or DB edits.
func (s *Service) automaticSkills(role storage.Character, state State) (map[uint16]byte, error) {
	out := map[uint16]byte{}
	if s.Learning == nil {
		return out, nil
	}
	p, ok := s.Catalog.Professions[role.Profession]
	if !ok || p.RawSHA256 != state.SourceSHA256 || s.Learning.Source.Checksum != role.ConfigVersion {
		return nil, fmt.Errorf("automatic skill source mismatch")
	}
	grants := p.AdvancementSkills[state.Advancement]
	if len(grants)%3 != 0 {
		return nil, fmt.Errorf("invalid automatic skill triples")
	}
	for i := 0; i < len(grants); i += 3 {
		id, rank, threshold := grants[i], grants[i+1], grants[i+2]
		d, exists := s.Learning.index[role.Profession][uint16(id)]
		if id < 1 || id > 65535 || rank < 1 || rank > 255 || threshold < 1 || threshold > 255 || !exists {
			return nil, fmt.Errorf("invalid automatic skill definition")
		}
		required := d.Ints("[required level]")
		if len(required) > 1 || len(required) == 1 && required[0] < 0 {
			return nil, fmt.Errorf("automatic skill level missing")
		}
		if len(required) == 1 && int(state.Level) < required[0] || int32(state.Level) < threshold {
			continue
		}
		// The .chr grant is authoritative, including utility skills whose
		// manual-purchase growtype caps are intentionally zero.
		out[uint16(id)] = byte(rank)
	}
	return out, nil
}

func (s *Service) knownSkills(role storage.Character, state State, tree int) (map[uint16]byte, error) {
	known, err := knownSkills(state, tree)
	if err != nil {
		return nil, err
	}
	free, err := s.automaticSkills(role, state)
	if err != nil {
		return nil, err
	}
	for id, rank := range free {
		if known[id] < rank {
			known[id] = rank
		}
	}
	return known, nil
}
