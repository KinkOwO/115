package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/channelrefresh"
	"encoding/hex"
)

// Publish native raid flags, CTP-bound raid channels and legion channels with
// source-defined availability. New CTP channels need not carry [isRaid].
// Local channel config remains the source of existing display names and IDs.
func publishSourceRaidChannels(c *channelrefresh.Config, source *catalog.ChannelDirectory, entries map[uint32]catalog.RaidEntrance) error {
	linked := map[uint32]bool{}
	for _, typ := range source.Types() {
		attr, _ := source.Attributes(typ)
		if !attr.IsRaid || attr.SubChannelType == 0 {
			continue
		}
		sub, found := source.Attributes(attr.SubChannelType)
		if found && (sub.IsPreRaid || sub.IsSemiRaid) {
			linked[sub.Type] = true
		}
	}
	var types []uint32
	for _, typ := range source.Types() {
		a, _ := source.Attributes(typ)
		_, bound := entries[typ]
		if a.IsRaid || bound || linked[typ] || (a.IsLegion && len(a.OpenEvents) > 0) {
			types = append(types, typ)
		}
	}
	return c.PublishSourceRaids(types, func(typ uint32) (channelrefresh.ChannelAttributes, bool) {
		a, ok := source.Attributes(typ)
		_, bound := entries[typ]
		return channelrefresh.ChannelAttributes{Type: a.Type, Area: "[none]", SourceValues: make([]int32, 11)}, ok && (a.IsRaid || bound || linked[typ] || (a.IsLegion && len(a.OpenEvents) > 0))
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
	_, bound := client.raidEntrances[typ]
	if !ok || (!a.IsRaid && !bound) {
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
