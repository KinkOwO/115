package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestStoryDigestRestoreAlwaysFourBytes(t *testing.T) {
	// 缺键 / 空对象 / 全零都必须产出恰好 4 字节：preparePackets 会丢弃
	// 零长载荷帧，空 body 会让 NOTI1370 静默消失（影片照播且日志无痕）。
	cases := []struct {
		name string
		raw  json.RawMessage
		want []byte
	}{
		{"missing key", json.RawMessage(`{"cinematic_skipped":[1,2,3]}`), []byte{0, 0, 0, 0}},
		{"empty object", json.RawMessage(`{}`), []byte{0, 0, 0, 0}},
		{"level zero", json.RawMessage(`{"story_digest_level":0}`), []byte{0, 0, 0, 0}},
		{"level one", json.RawMessage(`{"story_digest_level":1}`), []byte{1, 0, 0, 0}},
		{"level 115", json.RawMessage(`{"story_digest_level":115}`), []byte{0x73, 0, 0, 0}},
		{"level 256", json.RawMessage(`{"story_digest_level":256}`), []byte{0, 1, 0, 0}},
		{"level 16777216", json.RawMessage(`{"story_digest_level":16777216}`), []byte{0, 0, 0, 1}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := storyDigestRestore(c.raw)
			if err != nil {
				t.Fatalf("storyDigestRestore(%s) error: %v", c.raw, err)
			}
			if len(got) != 4 {
				t.Fatalf("len(got)=%d, want 4 (payload must never be empty)", len(got))
			}
			if !bytes.Equal(got, c.want) {
				t.Fatalf("got %x, want %x", got, c.want)
			}
		})
	}
}

func TestStoryDigestRestoreInvalidState(t *testing.T) {
	if _, err := storyDigestRestore(json.RawMessage(`not json`)); err == nil {
		t.Fatal("expected error for unparseable state")
	}
}

func TestAdvanceStoryDigestMonotonic(t *testing.T) {
	t.Run("missing key advances from zero", func(t *testing.T) {
		state, advanced, err := advanceStoryDigest(json.RawMessage(`{"cinematic_skipped":[1]}`), 5)
		if err != nil {
			t.Fatal(err)
		}
		if !advanced {
			t.Fatal("want advanced=true when key is absent")
		}
		var fields map[string]json.RawMessage
		if e := json.Unmarshal(state, &fields); e != nil {
			t.Fatal(e)
		}
		var lv uint32
		if e := json.Unmarshal(fields["story_digest_level"], &lv); e != nil {
			t.Fatal(e)
		}
		if lv != 5 {
			t.Fatalf("level=%d, want 5", lv)
		}
		// 其它字段必须原样保留。
		if !bytes.Contains(state, []byte(`"cinematic_skipped"`)) {
			t.Fatalf("state lost unrelated fields: %s", state)
		}
	})

	t.Run("lower level does not regress", func(t *testing.T) {
		raw := json.RawMessage(`{"story_digest_level":100,"cinematic_skipped":[9]}`)
		if _, advanced, err := advanceStoryDigest(raw, 90); err != nil || advanced {
			t.Fatalf("advanced=%v err=%v, want (false,nil)", advanced, err)
		}
	})

	t.Run("equal level does not advance", func(t *testing.T) {
		raw := json.RawMessage(`{"story_digest_level":100}`)
		if _, advanced, err := advanceStoryDigest(raw, 100); err != nil || advanced {
			t.Fatalf("advanced=%v err=%v, want (false,nil)", advanced, err)
		}
	})

	t.Run("higher level advances", func(t *testing.T) {
		raw := json.RawMessage(`{"story_digest_level":100}`)
		state, advanced, err := advanceStoryDigest(raw, 115)
		if err != nil {
			t.Fatal(err)
		}
		if !advanced {
			t.Fatal("want advanced=true")
		}
		var fields map[string]json.RawMessage
		if e := json.Unmarshal(state, &fields); e != nil {
			t.Fatal(e)
		}
		var lv uint32
		if e := json.Unmarshal(fields["story_digest_level"], &lv); e != nil {
			t.Fatal(e)
		}
		if lv != 115 {
			t.Fatalf("level=%d, want 115", lv)
		}
	})

	t.Run("invalid state errors", func(t *testing.T) {
		if _, _, err := advanceStoryDigest(json.RawMessage(`nope`), 1); err == nil {
			t.Fatal("expected error for unparseable state")
		}
	})
}

func TestStoryDigestRestoreAndEntryOrder(t *testing.T) {
	for _, tc := range []struct {
		state string
		want  []byte
	}{
		{`{"cinematic_skipped":[]}`, []byte{0, 0, 0, 0}},
		{`{"story_digest_level":115}`, []byte{115, 0, 0, 0}},
		{`{"story_digest_level":16777216}`, []byte{0, 0, 0, 1}},
	} {
		body, err := storyDigestRestore([]byte(tc.state))
		if err != nil || !bytes.Equal(body, tc.want) {
			t.Fatalf("restore %s: body=%x err=%v", tc.state, body, err)
		}
	}
	// 断言相对顺序而不是绝对下标：进场帧组会在 1352 之前插入 NOTI402 /
	// NOTI426（通知已读集），硬编码下标一旦帧组增减就失效——这里要保证的
	// 是 1352 → 1370 → 2 的先后关系，与中间插了多少帧无关。
	packets := (entryPayloads{CinematicSkips: []byte{0}, StoryDigest: []byte{115, 0, 0, 0}, Basic: []byte{1}}).packets()
	at := map[uint16]int{}
	kinds := map[uint16]byte{}
	for i, p := range packets {
		if _, seen := at[p.ID]; !seen {
			at[p.ID] = i
			kinds[p.ID] = p.Kind
		}
	}
	for _, id := range []uint16{1352, 1370, 2} {
		if _, ok := at[id]; !ok {
			t.Fatalf("entry payloads missing frame %d: %+v", id, packets)
		}
		if kinds[id] != 0 {
			t.Fatalf("frame %d kind = %d, want 0", id, kinds[id])
		}
	}
	if !(at[1352] < at[1370] && at[1370] < at[2]) {
		t.Fatalf("wrong entry sequence: 1352@%d 1370@%d 2@%d", at[1352], at[1370], at[2])
	}
	if !observedGameRequest(1438) {
		t.Fatal("story digest update is not retained")
	}
}

// NOTI708 now rides the LOGIN flood (TestLoginEventFloodOrder); this guard
// keeps the frame from creeping back into the post-selection town announce,
// where the client could not absorb it out of stage (next79 §13).
func TestWeeklyDungeonConfigLeftTheAnnounce(t *testing.T) {
	packets := (entryPayloads{CinematicSkips: []byte{0}, StoryDigest: []byte{115, 0, 0, 0}, Basic: []byte{1}}).packets()
	for _, p := range packets {
		if p.ID == 708 {
			t.Fatal("entry announce still carries NOTI708; it belongs to the login flood")
		}
	}
}

// The weekly-dungeon ledger split across stages in the official 21:18
// capture: NOTI1336 rides the LOGIN flood (pre-selection), while NOTI706 /
// NOTI537 ride the post-selection announce in that order. The NOTI108 that
// official order places between them is now the fixed V3 table
// (event_info_generated.go, next79 §15/§16): one count=1 raw table whose
// single Ispins record is the only 108 body the private-server client has
// ever accepted; TestEventInfoTableRidesAnnounce pins the frame's body and
// position.
func TestWeeklyDungeonLedgerFollowsConfig(t *testing.T) {
	packets := (entryPayloads{CinematicSkips: []byte{0}, StoryDigest: []byte{115, 0, 0, 0}, Basic: []byte{1}}).packets()
	bodies := map[uint16][]byte{}
	at := map[uint16]int{}
	for i, p := range packets {
		if _, seen := at[p.ID]; !seen {
			at[p.ID] = i
			bodies[p.ID] = p.Payload
		}
	}
	for _, id := range []uint16{706, 108, 537} {
		if _, ok := at[id]; !ok {
			t.Fatalf("entry payloads missing frame %d", id)
		}
	}
	// Official post-selection order: 706 -> 108 -> 537 (then the entry basics).
	if !(at[1370] < at[706] && at[706] < at[108] && at[108] < at[537] && at[537] < at[2]) {
		t.Fatalf("wrong ledger sequence: 1370@%d 706@%d 108@%d 537@%d 2@%d",
			at[1370], at[706], at[108], at[537], at[2])
	}
	// The login-flood frames must not creep back into the announce; 708 has
	// its own guard above, 1198/1336 are checked here.
	for _, id := range []uint16{1198, 1336} {
		if _, ok := at[id]; ok {
			t.Fatalf("entry announce still carries frame %d; it belongs to the login flood", id)
		}
	}
	if len(bodies[706]) != 1464 {
		t.Fatalf("NOTI706 body len=%d, want 1464 (official s1 frame 78)", len(bodies[706]))
	}
	if !bytes.HasPrefix(bodies[706], []byte{0xff, 0xff, 0xff, 0xff, 0x01, 0x01, 0x05}) {
		t.Fatalf("NOTI706 body drifted from the official capture: %x", bodies[706][:8])
	}
	want537 := []byte{0xc4, 0x0d, 0x00, 0x00, 0x05, 0x00, 0xd5, 0xcd,
		0x28, 0xca, 0x3b, 0x00, 0x00, 0x00, 0x00, 0x00}
	if !bytes.Equal(bodies[537], want537) {
		t.Fatalf("NOTI537 body drifted from the official capture: %x", bodies[537])
	}
}

// The 708/1198/1336 burst is LOGIN-stage traffic, not town-announce
// traffic: every business connection of the official 21:18 capture repeats
// 1759 -> 708 -> 1198 -> 108 -> 1336 before the SELECT_CHARACTER request.
// Round 10 replayed that verbatim - 108 included - and the client STILL
// froze at the selection screen, so the 108 left the flood for good
// (next79 §15): the flood must now be exactly 708 -> 1792 -> 1198 -> 1336
// (1792 joined in §20), and any NOTI108 anywhere in it voids the round (the
// announce probe is the only 108 this server may send).
func TestLoginEventFloodOrder(t *testing.T) {
	flood := loginFloodPackets()
	// next79 §20: 1792 rides the flood between 708 and 1198 (official order
	// switch f11 708 -> f12 1792 -> f18 1198), so the legion character-select
	// gate has quest-clear-group state before the selection is evaluated.
	wantOrder := []uint16{708, 1792, 1198, 1336}
	bodies := map[uint16][]byte{}
	for i, want := range wantOrder {
		if flood[i].ID != want {
			t.Fatalf("login flood position %d is frame %d, want %d", i, flood[i].ID, want)
		}
		if flood[i].Kind != 0 {
			t.Fatalf("login flood frame %d has kind %d, want NOTI(0)", flood[i].ID, flood[i].Kind)
		}
		bodies[flood[i].ID] = flood[i].Payload
	}
	if len(flood) != len(wantOrder) {
		t.Fatalf("login flood has %d frames, want %d", len(flood), len(wantOrder))
	}
	// NOTI1792 golden body (s4 frame 9 / switch frame 12): 96 bytes,
	// count=17 clear-group list, byte-identical across both sessions.
	want1792 := mustHexDecode(
		"1100000035000000013600000001370000000138000000013a" +
			"000000013b000000013c000000013d000000013e000000013f" +
			"000000014000000001450000000146000000014b000000014c" +
			"000000014d000000014e000000016289eda43c0000")
	if !bytes.Equal(bodies[1792], want1792) {
		t.Fatalf("NOTI1792 body drifted from the official capture: %x", bodies[1792])
	}
	// NOTI708 golden body (official s1 frame 8).
	want708 := []byte{0x32, 0x33, 0x36, 0x3f, 0x3e, 0x3b, 0x3a, 0x39, 0xff, 0xff,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x51, 0xfc, 0x40, 0xed, 0x35, 0x00}
	if !bytes.Equal(bodies[708], want708) {
		t.Fatalf("NOTI708 body drifted from the official capture: %x", bodies[708])
	}
	// NOTI1198 golden body (s1/s4 frame 15): 0x0942, flag 1 at +4, 2 at +25,
	// capture timestamp 0x3f1095d69f at +0xA5, rest zero.
	want1198 := make([]byte, 176)
	want1198[0], want1198[1], want1198[4] = 0x42, 0x09, 0x01
	want1198[25] = 0x02
	copy(want1198[165:], []byte{0x9f, 0xd6, 0x95, 0x10, 0x3f})
	if !bytes.Equal(bodies[1198], want1198) {
		t.Fatalf("NOTI1198 body drifted from the official capture: %x", bodies[1198])
	}
	// NOTI1336 golden body (s1 frame 60).
	want1336 := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xf4, 0xf7, 0x69, 0xb2, 0x3a, 0x00, 0x00, 0x00}
	if !bytes.Equal(bodies[1336], want1336) {
		t.Fatalf("NOTI1336 body drifted from the official capture: %x", bodies[1336])
	}
	// NOTI108 must be gone from the flood entirely (next79 §15): the only
	// 108 this server sends is the announce's fixed V3 table, so a CMD217
	// seen during login maps to a non-108 cause and voids the round.
	for i, p := range flood {
		if p.ID == 108 {
			t.Fatalf("login flood position %d carries NOTI108; the announce table is the only 108 allowed", i)
		}
	}
}

// The permanent fix after the round-11 differential experiment (next79
// §15/§16): the announce must carry exactly one NOTI108 whose body is the
// fixed V3 table - the count=1 raw table with the single Ispins Legion
// record - still positioned between 706 and 537 like the official order.
// Every other 108 body the experiment tried (zlib, or a 7-record raw table)
// froze the client with CMD217.
func TestEventInfoTableRidesAnnounce(t *testing.T) {
	packets := (entryPayloads{CinematicSkips: []byte{0}, StoryDigest: []byte{115, 0, 0, 0}, Basic: []byte{1}}).packets()
	at := map[uint16]int{}
	copies := 0
	var got []byte
	for j, p := range packets {
		if _, seen := at[p.ID]; !seen {
			at[p.ID] = j
		}
		if p.ID == 108 && len(p.Payload) > 0 {
			copies++
			got = p.Payload
		}
	}
	if copies != 1 {
		t.Fatalf("announce carries %d non-empty NOTI108 frames, want exactly 1", copies)
	}
	if !bytes.Equal(got, eventInfoTable) {
		t.Fatalf("announce 108 body drifted from the fixed table (%d vs %d bytes)", len(got), len(eventInfoTable))
	}
	// Official post-selection position: 706 -> 108 -> 537.
	if !(at[706] < at[108] && at[108] < at[537]) {
		t.Fatalf("table frame out of order: 706@%d 108@%d 537@%d", at[706], at[108], at[537])
	}
}

// Fixed-table body sanity（001 全频道方案）：19 条官服门记录 + 计数 + 空表尾，
// RAW 裸格式（zlib 体被私服客户端拒收 → CMD217 冻结，V2 判定）；必须含全部
// 关键门名（含 Venus Open / 巴卡尔 / 攻坚战各团本），且不带 URL/xui 横幅负载
// （选角界面渲染崩溃）。
func TestEventInfoTableBody(t *testing.T) {
	const wantLen = 1141 // 2-byte count + 19 gate records + 1-byte tail
	if len(eventInfoTable) != wantLen {
		t.Fatalf("fixed table len=%d, want %d (count + 19 gate records + tail)", len(eventInfoTable), wantLen)
	}
	if eventInfoTable[0] != 19 || eventInfoTable[1] != 0 {
		t.Fatalf("record count=%d, want 19", binary.LittleEndian.Uint16(eventInfoTable[:2]))
	}
	if eventInfoTable[len(eventInfoTable)-1] != 0 {
		t.Fatal("table must end with the empty 0x00 schedule tail")
	}
	for _, name := range []string{
		"Ispins Legion Open", "Apocalypse Channel",
		"DefaultEvent(ENTER_BAKAL_RAID)", "DefaultEvent(ENTER_ASRAHAN_RAID)",
		"DefaultEvent(ENTER_ARTIFICIAL_GOD_RAID)", "DefaultEvent(ENTER_DELEZIE_RAID)",
		"DefaultEvent(ENTER_INAE_DUSK_WAR)", "Venus Open", "Dusky Island Open",
	} {
		if !bytes.Contains(eventInfoTable, []byte(name)) {
			t.Fatalf("fixed table missing the %q gate record", name)
		}
	}
	if bytes.Contains(eventInfoTable, []byte("http")) || bytes.Contains(eventInfoTable, []byte(".xui")) {
		t.Fatal("gate table must stay payload-free; banner records crash the select screen")
	}
	if bytes.HasPrefix(eventInfoTable, []byte{0x78, 0x9c}) {
		t.Fatal("fixed table must stay raw; zlib bodies freeze the private client (V2 verdict)")
	}
}
