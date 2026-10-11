package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/boostup"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

func (w *worldSession) boostTrainingAPCGate() error {
	if w == nil || w.boostup == nil || w.role.ID == 0 || w.role.AccountID != w.account || len(w.boostup.TrainingAPCs) == 0 {
		return fmt.Errorf("training APC source or owned role unavailable")
	}
	st, e := boostup.ReadState(w.role.State)
	if e != nil {
		return e
	}
	var role character.State
	if e = json.Unmarshal(w.role.State, &role); e != nil {
		return e
	}
	if !st.Activated || st.Training.Finished || st.Training.Step == 0 || int(st.Training.Step) > len(w.boostup.Steps) || st.Variant != 1 || !w.boostup.BufferEligible(w.role.Profession, role.Advancement) {
		return fmt.Errorf("active support training character required")
	}
	if w.inTutorial || w.state.Position.Town != w.boostup.Town {
		return fmt.Errorf("owned training town required")
	}
	return nil
}
func (w *worldSession) boostTrainingAPCMerge(p []byte) ([]byte, error) {
	if !w.boostTrainingAPCRequested {
		return p, nil
	}
	if e := w.boostTrainingAPCGate(); e != nil {
		return p, nil
	}
	ids := make([]uint32, len(w.boostup.TrainingAPCs))
	for i, a := range w.boostup.TrainingAPCs {
		ids[i] = a.SourceIndex
	}
	return protocol.BoostTrainingAPCSettings(p, ids)
}
func (w *worldSession) requestBoostTrainingAPC(ctx context.Context, p []byte) ([]outboundPacket, error) {
	if e := protocol.DecodeBoostTrainingAPCRequest(p); e != nil {
		return nil, e
	}
	if e := w.boostTrainingAPCGate(); e != nil {
		return nil, e
	}
	if w.adventureEliteBusy() {
		return nil, fmt.Errorf("training APC must be prepared before dungeon entry")
	}
	profile, e := w.prepareAdventure(ctx)
	if e != nil {
		return nil, e
	}
	w.boostTrainingAPCRequested = true
	settings, e := w.adventureElitePayload(ctx, profile)
	if e != nil {
		w.boostTrainingAPCRequested = false
		return nil, e
	}
	// Always answer a new request: the client may have cleared its mode3 cache.
	w.adventureEliteSnapshot = sha256.Sum256(settings)
	return []outboundPacket{{"boost_training_apc_settings", 0, 1754, settings}}, nil
}
func (w *worldSession) loadBoostTrainingAPC() ([]outboundPacket, error) {
	if !w.boostTrainingAPCRequested {
		return nil, fmt.Errorf("training APC settings were not requested")
	}
	if e := w.boostTrainingAPCGate(); e != nil {
		return nil, e
	}
	if w.adventureEliteBusy() {
		return nil, fmt.Errorf("cannot rebuild an active training companion")
	}
	// N1879 looks up the owned type-2 actor container without creating it.
	// The four-byte owner-only N1382 initializes that container with zero
	// serialized companions. The native special-source factory supplies the NPC.
	owner, err := protocol.AdventureEliteCharacterInfo(w.role.WireID, nil, nil)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{
		{"boost_training_apc_owner_container", 0, 1382, owner},
		{"boost_training_apc_ready", 0, 1879, []byte{3, 0}},
	}, nil
}
