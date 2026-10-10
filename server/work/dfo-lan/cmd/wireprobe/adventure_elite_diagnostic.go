package main

import (
	"crypto/sha256"
	"dfolan/internal/adventureelite"
	"encoding/hex"
)

// Log the decision separately from sendPlan's actual send events. Diagnostics
// never perform a second storage read or change the packets/acknowledgements.
func adventureEliteDiagnostic(w *worldSession, opcode uint16, packets []outboundPacket, err error) map[string]any {
	event := map[string]any{"kind": "adventure_elite_transaction_decision", "id": opcode,
		"enabled": adventureelite.Enabled(), "accepted": err == nil}
	if err != nil {
		event["reason"] = err.Error()
	}
	if w != nil {
		event["character_id"], event["expected_owner_wire_id"] = w.role.ID, w.role.WireID
		event["server_id"], event["channel_id"], event["channel_type"] = w.serverID, w.eliteChannelID, w.channelType
		event["odyssey"], event["native_channel"] = w.odyssey, adventureEliteChannel(w.channelType)
		event["ordinary_source_allowed"] = w.ordinaryElitePreparationAllowed()
		event["channel_directory_bound"] = w.eliteChannelDirectory != nil
		if w.eliteChannelDirectory != nil {
			attributes, known := w.eliteChannelDirectory.Attributes(w.channelType)
			event["source_special_attributes_known"], event["source_channel_attributes"] = known, attributes
		}
		if w.eliteChannelInfo != nil {
			event["source_archive_checksum"], event["source_channel_script_sha256"] = w.eliteChannelInfo.Source.Checksum, w.eliteChannelInfo.SHA256
			rows, _ := w.eliteChannelInfo.Rows(w.serverID)
			for _, row := range rows {
				if row.ID == w.eliteChannelID {
					event["source_ordinary_row"] = row
					break
				}
			}
		}
		event["active_dungeon"], event["selecting_dungeon"] = w.activeDungeon != nil, w.selectingDungeon
		event["town_arrival"], event["tutorial"], event["special_warp"] = w.pendingTownArrival != nil, w.inTutorial, w.specialWarpPending
		event["channel_world_isolated"], event["bleeding_mine_started"] = w.channelWorldIsolated, w.bleedingMineStart != nil
		event["frozen_preparation"] = w.adventureElitePrepared
	}
	outputs := make([]map[string]any, 0, len(packets))
	for _, p := range packets {
		// Bounded native elite messages only; never log another system's body.
		if p.ID != 1754 && p.ID != 1382 && p.ID != 1879 && p.ID != 1719 {
			continue
		}
		digest := sha256.Sum256(p.Payload)
		output := map[string]any{"id": p.ID, "type": p.Kind, "bytes": len(p.Payload), "sha256": hex.EncodeToString(digest[:])}
		if len(p.Payload) <= 256*1024 {
			output["payload_hex"] = hex.EncodeToString(p.Payload)
		} else {
			output["body_omitted"] = "diagnostic-size-limit"
		}
		outputs = append(outputs, output)
	}
	event["outputs_planned"] = outputs
	return event
}
