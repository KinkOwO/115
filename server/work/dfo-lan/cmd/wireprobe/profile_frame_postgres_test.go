package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestProfileFrameSQLiteActorRestore(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixture, e := database.OpenTestFixture(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if e := fixture.Close(); e != nil {
			t.Error(e)
		}
	}()
	store := fixture.Storage()
	defer func() { store.Close() }()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateProfileSkins, store.MigrateSkinCargo, store.MigrateSkinSelectionList} {
		if e = migrate(ctx); e != nil {
			t.Fatal(e)
		}
	}
	account, e := store.DevelopmentAccount(ctx, "profile-frame-test")
	if e != nil {
		t.Fatal(e)
	}
	role, e := store.CreateCharacter(ctx, character.Character{AccountID: account, Name: "FrameRestore", Profession: 0, ConfigVersion: strings.Repeat("a", 64), State: json.RawMessage(`{"level":115,"advancement":0,"unrelated":123}`), Request: []byte{0}}, 24)
	if e != nil {
		t.Fatal(e)
	}
	role.WireID = 2
	entries := map[uint32]catalog.SkinStorageEntry{10420562: {Template: 10420562, SkinID: 20078, SkinType: "party frame"}}
	if e = store.UnlockSkin(ctx, account, 10420562, 20078); e != nil {
		t.Fatal(e)
	}
	if e = store.SetSkinSelectionList(ctx, role.ID, 0, []uint32{20078, 50000, 60000}); e != nil {
		t.Fatal(e)
	}
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	svc := &character.Service{Catalog: professions, ProfileFrames: func(ctx context.Context, r character.Character) ([2]uint32, error) {
		return loadProfileFrames(ctx, store, r, entries)
	}}
	check := func(want uint32) {
		t.Helper()
		for _, project := range []func(character.Character, [2]byte) ([]byte, error){svc.EntryBasicProbe, svc.AppearanceProbe} {
			p, e := project(role, [2]byte{4, 7})
			if e != nil {
				t.Fatal(e)
			}
			if got := binary.LittleEndian.Uint32(p[5+0x2c:]); got != want {
				t.Fatalf("actor frame=%d want=%d", got, want)
			}
			if binary.LittleEndian.Uint32(p[5+0x30:]) != 50000 {
				t.Fatal("request frame was lost")
			}
		}
	}
	check(20078)
	// Exercise the actual apply dispatcher: a warehouse-only acknowledgement
	// cannot update the town portrait, so the actor projection must follow it.
	w := &worldSession{role: role, store: store, characters: svc, skinCatalog: entries}
	for _, key := range []uint32{20078, 20000, 20078} {
		body := make([]byte, 88)
		binary.LittleEndian.PutUint32(body[8:], key)
		binary.LittleEndian.PutUint32(body[12:], 50000)
		binary.LittleEndian.PutUint32(body[16:], 60000)
		frames, err := w.selectSkin(body, func(map[string]any) {})
		if err != nil {
			t.Fatal(err)
		}
		if len(frames) != 3 || frames[2].ID != 2 {
			t.Fatalf("apply must refresh the actor after the selection: %+v", frames)
		}
		if got := binary.LittleEndian.Uint32(frames[2].Payload[5+0x2c:]); got != key {
			t.Fatalf("apply projected frame %d, want %d", got, key)
		}
	}
	store.Close()
	store, e = fixture.Reopen(ctx)
	if e != nil {
		t.Fatal(e)
	}
	check(20078) // New connection/service projection reads the persisted choice.
	if e = store.SetSkinSelectionList(ctx, role.ID, 0, []uint32{20000, 50000, 60000}); e != nil {
		t.Fatal(e)
	}
	check(20000)
	if string(role.State) != `{"level":115,"advancement":0,"unrelated":123}` {
		t.Fatal("profile projection changed character state")
	}
}
