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
	if !ok || p.RawSHA256 != state.SourceSHA256 || s.Learning.Source.SaveIdentity() != role.ConfigVersion {
		return nil, fmt.Errorf("automatic skill source mismatch")
	}
	grants := p.AdvancementSkills[state.Advancement]
	if len(grants)%3 != 0 {
		return nil, fmt.Errorf("invalid automatic skill triples")
	}
	for i := 0; i < len(grants); i += 3 {
		id, rank, threshold := grants[i], grants[i+1], grants[i+2]
		d, exists, sourceErr := s.Learning.Definition(role.Profession, uint16(id))
		if sourceErr != nil {
			return nil, sourceErr
		}
		if id < 1 || id > 65535 || rank < 1 || rank > 255 || threshold < 1 || threshold > 255 || !exists {
			return nil, fmt.Errorf("invalid automatic skill definition")
		}
		required := d.Ints("[required level]")
		if len(required) > 1 || len(required) == 1 && required[0] < 0 {
			return nil, fmt.Errorf("automatic skill level missing")
		}
		// A .chr condition of 1 is a source grant at this advancement, not the
		// skill's own purchase level. Knight branch starters such as 126/128/127
		// remain granted for legacy roles at levels 1, 2, 5, 14 and 15 even
		// though their .skl required level is 15. Only a real source threshold
		// greater than 1 is level-gated by the skill definition as well.
		if int32(state.Level) < threshold || threshold != 1 && len(required) == 1 && int(state.Level) < required[0] {
			continue
		}
		// The .chr grant is authoritative, including utility skills whose
		// manual-purchase growtype caps are intentionally zero.
		out[uint16(id)] = byte(rank)
	}
	return out, nil
}

// Awakening grants come from the [awakening N] blocks of the same .chr profile
// as the advancement grants, but unlike them they carry real [required level]
// gates: the second awakening unlocks at 75 while part of its branch skills are
// 85-level, and the third unlocks at 100 with 95-level rows. ApplyAwakening can
// therefore only write the grants whose gate is already satisfied, and a role
// that awakened at 75 never receives the 85-level ones. Deriving them from the
// persisted level/awakening (exactly like automaticSkills) repairs those roles
// without a DB edit - the client sees the grant as soon as the level is met.
func (s *Service) awakeningSkills(role storage.Character, state State) (map[uint16]byte, error) {
	out := map[uint16]byte{}
	if s.Learning == nil || state.Awakening == 0 {
		return out, nil
	}
	p, ok := s.Catalog.Professions[role.Profession]
	if !ok || p.RawSHA256 != state.SourceSHA256 || s.Learning.Source.SaveIdentity() != role.ConfigVersion {
		return nil, fmt.Errorf("awakening skill source mismatch")
	}
	for stage := byte(1); stage <= state.Awakening; stage++ {
		grants := p.AwakeningSkills[state.Advancement][stage]
		if len(grants)%2 != 0 {
			return nil, fmt.Errorf("invalid awakening skill triples")
		}
		for i := 0; i < len(grants); i += 2 {
			id, rank := grants[i], grants[i+1]
			d, exists, sourceErr := s.Learning.Definition(role.Profession, uint16(id))
			if sourceErr != nil {
				return nil, sourceErr
			}
			// Membership in the profession's .chr awakening block authorizes
			// this grant; awakened skills deliberately have zero base-growtype
			// caps, so the source definition is only read for its level gate.
			if id < 1 || id > 65535 || rank < 1 || rank > 255 || !exists {
				return nil, fmt.Errorf("invalid awakening skill grant")
			}
			levels := d.Ints("[required level]")
			if len(levels) != 1 {
				return nil, fmt.Errorf("awakening skill level missing")
			}
			// Below its own [required level] the grant stays pending rather
			// than being granted and then refunded.
			if int(state.Level) < levels[0] {
				continue
			}
			if out[uint16(id)] < byte(rank) {
				out[uint16(id)] = byte(rank)
			}
		}
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
	awakened, err := s.awakeningSkills(role, state)
	if err != nil {
		return nil, err
	}
	grants := map[uint16]byte{}
	for _, group := range []map[uint16]byte{free, awakened} {
		for id, rank := range group {
			if grants[id] < rank {
				grants[id] = rank
			}
		}
	}
	for id, rank := range grants {
		if known[id] < rank {
			known[id] = rank
		}
	}
	if s.Learning != nil && state.Advancement > 0 {
		initial := map[uint16]bool{}
		for _, sk := range initialSkills(state) {
			initial[sk.ID] = true
		}
		for id := range state.LearnedSkills[tree] {
			if initial[id] || grants[id] > 0 {
				continue
			}
			d, ok, sourceErr := s.Learning.Definition(role.Profession, id)
			if sourceErr != nil {
				return nil, sourceErr
			}
			if !ok || (!d.ForAdvancement(int(state.Advancement)) && !d.ForAwakening(int(state.Advancement), int(state.Awakening))) {
				delete(known, id)
			}
		}
	}
	return known, nil
}
