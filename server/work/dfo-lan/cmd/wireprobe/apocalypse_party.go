package main

import (
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"fmt"
)

// N9 mode+158 reads the COS waiting point and sends C36 through146D07390.
// This UI warp is not a map portal. Authorization stays with the real party
// owner; only the exact source recruiting -> waiting route bypasses portals.
func (s *legionSession) partyAreaTransition(w *worldSession, r protocol.AreaChangeRequest) (database.WorldPosition, error) {
	c, ok := s.contents.Contents["Apocalypse"]
	old := w.state.Position
	if w.channelType != 119 || !ok || old.Town != c.Recruiting.Town || old.Area != c.Recruiting.Area || r.Town != c.Waiting.Town || r.Area != c.Waiting.Area {
		return w.areaTransition(r)
	}
	w.specialWarpPending = false
	fail := func(reason string) (database.WorldPosition, error) {
		return old, fmt.Errorf("apocalypse party move: %s", reason)
	}
	if !c.Complete || s.contents.Source.Checksum == "" || w.service == nil || w.characters == nil || w.role.ID == 0 || w.activeDungeon != nil || w.selectingDungeon {
		return fail("native source and selected town character required")
	}
	if w.service.Catalog.Source.Checksum != s.contents.Source.Checksum {
		return fail("world and content source mismatch")
	}
	if s.party == nil || s.party.CharacterID != w.role.ID || s.party.Actor != w.role.WireID || s.party.Context != w.characters.ChannelContext {
		return fail("owned party required")
	}
	if r.PreviousTown != old.Town || uint32(r.PreviousArea) != old.Area || r.X != c.Waiting.X || r.Y != c.Waiting.Y {
		return fail("origin or source waiting point mismatch")
	}
	if err := w.service.ValidateRestoredPosition(w.level, w.odyssey, old); err != nil {
		return old, err
	}
	next := database.WorldPosition{Town: c.Waiting.Town, Area: c.Waiting.Area, X: c.Waiting.X, Y: c.Waiting.Y}
	if err := w.service.ValidatePosition(w.level, w.odyssey, next); err != nil {
		return old, err
	}
	return next, nil
}

// A connection owns one real local member. Capacity is the requested limit,
// never a fabricated member count. Shared recruitment/join is not bound yet.
type apocalypseParty struct {
	CharacterID int64
	Actor       uint16
	ID          uint16
	Context     [2]byte
	Options     protocol.PartyCreateOptions115
}

func (s *legionSession) handleParty(w *worldSession, body []byte, id uint16) (legionResult, error) {
	fail := func(reason string) (legionResult, error) {
		return legionResult{}, fmt.Errorf("apocalypse party: %s", reason)
	}
	if w == nil || w.channelType != 119 || w.role.ID == 0 || w.role.WireID == 0 || w.role.WireID == 65535 || w.activeDungeon != nil || w.characters == nil {
		return fail("selected town character and character service required")
	}
	c, ok := s.contents.Contents["Apocalypse"]
	if !ok || !c.Complete || s.contents.Source.Checksum == "" || c.Waiting.Town == 0 || c.Waiting.Area == 0 || c.Recruiting.Town == 0 || c.Recruiting.Area == 0 {
		return fail("native recruiting/waiting-area source unavailable")
	}
	// Native C12 is sent in the recruiting area. N9 mode+158 then invokes
	// 14250B890 ->146D07390, which sends the ordinary C36 waiting-area move.
	// Also retain the waiting area for owned-party edits and departure.
	position := w.state.Position
	inRecruiting := position.Town == c.Recruiting.Town && position.Area == c.Recruiting.Area
	inWaiting := position.Town == c.Waiting.Town && position.Area == c.Waiting.Area
	if !inRecruiting && !inWaiting {
		return fail("outside source recruiting and waiting areas")
	}
	if s.party != nil && (s.party.CharacterID != w.role.ID || s.party.Actor != w.role.WireID || s.party.Context != w.characters.ChannelContext) {
		return fail("party belongs to another character or channel context")
	}
	if id == 13 {
		if len(body) != 0 && len(body) != 8 {
			return fail("invalid leave length")
		}
		for _, b := range body {
			if b != 0 {
				return fail("unsupported leave options")
			}
		}
		if s.party == nil {
			return fail("no owned party to leave")
		}
		gone, err := protocol.ConquestPartyGone115(s.party.ID, s.party.Context, protocol.ApocalypsePartyRoute115)
		if err != nil {
			return legionResult{}, err
		}
		s.party = nil
		s.session, s.plan = nil, nil
		s.projection = nil
		return legionResult{Packets: []outboundPacket{{"apocalypse_party_gone", 0, 9, gone}}, Events: []map[string]any{{"kind": "apocalypse_party_left", "character_id": w.role.ID}}}, nil
	}
	if id != 12 {
		return fail("unsupported command")
	}
	request, err := protocol.DecodePartyCreate115(body)
	if err != nil {
		return legionResult{}, err
	}
	if request.Reserved != 0 || request.Action > 1 || request.Options.Mode != protocol.ApocalypsePartyMode115 || request.Options.Extra != 0 {
		return fail("unsupported create action or mode")
	}
	if request.Action == 1 && s.party == nil {
		return fail("update requires an owned party")
	}
	limits, err := s.catalog.CompileRoleLimits()
	if err != nil {
		return legionResult{}, err
	}
	// Quotas are keyed by actual members, not the requested party capacity.
	if _, ok := limits[1]; !ok {
		return fail("source does not support the real one-member roster")
	}
	party := &apocalypseParty{CharacterID: w.role.ID, Actor: w.role.WireID, ID: 1, Context: w.characters.ChannelContext, Options: request.Options}
	party.Options.Title = request.Name
	roster, err := protocol.LegionPartyRoster115(party.ID, party.Context, [4]uint16{party.Actor}, party.Actor, party.Options)
	if err != nil {
		return legionResult{}, err
	}
	basic, err := w.characters.EntryBasicProbe(w.role, party.Context)
	if err != nil {
		return legionResult{}, err
	}
	detail, err := w.characters.EntryAddition(w.role)
	if err != nil {
		return legionResult{}, err
	}
	// Current N9 member resolution1452F3110..391B uses the actor cache;
	// publish the existing native N2 basic/detail before the seat reference.
	s.party = party
	return legionResult{Packets: []outboundPacket{{"apocalypse_party_leader_basic", 0, 2, basic}, {"apocalypse_party_leader_detail", 0, 2, detail}, {"apocalypse_party_roster", 0, 9, roster}}, Events: []map[string]any{{"kind": "apocalypse_party_created", "character_id": party.CharacterID, "actor": party.Actor, "party_id": party.ID, "capacity": party.Options.Capacity, "members": 1, "source": s.contents.Path, "action": request.Action}}}, nil
}

func (s *legionSession) clearCharacter() {
	s.party, s.session, s.plan = nil, nil, nil
}
