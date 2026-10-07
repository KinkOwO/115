package main

import (
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"fmt"
	"sort"
	"time"
)

// Invoked only from the connection's serial tick; no asynchronous timer
// mutates a raid or sends a late start after leave/disconnect.
func (c *gameConnection) tickBakalOpening(now time.Time) error {
	w := c.worldState
	if w == nil || w.bakalOpening == nil || now.Before(w.bakalOpening.ReadyAt()) {
		return nil
	}
	if w.bakalOpening.Active() {
		if len(w.bakalRules.Events) == 0 {
			return nil
		}
		effects, err := w.bakalOpening.Tick(now)
		if err != nil {
			return err
		}
		plan, err := w.bakalScriptPlan(effects)
		if err != nil {
			return err
		}
		settlement, err := w.bakalSettlementPlan(now)
		if err != nil {
			// Send the scene transition even when an award remains pending.
			// The frozen plan is retried on the next serial tick/reconnect.
			c.event(map[string]any{"kind": "bakal_reward_pending", "reason": err.Error()})
		} else {
			// Publish the durable clear result while the client still owns raid phase.
			at := len(plan)
			for i, packet := range plan {
				if packet.Name == "bakal_script_phase_ended" {
					at = i
					break
				}
			}
			tail := append([]outboundPacket(nil), plan[at:]...)
			plan = append(append(plan[:at], settlement...), tail...)
		}
		if err := c.sendPlan(plan, c.logCharacterResponseBody); err != nil {
			return err
		}
		return c.releaseEndedBakalOpening()
	}
	team, owned := c.raidTeams.get(w.role.ID, c.channel)
	if !owned || team.Recruitment.ID != c.ownedRaidID || !w.raidWaiting || w.activeDungeon != nil {
		w.bakalOpening, w.bakalRules = nil, nil
		return nil
	}
	if err := w.bakalOpening.Activate(now); err != nil {
		return err
	}
	if err := c.raidTeams.transition(w.role.ID, team.Recruitment.ID, 1, 2); err != nil {
		return err
	}
	plan, err := w.bakalOpeningPlan(now)
	if err != nil {
		return err
	}
	if err = c.sendPlan(plan, c.logCharacterResponseBody); err != nil {
		return err
	}
	c.event(map[string]any{"kind": "bakal_started", "raid_id": team.Recruitment.ID, "members": 1, "party": w.bakalParty, "dungeons": len(w.bakalRules.Dungeons), "monsters": len(w.bakalRules.Monsters)})
	return nil
}

// Release only after returning to camp and publishing the terminal plan.
// A clear with an uncommitted award retains its opening for settlement retries.
func (c *gameConnection) releaseEndedBakalOpening() error {
	w := c.worldState
	if w == nil || w.bakalOpening == nil || w.activeDungeon != nil || w.bakalOpening.Ended() == "" {
		return nil
	}
	if w.bakalOpening.Ended() == "clear" {
		run, _, ready := w.bakalOpening.SettlementIdentity()
		if !ready || run != w.bakalSettledRun {
			return nil
		}
	}
	if c.gatewayRuntime != nil {
		if team, owned := c.raidTeams.get(w.role.ID, c.channel); owned && team.Recruitment.State == 2 {
			if err := c.raidTeams.transition(w.role.ID, team.Recruitment.ID, 2, 0); err != nil {
				return err
			}
		}
	}
	w.bakalOpening, w.bakalRules = nil, nil
	w.bakalRewardRetryAt = time.Time{}
	return nil
}

func (w *worldSession) bakalOpeningPlan(now time.Time) ([]outboundPacket, error) {
	if w.bakalOpening == nil || !w.bakalOpening.Active() || w.bakalRules == nil || w.bakalParty < 1 || w.bakalParty > 3 {
		return nil, fmt.Errorf("missing active Bakal opening")
	}
	// NOTI574 state2 reaches the Bakal start branch at144ce838c and
	// initializes14254a6b0. State6 skips that branch and only refreshes UI.
	plan := []outboundPacket{{"bakal_active", 0, 574, []byte{2, 0}}}
	party := protocol.BakalPartyInfo{Party: w.bakalParty, Location: 51 + w.bakalParty}
	for i := range party.Buffs {
		party.Buffs[i].Kind = 25
	}
	p, err := protocol.BakalPartyInfoPayload([]protocol.BakalPartyInfo{party})
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"bakal_party", 0, 2285, p})
	var monsters []protocol.BakalMonsterInfo
	for _, m := range w.bakalRules.Monsters {
		kind, e := protocol.BakalMonsterType(m.Kind)
		if e != nil {
			return nil, e
		}
		health := w.bakalRules.InitialVariables["[MONSTER MAX HP]"]
		if health <= 0 {
			return nil, fmt.Errorf("missing source monster health scale")
		}
		monsters = append(monsters, protocol.BakalMonsterInfo{Kind: kind, Location: m.Location, Action: 1, Health: uint32(health), Parties: [3]int8{-1, -1, -1}})
	}
	sort.Slice(monsters, func(i, j int) bool { return monsters[i].Location < monsters[j].Location })
	p, err = protocol.BakalMonsterInfoPayload(monsters)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"bakal_monsters", 0, 2286, p})
	var counts [5]byte
	for _, b := range w.bakalRules.Buffs {
		if b.ID >= 5 || b.Count > 255 {
			return nil, fmt.Errorf("unsupported source opening buff")
		}
		counts[b.ID] = byte(b.Count)
	}
	p, err = protocol.BakalBuffInfoPayload(counts, 25, ^uint32(0))
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"bakal_buffs", 0, 2288, p})
	// Only source-open dungeons are announced. Reader144ce2440 passes
	// the state byte unchanged to144cfb5d0. Bakal portal check142547880
	// requires the stored state to be zero; the old state2 blocked locally.
	p = []byte{0, 0}
	for _, d := range w.bakalRules.Dungeons {
		if d.State != "open" {
			continue
		}
		if p[1] == 255 {
			return nil, fmt.Errorf("too many phase dungeons")
		}
		p[1]++
		p = binary.LittleEndian.AppendUint32(p, d.ID)
		p = append(p, 0)
	}
	p = binary.LittleEndian.AppendUint32(p, 0)
	plan = append(plan, outboundPacket{"bakal_open_dungeons", 0, 572, p})
	p = binary.LittleEndian.AppendUint32([]byte{0}, w.bakalOpening.Remaining(now))
	// 144ce4c60 reads u8 plus TWO i32s; the second value is the
	// additional timer/status field. Omitting it triggers CMD217 overflow.
	p = binary.LittleEndian.AppendUint32(p, 0)
	plan = append(plan, outboundPacket{"bakal_remaining", 0, 584, p})
	symbols, err := w.bakalSymbolPlan()
	if err != nil {
		return nil, err
	}
	plan = append(plan, symbols...)
	return plan, nil
}

func (w *worldSession) bakalSymbolPlan() ([]outboundPacket, error) {
	if w.bakalOpening == nil {
		return nil, nil
	}
	values := w.bakalOpening.SymbolValues()
	return bakalSymbolValuePlan(values)
}

func (w *worldSession) bakalEnterSymbolPlan(s *dungeon.Session, commit bool) ([]outboundPacket, error) {
	if w.bakalOpening == nil {
		return nil, nil
	}
	if s == nil {
		return nil, fmt.Errorf("Bakal entry symbols require owned dungeon")
	}
	values, err := w.bakalOpening.EnterDungeon(s.Definition.ID, time.Now(), commit)
	if err != nil {
		return nil, err
	}
	return bakalSymbolValuePlan(values)
}

func bakalSymbolValuePlan(values map[uint32]int32) ([]outboundPacket, error) {
	var ids []uint32
	for id := range values {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var plan []outboundPacket
	for _, id := range ids {
		p, err := protocol.RaidGlobalSymbol115(id, values[id])
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"bakal_source_symbol", 0, 570, p})
	}
	return plan, nil
}

func (w *worldSession) bakalRoomPartyPlan(s *dungeon.Session) ([]outboundPacket, error) {
	if w.bakalOpening == nil {
		return nil, nil
	}
	if s == nil || w.bakalRules == nil {
		return nil, fmt.Errorf("Bakal party location requires source room")
	}
	var location uint32
	for id := range w.bakalRules.Slots {
		if _, err := w.bakalOpening.AuthorizeSlot(id, s.Definition.ID, s.Room.Map, time.Now()); err == nil {
			if location != 0 {
				return nil, fmt.Errorf("ambiguous Bakal room location")
			}
			location = id
		}
	}
	if location == 0 {
		if s.Definition.ID == w.bakalOpening.FinalDungeon() {
			for id, loc := range w.bakalRules.Locations {
				if loc.Dungeon == s.Definition.ID {
					location = id
					break
				}
			}
		}
		if location == 0 {
			return nil, fmt.Errorf("Bakal room has no active native location")
		}
	}
	party, err := w.bakalPartyAt(location)
	if err != nil {
		return nil, err
	}
	p, err := protocol.BakalPartyInfoPayload([]protocol.BakalPartyInfo{party})
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"bakal_room_party_location", 0, 2285, p}}, nil
}

// Resolve the current room through the native battlefield, then generate only
// its source-defined boss. Fixed map actors always win over a dynamic spawn.
func (w *worldSession) bakalLoadedBoss(s *dungeon.Session, now time.Time) ([]outboundPacket, error) {
	if s == nil || w.bakalRules == nil {
		return nil, fmt.Errorf("Bakal loading without an owned dungeon")
	}
	if err := w.bakalOpening.AuthorizeDungeon(s.Definition.ID, now); err != nil {
		return nil, err
	}
	if s.Definition.ID == w.bakalOpening.FinalDungeon() {
		return nil, nil
	}
	var slotIDs []uint32
	for id := range w.bakalRules.Slots {
		slotIDs = append(slotIDs, id)
	}
	sort.Slice(slotIDs, func(i, j int) bool { return slotIDs[i] < slotIDs[j] })
	for _, id := range slotIDs {
		if _, err := w.bakalOpening.AuthorizeSlot(id, s.Definition.ID, s.Room.Map, now); err != nil {
			continue
		}
		m, ok := w.bakalOpening.InitialMonster(id)
		if !ok {
			return nil, nil
		}
		grid := s.Maze.Boss
		if m.HasGrid {
			grid = m.Grid
		}
		if [2]byte{s.Room.X, s.Room.Y} != grid {
			return nil, nil
		}
		if m.SpecificMap >= 0 && uint32(m.SpecificMap) != s.Room.Map {
			return nil, nil
		}
		row, spawn, err := s.AddRaidBoss(m)
		if err != nil {
			return nil, err
		}
		if !spawn {
			return nil, nil
		}
		body, err := protocol.UnassignedMonsterAdd115([]protocol.UnassignedMonster115{row})
		if err != nil {
			return nil, err
		}
		return []outboundPacket{{"bakal_source_boss", 0, protocol.NotiUnassignedMonsterAdd, body}}, nil
	}
	return nil, fmt.Errorf("Bakal boss room is outside native battlefield")
}
