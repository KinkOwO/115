package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"encoding/binary"
	"fmt"
	"testing"
	"time"
)

func nativeSettlementWorld(t *testing.T, name string) *worldSession {
	t.Helper()
	w := ispinsSQLiteRewardWorld(t)
	w.ispins = nil
	w.activeDungeon = nil
	w.level = 115
	w.state = database.WorldState{Position: database.WorldPosition{Town: 160, Area: 2, X: 300, Y: 300}}
	limit := int64(-1)
	c := catalog.LegionContent{Name: name, ProbabilityRate: 1, PhaseReward: 1, LastPhase: 2, SelectLimit: 30, MovieTime: 72, RetryCount: &limit, ReselectCount: &limit,
		Dungeons:         []catalog.LegionDungeon{{Dungeon: 100, Boss: 200}, {Dungeon: 101, Boss: 201}, {Dungeon: 102, Boss: 202}},
		Rewards:          []catalog.LegionRewardRow{{PartyType: "solo", Difficulty: "normal", Kind: "basic", Values: []int64{1, 21, 8}}},
		NativeOperations: &catalog.NativeLegionOperations{Modes: map[string][]uint16{}, Definitions: map[uint16]catalog.NativeLegionOperation{}}}
	pools := []uint32{0, 1, 2, 3}
	if name == "DuskyIsland" {
		pools = []uint32{0, 1, 2, 4, 5, 6}
	}
	for i, pool := range pools {
		id := uint16(i + 1)
		c.NativeOperations.Modes["normal"] = append(c.NativeOperations.Modes["normal"], id)
		c.NativeOperations.Definitions[id] = catalog.NativeLegionOperation{SelectPool: pool, MemberLimit: "party", Probability: 1,
			Ranges:  []catalog.NativeLegionRange{{Party: "party", Difficulty: "normal", Minimum: 10, Maximum: 10}},
			Rewards: []catalog.NativeLegionReward{{Difficulty: "normal", Template: 22, Amount: 3, Weight: 1}}}
	}
	w.dungeons.LegionContents.Contents[name] = c
	r, err := legion.NewNativeRun(c, "normal", [3]int{2, 0, 1}, time.Time{}, func(int64) (int64, error) { return 0, nil })
	if err != nil {
		t.Fatal(err)
	}
	s := &legionSession{nativeRun: r, nativeOwnerID: w.role.ID, world: w}
	w.legion = s
	r.Ready(time.Now())
	return w
}

func nativeTestBattle(t *testing.T, w *worldSession, attempt string, clear bool) {
	t.Helper()
	r := w.legion.nativeRun
	now := time.Now()
	if r.State != legion.NativeReady {
		if err := r.Open(false, now); err != nil {
			t.Fatal(err)
		}
		if err := r.Select(r.Offers[1].Index, now); err != nil {
			t.Fatal(err)
		}
	}
	d := &dungeon.Session{Definition: catalog.DungeonDefinition{ID: r.Stage().Dungeon}, RunID: attempt, StartedAt: now.Add(-time.Second), Loaded: true, ArenaBoss: true, Dead: map[uint16]bool{}, Monsters: []protocol.DungeonMonster{{Entity: 4096, Template: r.Stage().Boss, Rank: 3, Team: 100}}}
	if err := r.Enter(r.Phase, d.Definition.ID, d.RunID); err != nil {
		t.Fatal(err)
	}
	w.activeDungeon = d
	w.completionSent = false
	w.legion.nativeSettlement = nativeLegionSettlement{}
	if clear {
		if _, err := d.ConfirmDeath(4096, w.role.WireID, w.role.WireID); err != nil || !d.Completed() {
			t.Fatal(err)
		}
	}
}

func nativeTestRewardEnd(w *worldSession) []byte {
	r := w.legion.nativeRun
	content, _, _, _ := nativeLegionIdentity(r.Source.Name)
	p := make([]byte, 22)
	binary.LittleEndian.PutUint32(p[13:], content)
	binary.LittleEndian.PutUint32(p[17:], uint32(r.Phase))
	p[21] = byte(r.Order[r.Phase])
	return p
}

func TestNativeSettlementThreePhasesSQLiteAndFinalReturn(t *testing.T) {
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		t.Run(name, func(t *testing.T) {
			w := nativeSettlementWorld(t, name)
			for phase := 0; phase < 3; phase++ {
				nativeTestBattle(t, w, fmt.Sprintf("stage-%d", phase), true)
				r := w.legion.nativeRun
				if _, err := w.legion.nativeRewardEnd(w, nativeTestRewardEnd(w), time.Now()); err == nil {
					t.Fatal("ACK before grant")
				}
				plan, err := w.completeDungeon()
				if err != nil {
					t.Fatal(err)
				}
				if !r.RewardCommitted || r.Phase != phase || r.State != legion.NativePhaseComplete {
					t.Fatal("progress before C72")
				}
				bag, err := inventory.ReadBag(w.role.State)
				if err != nil || len(bag.Items) != 2 || bag.Items[0].Amount != uint32(8*(phase+1)) || bag.Items[1].Amount != uint32(3*(phase+1)) {
					t.Fatal(bag, err)
				}
				if len(plan) != 5 || plan[0].ID != 13 || plan[1].ID != 31 || plan[2].ID != 2252 || plan[3].ID != 2253 {
					t.Fatal("native reward sequence", plan)
				}
				if plan[4].Payload[2] != 1 || binary.LittleEndian.Uint32(plan[4].Payload[11:]) != uint32(phase) {
					t.Fatal("lost selected offer/phase")
				}
				keys := make([]byte, wire.SessionKeyBytes)
				frames, err := preparePackets(keys, plan)
				if err != nil {
					t.Fatal(err)
				}
				for _, frame := range frames {
					want := map[uint16]int{2252: 7772, 2253: 2405}[frame.ID]
					if want == 0 {
						continue
					}
					plain, e := wire.DecryptPayload(keys, frame.ID, frame.Raw[wire.ServerHeaderSize:])
					if e != nil || len(frame.Payload) != want || len(plain) < want || !bytes.Equal(plain[:want], frame.Payload) {
						t.Fatal("compressed/truncated reward", e)
					}
				}
				before := string(w.role.State)
				if again, e := w.dungeonResult(nil); e != nil || len(again) != 0 || string(w.role.State) != before {
					t.Fatal("C46 paid twice", e)
				}
				bad := nativeTestRewardEnd(w)
				bad[21] = 255
				if _, e := w.legion.nativeRewardEnd(w, bad, time.Now()); e == nil {
					t.Fatal("foreign source stage accepted")
				}
				p := nativeTestRewardEnd(w)
				now := time.Now()
				result, err := w.legion.nativeRewardEnd(w, p, now)
				if err != nil || len(result.Packets) < 1 || len(result.Packets[0].Payload) != 14 {
					t.Fatal(result, err)
				}
				again, err := w.legion.nativeRewardEnd(w, p, now)
				if err != nil || len(again.Packets) != 1 || string(w.role.State) != before {
					t.Fatal("duplicate reward-end", err)
				}
				_, focus, err := w.exitNativeLegion([]byte{2, 2, 1}, now)
				if err != nil || len(focus) != 1 || r.Phase != phase || w.activeDungeon == nil {
					t.Fatal("focus advanced", err)
				}
				_, exit, err := w.exitNativeLegion([]byte{1, 2, 1}, now)
				if err != nil {
					t.Fatal(err)
				}
				if phase < 2 {
					if len(exit) < 5 || r.Phase != phase+1 || r.State != legion.NativeWaiting || r.Selected != -1 {
						t.Fatal("phase return failed", exit)
					}
					// dispatchDungeon clears the old scene after sending settlement_exit_ack.
					w.activeDungeon = nil
				} else {
					if len(exit) != 0 || len(w.legion.nativeSettlement.pendingExit) == 0 || r.State != legion.NativePhaseComplete {
						t.Fatal("movie did not retain early return")
					}
					end := w.legion.nativeSettlement.movieUntil
					if early, e := w.nativeLegionTick(end.Add(-time.Nanosecond)); e != nil || len(early) != 0 {
						t.Fatal("movie ended early")
					}
					done, e := w.nativeLegionTick(end)
					if e != nil || len(done) < 6 || r.State != legion.NativeComplete || w.activeDungeon != nil {
						t.Fatal("final movie/return stuck", done, e)
					}
					if again, e := w.nativeLegionTick(end.Add(time.Second)); e != nil || len(again) != 0 {
						t.Fatal("repeated final return")
					}
				}
				replay, handled, e := w.legion.handleNativeLegion(w, p, legion.CmdRewardEnd)
				if e != nil || !handled || len(replay.Packets) != 1 || string(w.role.State) != before {
					t.Fatal("late phase acknowledgement", e)
				}
			}
			if w.ownsNativeLegion() {
				t.Fatal("completed run still owns runtime")
			}
			w.activeDungeon = &dungeon.Session{RunID: "ordinary", Definition: catalog.DungeonDefinition{ID: 999}}
			if w.ownsNativeLegion() {
				t.Fatal("completed run captured ordinary dungeon")
			}
		})
	}
}

func TestNativeSettlementFullBagRetryAndQuota(t *testing.T) {
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		t.Run(name, func(t *testing.T) {
			w := nativeSettlementWorld(t, name)
			nativeTestBattle(t, w, "bag-full", true)
			w.loot.BagRules.Slots["[etc]"] = [2]uint16{68, 68}
			if p, e := w.completeDungeon(); e == nil || len(p) > 0 || w.completionSent || w.legion.nativeRun.RewardCommitted {
				t.Fatal("partial settlement published", e)
			}
			bag, _ := inventory.ReadBag(w.role.State)
			if len(bag.Items) != 0 {
				t.Fatal("partial grant committed")
			}
			if _, _, e := w.exitNativeLegion([]byte{1, 2, 1}, time.Now()); e == nil {
				t.Fatal("failed grant advanced")
			}
			w.loot.BagRules.Slots["[etc]"] = [2]uint16{68, 69}
			if _, e := w.dungeonResult(nil); e != nil {
				t.Fatal(e)
			}
			bag, _ = inventory.ReadBag(w.role.State)
			if len(bag.Items) != 2 || bag.Items[0].Amount != 8 {
				t.Fatal("retry did not claim frozen products", bag)
			}
			// A new attempt at the same source boss has no second weekly entitlement.
			r := w.legion.nativeRun
			r.State = legion.NativeReady
			r.RewardCommitted = false
			nativeTestBattle(t, w, "quota-exhausted", true)
			packets, e := w.completeDungeon()
			if e != nil || len(packets) != 5 || packets[3].Payload[0] != 0 || !r.RewardCommitted {
				t.Fatal("exhausted entitlement blocked progression", e)
			}
			after, _ := inventory.ReadBag(w.role.State)
			if after.Items[0].Amount != bag.Items[0].Amount || after.Items[1].Amount != bag.Items[1].Amount {
				t.Fatal("quota bypass")
			}
		})
	}
}

func TestNativeSettlementOwnerAndRetreat(t *testing.T) {
	w := nativeSettlementWorld(t, "DuskyIsland")
	nativeTestBattle(t, w, "retreat", false)
	r := w.legion.nativeRun
	if _, e := w.completeNativeLegionStage(); e == nil {
		t.Fatal("living boss paid")
	}
	if _, _, e := w.exitNativeLegion([]byte{1, 2, 0}, time.Now()); e != nil {
		t.Fatal(e)
	}
	if r.State != legion.NativeReady || r.Retries != 1 || r.Phase != 0 || r.Selected != 1 {
		t.Fatal("retreat lost selection/progress")
	}
	nativeTestBattle(t, w, "retry", true)
	w.legion.nativeOwnerID++
	if _, e := w.completeNativeLegionStage(); e == nil {
		t.Fatal("foreign owner paid")
	}
}

func TestNativePartyLeaveReleasesOwnership(t *testing.T) {
	w := nativeSettlementWorld(t, "DuskyIsland")
	s := w.legion
	if result, handled, e := s.nativeParty(w, nil, 13); e != nil || !handled || len(result.Packets) != 1 {
		t.Fatal(result, handled, e)
	}
	if w.boundNativeLegion() || s.nativeRun != nil || w.soloPartyReady {
		t.Fatal("party ownership retained")
	}
	if _, handled, e := s.nativeParty(w, nil, 13); handled || e != nil {
		t.Fatal("captured unrelated party leave", e)
	}
}

func TestNativeGlobalRewardUsesSourceDungeonOrdinal(t *testing.T) {
	w := nativeSettlementWorld(t, "Dimension Cloister")
	c := w.dungeons.LegionContents.Contents["Dimension Cloister"]
	c.NativeOperations.ClearRewards = []catalog.NativeLegionClearReward{{DungeonIndex: 2, Template: 21, Amount: 2, Weight: 1}}
	w.dungeons.LegionContents.Contents[c.Name] = c
	nativeTestBattle(t, w, "global-source-index", true)
	plan, e := w.completeDungeon()
	if e != nil {
		t.Fatal(e)
	}
	bag, _ := inventory.ReadBag(w.role.State)
	if bag.Items[0].Amount != 10 {
		t.Fatal("phase ordinal replaced source ordinal", bag)
	}
	if binary.LittleEndian.Uint32(plan[2].Payload[1644:]) != 21 || binary.LittleEndian.Uint32(plan[2].Payload[1648:]) != 2 {
		t.Fatal("extra reward not displayed")
	}
	before := string(w.role.State)
	if _, e = w.completeDungeon(); e != nil || string(w.role.State) != before {
		t.Fatal("global paid twice", e)
	}
}
