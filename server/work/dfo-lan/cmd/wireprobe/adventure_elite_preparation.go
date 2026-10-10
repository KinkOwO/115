package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventureelite"
	"dfolan/internal/database"
)

// The owner explicitly permits companions on every existing special entry.
// Channel categories do not hide the roster; source identity and the original
// content handlers still decide whether a dungeon can actually be entered.
func (w *worldSession) ordinaryEliteRosterVisible() bool {
	if w == nil || !adventureelite.Enabled() || adventureEliteChannel(w.channelType) || w.eliteChannelDirectory == nil {
		return false
	}
	a, known := w.eliteChannelDirectory.Attributes(w.channelType)
	if known && a.Type != w.channelType {
		return false
	}
	if !known {
		if w.eliteChannelInfo == nil {
			return false
		}
		rows, ok := w.eliteChannelInfo.Rows(w.serverID)
		ordinary := false
		if ok {
			for _, row := range rows {
				if row.ID == w.eliteChannelID {
					// Published ID is resolved by the existing channel route.
					// Local Type overrides are not a second content directory.
					ordinary = true
					break
				}
			}
		}
		if !ordinary {
			return false
		}
	}
	if w.inTutorial {
		return false
	}
	return w.boostup == nil || w.state.Position.Town != w.boostup.Town
}

// A town warp must not remove the account roster: its later restoration can
// race CMD15 and trigger a destructive native reload after selection began.
// Preserve the original loading/battle scope independently of that view.
func (w *worldSession) ordinaryEliteSelectionVisible() bool {
	return w.ordinaryEliteRosterVisible() && !w.specialWarpPending
}

// Selection identity remains stable while selecting/returning from a dungeon.
// Reloading native companions is still restricted to a settled town session.
func (w *worldSession) ordinaryElitePreparationAllowed() bool {
	return w.ordinaryEliteSelectionVisible() && w.activeDungeon == nil &&
		!w.selectingDungeon && w.pendingTownArrival == nil &&
		w.bleedingMineStart == nil &&
		(!w.eliteMoonChannel() || w.moon.owner == nil)
}

func (w *worldSession) eliteSourceSpecialChannel() bool {
	if w == nil || w.eliteChannelDirectory == nil {
		return false
	}
	a, ok := w.eliteChannelDirectory.Attributes(w.channelType)
	return ok && a.Type == w.channelType && (w.channelWorldIsolated || a.IsRaid ||
		a.IsLegion || a.IsPreRaid || a.IsSemiRaid || a.GuideDungeon != 0 || a.Panel != "")
}

// The account list can include the character now playing after a role switch.
// The ordinary extension projects that one slot as empty without changing the
// saved list, other modes, slot positions or the remaining skill settings.
func (w *worldSession) eliteProfileView(profile database.AccountAdventure) database.AccountAdventure {
	if !adventureelite.Enabled() || adventureEliteChannel(w.channelType) {
		return profile
	}
	ordinary := w.ordinaryEliteRosterVisible()
	view := make(map[uint16][3]int64, len(profile.Data.EliteSelections))
	for mode, ids := range profile.Data.EliteSelections {
		if mode == 2 {
			if !ordinary {
				continue
			}
			for i, id := range ids {
				if id == w.role.ID {
					ids[i] = 0
				}
			}
		}
		view[mode] = ids
	}
	profile.Data.EliteSelections = view
	return profile
}

func (w *worldSession) refreshAdventureEliteSelections(ctx context.Context, profile database.AccountAdventure) ([]outboundPacket, error) {
	elite, err := w.adventureElitePayload(ctx, profile)
	if err != nil {
		return nil, err
	}
	signature := sha256.Sum256(elite)
	if signature == w.adventureEliteSnapshot {
		return nil, nil
	}
	w.adventureEliteSnapshot = signature
	w.syncAdventureElitePreparation(profile)
	return []outboundPacket{{"精锐角色设置恢复", 0, 1754, elite}}, nil
}

// Every emitted selection view must agree with the native loaded identity.
// N1754 clears its selection map even for count=0 (142E5A6DD). Restoring a
// removed mode-2 row then releases actors (142E5AE64) and requests 1811
// (142E5AED6). This also happens during a town warp without a roster save.
func (w *worldSession) syncAdventureElitePreparation(profile database.AccountAdventure) {
	if !adventureelite.Enabled() || adventureEliteChannel(w.channelType) {
		return
	}
	if prepared := w.adventureElitePrepared; prepared != nil {
		view := w.eliteProfileView(profile).Data.EliteSelections[2]
		if view == ([3]int64{}) || view != prepared.Selected || prepared.Owner != w.role.ID || prepared.Channel != w.channelType {
			w.adventureElitePrepared = nil
		} else {
			// Identical slots retain native actors and only apply skill usage.
			prepared.Settings = w.adventureEliteSnapshot
		}
	}
}

// Session-only frozen identity. This candidate validates town loading first;
// battle admission remains closed until native registration/owner is verified.
type adventureElitePreparation struct {
	Channel  uint32
	Owner    int64
	Selected [3]int64
	Settings [32]byte
}
