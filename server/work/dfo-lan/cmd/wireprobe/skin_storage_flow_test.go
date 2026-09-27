package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"testing"
)

// TestDamageFontSkinIDs covers the page-2 filter: only skins whose own .skn
// declares a damage font belong to the page, an unknown template cannot claim
// one, and repeated grants of the same skin are sent once.
func TestDamageFontSkinIDs(t *testing.T) {
	entries := map[uint32]catalog.SkinStorageEntry{
		10305398: {Template: 10305398, SkinID: 12, SkinType: "damage font"},
		10358669: {Template: 10358669, SkinID: 59, SkinType: "Damage Font"},
		10900001: {Template: 10900001, SkinID: 30001, SkinType: "spray"},
	}
	skins := []storage.AccountSkin{
		{SourceTemplate: 10305398, SkinKey: 12},
		{SourceTemplate: 10900001, SkinKey: 30001},
		{SourceTemplate: 10358669, SkinKey: 59},
		{SourceTemplate: 10399999, SkinKey: 77},
		{SourceTemplate: 10305398, SkinKey: 12},
	}
	ids := damageFontSkinIDs(skins, entries)
	want := []uint32{12, 59}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}

	// The frame is the page rebuild the panel enumerates: page byte first, then
	// exactly the surviving entries.
	p, e := protocol.SkinCargoPage(protocol.SkinCargoDamageFontPage, ids)
	if e != nil {
		t.Fatal(e)
	}
	if p[0] != protocol.SkinCargoDamageFontPage || p[1] != 2 || p[3] != 12 || p[11] != 59 {
		t.Fatalf("page frame = %x", p)
	}
}

// TestSelectSkinRequestIsAlwaysDecrypted pins the routing gate the 应用 switch
// failed on: 1565 has a handler, so it must be decrypted on every occurrence.
// Under the sample cap the ninth click in a session arrived undecrypted, the
// dispatcher saw an unverified body and answered nothing, which is exactly how
// "the first apply works but I cannot switch" looked on screen.
func TestSelectSkinRequestIsAlwaysDecrypted(t *testing.T) {
	seen := map[uint16]int{}
	for i := 0; i < BodySampleLimit+20; i++ {
		if !retainRequestBody(1565, seen) {
			t.Fatal("a later 应用 request was not decrypted")
		}
	}
	if seen[1565] != 0 {
		t.Fatalf("1565 consumed %d sample slots", seen[1565])
	}
}

// TestDamageFontEntryOrder pins the entry sequence the client's readers need: the
// owned page has to be rebuilt before either panel tab's selection frame looks a
// skin up in it, and the two tabs replay back to back right after it.
func TestDamageFontEntryOrder(t *testing.T) {
	normal, e := protocol.SkinSelectionDamageFont(protocol.SkinSelectionDamageFontNormal, 12)
	if e != nil {
		t.Fatal(e)
	}
	cumulative, e := protocol.SkinSelectionDamageFont(protocol.SkinSelectionDamageFontCumulative, 18)
	if e != nil {
		t.Fatal(e)
	}
	p := entryPayloads{SkinCargoDamageFont: []byte{2, 0, 0, 0, 0},
		SkinSelectionDamageFontNormal:     normal,
		SkinSelectionDamageFontCumulative: cumulative}
	cargo, first, second := -1, -1, -1
	for i, packet := range p.packets() {
		switch {
		case packet.ID == 1545 && packet.Kind == 0 && cargo < 0:
			cargo = i
		case packet.ID == 1546 && packet.Kind == 0:
			if first < 0 {
				first = i
			} else if second < 0 {
				second = i
			}
		}
	}
	if cargo < 0 || first != cargo+1 || second != cargo+2 {
		t.Fatalf("damage font entry order: cargo %d selections %d %d", cargo, first, second)
	}
	if p.packets()[first].Payload[0] != protocol.SkinSelectionDamageFontNormal ||
		p.packets()[second].Payload[0] != protocol.SkinSelectionDamageFontCumulative {
		t.Fatal("damage font tabs replayed with the wrong categories")
	}
}

// TestDamageFontResetFrames pins which tab needs the CMD1565 echo to unequip.
// sub_1444EECA0 case 6 resets the font itself when the id is not owned, so the
// cumulative tab needs one frame; case 2's unowned branch only empties the 生效中
// vector and leaves the rendered id at holder+112, so the normal tab also gets
// the echo whose case 2 rewrites that field. An applied skin is not a reset and
// must not carry the echo. The two frames differ in envelope kind as well: the
// client only dispatches 1565 through its CMD registry when the first envelope
// byte is 1 (sub_1459A1BB0 → sub_14599D200 → sub_1459A2D70).
func TestDamageFontResetFrames(t *testing.T) {
	for _, tc := range []struct {
		name           string
		category, id   uint32
		wantFrameCount int
	}{
		// Each tab's 解除 names its own default: 1 is what the holder constructor
		// puts at holder+112 (df32), 99999999 is what holder+232 starts as.
		{"normal reset", protocol.SkinSelectionDamageFontNormal, protocol.SkinSelectionNormalDamageDefaultFont, 2},
		{"cumulative reset", protocol.SkinSelectionDamageFontCumulative, protocol.SkinSelectionDefaultFont, 1},
		{"normal apply", protocol.SkinSelectionDamageFontNormal, 12, 1},
		{"cumulative apply", protocol.SkinSelectionDamageFontCumulative, 12, 1},
		// The echo is keyed on the tab's own default id, not on any value that
		// looks reset-ish: the normal tab never asks for the cumulative sentinel.
		{"normal tab with the cumulative sentinel", protocol.SkinSelectionDamageFontNormal, protocol.SkinSelectionDefaultFont, 1},
	} {
		frames, e := damageFontSelectionFrames(tc.name, tc.category, tc.id)
		if e != nil {
			t.Fatal(e)
		}
		if len(frames) != tc.wantFrameCount {
			t.Fatalf("%s emitted %d frames, want %d", tc.name, len(frames), tc.wantFrameCount)
		}
		if frames[0].ID != 1546 || frames[0].Kind != 0 {
			t.Fatalf("%s: first frame is kind %d id %d, want the 1546 notification",
				tc.name, frames[0].Kind, frames[0].ID)
		}
		want, e := protocol.SkinSelectionDamageFont(tc.category, tc.id)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(frames[0].Payload, want) {
			t.Fatalf("%s: selection = %x, want %x", tc.name, frames[0].Payload, want)
		}
		if len(frames) == 1 {
			continue
		}
		echo := frames[1]
		if echo.ID != 1565 || echo.Kind != 1 || len(echo.Payload) != 1+protocol.SelectSkinBodySize {
			t.Fatalf("%s: second frame = kind %d id %d %d bytes, want 1565 kind 1 %d bytes",
				tc.name, echo.Kind, echo.ID, len(echo.Payload), 1+protocol.SelectSkinBodySize)
		}
		if echo.Payload[0] == 0 {
			t.Fatalf("%s: echo status byte is 0, the handler would only show a toast", tc.name)
		}
		if got, e := protocol.DecodeSelectSkin(echo.Payload[1:]); e != nil ||
			got != (protocol.SelectSkinRequest{Category: tc.category, SkinID: tc.id}) {
			t.Fatalf("%s: echo decodes back as %+v (%v)", tc.name, got, e)
		}
	}
}
