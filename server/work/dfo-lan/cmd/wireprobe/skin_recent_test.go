package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"testing"
	"time"
)

// recentSkin is one account registration at a chosen unlock instant.
func recentSkin(template, key uint32, at int) storage.AccountSkin {
	return storage.AccountSkin{
		SourceTemplate: template,
		SkinKey:        key,
		UnlockedAt:     time.Unix(int64(1700000000+at), 0).UTC(),
	}
}

// recentCatalog returns a family-tagged registry covering every page NOTI1547 renders.
func recentCatalog() map[uint32]catalog.SkinStorageEntry {
	return map[uint32]catalog.SkinStorageEntry{
		1001: {Template: 1001, SkinID: 20001, SkinType: "party frame"},
		1002: {Template: 1002, SkinID: 30001, SkinType: "skill cutscene"},
		1003: {Template: 1003, SkinID: 40001, SkinType: "damage font"},
		1004: {Template: 1004, SkinID: 50001, SkinType: "instant emoticon"},
		1005: {Template: 1005, SkinID: 70001, SkinType: "spray"},
		1006: {Template: 1006, SkinID: 80001, SkinType: "airship effect"},
		// A family no panel enumerates must never reach the strip: the row cell
		// refuses to draw it and the entry stays durable-only.
		1007: {Template: 1007, SkinID: 99001, SkinType: "unknown thing"},
		// Three more 伤害字体 templates so the unlock order can be made to disagree
		// with the storage's ORDER BY source_template.
		1008: {Template: 1008, SkinID: 40002, SkinType: "damage font"},
		1009: {Template: 1009, SkinID: 40003, SkinType: "damage font"},
	}
}

// decodeRecent reads a NOTI1547 body back the way sub_1444ED400 does.
func decodeRecent(t *testing.T, body []byte) []protocol.RecentAddSkinEntry {
	t.Helper()
	if len(body) == 0 {
		t.Fatal("empty body has no count byte")
	}
	count := int(body[0])
	if len(body) != 1+5*count {
		t.Fatalf("body is %d bytes for count %d, want %d", len(body), count, 1+5*count)
	}
	out := make([]protocol.RecentAddSkinEntry, 0, count)
	for i := 0; i < count; i++ {
		p := body[1+5*i:]
		out = append(out, protocol.RecentAddSkinEntry{
			Kind:   p[0],
			SkinID: uint32(p[1]) | uint32(p[2])<<8 | uint32(p[3])<<16 | uint32(p[4])<<24,
		})
	}
	return out
}

func checkRecent(t *testing.T, body []byte, want []protocol.RecentAddSkinEntry) {
	t.Helper()
	got := decodeRecent(t, body)
	if len(got) != len(want) {
		t.Fatalf("entries = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestSkinRecentListKindIsFamilyPage pins the byte the client actually compares: the
// row cell sub_1441E2620 looks up the id in the static registry and refuses to draw
// the whole cell unless the second field equals that record's family class
// (analysis/dumps/CLIENT-MECHANICS.md 20.2). The class is the same number the owned
// page and the selection category use, so the frame carries the page, not a flag.
func TestSkinRecentListKindIsFamilyPage(t *testing.T) {
	skins := []storage.AccountSkin{
		recentSkin(1001, 20001, 1),
		recentSkin(1002, 30001, 2),
		recentSkin(1003, 40001, 3),
		recentSkin(1004, 50001, 4),
		recentSkin(1005, 70001, 5),
		recentSkin(1006, 80001, 6),
		recentSkin(1007, 99001, 7),
	}
	body, e := skinRecentList(skins, recentCatalog(), nil)
	if e != nil {
		t.Fatal(e)
	}
	checkRecent(t, body, []protocol.RecentAddSkinEntry{
		{Kind: protocol.SkinCargoPartyFramePage, SkinID: 20001},
		{Kind: protocol.SkinCargoSkillCutscenePage, SkinID: 30001},
		{Kind: protocol.SkinCargoDamageFontPage, SkinID: 40001},
		{Kind: protocol.SkinCargoInstantEmoticonPage, SkinID: 50001},
		{Kind: protocol.SkinCargoSprayPage, SkinID: 70001},
		{Kind: protocol.SkinCargoAirshipEffectPage, SkinID: 80001},
	})
}

// TestSkinRecentListOrderIsOldestFirst pins the walk direction. sub_1441EAF20 reads
// the vector backwards from its end and fills the five cells in that order, so the
// tail of the frame is the newest acquisition and the strip's first cell shows it.
// ListSkins answers ORDER BY source_template, which is not the unlock order, so the
// sort here is the only thing that can restore it.
func TestSkinRecentListOrderIsOldestFirst(t *testing.T) {
	// Deliberately handed over in the storage's own ORDER BY source_template shape,
	// whose unlock instants run the other way.
	skins := []storage.AccountSkin{
		recentSkin(1003, 40001, 30),
		recentSkin(1008, 40002, 20),
		recentSkin(1009, 40003, 10),
	}
	body, e := skinRecentList(skins, recentCatalog(), nil)
	if e != nil {
		t.Fatal(e)
	}
	checkRecent(t, body, []protocol.RecentAddSkinEntry{
		{Kind: protocol.SkinCargoDamageFontPage, SkinID: 40003},
		{Kind: protocol.SkinCargoDamageFontPage, SkinID: 40002},
		{Kind: protocol.SkinCargoDamageFontPage, SkinID: 40001},
	})
}

// TestSkinRecentListWeaponUsesPage4IDNamespace checks the one family whose id is not a
// skin id: kind 4 rows are drawn from the item table (sub_14021BE90 against the id),
// so a replication of a 林纳斯的神奇模具 has to carry the weapon item template.
func TestSkinRecentListWeaponUsesPage4IDNamespace(t *testing.T) {
	body, e := skinRecentList(nil, recentCatalog(), []uint32{1000001, 1000002})
	if e != nil {
		t.Fatal(e)
	}
	checkRecent(t, body, []protocol.RecentAddSkinEntry{
		{Kind: byte(protocol.SkinCargoWeaponShape), SkinID: 1000001},
		{Kind: byte(protocol.SkinCargoWeaponShape), SkinID: 1000002},
	})
}

// TestSkinRecentListDropsZeroAndDuplicateWeaponID: a zero id would make
// RecentAddSkinList fail outright, and the same skin reachable twice (registered, then
// replicated onto the weapon page) must not steal a second cell.
func TestSkinRecentListDropsZeroAndDuplicateWeaponID(t *testing.T) {
	skins := []storage.AccountSkin{recentSkin(1003, 40001, 1)}
	body, e := skinRecentList(skins, recentCatalog(), []uint32{0, 40001, 40001, 500001})
	if e != nil {
		t.Fatal(e)
	}
	checkRecent(t, body, []protocol.RecentAddSkinEntry{
		{Kind: protocol.SkinCargoDamageFontPage, SkinID: 40001},
		{Kind: byte(protocol.SkinCargoWeaponShape), SkinID: 500001},
	})
}

// TestSkinRecentListTruncatesFromHead: the count is one byte and the strip only ever
// renders the last five, so an over-long list drops its oldest entries rather than
// its newest.
func TestSkinRecentListTruncatesFromHead(t *testing.T) {
	skins := make([]storage.AccountSkin, 0, 300)
	for i := 0; i < 300; i++ {
		skins = append(skins, recentSkin(1003, uint32(40000+i), i))
	}
	body, e := skinRecentList(skins, recentCatalog(), nil)
	if e != nil {
		t.Fatal(e)
	}
	entries := decodeRecent(t, body)
	if len(entries) != 255 {
		t.Fatalf("count = %d, want 255", len(entries))
	}
	if entries[0].SkinID != 40045 || entries[254].SkinID != 40299 {
		t.Fatalf("window = %d..%d, want 40045..40299", entries[0].SkinID, entries[254].SkinID)
	}
}

// TestSkinRecentListSilentWhenNothingAcquired: an empty body would be a valid frame,
// and the reader clears the vector before its loop, so sending it erases the strip.
// A character that has acquired nothing must get no frame at all.
func TestSkinRecentListSilentWhenNothingAcquired(t *testing.T) {
	body, e := skinRecentList(nil, recentCatalog(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if body != nil {
		t.Fatalf("body = %v, want nil", body)
	}
	// An unrenderable family alone is still "nothing acquired".
	only := []storage.AccountSkin{recentSkin(1007, 99001, 1)}
	body, e = skinRecentList(only, recentCatalog(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if body != nil {
		t.Fatalf("body = %v for an unknown family, want nil", body)
	}
}

// TestSkinRecentEntryByteLayout pins the wire order against the reader: it takes a u8
// count, then count groups of {u8, u32}, and stores them as {u32, u8} pairs, so the
// kind byte precedes the little-endian id on the wire.
func TestSkinRecentEntryByteLayout(t *testing.T) {
	skins := []storage.AccountSkin{recentSkin(1003, 0x12345678, 1)}
	body, e := skinRecentList(skins, recentCatalog(), nil)
	if e != nil {
		t.Fatal(e)
	}
	want := []byte{1, protocol.SkinCargoDamageFontPage, 0x78, 0x56, 0x34, 0x12}
	if !bytes.Equal(body, want) {
		t.Fatalf("body = %v, want %v", body, want)
	}
}

// TestEntrySendsSkinRecentOnlyWhenNonEmpty checks the entry plan: the restore frame is
// the strip's only absolute state at town entry, and the pre-fix path (no frame) must
// stay reachable so a storage read failure degrades to the old behavior rather than
// wiping the panel.
func TestEntrySendsSkinRecentOnlyWhenNonEmpty(t *testing.T) {
	if _, ok := packetIndex(t, entryPayloads{})["skin_recent_restored"]; ok {
		t.Fatal("empty SkinRecent must not be emitted")
	}
	at, ok := packetIndex(t, entryPayloads{SkinRecent: []byte{1}})["skin_recent_restored"]
	if !ok {
		t.Fatal("SkinRecent is not emitted at entry")
	}
	cargo, hasCargo := packetIndex(t, entryPayloads{
		SkinRecent:    []byte{1},
		SkinSelection: []byte{2},
	})["skin_cargo_selected"]
	if !hasCargo {
		t.Fatal("the appearance block vanished from the entry plan")
	}
	if at < cargo {
		t.Fatalf("recent frame at %d precedes the weapon-shape block at %d", at, cargo)
	}
}

func packetIndex(t *testing.T, p entryPayloads) map[string]int {
	t.Helper()
	index := map[string]int{}
	for i, packet := range p.packets() {
		index[packet.Name] = i
	}
	return index
}
