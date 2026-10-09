package main

// A successfully sent native entry/map packet is evidence of sending only.
// Some executors commit activeDungeon after their packet plan; record that
// state as-is rather than inventing an entry decision or native APC success.
func (w *worldSession) eliteSpecialPacketObservation(name string, kind byte, id uint16) map[string]any {
	if w == nil || kind != 0 || (id != 28 && id != 29) || !w.eliteSourceSpecialChannel() ||
		!w.ordinaryEliteRosterVisible() || w.adventureElitePrepared == nil {
		return nil
	}
	a, _ := w.eliteChannelDirectory.Attributes(w.channelType)
	return map[string]any{"kind": "adventure_elite_special_packet_sent", "candidate_version": "0.3.19",
		"special_channel_attempt": "1/3", "id": id, "packet_name": name, "character_id": w.role.ID,
		"owner_wire_id": w.role.WireID, "channel_type": w.channelType, "source_channel_attributes": a,
		"frozen_preparation": w.adventureElitePrepared, "state_at_send": w.eliteCombatState(id, nil),
		"client_acceptance": "pending"}
}
