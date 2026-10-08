package gamedata

import "dfolan/internal/catalog"

func (s *Source) RaidEntrances(d *catalog.ChannelDirectory) (map[uint32]catalog.RaidEntrance, error) {
	return catalog.ImportRaidEntrances(s.archive, d)
}
