package main

import (
	"bytes"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"dfolan/internal/storage"
	"encoding/hex"
	"testing"

	"dfolan/internal/game/protocol"
)

// The live crash is the fixed-size native reader rejecting the compressed
// N2252 body. Exercise boss death -> actual settlement plan -> wire frame,
// so a future reintroduction of compression cannot pass by inflating in tests.
func TestIspinsSettlementNativeRewardDelivery(t *testing.T) {
	for stage := 0; stage < 4; stage++ {
		run := &dungeon.Session{
			ArenaBoss: true, Loaded: true, Dead: map[uint16]bool{},
			Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3, Team: 100}},
		}
		if changed, err := run.ConfirmDeath(4096, 7, 7); err != nil || !changed || !run.Completed() {
			t.Fatalf("stage %d boss death: changed=%v completed=%v err=%v", stage, changed, run.Completed(), err)
		}
		w := &worldSession{activeDungeon: run, ispins: &ispinsRun{stage: stage},
			role:       storage.Character{Name: "001", WireID: 7, State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)},
			characters: &character.Service{ChannelContext: [2]byte{3, 86}},
		}
		plan, err := w.completeIspinsStage()
		if err != nil {
			t.Fatal(err)
		}
		basicInfo, err := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
		if err != nil {
			t.Fatal(err)
		}
		detailInfo, err := w.characters.EntryAddition(w.role)
		if err != nil {
			t.Fatal(err)
		}
		seenInfo := map[string]bool{}
		for _, packet := range plan {
			want := map[string][]byte{"ispins_settlement_character_info": basicInfo, "ispins_settlement_character_detail": detailInfo}[packet.Name]
			if want != nil {
				seenInfo[packet.Name] = true
				if packet.ID != 2 || !bytes.Equal(packet.Payload, want) {
					t.Fatalf("stage %d %s must use local persisted native character data", stage, packet.Name)
				}
			}
		}
		if len(seenInfo) != 2 {
			t.Fatalf("stage %d missing native character refresh: %v", stage, seenInfo)
		}
		keys := make([]byte, wire.SessionKeyBytes)
		packets, err := preparePackets(keys, plan)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[uint16]bool{}
		for _, packet := range packets {
			want := map[uint16]int{2252: 7772, 2253: 2405}[packet.ID]
			if want == 0 {
				continue
			}
			seen[packet.ID] = true
			if len(packet.Payload) != want {
				t.Fatalf("stage %d NOTI%d body=%d, native reader requires %d", stage, packet.ID, len(packet.Payload), want)
			}
			plain, err := wire.DecryptPayload(keys, packet.ID, packet.Raw[wire.ServerHeaderSize:])
			if err != nil || len(plain) < want || !bytes.Equal(plain[:want], packet.Payload) {
				t.Fatalf("stage %d NOTI%d native body not delivered: len=%d err=%v", stage, packet.ID, len(plain), err)
			}
		}
		if !seen[2252] || !seen[2253] {
			t.Fatalf("stage %d missing reward packets: %v", stage, seen)
		}
	}
}

func TestIspinsChangeOperationResetsConfirmation(t *testing.T) {
	request := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 255, 255}
	request = append(request, make([]byte, 13)...)
	w := &worldSession{ispins: &ispinsRun{confirmed: true, cleared: [4]bool{true}}}
	plan, events, err := w.ispinsOperation(request)
	if err != nil || len(plan) != 1 || len(events) != 1 {
		t.Fatal(plan, events, err)
	}
	p := plan[0]
	if p.Kind != 1 || p.ID != 2047 || len(p.Payload) != 20 || !bytes.Equal(p.Payload[:8], []byte{1, 4, 0, 0, 0, 255, 255, 0}) {
		t.Fatalf("reset ACK must enter native action4/flag0 branch: %+v", p)
	}
	if w.ispins.confirmed || !w.ispins.cleared[0] || w.ispins.stage != 0 {
		t.Fatalf("reset changed progress or retained confirmation: %+v", w.ispins)
	}
	// A dungeon remains blocked until the new selection has been confirmed.
	enter := make([]byte, 24)
	enter[13] = 101
	if _, _, err := w.enterIspinsStage(enter); err == nil {
		t.Fatal("entry after operation reset must require reconfirmation")
	}
	// The regular select -> confirm cycle still works after a reset.
	request[13] = 1
	if _, _, err := w.ispinsOperation(request); err != nil || w.ispins.confirmed {
		t.Fatal("reselect", err)
	}
	request[13], request[17], request[18] = 2, 11, 0
	if _, _, err := w.ispinsOperation(request); err != nil || !w.ispins.confirmed {
		t.Fatal("reconfirm", err)
	}
}

// Live stage1 sends focus 020201 before the actual town exit 010201.
// Only the latter may carry main.go's dungeon-cleanup event name.
func TestIspinsSettlementFocusThenTownExit(t *testing.T) {
	for stage := 0; stage < 4; stage++ {
		run := &dungeon.Session{Loaded: true, ArenaBoss: true, Dead: map[uint16]bool{},
			Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3, Team: 100}}}
		if _, err := run.ConfirmDeath(4096, 7, 7); err != nil || !run.Completed() {
			t.Fatal(err)
		}
		w := &worldSession{activeDungeon: run, ispins: &ispinsRun{stage: stage},
			role: storage.Character{ID: 7, WireID: 7}, completionSent: true,
			state: storage.WorldState{Position: storage.WorldPosition{Town: 146, Area: 0, X: 700, Y: 300}}}
		w.ispins.cleared[stage] = true
		focus := make([]byte, 16)
		focus[0], focus[1], focus[2] = 2, 2, 1
		for n := 0; n < 2; n++ {
			pending, plan, err := w.settlementExit(focus)
			if err != nil || pending != nil || len(plan) != 1 {
				t.Fatalf("stage %d focus: pending=%v plan=%v err=%v", stage, pending, plan, err)
			}
			p := plan[0]
			if p.Name != "settlement_focus_ack" || p.Kind != 1 || p.ID != 72 || !bytes.Equal(p.Payload, []byte{1, 2, 2}) {
				t.Fatalf("stage %d focus must not signal exit cleanup: %+v", stage, p)
			}
			if w.activeDungeon != run || !w.completionSent || !w.ispins.cleared[stage] {
				t.Fatal("focus discarded the completed Ispins run")
			}
		}
		exit := append([]byte(nil), focus...)
		exit[0] = 1
		pending, plan, err := w.settlementExit(exit)
		if err != nil || pending != nil || len(plan) < 4 {
			t.Fatalf("stage %d actual exit after focus: plan=%v err=%v", stage, plan, err)
		}
		if plan[0].Name != "settlement_exit_ack" || !bytes.Equal(plan[0].Payload, []byte{1, 1, 2}) {
			t.Fatalf("stage %d actual exit ACK: %+v", stage, plan[0])
		}
		for i, want := range []uint16{72, 3, 23, 24} {
			if plan[i].ID != want {
				t.Fatalf("stage %d route[%d]=%d want%d", stage, i, plan[i].ID, want)
			}
		}
	}
}

func TestIspinsFinalMovieTownExitThenPartyLeave(t *testing.T) {
	run := &dungeon.Session{Loaded: true, ArenaBoss: true, Dead: map[uint16]bool{},
		Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3, Team: 100}}}
	if _, err := run.ConfirmDeath(4096, 7, 7); err != nil || !run.Completed() {
		t.Fatal(err)
	}
	w := &worldSession{activeDungeon: run, completionSent: true, channelType: 81, soloPartyReady: true,
		ispins:     &ispinsRun{stage: 3, cleared: [4]bool{true, true, true, true}},
		role:       storage.Character{ID: 7, WireID: 7, Name: "001", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)},
		characters: &character.Service{ChannelContext: [2]byte{3, 86}},
		state:      storage.WorldState{Position: storage.WorldPosition{Town: 146, Area: 0, X: 700, Y: 300}}}
	claim := make([]byte, 32)
	claim[13], claim[17], claim[21] = 101, 3, 1
	if _, _, err := w.ispinsRewardEnd(claim); err != nil || !w.ispins.finalDone {
		t.Fatal("final claim", err)
	}
	pause := make([]byte, 16)
	pause[1] = 1
	if _, _, err := w.ispinsStoryPause(pause); err != nil || w.ispins == nil || w.ispins.storyFinished {
		t.Fatal("movie start", err)
	}
	pause[0] = 1
	if _, _, err := w.ispinsStoryPause(pause); err != nil {
		t.Fatal(err)
	}
	if w.ispins == nil || !w.ispins.storyFinished || w.activeDungeon != run {
		t.Fatal("movie completion released settlement ownership before CMD72")
	}
	focus := make([]byte, 16)
	focus[0], focus[1], focus[2] = 2, 2, 1
	if _, plan, err := w.settlementExit(focus); err != nil || len(plan) != 1 || plan[0].Name != "settlement_focus_ack" || w.ispins == nil {
		t.Fatal("final focus", plan, err)
	}
	focus[0] = 1
	if _, plan, err := w.settlementExit(focus); err != nil || len(plan) < 4 || plan[0].Name != "settlement_exit_ack" || w.ispins != nil {
		t.Fatal("final town exit", plan, err)
	}
	// The existing post-send exit callback clears the active map. The party
	// remains owned until the separate native CMD13 is received in town.
	w.activeDungeon = nil
	if !w.soloPartyReady {
		t.Fatal("town exit silently abandoned the owned party")
	}
	handled, gone, err := w.ispinsStandbyPartyHandle(13, make([]byte, 8))
	if err != nil || !handled || len(gone) != 4 || gone[0].Kind != 0 || gone[0].ID != 9 {
		t.Fatal("leave party", handled, gone, err)
	}
	golden, _ := hex.DecodeString("01000f270100035600030100")
	if !bytes.Equal(gone[0].Payload, golden) || w.soloPartyReady || w.ispins != nil {
		t.Fatal("native party-gone notification or server ownership mismatch")
	}
	if w.ispinsRepeatPending {
		t.Fatal("quota restore was still pending after party leave delivered it")
	}
	// Leaving must not prevent a fresh party from being created.
	create, _ := hex.DecodeString(ispinsStandbyPartyRequestHex)
	if handled, plan, err := w.ispinsStandbyPartyHandle(12, create); err != nil || !handled || len(plan) != 3 || !w.soloPartyReady {
		t.Fatal("recreate party", handled, plan, err)
	}
	start := make([]byte, 24)
	start[13] = 101
	if _, _, err := w.startIspins(start); err != nil || w.ispins == nil || w.ispins.cleared != [4]bool{} || w.ispins.finalDone {
		t.Fatal("fresh replay start after a completed run", err)
	}
}

func TestIspinsPartyLeaveRequiresTownAndNativeEmptyRequest(t *testing.T) {
	w := &worldSession{channelType: 81, role: storage.Character{ID: 7, WireID: 7},
		characters: &character.Service{ChannelContext: [2]byte{3, 86}}, soloPartyReady: true,
		ispins: &ispinsRun{stage: 1}, activeDungeon: &dungeon.Session{}}
	for _, request := range [][]byte{make([]byte, 8), {1, 0, 0, 0, 0, 0, 0, 0}, make([]byte, 9)} {
		if handled, plan, err := w.ispinsStandbyPartyHandle(13, request); !handled || err == nil || len(plan) != 0 {
			t.Fatal("invalid/in-map leave accepted", handled, plan, err)
		}
		if !w.soloPartyReady || w.ispins == nil {
			t.Fatal("refused leave discarded ownership")
		}
	}
	w.activeDungeon = nil
	w.channelType = 73
	if handled, _, _ := w.ispinsStandbyPartyHandle(13, make([]byte, 8)); handled {
		t.Fatal("Ispins handler intercepted another channel's party leave")
	}
}

func TestIspinsRepeatQuotaRestoresOnlyAfterFullTownReturn(t *testing.T) {
	w := &worldSession{channelType: 81, role: storage.Character{ID: 7, WireID: 7},
		ispins:        &ispinsRun{stage: 3, finalDone: true, storyFinished: true, cleared: [4]bool{true, true, true, true}},
		activeDungeon: &dungeon.Session{}}
	if plan, err := w.ispinsRepeatRestorePackets(); err != nil || len(plan) != 0 {
		t.Fatal("quota refilled while still in the last dungeon", plan, err)
	}
	w.activeDungeon = nil
	w.ispins.storyFinished = false
	if plan, _ := w.ispinsRepeatRestorePackets(); len(plan) != 0 {
		t.Fatal("quota refilled before movie completion")
	}
	w.ispins.storyFinished = true
	w.ispins.cleared[3] = false
	if plan, _ := w.ispinsRepeatRestorePackets(); len(plan) != 0 {
		t.Fatal("partial clear/retreat refilled quota")
	}
	w.ispins.cleared[3] = true
	// ESC returns with the completed run retained; CMD72 returns with a
	// pending flag after it releases that run. Both use the same known body.
	for _, usePending := range []bool{false, true} {
		if usePending {
			w.ispins, w.ispinsRepeatPending = nil, true
		}
		plan, err := w.ispinsRepeatRestorePackets()
		if err != nil || len(plan) != 3 {
			t.Fatal("completed replay restore", plan, err)
		}
		fresh, err := legion.IspinsStandbyEntryCharacterInfo([5]byte{})
		if err != nil {
			t.Fatal(err)
		}
		if plan[0].ID != 2254 || !bytes.Equal(plan[0].Payload, fresh) ||
			plan[1].ID != 781 || !bytes.Equal(plan[1].Payload, weeklyDifficultyInfoUserStandby) ||
			plan[2].ID != 782 || !bytes.Equal(plan[2].Payload, weeklyDifficultyInfoCharacStandby) {
			t.Fatal("repeat quota differs from accepted fresh-standby packets")
		}
		if !bytes.Equal(fresh[:10], make([]byte, 10)) {
			t.Fatal("old clear/entry marks retained in restored quota")
		}
	}
	w.channelType = 73
	if plan, _ := w.ispinsRepeatRestorePackets(); len(plan) != 0 {
		t.Fatal("another channel's quota changed")
	}
	w.channelType = 81
	clearSelectedWorld(w)
	if w.ispinsRepeatPending {
		t.Fatal("pending quota restore survived character deselection")
	}
	if w.ispins != nil {
		t.Fatal("old character's completed replay state survived deselection")
	}
}

// TestIspinsAuxReplayTables guards the §26 settlement aux tables: the bodies
// are verbatim official s4 dumps with fixed shapes (2204=32B, 2201=32B,
// 279=16B, 2168=48B, 14=192B) and the stage0 packet mix matches the official
// chain (pre31=2204+2201+279, pre2252=4×2168+1×14, pre2253=3×2168+5×14,
// pre115=1×2168).
func TestIspinsAuxReplayTables(t *testing.T) {
	sizes := map[uint16]int{2204: 32, 2201: 32, 279: 16, 2168: 48, 14: 192}
	for stage := 0; stage < 4; stage++ {
		tables := [][]ispinsAuxPacket{
			ispinsAuxPre31[stage], ispinsAuxPre2252[stage], ispinsAuxPre2253[stage], ispinsAuxPre115[stage],
		}
		for _, aux := range tables {
			if len(aux) == 0 {
				t.Fatalf("stage%d aux table is empty", stage)
			}
			for _, a := range aux {
				if len(a.Body) != sizes[a.ID] {
					t.Errorf("stage%d %s id=%d len=%d want %d", stage, a.Name, a.ID, len(a.Body), sizes[a.ID])
				}
			}
		}
	}
	if len(ispinsAuxPre31[0]) != 3 {
		t.Errorf("stage0 pre31 count=%d want 3", len(ispinsAuxPre31[0]))
	}
	countMix := func(aux []ispinsAuxPacket) (n2168, n14 int) {
		for _, a := range aux {
			switch a.ID {
			case 2168:
				n2168++
			case 14:
				n14++
			}
		}
		return
	}
	if n2168, n14 := countMix(ispinsAuxPre2252[0]); n2168 != 4 || n14 != 1 {
		t.Errorf("stage0 pre2252 mix 2168×%d 14×%d want 2168×4 14×1", n2168, n14)
	}
	if n2168, n14 := countMix(ispinsAuxPre2253[0]); n2168 != 3 || n14 != 5 {
		t.Errorf("stage0 pre2253 mix 2168×%d 14×%d want 2168×3 14×5", n2168, n14)
	}
	if aux := ispinsAuxPre115[0]; len(aux) != 1 || aux[0].ID != 2168 {
		t.Errorf("stage0 pre115=%v want exactly one 2168", aux)
	}
}

// TestIspinsDeathTokenPerStage guards the per-stage NOTI38 nonce (next79 §26):
// body = `<u32 entity> <4B zero> <5B token> <3B zero>`, token replayed per
// stage (official s4 frames 468/565/647/749).
func TestIspinsDeathTokenPerStage(t *testing.T) {
	tokens := [4][]byte{
		{0x58, 0x49, 0xff, 0x0b, 0x3e},
		{0xb4, 0x4e, 0xb2, 0x02, 0x44},
		{0x12, 0x14, 0x19, 0x62, 0x42},
		{0xb1, 0x5b, 0xb2, 0xb1, 0x39},
	}
	for stage, want := range tokens {
		p := protocol.IspinsMonsterDeathConfirmed(0x00100000, stage)
		if len(p) != 16 {
			t.Fatalf("stage%d len=%d want 16", stage, len(p))
		}
		if !bytes.Equal(p[8:13], want) {
			t.Errorf("stage%d token=% x want % x", stage, p[8:13], want)
		}
		if !bytes.Equal(p[4:8], []byte{0, 0, 0, 0}) || !bytes.Equal(p[13:16], []byte{0, 0, 0}) {
			t.Errorf("stage%d padding=% x", stage, p[4:8])
		}
	}
	p := protocol.IspinsMonsterDeathConfirmed(0x00100000, 0)
	if !bytes.Equal(p[0:4], []byte{0x00, 0x00, 0x10, 0x00}) {
		t.Errorf("entity=% x want little-endian 0x00100000", p[0:4])
	}
}

// TestIspinsBossCheckStageGate guards the §26 NOTI115 stage gate: only
// stage0/stage3 carry the packet (official c2s frames 337/488), each with its
// own 5B nonce; stage1/2 return a nil body the caller must not send.
func TestIspinsBossCheckStageGate(t *testing.T) {
	for _, stage := range []int{1, 2} {
		p, err := protocol.IspinsBossCheckConfirmed(0x0b10, stage)
		if err != nil || p != nil {
			t.Errorf("stage%d = (%x, %v) want (nil, nil)", stage, p, err)
		}
	}
	p, err := protocol.IspinsBossCheckConfirmed(0x0b10, 0)
	if err != nil || len(p) != 16 {
		t.Fatalf("stage0 = (%d bytes, %v) want 16B", len(p), err)
	}
	if p[0] != 1 || p[1] != 1 || p[2] != 0x10 || p[3] != 0x0b {
		t.Errorf("stage0 head=% x want 01 01 10 0b", p[:4])
	}
	if !bytes.Equal(p[4:9], []byte{0xaa, 0x53, 0x06, 0x2f, 0x42}) {
		t.Errorf("stage0 token=% x want aa 53 06 2f 42", p[4:9])
	}
	if !bytes.Equal(p[9:16], []byte{0, 0, 0, 0, 0, 0, 0}) {
		t.Errorf("stage0 tail=% x want 7B zero", p[9:16])
	}
	p, err = protocol.IspinsBossCheckConfirmed(0x0b10, 3)
	if err != nil || len(p) != 16 {
		t.Fatalf("stage3 = (%d bytes, %v) want 16B", len(p), err)
	}
	if !bytes.Equal(p[4:9], []byte{0xed, 0x9b, 0x7c, 0xfd, 0x35}) {
		t.Errorf("stage3 token=% x want ed 9b 7c fd 35", p[4:9])
	}
}

// TestIspinsSettlementCharacterInfoRebuild guards the §27 chain-tail N2: the
// official s4 frame 488 template (480B, embedded "yan55" + actor 0xed) must be
// rebuilt with the local role name and WireID, all post-name fields shifting
// with the name length (the op=2 row reader is cursor-sequential), and the
// per-stage patch (template bytes 416..419: nonce + progress 4*stage+3) must
// land at the shifted offset.
// Historical template splicing only; this does not validate local compatibility.
// Production delivery is checked by TestIspinsSettlementNativeRewardDelivery.
func TestIspinsHistoricalSettlementCharacterInfoRebuild(t *testing.T) {
	patches := [4][]byte{
		{0x0f, 0x80, 0x03, 0x00}, // stage0 (frame 488)
		{0xbe, 0x9e, 0x07, 0x00}, // stage1 (frame 583)
		{0x6d, 0xbd, 0x0b, 0x00}, // stage2 (frame 666)
		{0x1c, 0xdc, 0x0f, 0x00}, // stage3 (frame 772)
	}
	for stage := 0; stage < 4; stage++ {
		p, err := protocol.IspinsSettlementCharacterInfo("001", 7, stage)
		if err != nil {
			t.Fatalf("stage%d: %v", stage, err)
		}
		// 480 - 2×(5-3) = 476: both name blocks shrink by the name delta.
		if len(p) != 476 {
			t.Fatalf("stage%d len=%d want 476", stage, len(p))
		}
		// Row head is verbatim: mode 0, count 1, context {3, 0x56}.
		if !bytes.Equal(p[:5], []byte{0, 1, 0, 3, 0x56}) {
			t.Errorf("stage%d head=% x want 00 01 00 03 56", stage, p[:5])
		}
		// First name block: len u32 at 28, name at 32.
		if !bytes.Equal(p[28:32], []byte{3, 0, 0, 0}) || string(p[32:35]) != "001" {
			t.Errorf("stage%d name1=% x", stage, p[28:35])
		}
		// Second block (shifted -2 by the 3-char name): local actor u16 at
		// 163, len at 165, name at 169.
		if !bytes.Equal(p[163:165], []byte{7, 0}) {
			t.Errorf("stage%d actor=% x want 07 00", stage, p[163:165])
		}
		if !bytes.Equal(p[165:169], []byte{3, 0, 0, 0}) || string(p[169:172]) != "001" {
			t.Errorf("stage%d name2=% x", stage, p[165:172])
		}
		// Post-name mini row (3-char name: profession@172, advancement@173,
		// level@174, pvp@175, settlement state@176): level 0x73=115 then
		// state 01 (the standby variant, official frame 507, carries 00).
		if p[174] != 0x73 || p[176] != 1 {
			t.Errorf("stage%d row=% x want level=73 state=01", stage, p[172:177])
		}
		// Stage patch shifted by 2×(5-3): template 416..419 -> local 412..415.
		if !bytes.Equal(p[412:416], patches[stage]) {
			t.Errorf("stage%d patch=% x want % x", stage, p[412:416], patches[stage])
		}
	}
	// A name of the official length (5) keeps the template layout unchanged.
	p, err := protocol.IspinsSettlementCharacterInfo("12345", 9, 1)
	if err != nil {
		t.Fatalf("5-char name: %v", err)
	}
	if len(p) != 480 {
		t.Fatalf("5-char name len=%d want 480", len(p))
	}
	if !bytes.Equal(p[416:420], patches[1]) {
		t.Errorf("5-char name patch offset=% x", p[416:420])
	}
	// Rejections: empty/oversized name, unset actor, out-of-range stage.
	if _, err := protocol.IspinsSettlementCharacterInfo("", 7, 0); err == nil {
		t.Error("empty name accepted")
	}
	if _, err := protocol.IspinsSettlementCharacterInfo("001", 0, 0); err == nil {
		t.Error("actor 0 accepted")
	}
	if _, err := protocol.IspinsSettlementCharacterInfo("001", 0xffff, 0); err == nil {
		t.Error("actor 0xffff accepted")
	}
	if _, err := protocol.IspinsSettlementCharacterInfo("001", 7, 4); err == nil {
		t.Error("stage 4 accepted")
	}
}
