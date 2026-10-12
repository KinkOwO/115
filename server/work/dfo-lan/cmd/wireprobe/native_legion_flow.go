package main

import (
	"crypto/rand"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"
)

func nativeLegionDraw(n int64) (int64, error) {
	if n <= 0 {
		return 0, fmt.Errorf("empty native source lottery")
	}
	v, err := rand.Int(rand.Reader, big.NewInt(n))
	if err != nil {
		return 0, err
	}
	return v.Int64(), nil
}

func nativeLegionIdentity(name string) (uint32, uint16, uint16, error) {
	switch name {
	case "Dimension Cloister":
		return 102, legion.NotiDimCloisterInfo, legion.CmdDimCloisterOperationSelect, nil
	case "DuskyIsland":
		return legion.DuskyContentID, legion.NotiDuskyInfo, legion.CmdDuskyOperationSelect, nil
	}
	return 0, 0, 0, fmt.Errorf("unknown native legion content")
}

func (s *legionSession) startNativeLegion(w *worldSession, p []byte, name string) (legionResult, error) {
	content, notice, _, err := nativeLegionIdentity(name)
	if err != nil {
		return legionResult{}, err
	}
	req, err := legion.DecodeStart(p)
	if err != nil || req.Argument != content {
		return legionResult{}, fmt.Errorf("native start content mismatch")
	}
	if w == nil || w.role.ID == 0 || w.activeDungeon != nil || w.dungeons == nil || w.dungeons.LegionContents == nil {
		return legionResult{}, fmt.Errorf("native legion start requires selected owner and current source")
	}
	if s.nativeRun != nil && s.nativeRun.State != legion.NativeComplete && s.nativeRun.State != legion.NativeAbandoned {
		return legionResult{}, fmt.Errorf("native legion already in progress")
	}
	c, ok := w.dungeons.LegionContents.Contents[name]
	if !ok || c.NeedFatigue != 0 || int64(w.level) < c.LimitLevel {
		return legionResult{}, fmt.Errorf("native legion source admission unavailable")
	}
	order := [3]int{0, 1, 2}
	if name == "Dimension Cloister" {
		// Owner policy: each new run draws three of the source's five stages.
		// Content IDs and monster records remain entirely source-derived.
		pool := make([]int, len(c.Dungeons))
		for i := range pool {
			pool[i] = i
		}
		for i := len(pool) - 1; i > 0; i-- {
			j, e := nativeLegionDraw(int64(i + 1))
			if e != nil {
				return legionResult{}, e
			}
			pool[i], pool[j] = pool[j], pool[i]
		}
		if len(pool) < 3 {
			return legionResult{}, fmt.Errorf("native source route too short")
		}
		copy(order[:], pool[:3])
	}
	for _, i := range order {
		if i < 0 || i >= len(c.Dungeons) {
			return legionResult{}, fmt.Errorf("source stage absent")
		}
		if _, ok := w.dungeons.Dungeons[c.Dungeons[i].Dungeon]; !ok {
			return legionResult{}, fmt.Errorf("source dungeon not loaded")
		}
	}
	now := time.Now()
	run, err := legion.NewNativeRun(c, "normal", order, now.Add(dimCloisterWindowDelay), nativeLegionDraw)
	if err != nil {
		return legionResult{}, err
	}
	initial, err := run.Info(1, 0)
	if err != nil {
		return legionResult{}, err
	}
	plan := []outboundPacket{}
	if name == "Dimension Cloister" {
		ledger, e := dimCloisterEntryLedgerAt(uint64(now.UnixMilli()), dimCloisterLedgerStart)
		if e != nil {
			return legionResult{}, e
		}
		plan = append(plan, outboundPacket{"native_legion_entry_ledger", 0, legion.NotiEntryCharacterInfo, ledger})
	}
	plan = append(plan, outboundPacket{"native_legion_countdown", 0, notice, initial}, outboundPacket{"native_legion_start_ack", 1, legion.CmdStart, legion.StartAck()})
	s.nativeRun = run
	s.nativeAcknowledged = [3]bool{}
	s.nativeSettlement = nativeLegionSettlement{}
	s.nativeOwnerID = w.role.ID
	s.world = w
	w.legion = s
	s.dimCloisterEvents = nil
	s.dimCloisterWindowDeadline = time.Time{}
	s.dimCloisterWindowAcked = false
	s.dimCloisterStage = -1
	s.dimCloisterCleared = 0
	s.dimCloisterStages = nil
	for _, i := range order {
		s.dimCloisterStages = append(s.dimCloisterStages, c.Dungeons[i].Dungeon)
	}
	return legionResult{Packets: plan, Events: []map[string]any{{"kind": "native_legion_started", "content": content, "source": w.dungeons.LegionContents.Source.Checksum, "order": order}}}, nil
}

func (s *legionSession) selectNativeLegion(w *worldSession, p []byte) (legionResult, error) {
	r := s.nativeRun
	if r == nil || w == nil || s.world != w || s.nativeOwnerID != w.role.ID || w.role.ID == 0 || w.activeDungeon != nil {
		return legionResult{}, fmt.Errorf("native selection outside owned town run")
	}
	_, notice, opcode, err := nativeLegionIdentity(r.Source.Name)
	if err != nil {
		return legionResult{}, err
	}
	// C2080/C2143 share the current writer's action:u32, index:i16 layout.
	req, err := legion.DecodeDuskySelection(p)
	if err != nil {
		return legionResult{}, err
	}
	now := time.Now()
	r.Ready(now)
	var ack []byte
	state := int32(2)
	if req.Action == 2 {
		if err = r.Select(uint16(req.Index), now); err != nil {
			return legionResult{}, err
		}
		ack = legion.DuskySelectionAck(2, req.Index, false, 0, 0, int16(r.Reselections))
		s.dimCloisterDropWindowEvents()
	} else {
		if err = r.Open(req.Action == 4, now); err != nil {
			return legionResult{}, err
		}
		ack = legion.DuskySelectionAck(req.Action, -1, false, uint32(r.SelectionDeadline.Unix()), 0, int16(r.Reselections))
	}
	info, err := r.Info(state, 0)
	if err != nil {
		return legionResult{}, err
	}
	return legionResult{Packets: []outboundPacket{{"native_legion_selection_state", 0, notice, info}, {"native_legion_selection_ack", 1, opcode, ack}}, Events: []map[string]any{{"kind": "native_legion_selection", "phase": r.Phase, "action": req.Action, "operation": req.Index}}}, nil
}

func (s *legionSession) enterNativeLegion(w *worldSession, p []byte) (legionResult, error) {
	r := s.nativeRun
	if r == nil || w == nil || s.world != w || s.nativeOwnerID != w.role.ID || w.role.ID == 0 || len(p) < legion.EnvelopeSize+8 {
		return legionResult{}, fmt.Errorf("native entry missing run or phase")
	}
	content, _, _, err := nativeLegionIdentity(r.Source.Name)
	if err != nil {
		return legionResult{}, err
	}
	phase := int(binary.LittleEndian.Uint32(p[17:]))
	if binary.LittleEndian.Uint32(p[13:]) != content || phase != r.Phase {
		return legionResult{}, fmt.Errorf("native entry content/phase mismatch")
	}
	d, plan, err := w.prepareNativeLegionStage(r)
	if err != nil {
		return legionResult{}, err
	}
	if err = r.Enter(phase, d.Definition.ID, d.RunID); err != nil {
		return legionResult{}, err
	}
	w.deathSent = map[uint16]bool{}
	w.drops = nil
	w.resetCards()
	w.completionSent = false
	w.completionErr = nil
	w.resultSent = false
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.pendingTownArrival = nil
	w.leaveScene()
	w.activeDungeon = d
	s.nativeSettlement = nativeLegionSettlement{}
	s.dimCloisterStage = phase
	s.dimCloisterWindowAcked = false
	s.dimCloisterWindowDeadline = time.Time{}
	s.dimCloisterEvents = nil
	return legionResult{Packets: plan, Events: []map[string]any{{"kind": "native_legion_stage_entered", "content": content, "phase": phase, "dungeon": d.Definition.ID, "run": d.RunID}}}, nil
}
