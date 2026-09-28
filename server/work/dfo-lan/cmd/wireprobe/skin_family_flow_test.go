package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"reflect"
	"testing"
)

// TestSkinFamilyEntryOrder pins where the two list families sit in the entry group.
// A NOTI1545 page frame rebuilds the page it names and the profile-decoration feature
// pushes page 0 too, so the family pages have to arrive after that pair — they carry
// its built-in rows, and the last frame for a page is the state the client keeps.
func TestSkinFamilyEntryOrder(t *testing.T) {
	p := entryPayloads{
		ProfileSkinCargo:           []byte{1},
		ProfileSkinSelection:       []byte{2},
		SkinCargoPartyFrame:        []byte{3},
		SkinSelectionPartyFrame:    []byte{4},
		SkinCargoSkillCutscene:     []byte{5},
		SkinSelectionSkillCutscene: []byte{6},
	}
	index := map[string]int{}
	for i, packet := range p.packets() {
		index[packet.Name] = i
	}
	profile := []string{"profile_skin_cargo_restored", "profile_skin_selection_restored"}
	ordered := append([]string{}, profile...)
	ordered = append(ordered, "skin_cargo_party_frame_restored", "skin_selection_party_frame_restored",
		"skin_cargo_skill_cutscene_restored", "skin_selection_skill_cutscene_restored")
	for i, name := range ordered {
		at, ok := index[name]
		if !ok {
			t.Fatalf("%s is not emitted at entry", name)
		}
		if i > 0 && at <= index[ordered[i-1]] {
			t.Fatalf("%s = %d follows %s = %d", name, at, ordered[i-1], index[ordered[i-1]])
		}
	}
	world, ok := index["enter_gameworld_complete_sent"]
	if !ok || world < index[ordered[len(ordered)-1]] {
		t.Fatalf("family pages must arrive before the town refresh, got %d", world)
	}
}

// TestSkinFamilyPageIDsKeepsBuiltins checks the page a 边框 push carries: the client's
// own default rows survive, an owned skin that duplicates a default is not listed
// twice, and the raid list default is included because the panel renders it out of
// this same page.
func TestSkinFamilyPageIDsKeepsBuiltins(t *testing.T) {
	ids := skinFamilyPageIDs(catalog.SkinFamilyPartyFrame, []uint32{20001, 20000, 80001})
	want := []uint32{20000, 50000, 60000, 80000, 20001, 80001}
	if len(ids) != len(want) {
		t.Fatalf("page ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("page ids = %v, want %v", ids, want)
		}
	}
	cutscene := skinFamilyPageIDs(catalog.SkinFamilySkillCutscene, []uint32{30001})
	if len(cutscene) != 3 || cutscene[0] != 30000 || cutscene[1] != 100000 || cutscene[2] != 30001 {
		t.Fatalf("cutscene page = %v", cutscene)
	}
}

// TestSkinFamilySelectionPayloadPartitionsRaidList pins the one partition the 边框
// frame really needs: sub_1444EECA0 case 0 sends its trailing id list to the acquired
// set and the three single slots to the selection vector, so the raid party list
// frames — the only family the PVF labels distinctly — have to land in the list.
func TestSkinFamilySelectionPayloadPartitionsRaidList(t *testing.T) {
	entries := map[uint32]catalog.SkinStorageEntry{
		1: {Template: 1, SkinID: 20001, SkinType: "party frame"},
		2: {Template: 2, SkinID: 50002, SkinType: "party frame", SkinSubType: "party request frame"},
		3: {Template: 3, SkinID: 80001, SkinType: "party frame", SkinSubType: "raid party list frame"},
	}
	p, e := skinFamilySelectionPayload(protocol.SkinCategoryPartyFrame,
		[]uint32{20001, 80001, 50002}, nil, skinByID(entries))
	if e != nil {
		t.Fatal(e)
	}
	want, e := protocol.SkinSelectionPartyFrame([]uint32{20001, 50002}, []uint32{80001})
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(p, want) {
		t.Fatalf("party frame payload = %x, want %x", p, want)
	}
}

// TestSkinFamilyForEntryRefusesUnprovenFamilies keeps the durable-only behaviour the
// damage-font work established: a family with no measured panel consumer gets no page
// frame, and the damage font is not in this table because its two tabs already have a
// live chain of their own.
func TestSkinFamilyForEntryRefusesUnprovenFamilies(t *testing.T) {
	for _, family := range []catalog.SkinFamily{catalog.SkinFamilyDamageFont, catalog.SkinFamilyUnknown} {
		if _, ok := skinFamilyForEntry(family); ok {
			t.Fatalf("family %d has a page frame in this table", family)
		}
	}
	if frame, ok := skinFamilyForEntry(catalog.SkinFamilyPartyFrame); !ok ||
		frame.page != protocol.SkinCargoPartyFramePage || frame.category != protocol.SkinCategoryPartyFrame {
		t.Fatalf("party frame mapping = %+v", frame)
	}
	if frame, ok := skinFamilyForEntry(catalog.SkinFamilySkillCutscene); !ok ||
		frame.page != protocol.SkinCargoSkillCutscenePage || frame.category != protocol.SkinCategorySkillCutscene {
		t.Fatalf("skill cutscene mapping = %+v", frame)
	}
}

// TestSkinCutscenePickedListsSplitsTheBodyByPosition covers the 觉醒插图 request body with
// the rows the client really flushed on 2026-09-28: slots 10..19 are the panel's 一觉 rows
// and slots 0..9 its 二觉 rows, so each tab keeps its own draw pool. A tab's default id is a
// row like any other once the player ticks it beside a real skin, and only a list holding
// nothing else at all means 解除.
func TestSkinCutscenePickedListsSplitsTheBodyByPosition(t *testing.T) {
	byID := map[uint32]catalog.SkinStorageEntry{
		30117:  {SkinID: 30117, SkinType: "skill cutscene"},
		30118:  {SkinID: 30118, SkinType: "skill cutscene"},
		100001: {SkinID: 100001, SkinType: "skill cutscene", SkinSubType: "second awakening cutscene"},
	}
	cases := []struct {
		name                      string
		awakening                 []uint32
		secondAwakening           []uint32
		wantAwakening, wantSecond []uint32
	}{
		{"nothing chosen", []uint32{30000}, []uint32{100000}, nil, nil},
		{"one row each, 17:11:02", []uint32{30117}, []uint32{100047},
			[]uint32{30117}, []uint32{100047}},
		{"default ticked with a pick, 17:12:19", []uint32{30000, 30117}, []uint32{100000, 100047},
			[]uint32{30000, 30117}, []uint32{100000, 100047}},
		{"default ticked last, 17:13:07", []uint32{30117, 30000}, []uint32{100047, 100000},
			[]uint32{30117, 30000}, []uint32{100047, 100000}},
		{"three rows, one tab untouched", []uint32{30000, 30117, 30118}, []uint32{100000},
			[]uint32{30000, 30117, 30118}, nil},
		{"the other tab's marker is not a row here", []uint32{100000, 30117}, []uint32{100047},
			[]uint32{30117}, []uint32{100047}},
		{"a moved row replaces the marker it landed on", []uint32{30000, 100001, 30117},
			[]uint32{100000}, []uint32{30000, 30117}, []uint32{100001}},
	}
	for _, tc := range cases {
		request := protocol.SelectSkinRequest{
			Category:        protocol.SkinCategorySkillCutscene,
			Awakening:       tc.awakening,
			SecondAwakening: tc.secondAwakening,
		}
		gotAwakening, gotSecond := skinCutscenePickedLists(request, byID)
		if !reflect.DeepEqual(gotAwakening, tc.wantAwakening) || !reflect.DeepEqual(gotSecond, tc.wantSecond) {
			t.Fatalf("%s: got (%v, %v), want (%v, %v)", tc.name, gotAwakening, gotSecond,
				tc.wantAwakening, tc.wantSecond)
		}
	}
}

// TestSkinCutsceneStoredListsRoutesBySkinLabel covers the 进城 / 入副本 re-push, where the
// storage rows carry no slot position: each tab's own default is matched by value and every
// other family comes from the skin's PVF sub type, so a stored 二觉 row never lands back in
// the 一觉 pool.
func TestSkinCutsceneStoredListsRoutesBySkinLabel(t *testing.T) {
	byID := map[uint32]catalog.SkinStorageEntry{
		30117:  {SkinID: 30117, SkinType: "skill cutscene"},
		100001: {SkinID: 100001, SkinType: "skill cutscene", SkinSubType: "second awakening cutscene"},
	}
	awakening, second := skinCutsceneStoredLists([]uint32{30117, 100001, 30000}, byID)
	if !reflect.DeepEqual(awakening, []uint32{30117, 30000}) || !reflect.DeepEqual(second, []uint32{100001}) {
		t.Fatalf("stored split = (%v, %v)", awakening, second)
	}
	if a, b := skinCutsceneStoredLists([]uint32{30000, 100000}, byID); a != nil || b != nil {
		t.Fatalf("解除 markers kept: (%v, %v)", a, b)
	}
	if a, b := skinCutsceneStoredLists([]uint32{30117}, byID); !reflect.DeepEqual(a, []uint32{30117}) || b != nil {
		t.Fatalf("single 一觉 row = (%v, %v)", a, b)
	}
}

// TestSkinKeepOwnedReportsUnownedIDs pins the guard the client does not do for the 二觉
// list: sub_1444EECA0 stores any id greater than zero there without an ownership check,
// so the server has to refuse a pick the account does not hold instead of pushing it.
func TestSkinKeepOwnedReportsUnownedIDs(t *testing.T) {
	owned := map[uint32]bool{30117: true}
	kept, missing := skinKeepOwned([]uint32{30117, 30999}, owned)
	if !reflect.DeepEqual(kept, []uint32{30117}) || !reflect.DeepEqual(missing, []uint32{30999}) {
		t.Fatalf("kept = %v, missing = %v", kept, missing)
	}
}

// TestSkinFamilyFramePayloadRoutesByShape pins the dispatcher the three newer families
// need, because their bodies are not sets: 表情 is four positional words (zeros included,
// so a cell's index survives), 涂鸦 and 飞空艇特效 carry one id, and an empty selection in
// either shape sends no frame at all — the 7 / 8 reader's unowned branch stores nothing,
// and the 3 reader appends the zeros it is given, so an absent frame and a cleared bar are
// the same client state.
func TestSkinFamilyFramePayloadRoutesByShape(t *testing.T) {
	byID := map[uint32]catalog.SkinStorageEntry{
		40001: {SkinID: 40001, SkinType: "instant emoticon"},
		90001: {SkinID: 90001, SkinType: "spray"},
	}
	p, e := skinFamilyFramePayload(protocol.SkinCategoryInstantEmoticon,
		nil, nil, []uint32{40001, 0, 0, 0}, byID)
	if e != nil {
		t.Fatal(e)
	}
	want, e := protocol.SkinSelectionInstantEmoticon([]uint32{40001, 0, 0, 0})
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(p, want) {
		t.Fatalf("emoticon frame = %x, want %x", p, want)
	}
	if p, e = skinFamilyFramePayload(protocol.SkinCategoryInstantEmoticon,
		nil, nil, []uint32{0, 0, 0, 0}, byID); e != nil || p != nil {
		t.Fatalf("empty bar encoded as %x (%v)", p, e)
	}
	if p, e = skinFamilyFramePayload(protocol.SkinCategorySpray, []uint32{90001}, nil, nil, byID); e != nil {
		t.Fatal(e)
	}
	want, e = protocol.SkinSelectionSingle(protocol.SkinCategorySpray, 90001)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(p, want) {
		t.Fatalf("spray frame = %x, want %x", p, want)
	}
	if p, e = skinFamilyFramePayload(protocol.SkinCategoryAirshipEffect, nil, nil, nil, byID); e != nil || p != nil {
		t.Fatalf("empty single frame = %x (%v)", p, e)
	}
	// The set-shaped families still go through their own two-channel encoder.
	if p, e = skinFamilyFramePayload(protocol.SkinCategoryPartyFrame,
		[]uint32{20001}, nil, nil, byID); e != nil || p == nil {
		t.Fatalf("party frame through the dispatcher = %x (%v)", p, e)
	}
}

// TestSkinFamilyEntryRestoresCarryTheNewFamiliesAndStars pins where the appended entry
// frames sit: a NOTI1545 page rebuilds the page it names, so the newer family pages have
// to follow the profile-decoration pair that owns page 0 and precede the town refresh,
// exactly like the two families the named fields carry.
func TestSkinFamilyEntryRestoresCarryTheNewFamiliesAndStars(t *testing.T) {
	p := entryPayloads{
		ProfileSkinCargo: []byte{1},
		SkinFamilyRestores: []outboundPacket{
			{"skin_cargo_family_restored", 0, 1545, []byte{3}},
			{"skin_selection_family_restored", 0, 1546, []byte{3, 1, 2, 3, 4}},
			{"skin_favorites_restored", 0, 2641, []byte{0, 0, 0, 0}},
		},
	}
	index := map[string]int{}
	for i, packet := range p.packets() {
		index[packet.Name] = i
	}
	profile, ok := index["profile_skin_cargo_restored"]
	if !ok {
		t.Fatal("profile page is not emitted at entry")
	}
	world, ok := index["enter_gameworld_complete_sent"]
	if !ok {
		t.Fatal("town refresh is not emitted at entry")
	}
	for _, name := range []string{"skin_cargo_family_restored", "skin_selection_family_restored",
		"skin_favorites_restored"} {
		at, ok := index[name]
		if !ok {
			t.Fatalf("%s is not emitted at entry", name)
		}
		if at <= profile {
			t.Fatalf("%s = %d arrives before the profile page at %d", name, at, profile)
		}
		if at >= world {
			t.Fatalf("%s = %d must arrive before the town refresh at %d", name, at, world)
		}
	}
}

// TestSkinFavoriteAnswerPushesListBeforeRefreshEcho pins the two frames a star click is
// answered with. The echo's only effect for result 2/3 is the page rebuild it tail-calls,
// and that rebuild reads the manager's 收藏 table — so an echo that goes out first repaints
// the window from the state before the click, which is exactly the live report「星星点击不点
// 亮、概要不随应用实时更改」while the storage itself had already recorded the toggle.
func TestSkinFavoriteAnswerPushesListBeforeRefreshEcho(t *testing.T) {
	frames := skinFavoriteAnswer([]byte{1, 9}, []byte{3, 4})
	if len(frames) != 2 {
		t.Fatalf("frames %+v, want the absolute list then the command echo", frames)
	}
	if frames[0].Name != "skin_favorites_restored" || frames[0].ID != 2641 ||
		!bytes.Equal(frames[0].Payload, []byte{3, 4}) {
		t.Fatalf("first frame %+v, want NOTI2641 carrying the stored list", frames[0])
	}
	if frames[1].Name != "skin_selection_echo_only" || frames[1].Kind != 1 || frames[1].ID != 1565 {
		t.Fatalf("second frame %+v, want the kind-1 CMD1565 echo that refreshes the page", frames[1])
	}
}
