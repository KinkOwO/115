package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
)

// Native N574 and the N578 owned-raid descriptor are distinct caches.
// Official active mode2 has state2/phase0/nonzero Unix start; idle is 0/ff/0.
func (w *worldSession) bakalLifecyclePlan() ([]outboundPacket, error) {
	if w.bakal == nil || w.bakalRecruitment == nil {
		return nil, nil
	}
	next := *w.bakalRecruitment
	next.State, next.ActiveStartedAt = 0, 0
	switch w.bakal.Stage() {
	case legion.BakalOpeningPreparing:
		next.State = 1
	case legion.BakalOpeningActive, legion.BakalOpeningFinal:
		next.State = 2
		next.ActiveStartedAt = uint32(w.bakal.ActiveSince().Unix())
	}
	if next.State == w.bakalRecruitment.State && next.ActiveStartedAt == w.bakalRecruitment.ActiveStartedAt {
		return nil, nil
	}
	body, e := protocol.RaidOwnedDetails(next)
	if e != nil {
		return nil, e
	}
	binary.LittleEndian.PutUint32(body[4:], 2)
	*w.bakalRecruitment = next
	return []outboundPacket{{"bakal_lifecycle_owned_raid", 0, 578, body}}, nil
}
