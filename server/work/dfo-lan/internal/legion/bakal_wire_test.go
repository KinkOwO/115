// Bakal wire contract tests: every constructor is asserted byte-exact against
// the handover-binary capture (testdata/bakal_wire_fixtures.json, full clear
// run 2026-10-05 12:29:29→12:39:42) and every decoder against the captured
// c2s bodies. A failure here means the layout moved — re-derive from the
// fixtures, never patch the fixture.
package legion

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

type bakalFixture struct {
	Name     string `json:"name"`
	Dir      string `json:"dir"`
	ID       uint16 `json:"id"`
	Bytes    int    `json:"bytes"`
	Time     string `json:"time"`
	PlainHex string `json:"plain_hex"`
}

func bakalFixtures(t *testing.T) []bakalFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/bakal_wire_fixtures.json")
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var doc struct {
		Frames []bakalFixture `json:"frames"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}
	return doc.Frames
}

func bakalFixtureByName(t *testing.T, name string, occurrence int) []byte {
	t.Helper()
	seen := 0
	for _, f := range bakalFixtures(t) {
		if f.Name != name {
			continue
		}
		seen++
		if seen == occurrence {
			got, err := hex.DecodeString(f.PlainHex)
			if err != nil {
				t.Fatalf("fixture %s: bad hex: %v", name, err)
			}
			return got
		}
	}
	t.Fatalf("fixture %s occurrence %d not found", name, occurrence)
	return nil
}

func bakalAssertBytes(t *testing.T, name string, want, got []byte) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("%s: built %d bytes, fixture %d", name, len(got), len(want))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("%s: byte %d = %#02x, fixture %#02x\nbuilt:   % x\nfixture: % x",
				name, i, got[i], want[i], got, want)
		}
	}
}

func TestBakalVoteFrames(t *testing.T) {
	bakalAssertBytes(t, "vote_start", bakalFixtureByName(t, "bakal_vote_start", 1), BakalVoteStartFrame())
	bakalAssertBytes(t, "vote_state", bakalFixtureByName(t, "bakal_vote_state", 1), BakalVoteStateFrame())
}

func TestBakalMemberAssignedFrame(t *testing.T) {
	bakalAssertBytes(t, "member_assigned",
		bakalFixtureByName(t, "bakal_real_member_assigned", 1),
		BakalMemberAssignedFrame("Lansmt"))
}

func TestBakalSubpartyCreatedFrame(t *testing.T) {
	bakalAssertBytes(t, "subparty_created",
		bakalFixtureByName(t, "bakal_subparty_created", 1),
		BakalSubpartyCreatedFrame("Party : 1", 4, 5))
}

func TestBakalPartyFrames(t *testing.T) {
	empty := BakalPartyFrame(52, nil)
	bakalAssertBytes(t, "party(open)", bakalFixtureByName(t, "bakal_party", 1), empty)

	// room_party_location 12:29:54 — location 22, all slots unused.
	bakalAssertBytes(t, "room_party_location@22",
		bakalFixtureByName(t, "bakal_room_party_location", 1),
		BakalPartyFrame(22, nil))

	// room_party_location 12:30:44 — location 2, three visited slots.
	visited := []BakalPartyLocation{
		{1, 0x6ac39a54}, {2, 0x6ac39a45}, {3, 0x6ac399dc},
	}
	bakalAssertBytes(t, "room_party_location@2",
		bakalFixtureByName(t, "bakal_room_party_location", 2),
		BakalPartyFrame(2, visited))

	// room_party_location 12:31:44 — location 36, five visited slots.
	visited = []BakalPartyLocation{
		{1, 0x6ac39a90}, {2, 0x6ac39a45}, {3, 0x6ac399dc}, {5, 0x6ac39a18},
	}
	bakalAssertBytes(t, "room_party_location@36",
		bakalFixtureByName(t, "bakal_room_party_location", 3),
		BakalPartyFrame(36, visited))

	// script_return_to_camp — location 52, camp slots 0/1/8.
	camp := []BakalPartyLocation{
		{0, 0x6ac39b57}, {1, 0x6ac39aa4}, {8, 0x6ac39adf},
	}
	bakalAssertBytes(t, "return_to_camp",
		bakalFixtureByName(t, "bakal_script_return_to_camp", 1),
		BakalPartyFrame(52, camp))
}

// bakalMonsterRows decodes a captured N2286 body into constructor rows.
func bakalMonsterRows(t *testing.T, body []byte) []BakalMonster {
	t.Helper()
	if len(body) == 0 || len(body) != 1+int(body[0])*19 {
		t.Fatalf("monsters body length %d does not match row count %d", len(body), body[0])
	}
	rows := make([]BakalMonster, body[0])
	for i := range rows {
		at := 1 + i*19
		rows[i] = BakalMonster{
			Slot:     le32(body[at:]),
			Location: le32(body[at+4:]),
			Count:    le32(body[at+8:]),
			MaxHP:    le32(body[at+12:]),
		}
	}
	return rows
}

func le32(b []byte) uint32 {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
}

func TestBakalMonstersFrames(t *testing.T) {
	for name, occurrence := range map[string]int{
		"bakal_monsters": 1,
	} {
		body := bakalFixtureByName(t, name, occurrence)
		bakalAssertBytes(t, name, body, BakalMonstersFrame(bakalMonsterRows(t, body)))
	}
	for i := 1; i <= 5; i++ {
		body := bakalFixtureByName(t, "bakal_script_monsters", i)
		bakalAssertBytes(t, "script_monsters", body, BakalMonstersFrame(bakalMonsterRows(t, body)))
	}
	for i := 1; i <= 2; i++ {
		body := bakalFixtureByName(t, "bakal_script_actor_health", i)
		bakalAssertBytes(t, "script_actor_health", body, BakalMonstersFrame(bakalMonsterRows(t, body)))
	}
}

func TestBakalBuffInventoryFrames(t *testing.T) {
	bakalAssertBytes(t, "buffs(open)",
		bakalFixtureByName(t, "bakal_buffs", 1),
		BakalBuffInventoryFrame(BakalOpenBuffCounts(bakalTestRules().NormalPhase), BakalBuffTailOpen))
	for i := 1; i <= 2; i++ {
		body := bakalFixtureByName(t, "bakal_script_buff_inventory", i)
		counts := [5]byte{body[0], body[1], body[2], body[3], body[4]}
		var tail [7]byte
		copy(tail[:], body[6:13])
		bakalAssertBytes(t, "script_buff_inventory", body,
			BakalBuffInventoryFrame(counts, tail))
	}
}

func TestBakalDungeonFrames(t *testing.T) {
	bakalAssertBytes(t, "open_dungeons",
		bakalFixtureByName(t, "bakal_open_dungeons", 1),
		BakalOpenDungeonsFrame(bakalTestDungeonIDs()))
	for i := 1; i <= 3; i++ {
		body := bakalFixtureByName(t, "bakal_script_dungeon_states", i)
		bakalAssertBytes(t, "dungeon_states", body,
			BakalDungeonStateFrame(le32(body[2:6])))
	}
	bakalAssertBytes(t, "remaining",
		bakalFixtureByName(t, "bakal_remaining", 1),
		BakalRemainingFrame(9999))
}

func TestBakalBossAndFinalFrames(t *testing.T) {
	bakalAssertBytes(t, "portal_ready",
		bakalFixtureByName(t, "bakal_portal_ready", 1), BakalPortalReadyFrame())
	bakalAssertBytes(t, "final_move",
		bakalFixtureByName(t, "bakal_final_move", 1), BakalPortalReadyFrame())
	bakalAssertBytes(t, "final_selection",
		bakalFixtureByName(t, "bakal_final_selection", 1), BakalFinalSelectionFrame())
	bakalAssertBytes(t, "clear_result",
		bakalFixtureByName(t, "bakal_clear_result", 1), BakalClearResultFrame())

	// source_boss instances (slot, kind, room, run, hp pair).
	for i, want := range []struct {
		slot, kind byte
		room, run  uint32
		hpCur      uint32
		hpMax      uint32
	}{
		{3, 0, 0x105c, 0x067f6dcf, 650, 250},
		{4, 0, 0x103c, 0x067f6dd1, 650, 250},
		{1, 5, 0x1001, 0x067f6dd3, 945, 311},
	} {
		body := bakalFixtureByName(t, "bakal_source_boss", i+1)
		bakalAssertBytes(t, "source_boss", body,
			BakalSourceBossFrame(want.slot, want.kind, want.room, want.run, want.hpCur, want.hpMax))
	}
}

func TestBakalRaidStateAndAcks(t *testing.T) {
	bakalAssertBytes(t, "preparing",
		bakalFixtureByName(t, "bakal_preparing", 1), []byte(BakalRaidStatePreparing))
	bakalAssertBytes(t, "active",
		bakalFixtureByName(t, "bakal_active", 1), []byte(BakalRaidStateActive))
	bakalAssertBytes(t, "phase_ended",
		bakalFixtureByName(t, "bakal_script_phase_ended", 1), []byte(BakalRaidStateEnded))
	bakalAssertBytes(t, "start_ack",
		bakalFixtureByName(t, "bakal_start_ack", 1), BakalAck())
	bakalAssertBytes(t, "loading_ack",
		bakalFixtureByName(t, "bakal_loading_ack", 1), BakalAck())
	bakalAssertBytes(t, "script_warp_ack",
		bakalFixtureByName(t, "bakal_script_warp_ack", 1), BakalAck())
	bakalAssertBytes(t, "countdown",
		bakalFixtureByName(t, "bakal_countdown", 1), nil)
}

func TestBakalSourceSymbolFrame(t *testing.T) {
	// Captured N570 shapes: 01 + symbol + u32 counter + u16 counter + 0.
	for _, want := range [][]byte{
		{1, 0x66, 0, 0, 0, 0, 0, 0, 0},
		{1, 0x6b, 0, 0, 0, 0, 1, 0, 0},
		{1, 0xc9, 0, 0, 0, 0, 0x10, 0x27, 0},
	} {
		got := BakalSourceSymbolFrame(le32(want[1:]), int32(le32(want[5:])))
		bakalAssertBytes(t, "source_symbol", want, got)
	}
}

func TestBakalNativeSymbolValue(t *testing.T) {
	want, _ := hex.DecodeString("01c900000010270000")
	bakalAssertBytes(t, "source BAKAL HP", want, BakalSourceSymbolFrame(201, 10000))
}

func TestBakalDecoders(t *testing.T) {
	// c2s 656 — name at offset 5.
	req, err := DecodeBakalCreateRaid(bakalFixtureByName(t, "client_656", 1))
	if err != nil || req.Name != "Lansmt" {
		t.Fatalf("create raid decode = %+v, %v", req, err)
	}
	// c2s 2089 — marker validated, counter ignored.
	if err := DecodeBakalStartRaid(bakalFixtureByName(t, "client_2089", 1)); err != nil {
		t.Fatalf("start raid decode: %v", err)
	}
	// c2s 12 — party name, capacity 4, type 5.
	sub, err := DecodeBakalSubparty(bakalFixtureByName(t, "client_12", 1))
	if err != nil || sub.Name != "Party : 1" || sub.Capacity != 4 || sub.PartyType != 5 {
		t.Fatalf("subparty decode = %+v, %v", sub, err)
	}
	// c2s 2069 — dungeon @17, monster @21.
	for occurrence, want := range map[int]BakalBattleReport{
		1: {Dungeon: 0x05F5ED58, Map: 0x05F5FE49, DamageTotal: 0},
		2: {Dungeon: 0x05F5ED58, Map: 0x05F5FE4C, DamageTotal: 0x03f6af6de & 0xffffffff},
	} {
		report, err := DecodeBakalBattleReport(bakalFixtureByName(t, "client_2069", occurrence))
		if err != nil {
			t.Fatalf("battle report %d: %v", occurrence, err)
		}
		if report.Dungeon != want.Dungeon || report.Map != want.Map {
			t.Fatalf("battle report %d = %+v, want %+v", occurrence, report, want)
		}
	}
	// c2s 2073 — empty body.
	if err := DecodeBakalLoadingDone(bakalFixtureByName(t, "client_2073", 1)); err != nil {
		t.Fatalf("loading done: %v", err)
	}
	// c2s 2070 — length gate only.
	if err := DecodeBakalScriptWarp(bakalFixtureByName(t, "client_2070", 1)); err != nil {
		t.Fatalf("script warp: %v", err)
	}
	// c2s 1134 — final map @4.
	final, err := DecodeBakalFinalConfirm(bakalFixtureByName(t, "client_1134", 1))
	if err != nil || final.FinalMap != BakalFinalMap {
		t.Fatalf("final confirm = %+v, %v", final, err)
	}
	// c2s 13 — 8 zero bytes.
	if err := DecodeBakalRewardClaim(bakalFixtureByName(t, "client_13", 1)); err != nil {
		t.Fatalf("reward claim: %v", err)
	}
}

func TestBakalCampReturnDecoder(t *testing.T) {
	// From testdata/bakal_refusals.json: 32B, option byte @13 (04 and 03).
	for _, hexBody := range []string{
		"0000000000000000ffffffff0004000000000000000000000000000000000000",
		"0000000000000000ffffffff0003000000000000000000000000000000000000",
	} {
		body, err := hex.DecodeString(hexBody)
		if err != nil {
			t.Fatalf("camp return hex: %v", err)
		}
		req, err := DecodeBakalCampReturn(body)
		if err != nil {
			t.Fatalf("camp return decode: %v", err)
		}
		if req.Option != 3 && req.Option != 4 {
			t.Fatalf("camp return option = %d", req.Option)
		}
	}
}

func TestBakalDungeonRange(t *testing.T) {
	o, err := PrepareBakalOpening(bakalTestRules(), false)
	if err != nil {
		t.Fatal(err)
	}
	if !o.HasDungeon(0x05F5ED4D) || !o.HasDungeon(0x05F5ED5D) {
		t.Fatal("17-map range must cover 0x05F5ED4D..0x05F5ED5D")
	}
	if o.HasDungeon(0x05F5ED4C) || o.HasDungeon(0x05F5ED5E) || o.HasDungeon(BakalFinalMap) {
		t.Fatal("range must not cover ids outside the 17 maps or the final map")
	}
}
