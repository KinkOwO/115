package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/channelrefresh"
	"encoding/hex"
)

// Source raid flags decide publication. Local channel config remains the
// source of existing display names and IDs, not a second raid content list.
func publishSourceRaidChannels(c *channelrefresh.Config, source *catalog.ChannelDirectory) error {
	var types []uint32
	for _, typ := range source.Types() {
		a, _ := source.Attributes(typ)
		if a.IsRaid {
			types = append(types, typ)
		}
	}
	return c.PublishSourceRaids(types, func(typ uint32) (channelrefresh.ChannelAttributes, bool) {
		a, ok := source.Attributes(typ)
		return channelrefresh.ChannelAttributes{Type: a.Type, Area: "[none]", SourceValues: make([]int32, 11)}, ok && a.IsRaid
	})
}

// Only requests that passed all existing handlers are logged here. No guessed
// ACK is sent: raid state packet layouts must be verified before implementation.
func (client *gameConnection) logUnhandledRaidRequest(request *clientRequest) {
	if client.gatewayRuntime == nil || client.channelDirectory == nil || client.event == nil || !request.verified || request.frame.Type != 1 {
		return
	}
	typ := client.channelTypes[client.channel]
	a, ok := client.channelDirectory.Attributes(typ)
	if !ok || !a.IsRaid {
		return
	}
	name := ""
	// Names/IDs verified against this client's native opcodes.tsv registry.
	switch request.frame.ID {
	case 650:
		name = "RAID_ENTRY_COST_INFO"
	case 656:
		name = "CREATE_RAID"
	case 657:
		name = "LEAVE_RAID"
	case 658:
		name = "START_RAID"
	case 659:
		name = "SET_RAID_WAITING"
	case 660:
		name = "REJOIN_RAID"
	case 661:
		name = "RAID_MANAGER_WORK"
	case 662:
		name = "MODIFY_RAID_INFO"
	case 882:
		name = "RAID_REQUEST_RAID_MEMBERS"
	case 883:
		name = "RAID_CHECK_RAID_USER"
	case 886:
		name = "PASS_RAID_PHASE"
	case 1353:
		name = "REQUEST_RAID_INFO"
	case 1361:
		name = "REQUEST_RAID_ENTRANCE_INFO"
	default:
		return
	}
	body := request.plaintext
	if len(body) > 512 {
		body = body[:512]
	}
	client.event(map[string]any{"kind": "raid_unhandled_request", "id": request.frame.ID, "command": name, "channel": client.channel, "channel_type": typ, "character_id": client.selectedCharacterID, "bytes": len(request.plaintext), "plain_hex": hex.EncodeToString(body)})
}
