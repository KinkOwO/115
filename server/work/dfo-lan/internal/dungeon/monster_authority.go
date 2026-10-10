package dungeon

// The pinned native145b89780 assigns an unowned entity to abs(type<<16|id)%8,
// falling back among present seats. Native145b8e3a0 compares that result with
// the local seat. A local human + synthetic party always has its human at0.
// Allocate its source monsters on residue0 so combat stays with the human;
// synthetic endpoints cannot simulate the native monster damage/AI owner.
// Source indices, templates, stats, drops and genuine multiplayer are unchanged.
func (s *Session) monsterEntityStride() uint16 {
	if s != nil && s.localMonsterAuthority {
		return 8
	}
	return 1
}

func advanceMonsterEntity(id, stride uint16) uint16 {
	next := uint32(id) + uint32(stride)
	if next >= 65535 {
		return 65535
	}
	return uint16(next)
}
