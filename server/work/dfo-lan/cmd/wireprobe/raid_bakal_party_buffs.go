package main

import (
	"dfolan/internal/game/protocol"
	"fmt"
	"sort"
	"time"
)

func (w *worldSession) bakalPartyAt(location uint32) (protocol.BakalPartyInfo, error) {
	party := protocol.BakalPartyInfo{Party: w.bakalParty, Location: location}
	for i := range party.Buffs {
		party.Buffs[i].Kind = 25
	}
	buffs := w.bakalOpening.PartyBuffs(time.Now())
	var ids []int
	for id := range buffs {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	if len(ids) > len(party.Buffs) {
		return party, fmt.Errorf("native party buff record capacity exceeded")
	}
	for i, id := range ids {
		party.Buffs[i] = protocol.BakalPartyBuff{Kind: uint32(id), ExpiresAt: buffs[uint32(id)]}
	}
	return party, nil
}
