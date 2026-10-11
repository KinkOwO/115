package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"fmt"
)

// Authentication of a character is not ownership of another connection's raid.
// The connection and registry must agree on both role and allocated team ID.
func (c *gameConnection) ownedRaidTeam() (raidTeam, bool) {
	if c.gatewayRuntime == nil || c.worldState == nil || c.raidOwnerRole != c.worldState.role.ID || c.raidOwnerRole == 0 || c.ownedRaidID == 0 {
		return raidTeam{}, false
	}
	t, ok := c.raidTeams.get(c.raidOwnerRole, c.channel)
	return t, ok && t.Recruitment.ID == c.ownedRaidID && t.Recruitment.Leader == c.worldState.role.WireID
}

// Release only this connection's exact allocation. An old disconnect or failed
// delivery cannot remove a subsequently allocated team with a different ID.
func (c *gameConnection) releaseOwnedRaid() {
	if c.gatewayRuntime == nil || c.raidOwnerRole == 0 || c.ownedRaidID == 0 {
		return
	}
	t, owned := c.raidTeams.get(c.raidOwnerRole, c.channel)
	if owned && t.Recruitment.ID == c.ownedRaidID {
		c.raidTeams.leave(c.raidOwnerRole, c.ownedRaidID)
		if w := c.worldState; w != nil && w.role.ID == c.raidOwnerRole {
			w.state.Position = t.Origin
			w.raidWaiting = false
			w.bakalOpening, w.bakalRules = nil, nil
			w.bakalParty, w.bakalTown = 0, 0
			w.bakalCardPlan = nil
			w.soloPartyReady = false
		}
	}
	c.raidOwnerRole, c.ownedRaidID = 0, 0
}

func (w *worldSession) loadWorldEntry(ctx context.Context, role int64, level byte, odyssey bool, spawn database.WorldPosition, worldType uint32) (database.WorldState, error) {
	if w.raidEntrance == nil || w.raidEntrance.Recruiting == nil {
		return w.service.Enter(ctx, w.account, role, level, odyssey, spawn, worldType)
	}
	room := w.raidEntrance.Recruiting
	x, y := room.Spawn()
	pos := database.WorldPosition{Town: room.TownID, Area: room.AreaID, X: x, Y: y}
	if err := w.service.ValidatePosition(level, odyssey, pos); err != nil {
		return database.WorldState{}, fmt.Errorf("raid recruiting room: %w", err)
	}
	version := w.service.Catalog.Source.SaveIdentity()
	saved, err := w.service.Store.LoadWorld(ctx, w.account, role, worldType, pos, version)
	if err != nil {
		return saved, err
	}
	if saved.ConfigVersion != version {
		return saved, fmt.Errorf("saved raid room requires catalog migration")
	}
	// A previous saved area is not proof of a live team. Keep its revision and
	// identity, but choose the source preparation area for this new session.
	saved.Position = pos
	return saved, nil
}

func (w *worldSession) authorizeRaidArea(r protocol.AreaChangeRequest) error {
	if w.raidEntrance == nil || w.raidEntrance.Recruiting == nil {
		return nil
	}
	waiting := w.raidEntrance.Waiting
	toWaiting := r.Town == waiting.TownID && r.Area == waiting.AreaID
	if toWaiting && !w.raidWaiting {
		return fmt.Errorf("raid waiting room requires a connection-owned team")
	}
	if w.raidWaiting && !toWaiting {
		return fmt.Errorf("leave the owned raid before leaving its waiting room")
	}
	return nil
}
