package main

import "fmt"

// Bind only the existing PVF-backed Moon solo executor. Do not open every
// semi-raid/panel/isolated channel, or replace its native one-player roster.
func (w *worldSession) eliteMoonChannel() bool {
	if w == nil || w.moonConfig == nil || w.eliteChannelDirectory == nil ||
		w.channelType != w.moonConfig.Channel || w.moonConfig.SourceRewards.Source == "" {
		return false
	}
	a, ok := w.eliteChannelDirectory.Attributes(w.channelType)
	return ok && a.Type == w.channelType && a.IsSemiRaid && !a.IsRaid &&
		!a.IsLegion && !a.IsPreRaid && a.GuideDungeon != 0 &&
		a.SlotDungeon == a.GuideDungeon && a.Panel != ""
}

// Called only for an existing moon_dungeon packet, before its plan is sent.
// It records the original solo executor's entry, without changing its packets,
// party members, content state, rewards, or player save.
func (w *worldSession) eliteMoonEntryObservation() map[string]any {
	if !w.eliteMoonChannel() || w.adventureElitePrepared == nil {
		return nil
	}
	p := w.adventureElitePrepared
	var err error
	if w.moon.owner == nil || w.activeDungeon == nil || w.activeDungeon != w.moon.owner.Session() ||
		w.role.WireID == 0 || w.role.WireID == 65535 ||
		p.Owner != w.role.ID || p.Channel != w.channelType || p.Settings != w.adventureEliteSnapshot ||
		p.Selected == ([3]int64{}) || !w.ordinaryEliteSelectionVisible() {
		err = fmt.Errorf("moon solo entry lacks the matching prepared elite identity")
	} else {
		w.adventureEliteEntryProbeUsed = true
		w.adventureEliteEntrySerial++
	}
	row := eliteEntryProbeDiagnostic(w, w.activeDungeon, err)
	row["candidate_stage"] = "moon-solo"
	row["candidate_version"] = "0.3.18"
	row["moon_attempt"] = "1/3"
	row["entry_opcode"] = uint16(0) // server-pushed plan, not an invented CMD16
	row["entry_path"] = "existing-moon-solo-plan"
	row["moon_resuming"] = w.moon.resuming
	row["source_channel_attributes"], _ = w.eliteChannelDirectory.Attributes(w.channelType)
	return row
}
