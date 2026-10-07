package main

import (
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"dfolan/internal/world"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"
)

func recoveryClient(t *testing.T) (*gameConnection, time.Time) {
	c, at := bakalNativeClient(t)
	w := c.worldState
	body, _ := hex.DecodeString("2e20d46cfc7f0000d8f7198a0158edf50500000000000000000000000003000000fc0000006400000064000000000000")
	if _, _, e := c.handleBakalRequest(2062, body, at); e != nil {
		t.Fatal(e)
	}
	if _, _, e := c.bakalLoadingDone(nil); e != nil {
		t.Fatal(e)
	}
	w.service = &world.Service{}
	w.state.Position = database.WorldPosition{Town: 152, Area: 2, X: 350, Y: 246}
	w.bakalRecruitment = &protocol.RaidRecruitment{ID: 1, Create: protocol.RaidCreateRequest{Title: "owned"}, Leader: w.role.WireID, LeaderName: w.role.Name, LeaderLevel: 115, MemberCount: 1, MemberMax: 12, MemberPosition: 1, MemberArea: 2}
	return c, at
}

func TestBakalDeathReturnsToCampWithRecoveryInsteadOfOrdinaryFailure(t *testing.T) {
	c, at := recoveryClient(t)
	w := c.worldState
	w.pilotDeath = &odysseyDeath{Run: w.activeDungeon.RunID, Sequence: 1, Dead: true}
	w.scheduleBakalDeathReturn(at)
	w.scheduleBakalDeathReturn(at.Add(5 * time.Second))
	if w.bakalDeathReturnAt != at.Add(deathFailTimeout) {
		t.Fatal("duplicate death extended revival offer")
	}
	if p, e := c.tickBakalDeathReturn(at.Add(deathFailTimeout - time.Nanosecond)); e != nil || len(p) != 0 {
		t.Fatal("death return before native offer deadline")
	}
	plan, e := c.tickBakalDeathReturn(at.Add(deathFailTimeout))
	if e != nil {
		t.Fatal(e)
	}
	until := at.Add(deathFailTimeout + time.Duration(w.bakalRules.RevivalTimes[0])*time.Second)
	if !w.bakal.RecoveryUntil().Equal(until) || w.bakalRecruitment.MemberRecoveryUntil != uint32(until.Unix()) {
		t.Fatal("member recovery deadline missing")
	}
	seen := false
	for _, p := range plan {
		if p.ID == 33 || p.Kind == 1 && p.ID == 42 {
			t.Fatal("ordinary failure/unsolicited GIVEUP ack sent")
		}
		if p.Name == "bakal_recovery_started" {
			seen = true
			off := 35 + len(w.role.Name)
			if binary.LittleEndian.Uint32(p.Payload[off:]) != uint32(until.Unix()) {
				t.Fatal("wrong N578 recovery offset")
			}
		}
	}
	if !seen || w.activeDungeon != nil || w.pilotDeath != nil || w.bakal.Stage() != legion.BakalOpeningActive {
		t.Fatal("death return did not retain active raid and clear battle")
	}
	if p, e := c.tickBakalDeathReturn(until); e != nil || len(p) != 0 {
		t.Fatal("duplicate death return")
	}
}

func TestBakalRevivalAndNoPenaltyKickCancelDeathSchedule(t *testing.T) {
	c, at := recoveryClient(t)
	w := c.worldState
	w.pilotDeath = &odysseyDeath{Run: w.activeDungeon.RunID, Sequence: 1, Dead: true}
	w.scheduleBakalDeathReturn(at)
	w.pilotDeath.Dead = false
	if p, e := c.tickBakalDeathReturn(at.Add(deathFailTimeout)); e != nil || len(p) != 0 || !w.bakal.RecoveryUntil().IsZero() {
		t.Fatal("revived player was returned/penalized")
	}
	w.pilotDeath.Dead = true
	w.pilotDeath.Sequence++
	w.scheduleBakalDeathReturn(at)
	if _, e := w.bakal.CampReturnAt(at, legion.BakalReturnNoPenalty); e != nil {
		t.Fatal(e)
	}
	w.activeDungeon = nil
	if p, e := c.tickBakalDeathReturn(at.Add(deathFailTimeout)); e != nil || len(p) != 0 || !w.bakal.RecoveryUntil().IsZero() {
		t.Fatal("no-penalty return became death penalty")
	}
}

func TestBakalRecoveryRejectsPortalBeforeDecodeAndAllowsAtDeadline(t *testing.T) {
	c, at := recoveryClient(t)
	w := c.worldState
	if _, e := w.bakalRetreatPlanAt(at, legion.BakalReturnVoluntary); e != nil {
		t.Fatal(e)
	}
	until := w.bakal.RecoveryUntil()
	if handled, p, e := c.enterBakalPortal(2062, nil, until.Add(-time.Nanosecond)); !handled || e == nil || len(p) != 0 {
		t.Fatal("recovery entry gate bypassed")
	}
	body, _ := hex.DecodeString("2e20d46cfc7f0000d8f7198a0158edf50500000000000000000000000003000000fc0000006400000064000000000000")
	if handled, _, e := c.enterBakalPortal(2062, body, until); !handled || e != nil {
		t.Fatal("entry at recovery deadline rejected", e)
	}
}

func TestBakalRecoveryFinishPublishedOnceOnSerialTick(t *testing.T) {
	c, at := recoveryClient(t)
	w := c.worldState
	if _, e := w.bakalRetreatPlanAt(at, legion.BakalReturnVoluntary); e != nil {
		t.Fatal(e)
	}
	until := w.bakal.RecoveryUntil()
	out, _, events := newDispatchTestClient()
	out.worldState = w
	if e := out.tickBakalOpening(until.Add(-time.Nanosecond)); e != nil {
		t.Fatal(e)
	}
	if w.bakalRecruitment.MemberRecoveryUntil == 0 {
		t.Fatal("recovery cleared early")
	}
	if e := out.tickBakalOpening(until); e != nil {
		t.Fatal(e)
	}
	if w.bakalRecruitment.MemberRecoveryUntil != 0 {
		t.Fatal("recovery member state not cleared at deadline")
	}
	if e := out.tickBakalOpening(until.Add(time.Second)); e != nil {
		t.Fatal(e)
	}
	n := 0
	for _, e := range *events {
		if e["kind"] == "bakal_recovery_finished" && e["id"] == uint16(578) {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("recovery finish packets=%d", n)
	}
}

func TestBakalCommanderCampReturnDoesNotPenalizePendingDeath(t *testing.T) {
	c, at := recoveryClient(t)
	w := c.worldState
	w.pilotDeath = &odysseyDeath{Run: w.activeDungeon.RunID, Sequence: 1, Dead: true}
	w.scheduleBakalDeathReturn(at)
	if _, e := w.bakal.UseRaidBuffAt(4, at); e != nil {
		t.Fatal(e)
	}
	if !w.bakal.NeedsCampReturn() {
		t.Fatal("source commander did not request camp")
	}
	if p, e := c.tickBakalDeathReturn(at.Add(deathFailTimeout)); e != nil || len(p) != 0 || !w.bakal.RecoveryUntil().IsZero() {
		t.Fatal("commander no-penalty return received death penalty")
	}
}

func TestBakalStaleDeathRunDoesNotReturnNewDungeon(t *testing.T) {
	c, at := recoveryClient(t)
	w := c.worldState
	w.pilotDeath = &odysseyDeath{Run: w.activeDungeon.RunID, Sequence: 1, Dead: true}
	w.scheduleBakalDeathReturn(at)
	w.activeDungeon.RunID = "different-run"
	if p, e := c.tickBakalDeathReturn(at.Add(deathFailTimeout)); e != nil || len(p) != 0 || !w.bakalDeathReturnAt.IsZero() {
		t.Fatal("stale death returned new run")
	}
}
