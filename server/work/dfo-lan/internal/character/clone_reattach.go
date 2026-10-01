package character

import (
	"dfolan/internal/inventory"
)

// CloneReattachPackets makes the native mode-1 reader remove the old Clone
// instance, then create a new one with its visual cover already assigned.
// Neither packet changes persisted equipment. The caller must restore
// non-avatar worn rows after it:
// mode-1 clears every omitted slot in the client's 48-slot equipment table.
func (s *Service) CloneReattachPackets(role Character) (reset, full []byte, ok bool, err error) {
	if s == nil || !s.DetailedWornCandidate || s.Equipment == nil {
		return nil, nil, false, nil
	}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return nil, nil, false, err
	}
	looks := make(map[uint16]uint32)
	for _, item := range bag.Worn {
		if item.Group == 1 && item.Slot <= 11 {
			looks[item.Slot] = item.Template
		}
	}
	resolved := make(map[uint16]uint32)
	for _, item := range bag.WornBaseItems() {
		if item.Group != 0 || item.Slot > 11 {
			continue
		}
		definition, lookupErr := s.Equipment.Definition(item.Template)
		if lookupErr != nil || !definition.IsCloneAvatar() {
			continue
		}
		cover := looks[item.Slot]
		if cover == 0 {
			cover = s.defaultCloneCover(role.Profession, item.Slot)
			if cover != 0 {
				if _, lookupErr = s.Equipment.Definition(cover); lookupErr != nil {
					cover = 0
				}
			}
		}
		if cover != 0 {
			resolved[item.Slot] = cover
		}
	}
	if len(resolved) == 0 {
		return nil, nil, false, nil
	}
	reset, err = s.entryAddition(role, resolved, true)
	if err != nil {
		return nil, nil, false, err
	}
	full, err = s.entryAddition(role, resolved, false)
	if err != nil {
		return nil, nil, false, err
	}
	return reset, full, true, nil
}

// Characterinfo.etc provides the ordinary professions' default equipment
// indices. The current client has additional built-in defaults for jobs
// 11..16 in sub_145CDB1D0; these avatar slots are absent from the imported
// characterinfo table. All values below are direct returns of that function.
var nativeExtraAvatarDefaults = map[byte]map[uint16]uint32{
	11: {1: 112560000, 3: 112500000, 4: 112510000, 5: 112540000, 6: 112520000, 8: 112580001},
	12: {1: 113560000, 3: 113500000, 4: 113510000, 5: 113540000, 6: 113520000, 8: 113580000},
	13: {1: 114560000, 3: 114500000, 4: 114510000, 5: 114540000, 8: 114580091},
	14: {1: 115560000, 3: 115500000, 4: 115510000, 5: 115540000, 8: 115580000},
	15: {1: 116560000, 2: 116570000, 3: 116500000, 4: 116510000, 5: 116540000, 7: 116530000, 8: 116580000},
	16: {0: 117550000, 1: 117560000, 3: 117500000, 4: 117510000, 5: 117540000, 8: 117580000},
}

func (s *Service) defaultCloneCover(profession byte, slot uint16) uint32 {
	if prof, ok := s.Catalog.Professions[profession]; ok && int(slot) < len(prof.DefaultAppearance) {
		if id := prof.DefaultAppearance[slot]; id > 0 {
			return uint32(id)
		}
	}
	return nativeExtraAvatarDefaults[profession][slot]
}
