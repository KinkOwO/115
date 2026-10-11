package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

func (w *worldSession) armDungeonTitleRestore() error {
	w.titleRestoreCharacter = 0
	if w.characters == nil || w.role.ID == 0 {
		return nil
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return err
	}
	if bag.TitleSkin != 0 {
		w.titleRestoreCharacter = w.role.ID
	}
	return nil
}

func (w *worldSession) dungeonTitleActorReady(op uint16, body []byte, verified bool) ([]outboundPacket, error) {
	// Both native C2423 writers read the local active actor; their body is
	// exactly two u64 values (140B4FEB0). PROCESS_SCAN does not prove readiness.
	if w == nil || !verified || op != 2423 || len(body) != 16 || w.titleRestoreCharacter == 0 {
		return nil, nil
	}
	if w.titleRestoreCharacter != w.role.ID || w.characters == nil || w.activeDungeon == nil {
		w.titleRestoreCharacter = 0
		return nil, nil
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, err
	}
	w.titleRestoreCharacter = 0
	if bag.TitleSkin == 0 {
		return nil, nil
	}
	return []outboundPacket{{"dungeon_title_appearance_restored_actor_ready", 1, 560, protocol.TitleAppearanceSuccess(bag.TitleSkin)}}, nil
}

func (client *gameConnection) dispatchTitleActorReady(r *clientRequest) dispatchAction {
	if r.frame.ID != 2423 || !client.bootstrapped || client.worldState == nil {
		return dispatchNext
	}
	plan, err := client.worldState.dungeonTitleActorReady(r.frame.ID, r.plaintext, r.verified)
	if err != nil {
		client.event(map[string]any{"kind": "dungeon_title_restore_error", "error": err.Error()})
		return dispatchHandled
	}
	if client.sendPlan(plan, client.logWorldAction) != nil {
		return dispatchClose
	}
	return dispatchHandled
}
