package channelrefresh

import (
	"fmt"
	"sort"
)

// PublishSourceRaids fills missing raid types from the caller's live PVF
// projection. Directory IDs and gameplay types are different namespaces:
// official Ispins IDs 86/87 must survive when source raid type 86 is published.
// This publishes channels only; it does not claim a raid session exists.
func (c *Config) PublishSourceRaids(types []uint32, attrs func(uint32) (ChannelAttributes, bool)) error {
	if attrs == nil {
		return fmt.Errorf("source raid channel attributes missing")
	}
	channels := append([]Channel(nil), c.Channels...)
	ids, published := map[uint32]bool{}, map[uint32]bool{}
	for _, ch := range channels {
		ids[ch.ID], published[ch.Type] = true, true
	}
	ordered := append([]uint32(nil), types...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	for _, channelType := range ordered {
		if published[channelType] {
			continue
		}
		a, ok := attrs(channelType)
		if !ok || channelType == 0 || channelType > 255 || a.Type != channelType || len(a.SourceValues) != 11 || a.Area == "" {
			return fmt.Errorf("invalid source raid channel type %d", channelType)
		}
		if len(channels) >= 128 {
			return fmt.Errorf("source raids exceed directory channel limit")
		}
		id := channelType
		if ids[id] {
			id = 255
			for id > 0 && ids[id] {
				id--
			}
			if id == 0 {
				return fmt.Errorf("no directory ID for source raid type %d", channelType)
			}
		}
		channels = append(channels, Channel{ID: id, Name: fmt.Sprintf("Raid %d", channelType), Type: a.Type, Area: a.Area, SourceValues: append([]int32(nil), a.SourceValues...)})
		ids[id], published[channelType] = true, true
	}
	c.Channels = channels
	return nil
}
