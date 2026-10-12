package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

// Presentation is connection-owned. The inventory transaction and its replay
// receipt live in SQLite, through the shared LegionRewardService.
type nativeLegionSettlement struct {
	attempt       string
	elapsed       time.Duration
	acknowledged  bool
	movieUntil    time.Time
	movieFinished bool
	pendingExit   []byte
}

// Open the source reward wrapper and authored instant children, while keeping
// normal player-openable pots intact. A directly awarded item is not a box.
type nativeLegionRewardSource struct {
	loot.RewardBoxSource
	catalog catalog.LootCatalog
	roots   map[uint32]bool
}

func (s nativeLegionRewardSource) RewardBox(id uint32) (loot.RewardBox, bool) {
	if s.roots[id] {
		return s.RewardBoxSource.RewardBox(id)
	}
	if !s.RewardBoxSource.Container(id) {
		return loot.RewardBox{}, false
	}
	script, err := s.catalog.ItemScript(id)
	if err != nil {
		return loot.RewardBox{}, false
	}
	for _, cell := range script.Cells {
		if cell.Text == "[instantly open]" {
			return s.RewardBoxSource.RewardBox(id)
		}
	}
	return loot.RewardBox{}, false
}

func (s nativeLegionRewardSource) Container(id uint32) bool {
	if s.roots[id] {
		return s.RewardBoxSource.Container(id)
	}
	item, ok := s.catalog.Items[id]
	if !ok || item.StackableType == "[virtual]" {
		return true
	}
	if !s.RewardBoxSource.Container(id) {
		return false
	}
	script, err := s.catalog.ItemScript(id)
	if err != nil {
		return true
	}
	for _, cell := range script.Cells {
		if cell.Text == "[instantly open]" {
			return true
		}
	}
	return false
}

func (w *worldSession) boundNativeLegion() bool {
	return w != nil && w.legion != nil && w.legion.world == w && w.role.ID != 0 && w.legion.nativeOwnerID == w.role.ID && w.legion.nativeRun != nil
}

// A completed party must not capture a later ordinary dungeon. An abandoned
// run retains only its own scene until its pending return is acknowledged.
func (w *worldSession) ownsNativeLegion() bool {
	if !w.boundNativeLegion() {
		return false
	}
	r := w.legion.nativeRun
	if r.State != legion.NativeComplete && r.State != legion.NativeAbandoned {
		return true
	}
	return w.activeDungeon != nil && w.activeDungeon.RunID == r.Attempt && w.activeDungeon.Definition.ID == r.Stage().Dungeon
}

func (w *worldSession) completeNativeLegionStage() ([]outboundPacket, error) {
	if !w.ownsNativeLegion() || w.activeDungeon == nil {
		return nil, fmt.Errorf("native settlement has no owned dungeon")
	}
	s, d := w.legion, w.activeDungeon
	r := s.nativeRun
	if !d.Completed() {
		return nil, fmt.Errorf("native settlement before boss completion")
	}
	if d.RunID != r.Attempt || d.Definition.ID != r.Stage().Dungeon {
		return nil, fmt.Errorf("native settlement attempt mismatch")
	}
	if w.completionSent {
		return nil, nil
	}
	if w.store == nil || w.loot == nil || w.loot.RewardBoxes == nil || w.dungeons == nil {
		return nil, fmt.Errorf("native settlement requires durable source awards")
	}
	if r.Selected < 0 || r.Selected >= len(r.Offers) {
		return nil, fmt.Errorf("native settlement selected operation absent")
	}
	if err := r.Clear(d.Definition.ID, d.RunID, d.Completed()); err != nil {
		return nil, err
	}
	if s.nativeSettlement.attempt == "" {
		elapsed := time.Since(d.StartedAt)
		if d.StartedAt.IsZero() || elapsed <= 0 {
			return nil, fmt.Errorf("native clear clock missing")
		}
		s.nativeSettlement = nativeLegionSettlement{attempt: d.RunID, elapsed: elapsed}
	}
	if s.nativeSettlement.attempt != d.RunID {
		return nil, fmt.Errorf("native presentation belongs to another attempt")
	}
	// Operation reward difficulty is an independent source dimension. These two
	// native contents author their operation awards under normal even in expert.
	rewards, err := catalog.NativePhaseRewards(w.dungeons.LegionContents, r.Source.Name, d.Definition.ID, r.Offers[r.Selected].Index, r.Mode, "normal")
	if err != nil {
		return nil, err
	}
	roots := map[uint32]bool{}
	for _, row := range append(append([]catalog.LegionOperationReward{}, rewards.Basic...), append(rewards.Additional, rewards.GlobalClear...)...) {
		roots[row.Template] = true
	}
	boxes := nativeLegionRewardSource{RewardBoxSource: w.loot.RewardBoxes, catalog: w.loot.Catalog, roots: roots}
	svc := &workflow.LegionRewardService{Store: w.store, Awarder: &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.loot.Equipment}, Boxes: boxes}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, frozen, err := svc.Freeze(ctx, w.role, rewards, "native-legion:"+d.RunID, time.Now())
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return nil, err
	}
	project := func(items []loot.Award) []legion.IspinsRewardItem {
		out := make([]legion.IspinsRewardItem, 0, len(items))
		for _, item := range items {
			out = append(out, legion.IspinsRewardItem{Template: item.Template, Amount: item.Amount})
		}
		return out
	}
	basic, additional, err := legion.IspinsSourceRewardPayloads(project(frozen.Basic), project(frozen.Additional), s.nativeSettlement.elapsed, frozen.Eligible)
	if err != nil {
		return nil, err
	}
	_, notice, _, err := nativeLegionIdentity(r.Source.Name)
	if err != nil {
		return nil, err
	}
	cleared, err := r.Info(2, 3)
	if err != nil {
		return nil, err
	}
	if frozen.Eligible {
		saved, _, err = svc.Claim(ctx, w.role, frozen.Run)
		if saved.ID != 0 {
			w.role = saved
		}
		if err != nil {
			return nil, err
		}
	}
	snapshot, err := w.loot.Bootstrap(workflow.LootRole(w.role))
	if err != nil {
		return nil, err
	}
	if err = r.Settle(d.RunID); err != nil {
		return nil, err
	}
	// N31's current reader consumes one int32. No captured account token or
	// foreign inventory is reused. The selected offer remains alive until C72.
	plan := []outboundPacket{
		{"native_legion_inventory_committed", 0, 13, snapshot},
		{"dungeon_clear_enabled", 0, 31, protocol.DungeonClearEnabled()},
		{"native_legion_basic_reward", 0, legion.NotiIspinsBasicClearReward, basic},
		{"native_legion_operation_reward", 0, legion.NotiIspinsAdditionalClearReward, additional},
		{"native_legion_phase_cleared", 0, notice, cleared},
	}
	w.completionSent = true
	return plan, nil
}

func (s *legionSession) nativeRewardEnd(w *worldSession, p []byte, now time.Time) (legionResult, error) {
	if !w.boundNativeLegion() || w.legion != s {
		return legionResult{}, fmt.Errorf("native reward-end owner mismatch")
	}
	req, err := legion.DecodeRewardEnd(p)
	if err != nil {
		return legionResult{}, err
	}
	r := s.nativeRun
	content, notice, _, err := nativeLegionIdentity(r.Source.Name)
	if err != nil {
		return legionResult{}, err
	}
	if req.First != content || req.Second >= uint32(len(r.Order)) || req.Flag != byte(r.Order[req.Second]) {
		return legionResult{}, fmt.Errorf("native reward-end source phase mismatch")
	}
	// Current C2046 reader 1424fd3a0 consumes 13 bytes after success, then
	// cleans up the UI. Late repeats acknowledge only, even after C72 advances.
	result := legionResult{Packets: []outboundPacket{{"native_legion_reward_end_ack", 1, legion.CmdRewardEnd, legion.RewardEndAck()}}}
	if s.nativeAcknowledged[req.Second] {
		return result, nil
	}
	if req.Second != uint32(r.Phase) || r.State != legion.NativePhaseComplete || !r.RewardCommitted || !w.completionSent || w.activeDungeon == nil || w.activeDungeon.RunID != r.Attempt || s.nativeSettlement.attempt != r.Attempt {
		return legionResult{}, fmt.Errorf("native reward-end differs from committed owned phase")
	}
	if r.Phase == 2 {
		if r.Source.MovieTime <= 0 || r.Source.MovieTime > int64((time.Duration(1<<63-1))/time.Second) {
			return legionResult{}, fmt.Errorf("native source movie duration missing")
		}
		movie, e := r.Info(3, 3)
		if e != nil {
			return legionResult{}, e
		}
		result.Packets = append(result.Packets, outboundPacket{"native_legion_final_movie", 0, notice, movie})
		s.nativeSettlement.movieUntil = now.Add(time.Duration(r.Source.MovieTime) * time.Second)
	}
	s.nativeSettlement.acknowledged = true
	s.nativeAcknowledged[r.Phase] = true
	result.Events = []map[string]any{{"kind": "native_legion_reward_acknowledged", "phase": r.Phase, "attempt": r.Attempt}}
	return result, nil
}

func (w *worldSession) exitNativeLegion(p []byte, now time.Time) (*dungeon.Session, []outboundPacket, error) {
	if !w.ownsNativeLegion() || w.activeDungeon == nil {
		return nil, nil, fmt.Errorf("native exit without owned dungeon")
	}
	req, err := protocol.DecodeSettlementExit(p)
	if err != nil {
		return nil, nil, err
	}
	s, r, d := w.legion, w.legion.nativeRun, w.activeDungeon
	if r.Attempt != d.RunID || r.Stage().Dungeon != d.Definition.ID {
		return nil, nil, fmt.Errorf("native exit attempt mismatch")
	}
	ack := outboundPacket{"settlement_focus_ack", 1, 72, protocol.SettlementExitSuccess(req)}
	if req.State == 2 || req.Option == 3 {
		return nil, []outboundPacket{ack}, nil
	}
	if req.Option != 2 {
		return nil, nil, fmt.Errorf("native legion return requires town option")
	}
	next := *r
	if r.State == legion.NativeAbandoned {
		// C2044 has already closed the run; C72 only returns its owned dungeon.
	} else if d.Completed() {
		if !r.RewardCommitted || !s.nativeSettlement.acknowledged {
			return nil, nil, fmt.Errorf("native return before durable reward acknowledgement")
		}
		if !s.nativeSettlement.movieUntil.IsZero() && now.Before(s.nativeSettlement.movieUntil) {
			s.nativeSettlement.pendingExit = append([]byte(nil), p...)
			return nil, nil, nil
		}
		if err = next.Advance(); err != nil {
			return nil, nil, err
		}
	} else {
		if err = next.Retry(d.RunID); err != nil {
			return nil, nil, err
		}
	}
	_, notice, _, err := nativeLegionIdentity(r.Source.Name)
	if err != nil {
		return nil, nil, err
	}
	state := int32(6)
	if next.State == legion.NativeComplete || next.State == legion.NativeAbandoned {
		state = 5
	}
	info, err := next.Info(state, 0)
	if err != nil {
		return nil, nil, err
	}
	route, err := w.leaveDungeon()
	if err != nil {
		return nil, nil, err
	}
	if len(route) == 0 {
		return nil, nil, fmt.Errorf("native return route absent")
	}
	*r = next
	s.nativeSettlement = nativeLegionSettlement{}
	s.dimCloisterEvents = nil
	w.selectingDungeon = false
	ack.Name = "settlement_exit_ack"
	plan := append([]outboundPacket{ack}, route[1:]...)
	plan = append(plan, outboundPacket{"native_legion_return_state", 0, notice, info})
	return nil, plan, nil
}
