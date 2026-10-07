package main

import (
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"fmt"
	"sync"
)

// Shared by connections in one gateway. A reconnect cannot create a second
// team for the same character. This candidate supports an owned solo team;
// joining other teams requires the separate verified member-state protocol.
type raidTeam struct {
	Role        int64
	Channel     uint32
	Recruitment protocol.RaidRecruitment
	Origin      database.WorldPosition
}
type raidTeamRegistry struct {
	mu    sync.Mutex
	next  uint32
	roles map[int64]raidTeam
}

func (r *raidTeamRegistry) create(role int64, channel, server uint32, entry protocol.RaidRecruitment, origin database.WorldPosition) (raidTeam, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if role <= 0 {
		return raidTeam{}, fmt.Errorf("missing raid owner")
	}
	if _, ok := r.roles[role]; ok {
		return raidTeam{}, fmt.Errorf("character already owns a raid")
	}
	if r.next >= 65535 {
		return raidTeam{}, fmt.Errorf("raid IDs exhausted")
	}
	identity, err := protocol.RaidTeamID(server, channel, r.next+1)
	if err != nil {
		return raidTeam{}, err
	}
	r.next++
	entry.ID = identity
	team := raidTeam{Role: role, Channel: channel, Recruitment: entry, Origin: origin}
	if r.roles == nil {
		r.roles = map[int64]raidTeam{}
	}
	r.roles[role] = team
	return team, nil
}
func (r *raidTeamRegistry) get(role int64, channel uint32) (raidTeam, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.roles[role]
	return t, ok && t.Channel == channel
}
func (r *raidTeamRegistry) leave(role int64, id uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.roles[role]; ok && t.Recruitment.ID == id {
		delete(r.roles, role)
	}
}

func (r *raidTeamRegistry) transition(role int64, id uint32, from, to byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.roles[role]
	if !ok || t.Recruitment.ID != id || t.Recruitment.State != from {
		return fmt.Errorf("raid transition does not match owned team state")
	}
	t.Recruitment.State = to
	r.roles[role] = t
	return nil
}

func (r *raidTeamRegistry) assign(role int64, channel uint32, actor uint16, position, memberMax uint32) (raidTeam, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.roles[role]
	if !ok || t.Channel != channel || t.Recruitment.Leader != actor {
		return raidTeam{}, fmt.Errorf("assignment does not name owned member")
	}
	if memberMax == 0 || position >= memberMax || position > 255 {
		return raidTeam{}, fmt.Errorf("assignment outside PVF member limit")
	}
	t.Recruitment.MemberPosition = byte(position)
	r.roles[role] = t
	return t, nil
}
