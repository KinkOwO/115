package main

import (
	"dfolan/internal/legion"
	"encoding/binary"
	"testing"
	"time"
)

func TestNativeNavigationInitializesDuskyWithoutSelecting(t *testing.T) {
	for _, name := range []string{"DuskyIsland", "Dimension Cloister"} {
		t.Run(name, func(t *testing.T) {
			w := nativeSettlementWorld(t, name)
			r := w.legion.nativeRun
			r.State = legion.NativeCountdown
			r.ReadyAt = time.Now().Add(time.Minute)
			if plan, err := w.nativeLegionTick(r.ReadyAt.Add(-time.Nanosecond)); err != nil || len(plan) != 0 {
				t.Fatal("navigation published before countdown completed", err)
			}
			plan, err := w.nativeLegionTick(r.ReadyAt)
			if err != nil || len(plan) == 0 || plan[0].Kind != 0 {
				t.Fatal("waiting snapshot missing", err)
			}
			if name == "DuskyIsland" {
				if len(plan) != 2 || plan[0].ID != legion.NotiDuskyInfo {
					t.Fatal("navigation must follow its own authoritative snapshot")
				}
				ack := plan[1]
				if ack.Kind != 1 || ack.ID != legion.CmdDuskyOperationSelect || len(ack.Payload) != 20 ||
					ack.Payload[0] != 1 || binary.LittleEndian.Uint32(ack.Payload[1:]) != 1 ||
					binary.LittleEndian.Uint16(ack.Payload[5:]) != 65535 || ack.Payload[7] != 1 ||
					binary.LittleEndian.Uint32(ack.Payload[12:]) != 0 || ack.Payload[16] != 0 ||
					binary.LittleEndian.Uint16(ack.Payload[18:]) != 0 {
					t.Fatal("native navigation-state ACK contract changed", ack)
				}
			} else if len(plan) != 1 || plan[0].ID != legion.NotiDimCloisterInfo {
				t.Fatal("Cloister must retain its own initialization path")
			}
			if r.State != legion.NativeWaiting || r.Selected != -1 || r.Reselections != 0 || !r.SelectionDeadline.IsZero() {
				t.Fatal("navigation priming changed a gameplay selection")
			}
			if repeat, err := w.nativeLegionTick(r.ReadyAt.Add(time.Second)); err != nil || len(repeat) != 0 {
				t.Fatal("timer reopened navigation", err)
			}
			if err := r.Open(false, r.ReadyAt.Add(2*time.Second)); err != nil {
				t.Fatal(err)
			}
			deadline := r.SelectionDeadline
			if repeat, err := w.nativeLegionTick(r.ReadyAt.Add(3 * time.Second)); err != nil || len(repeat) != 0 || r.SelectionDeadline != deadline || r.State != legion.NativeSelecting {
				t.Fatal("timer disrupted an actual selection", err)
			}
		})
	}
}

func TestNativeNavigationDoesNotInitializeForForeignOwner(t *testing.T) {
	w := nativeSettlementWorld(t, "DuskyIsland")
	r := w.legion.nativeRun
	r.State = legion.NativeCountdown
	w.legion.nativeOwnerID++
	if plan, err := w.nativeLegionTick(time.Now()); err != nil || len(plan) != 0 || r.State != legion.NativeCountdown {
		t.Fatal("foreign owner initialized the navigation", err)
	}
}
