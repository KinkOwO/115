package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"fmt"
)

// 142E5EDA0 chooses3 for the native Boost event; this is a protocol enum,
// not a content ID. AIC template/eligibility come from the selected PVF.
const boostAPCMode uint16 = 3

func (w *worldSession) boostAPCSelection() (protocol.AdventureEliteSelection, error) {
	row, err := w.boostAPCSelectionForView()
	if err != nil {
		return row, err
	}
	if w.activeDungeon != nil || w.selectingDungeon || w.pendingTownArrival != nil || w.bleedingMineStart != nil || w.specialWarpPending {
		return row, fmt.Errorf("请完成回城后准备活动同伴")
	}
	return row, nil
}

// Existing native actors keep this same selection across periodic account
// refreshes and dungeon transitions. Only new loading requires a settled town.
func (w *worldSession) boostAPCSelectionForView() (protocol.AdventureEliteSelection, error) {
	row := protocol.AdventureEliteSelection{Mode: boostAPCMode, Slots: [3]int32{-1, -1, -1}}
	if w == nil || w.boostup == nil || w.role.ID == 0 || w.role.WireID == 0 {
		return row, fmt.Errorf("活动角色尚未准备")
	}
	state, err := boostup.ReadState(w.role.State)
	if err != nil {
		return row, err
	}
	if !state.Activated || state.Training.Finished || w.state.Position.Town != w.boostup.Town || !w.ordinaryBoostChannel() || adventureEliteChannel(w.channelType) {
		return row, fmt.Errorf("请在当前角色的Boost教学区域准备同伴")
	}
	count := 0
	for _, apc := range w.boostup.TeachingAPCs {
		if apc.Event != boostup.EventID {
			continue
		}
		if count == len(row.APCIndices) {
			return row, fmt.Errorf("源定义的活动同伴超过客户端容量")
		}
		// 142E5C6B6 compares source node+52 ([index]), not its AIC
		// template at+44. Live PID2220: 2504 stayed unresolved (-1);
		// source index1 maps to virtual character4 and AIC2504.
		row.APCIndices[count] = apc.Index
		count++
	}
	if count == 0 {
		return row, fmt.Errorf("当前PVF缺少本活动的教学同伴")
	}
	return row, nil
}

func mergeBoostAPCSelection(rows []protocol.AdventureEliteSelection, row protocol.AdventureEliteSelection) []protocol.AdventureEliteSelection {
	var selections []protocol.AdventureEliteSelection
	for _, old := range rows {
		if old.Mode != boostAPCMode {
			selections = append(selections, old)
		}
	}
	return append(selections, row)
}

// Live030159 events:175 sent1754(count0) after the successful1811 load.
// Native142E5A4C0 clears its entire map and142E5AE64 releases the actors.
// All account projections must therefore retain the owned event mode3 row.
func (w *worldSession) withBoostAPCSelection(rows []protocol.AdventureEliteSelection) ([]protocol.AdventureEliteSelection, error) {
	if !w.boostAPCSelectionSent {
		return rows, nil
	}
	if w.boostup == nil {
		w.boostAPCSelectionSent = false
		return rows, nil
	}
	state, err := boostup.ReadState(w.role.State)
	if err != nil {
		return nil, err
	}
	if !state.Activated || state.Training.Finished || w.state.Position.Town != w.boostup.Town || !w.ordinaryBoostChannel() || adventureEliteChannel(w.channelType) {
		w.boostAPCSelectionSent = false
		return rows, nil
	}
	row, err := w.boostAPCSelectionForView()
	if err != nil {
		return nil, err
	}
	return mergeBoostAPCSelection(rows, row), nil
}

func (w *worldSession) requestBoostAPC(ctx context.Context, p []byte) ([]outboundPacket, error) {
	if err := protocol.DecodeBoostAPCRequest(p); err != nil {
		return nil, err
	}
	row, err := w.boostAPCSelection()
	if err != nil {
		return nil, err
	}
	profile, err := w.prepareAdventure(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := w.store.Characters(ctx, w.account)
	if err != nil {
		return nil, err
	}
	rows := adventureEliteSelectionsForRoles(w.eliteProfileView(profile), roles)
	body, err := protocol.AdventureEliteSelections(mergeBoostAPCSelection(rows, row))
	if err != nil {
		return nil, err
	}
	w.boostAPCSelectionSent = true
	w.adventureEliteSnapshot = sha256.Sum256(body)
	// 2333 ACK is nullsub_1.1754 converts the special index to the client's
	// virtual index and automatically sends1811(mode3); do not guess an ACK.
	return []outboundPacket{{"Boost教学同伴选择（attempt 3/3）", 0, 1754, body}}, nil
}

func (w *worldSession) loadBoostAPC() ([]outboundPacket, error) {
	if _, err := w.boostAPCSelection(); err != nil {
		return nil, err
	}
	if !w.boostAPCSelectionSent {
		return nil, fmt.Errorf("活动同伴选择尚未下发")
	}
	// 1444FCF90 chooses container2 while Boost is active. An empty native
	// record clears old role snapshots;1879 creates the AIC locally through
	// 1444FABF0/1459628F0, then binds it to the native APC slot0.
	body, err := protocol.AdventureEliteCharacterInfo(w.role.WireID, nil, nil)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{
		{"Boost原生APC资料容器（attempt 3/3）", 0, 1382, body},
		{"Boost教学同伴资料完成", 0, 1879, []byte{byte(boostAPCMode), 0}},
	}, nil
}
