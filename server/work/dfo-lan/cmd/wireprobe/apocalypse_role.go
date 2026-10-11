package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
)

func (s *legionSession) adoptProjection(packets []outboundPacket) {
	for _, p := range packets {
		if p.Kind == 0 && p.ID == legion.NotiLegionInfo && len(p.Payload) == legion.LegionInfoSize && binary.LittleEndian.Uint16(p.Payload) == legion.ApocalypseContent {
			s.projection = append([]byte(nil), p.Payload...)
		}
	}
}

// Current N2895 ->14069A4D0 ->14069B820 applies roles to real party
// seats through140690F90. Only the accepted source navigation owner may
// publish this update; an ordinary activeDungeon is never an admission token.
func (s *legionSession) prepareNavigationRole(w *worldSession, role uint32, limits catalog.ApocalypseRoleLimits) (legionResult, error) {
	fail := func(reason string) (legionResult, error) {
		return legionResult{}, fmt.Errorf("apocalypse role refused: %s", reason)
	}
	if w.channelType != 119 || w.activeDungeon == nil || w.apocalypseNavigation != w.activeDungeon || !w.activeDungeon.Loaded || w.characters == nil || s.party == nil || s.party.CharacterID != w.role.ID || s.party.Actor != w.role.WireID || s.party.Context != w.characters.ChannelContext {
		return fail("navigation-room owner and party seat are not bound")
	}
	c, ok := s.contents.Contents["Apocalypse"]
	if !ok || !c.Complete || len(c.Dungeons) == 0 || w.dungeons == nil || w.service == nil || s.contents.Source.Checksum == "" || s.contents.Source.Checksum != w.dungeons.Source.Checksum || s.contents.Source.Checksum != w.service.Catalog.Source.Checksum || s.catalog.Source != catalog.ApocalypseSource || w.activeDungeon.Definition.ID != c.Dungeons[0].Dungeon {
		return fail("navigation source generation or destination mismatch")
	}
	if len(s.projection) != legion.LegionInfoSize || binary.LittleEndian.Uint16(s.projection) != legion.ApocalypseContent {
		return fail("delivered native state unavailable")
	}
	b := s.projection[2:]
	if s.session.Phase != 0 || binary.LittleEndian.Uint32(b[11:]) != 0 || b[2] != byte(s.plan.OperationID-1) {
		return fail("navigation stage or confirmed choice mismatch")
	}
	// The local party has exactly one real member at seat0. Requested capacity
	// is not member count. Shared recruitment/join must supply its own roster.
	for seat := 1; seat < 4; seat++ {
		if binary.LittleEndian.Uint32(b[107+seat*4:]) != 0 {
			return fail("projection contains an unowned party seat")
		}
	}
	if role != 0 {
		quota, ok := limits.Quota(1, role)
		if !ok || quota < 1 {
			return fail("role has no quota for the actual source member count")
		}
	}
	next := append([]byte(nil), s.projection...)
	binary.LittleEndian.PutUint32(next[2+107:], role)
	return legionResult{Packets: []outboundPacket{
		{"apocalypse_role_state", 0, legion.NotiLegionInfo, next},
		{"apocalypse_role_ack", 1, legion.CmdRoleSelect, legion.RoleSelectAck()},
	}, Events: []map[string]any{{"kind": "apocalypse_role_prepared", "character_id": w.role.ID, "actor": w.role.WireID, "seat": 0, "members": 1, "role": role, "source": s.contents.Source.Checksum, "attempt": "1/3"}}}, nil
}
