package main

import (
	"bytes"
	"dfolan/internal/boostup"
	"dfolan/internal/loot"
	"dfolan/internal/database"
	"testing"
)

func TestBoostCapsulePacketsPrimeAwakeningAndKeepConsumedSlotEmpty(t *testing.T) {
	w := moonWorlds(t, 1)[0]
	w.boostup = &boostup.Catalog{}
	role := w.role
	var e error
	role.State, e = boostup.WriteState(role.State, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 1, Phase: 1}})
	must115(t, e)
	plan, e := boostCapsuleRefresh(w, role, loot.BoostCapsuleReceipt{Slot: 65, BeforeLevel: 1, Level: 115}, []database.Character{role}, false)
	must115(t, e)
	want := []uint16{14, 2638, 2639, 19, 2, 37, 2}
	for i, id := range want {
		if plan[i].ID != id {
			t.Fatal("capsule growth order", i, plan[i].ID)
		}
	}
	ack := plan[len(plan)-1]
	if ack.Kind != 1 || ack.ID != 507 || !bytes.Equal(ack.Payload, []byte{1, 65, 0, 0, 81, 1, 0, 0}) {
		t.Fatal("actual capsule ACK shape")
	}
	// Empty-list sample: use a slot absent from this role. It must NOT become
	// template0 (gold), irrespective of its previous capsule contents.
	empty, e := boostCapsuleRefresh(w, role, loot.BoostCapsuleReceipt{Slot: 60000, BeforeLevel: 1, Level: 115}, []database.Character{role}, true)
	must115(t, e)
	if !bytes.Equal(empty[0].Payload[5:9], []byte{255, 255, 255, 255}) {
		t.Fatal("empty capsule slot became currency", empty[0].Payload[:12])
	}
	if _, e = (&worldSession{}).useBoostCapsule(make([]byte, 64)); e == nil {
		t.Fatal("unowned capsule accepted")
	}
}
