package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/workflow"
	"encoding/binary"
	"fmt"
	"reflect"
	"time"
)

func (w *worldSession) apocalypseRewardService() *workflow.ApocalypseRewardService {
	if w == nil || w.store == nil || w.loot == nil {
		return nil
	}
	return &workflow.ApocalypseRewardService{Store: w.store, Awarder: &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.loot.Equipment}}
}

func (w *worldSession) recoverApocalypseRewards() ([]outboundPacket, []map[string]any) {
	rewards := w.apocalypseRewardService()
	if rewards == nil {
		return nil, nil
	}
	before, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, []map[string]any{{"kind": "apocalypse_reward_recovery_pending", "reason": err.Error()}}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, err := rewards.Recover(ctx, w.role)
	if saved.ID != 0 {
		w.role = saved
	}
	var events []map[string]any
	if err != nil {
		events = append(events, map[string]any{"kind": "apocalypse_reward_recovery_pending", "reason": err.Error()})
	}
	after, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, append(events, map[string]any{"kind": "apocalypse_reward_recovery_pending", "reason": err.Error()})
	}
	rows := inventory.ChangedItemRows(before, after)
	if len(rows) == 0 {
		return nil, events
	}
	body, err := protocol.InventoryUpdate(rows)
	if err != nil {
		return nil, append(events, map[string]any{"kind": "apocalypse_reward_recovery_pending", "reason": err.Error()})
	}
	return []outboundPacket{{"apocalypse_pending_clear_inventory_recovered", 0, 14, body}}, events
}

// A no-next-action boss is not itself a full-clear signal. Only the source
// fixed flow or schedule-stable full main route's final combat, with every
// earlier source death delivered, owns settlement. Shortcuts still require
// their actual gate selection and elapsed clock executor.
func (s *legionSession) apocalypseTerminalClear(w *worldSession) (*apocalypseRoute, bool, error) {
	r, err := s.validateRoute(w)
	if err != nil {
		return nil, false, err
	}
	if r.RewardShown || r.PhaseKey == 0 || r.PendingPhase != 0 || !apocalypseBossDeathDelivered(w, r) {
		return r, false, nil
	}
	path, fixed, err := s.plan.Gates.LinearRoute()
	if err != nil {
		return nil, false, err
	}
	if !fixed {
		path, fixed, err = s.plan.Gates.StableMainRoute(r.First)
		if err != nil {
			return nil, false, err
		}
	}
	if !fixed || len(path) == 0 || path[len(path)-1] != r.PhaseKey {
		return r, false, nil
	}
	if r.Stage != uint32(len(path)) || r.First != path[0] || s.projection[2+27+12*int(r.Stage)] != r.PhaseKey || w.apocalypseNavigation == nil || w.apocalypseNavigation.RunID == "" || r.Active == w.apocalypseNavigation || r.Active.Definition.SourceBoss != r.Entry.Bosses[r.PhaseKey] {
		return nil, false, fmt.Errorf("apocalypse terminal run or source target mismatch")
	}
	for _, phase := range path[:len(path)-1] {
		if int(phase) >= len(r.Cleared) || !r.Cleared[phase] {
			return nil, false, fmt.Errorf("apocalypse earlier source boss not cleared: phase=%d", phase)
		}
	}
	for ordinal, phase := range path {
		if s.projection[2+27+12*(ordinal+1)] != phase {
			return nil, false, fmt.Errorf("delivered battle history differs from source route")
		}
	}
	current, err := legion.BuildRunPlan(s.catalog, s.clock, r.Operation)
	if err != nil || !reflect.DeepEqual(current, s.plan) {
		return nil, false, fmt.Errorf("apocalypse source plan changed before settlement")
	}
	return r, true, nil
}

func (s *legionSession) prepareApocalypseClear(w *worldSession, rewards *workflow.ApocalypseRewardService, now time.Time) (legionResult, error) {
	r, ready, err := s.apocalypseTerminalClear(w)
	if err != nil || !ready {
		return legionResult{}, err
	}
	reward, err := legion.GuaranteedClearReward(s.catalog, r.Operation)
	if err != nil {
		return legionResult{}, err
	}
	if rewards == nil || rewards.Awarder == nil || rewards.Awarder.Catalog.Source.Checksum != r.Entry.Source {
		return legionResult{}, fmt.Errorf("apocalypse durable source reward service unavailable")
	}
	if w.loot == nil {
		return legionResult{}, fmt.Errorf("apocalypse basic reward source unavailable")
	}
	items, err := apocalypseBasicProducts(s.contents, rewards.Awarder.Catalog, w.loot.RewardBoxes, reward.Label, w.apocalypseNavigation.RunID)
	if err != nil {
		return legionResult{}, err
	}
	elapsed := now.Sub(w.apocalypseNavigation.StartedAt).Milliseconds()
	if elapsed < 0 {
		return legionResult{}, fmt.Errorf("apocalypse clock predates admission")
	}
	plan := workflow.ApocalypseRewardPlan{Run: w.apocalypseNavigation.RunID, Source: w.role.ConfigVersion, Content: r.Entry.Source, Rules: r.CatalogHash, Operation: r.Operation, Phase: r.PhaseKey, ElapsedMillis: uint64(elapsed), Reward: reward, Additional: items, AdditionalRules: s.contents.SHA256}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, frozen, err := rewards.Freeze(ctx, w.role, plan)
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return legionResult{}, err
	}
	if frozen.Run != plan.Run || frozen.Source != plan.Source || frozen.Content != plan.Content || frozen.Rules != plan.Rules || frozen.Operation != plan.Operation || frozen.Phase != plan.Phase || frozen.Reward != reward {
		return legionResult{}, fmt.Errorf("apocalypse persisted clear identity differs")
	}
	if len(frozen.Additional) > 0 && frozen.AdditionalRules != s.contents.SHA256 {
		return legionResult{}, fmt.Errorf("apocalypse persisted basic reward rules differ")
	}
	saved, grant, err := rewards.Claim(ctx, w.role, plan.Run)
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return legionResult{}, err
	}
	if !grant.Matches(frozen) {
		return legionResult{}, fmt.Errorf("apocalypse reward receipt does not match source")
	}
	// Projection contains only committed items. Never advertise a grant before
	// storage accepts it, and never use a per-room RunID as the event key.
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return legionResult{}, err
	}
	var rows [][protocol.CurrentItemRecordSize]byte
	wanted := map[uint16]bool{}
	for _, slot := range grant.Slots {
		wanted[slot] = true
	}
	for _, item := range grant.Additional {
		for _, slot := range item.Slots {
			wanted[slot] = true
		}
	}
	for _, row := range bag.Rows() {
		if wanted[binary.LittleEndian.Uint16(row[:])] {
			rows = append(rows, row)
		}
	}
	if len(rows) != len(wanted) {
		return legionResult{}, fmt.Errorf("apocalypse grant slots unavailable")
	}
	updates, err := protocol.InventoryUpdate(rows)
	if err != nil {
		return legionResult{}, err
	}
	basic, err := legion.ApocalypseClearRewardItems(frozen.Reward, frozen.Additional, frozen.ElapsedMillis)
	if err != nil {
		return legionResult{}, err
	}
	projection := append([]byte(nil), s.projection...)
	binary.LittleEndian.PutUint32(projection[9:], legion.ApocalypseRewardOutcome(projection[4]))
	route := *r
	route.Cleared[r.PhaseKey], route.RewardShown = true, true
	return legionResult{Route: &route, Packets: []outboundPacket{
		{"apocalypse_committed_reward_inventory", 0, 14, updates},
		{"apocalypse_source_clear_outcome", 0, legion.NotiLegionInfo, projection},
		{"apocalypse_basic_clear_reward", 0, legion.NotiClearRewardBasic, basic},
		{"apocalypse_additional_clear_reward", 0, legion.NotiClearRewardAdditional, legion.ApocalypseNoAdditionalReward()},
	}, Events: []map[string]any{{"kind": "apocalypse_full_clear_reward_committed", "run": frozen.Run, "operation": r.Operation, "phase": r.PhaseKey, "item": grant.Template, "amount": grant.Amount, "basic_items": grant.Additional, "basic_rules": frozen.AdditionalRules, "source": r.Entry.Source, "attempt": "1/3", "client_acceptance": "pending"}}}, nil
}

func (s *legionSession) prepareApocalypseRewardEnd(w *worldSession, request legion.RewardEndRequest, rewards *workflow.ApocalypseRewardService) (legionResult, error) {
	r, err := s.validateRoute(w)
	if err != nil {
		return legionResult{}, err
	}
	if !r.RewardShown || request.First != uint32(legion.ApocalypseContent) || request.Second != legion.ApocalypseRewardOutcome(s.projection[4]) || request.Flag != r.PhaseKey || binary.LittleEndian.Uint32(s.projection[9:]) != request.Second || w.apocalypseNavigation == nil || rewards == nil {
		return legionResult{}, fmt.Errorf("apocalypse reward-end does not match delivered full clear")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	plan, err := rewards.Granted(ctx, w.role, w.apocalypseNavigation.RunID)
	if err != nil || plan.Rules != r.CatalogHash || plan.Operation != r.Operation || plan.Phase != r.PhaseKey {
		return legionResult{}, fmt.Errorf("apocalypse reward-end requires committed owned award: %v", err)
	}
	town, err := s.apocalypseWaitingTransport(w)
	if err != nil {
		return legionResult{}, err
	}
	reset := s.apocalypseClosedProjection()
	packets := []outboundPacket{{"apocalypse_reward_end_ack", 1, legion.CmdRewardEnd, legion.RewardEndAck()}, {"apocalypse_completed_roles_and_ui_cleared", 0, legion.NotiLegionInfo, reset}}
	return legionResult{Packets: append(packets, town[1:]...), ReturnWaiting: true, Events: []map[string]any{{"kind": "apocalypse_reward_end_prepared", "run": plan.Run, "attempt": "1/3", "client_acceptance": "pending"}}}, nil
}
