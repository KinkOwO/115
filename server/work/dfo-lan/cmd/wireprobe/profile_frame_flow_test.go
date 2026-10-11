package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"reflect"
	"testing"
)

type frameStore struct {
	profile character.ProfileSkinState
	keys    []uint32
	skins   []database.AccountSkin
}

func (s *frameStore) RestoreProfileSkins(context.Context, int64, int64) (character.ProfileSkinState, error) {
	return s.profile, nil
}
func (s *frameStore) SkinSelectionList(context.Context, int64, uint32) ([]uint32, error) {
	return s.keys, nil
}
func (s *frameStore) ListSkins(context.Context, int64) ([]database.AccountSkin, error) {
	return s.skins, nil
}

func TestProfileFrameSelectionOwnershipSubtypeAndRestore(t *testing.T) {
	entries := map[uint32]catalog.SkinStorageEntry{
		1: {Template: 1, SkinID: 500020003, SkinType: "party frame"},
		2: {Template: 2, SkinID: 500050003, SkinType: "party frame", SkinSubType: "party request frame"},
		3: {Template: 3, SkinID: 60015, SkinType: "party frame", SkinSubType: "character info BG"},
		4: {Template: 4, SkinID: 80001, SkinType: "party frame", SkinSubType: "raid party list frame"},
		5: {Template: 5, SkinID: 20078, SkinType: "party frame"},
	}
	s := &frameStore{profile: character.ProfileSkinDefaults()}
	for template, entry := range entries {
		if template != 5 {
			s.skins = append(s.skins, database.AccountSkin{SourceTemplate: template, SkinKey: entry.SkinID})
		}
	}
	role := character.Character{ID: 2, AccountID: 1}
	for _, tt := range []struct {
		keys []uint32
		want [2]uint32
	}{
		{nil, [2]uint32{20000, 50000}},
		{[]uint32{500020003, 500050003, 60015, 80001}, [2]uint32{500020003, 500050003}},
		{[]uint32{20078, 500050003}, [2]uint32{20000, 500050003}},
		{[]uint32{20000, 50000, 60000}, [2]uint32{20000, 50000}},
	} {
		s.keys = tt.keys
		got, e := loadProfileFrames(context.Background(), s, role, entries)
		if e != nil || got != tt.want {
			t.Fatalf("keys=%v got=%v want=%v error=%v", tt.keys, got, tt.want, e)
		}
	}
	// A legacy custom profile survives when no new selection list exists.
	s.keys = nil
	s.profile.Selected[0] = 20123
	got, e := loadProfileFrames(context.Background(), s, role, entries)
	if e != nil || got[0] != 20123 {
		t.Fatalf("legacy selection was reset: %v %v", got, e)
	}
}

func TestProfileFrameDoesNotRebuildDungeonActor(t *testing.T) {
	w := &worldSession{activeDungeon: &dungeon.Session{}}
	got, e := w.profileFrameRefresh()
	if e != nil || !reflect.DeepEqual(got, []outboundPacket(nil)) {
		t.Fatal("skin apply rebuilt an active dungeon actor")
	}
}
