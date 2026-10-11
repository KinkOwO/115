package main

import (
	"bytes"
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/social"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"golang.org/x/text/encoding/simplifiedchinese"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestSocialIdentityRestoresWithoutOpeningAdventure(t *testing.T) {
	w, _ := adventureTestWorld(t)
	client, conn, _ := newDispatchTestClient()
	client.socialProfiles, client.worldState = true, w
	client.selectedCharacterID = w.role.ID
	// Legacy unnamed accounts must not receive the native creation dialog.
	if err := client.restoreSocialIdentity(); err != nil || conn.Len() != 0 {
		t.Fatalf("passive login opened naming dialog: %v", err)
	}
	ctx := context.Background()
	_, _, _, err := w.store.CommitAdventure(ctx, w.account, w.role.ID, "choose-social-name", func(role database.Character, a *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		a.Name, a.Data.NameChosen = "中文团名", true
		return role.State, json.RawMessage(`{}`), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.restoreSocialIdentity(); err != nil {
		t.Fatal(err)
	}
	raw := conn.Bytes()
	if len(raw) < 16 || raw[0] != 0 || binary.LittleEndian.Uint16(raw[1:]) != 1363 {
		t.Fatalf("missing native own-name notification: %x", raw)
	}
	p, err := wire.DecryptPayload(client.keys, 1363, raw[16:])
	want, _ := protocol.AdventureNameInfo("中文团名", false)
	if err != nil || !bytes.HasPrefix(p, want) || w.adventureIdentitySent {
		t.Fatalf("wrong identity or suppressed adventure restore: %x %v", p, err)
	}
	before := conn.Len()
	client.selectedCharacterID++
	if err := client.restoreSocialIdentity(); err != nil || conn.Len() != before {
		t.Fatal("identity sent for mismatched selection")
	}
	client.selectedCharacterID = w.role.ID
	clearSelectedWorld(w)
	if err := client.restoreSocialIdentity(); err != nil || conn.Len() != before {
		t.Fatal("identity sent after leaving character")
	}
}

func TestSocialProfileOwnedCurrentRoleSwitchAndReopen(t *testing.T) {
	w, f := adventureTestWorld(t)
	ctx := context.Background()
	first := w.role
	second, err := w.store.CreateCharacter(ctx, database.Character{AccountID: w.account, Name: "Second", Profession: 2, Request: []byte{}, ConfigVersion: first.ConfigVersion, State: json.RawMessage(`{"level":115,"advancement":3,"awakening":3,"future_field":{"keep":true}}`)}, 100)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := hex.DecodeString("4006dcc2010000003c9ef64501000000")
	for _, role := range []database.Character{first, second, first} {
		w.role = role
		packets, err := w.handleSocialProfile(ctx, 2387, p, p, "test")
		if err != nil || !bytes.Equal(adventurePacket(packets, 2387), []byte{1}) {
			t.Fatalf("representative save: %v %v", packets, err)
		}
		profile, err := w.prepareAdventure(ctx)
		if err != nil || profile.Data.Social == nil || profile.Data.Social.RepresentativeID != role.ID || profile.Data.Social.Representative.CharacterName != role.Name {
			t.Fatalf("wrong representative after A/B/A: %+v %v", profile.Data.Social, err)
		}
	}
	message := make([]byte, 48)
	gbk, _ := simplifiedchinese.GBK.NewEncoder().Bytes([]byte("中文签名"))
	copy(message[13:], gbk)
	if _, err := w.handleSocialProfile(ctx, 2388, message, message, "test"); err != nil {
		t.Fatal(err)
	}
	groups := make([]byte, 160)
	for i := 0; i < 8; i++ {
		groups[45+i] = byte(i + 1)
		binary.LittleEndian.PutUint32(groups[13+i*4:], uint32(i+90))
	}
	copy(groups[53:], []byte("Friends"))
	if _, err := w.handleSocialProfile(ctx, 1676, groups, groups, "test"); err != nil {
		t.Fatal(err)
	}
	before, err := w.prepareAdventure(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// An ordinary adventure write must preserve the new account settings and
	// character data. The existing lock projects the shared season_level field.
	if _, _, _, err := w.store.CommitAdventure(ctx, w.account, w.role.ID, "unrelated-symbol", func(role database.Character, a *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		a.Data.SymbolID = 3
		return role.State, json.RawMessage(`{}`), nil
	}); err != nil {
		t.Fatal(err)
	}
	reopened, err := f.Reopen(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := reopened.MigrateAdventure(ctx); err != nil {
		t.Fatal(err)
	}
	profile, err := reopened.LoadAdventure(ctx, w.account, second.ID, "ignored")
	if err != nil || !reflect.DeepEqual(profile.Data.Social, before.Data.Social) {
		t.Fatalf("reopen/switch lost settings %+v %v", profile.Data.Social, err)
	}
	roles, err := reopened.Characters(ctx, w.account)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		wanted := first.State
		if role.ID == second.ID {
			wanted = second.State
		}
		var gotState, wantState map[string]json.RawMessage
		if json.Unmarshal(role.State, &gotState) != nil || json.Unmarshal(wanted, &wantState) != nil {
			t.Fatal("invalid role JSON")
		}
		delete(gotState, "season_level")
		delete(wantState, "season_level")
		if !reflect.DeepEqual(gotState, wantState) {
			t.Fatalf("character state rewritten: %s", role.State)
		}
	}
	rows, err := reopened.SocialCharacterInfo(ctx, []string{profile.Name, "missing", profile.Name})
	if err != nil || len(rows) != 1 || rows[0].CharacterName != first.Name || rows[0].StateMessage != "中文签名" {
		t.Fatalf("read-only profile lookup %+v %v", rows, err)
	}
	// Full HTTP request passes the native record's required strings/vectors.
	recorder := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]any{"api": "opaque", "req_type": 6, "member_name": []string{profile.Name}})
	social.Handler(reopened.SocialCharacterInfo, nil).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/community", bytes.NewReader(body)))
	var response struct {
		Info []social.CharacterInfo `json:"character_info"`
	}
	if recorder.Code != 200 || json.Unmarshal(recorder.Body.Bytes(), &response) != nil || len(response.Info) != 1 || response.Info[0] != rows[0] {
		t.Fatalf("HTTP profile %d %s", recorder.Code, recorder.Body.String())
	}
	// A different account cannot choose this role, even with forged stack bytes.
	other, err := reopened.DevelopmentAccount(ctx, "other-social")
	if err != nil {
		t.Fatal(err)
	}
	w.account = other
	if _, err := w.handleSocialProfile(ctx, 2387, p, p, "test"); err == nil {
		t.Fatal("foreign role accepted")
	}
}

func TestSocialProfileDispatchChecksumAndUnsupportedMode(t *testing.T) {
	if (&gameConnection{}).dispatchSocialProfile(&clientRequest{frame: wire.Frame{Type: 1, ID: 1395}}) != dispatchNext {
		t.Fatal("unrelated command intercepted")
	}
	w, _ := adventureTestWorld(t)
	client, conn, _ := newDispatchTestClient()
	client.socialProfiles = true
	client.worldState = w
	client.selectedCharacterID = w.role.ID
	p := make([]byte, 16)
	r := &clientRequest{frame: wire.Frame{Type: 1, ID: 2387}, plaintext: p}
	if client.dispatchAccountQueries(r) != dispatchHandled || conn.Len() != 0 {
		t.Fatal("bad checksum reached writer")
	}
	r.verified = true
	p[13] = 1
	p[14] = 3
	if client.dispatchAccountQueries(r) != dispatchHandled || conn.Len() != 0 {
		t.Fatal("unclosed season-costume choice received success")
	}
	p[13], p[14] = 0, 0
	if client.dispatchAccountQueries(r) != dispatchHandled {
		t.Fatal("closed representative request fell through")
	}
	body, err := wire.DecryptPayload(client.keys, 2387, conn.Bytes()[16:])
	if err != nil || body[0] != 1 {
		t.Fatalf("native representative ACK %x %v", body, err)
	}
}

func TestSocialLoginNativeFixturePreservesFollowingFields(t *testing.T) {
	raw, err := os.ReadFile("testdata/login-normal22.bin")
	if err != nil {
		t.Fatal(err)
	}
	keys := make([]byte, wire.SessionKeyBytes)
	for i := range keys {
		keys[i] = byte(i%127 + 1)
	}
	original, err := wire.DecryptPayload(keys, 1, raw[16:])
	if err != nil {
		t.Fatal(err)
	}
	firstLength := int(binary.LittleEndian.Uint32(original[9:]))
	offset := 13 + firstLength + 8
	if binary.LittleEndian.Uint32(original[offset:]) != 0 {
		t.Fatal("native fixture's common API was not empty")
	}
	patched, err := socialLoginResponse(keys, raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := wire.ValidateServer(patched); err != nil {
		t.Fatal(err)
	}
	p, err := wire.DecryptPayload(keys, 1, patched[16:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(p[:offset], original[:offset]) || binary.LittleEndian.Uint32(p[offset:]) != uint32(len(social.LocalURL)) || string(p[offset+4:offset+4+len(social.LocalURL)]) != social.LocalURL || !bytes.HasPrefix(p[offset+4+len(social.LocalURL):], original[offset+4:]) {
		t.Fatal("changed login data outside empty common API field")
	}
	if got, err := socialLoginResponse(keys, patched); err != nil || !bytes.Equal(got, patched) {
		t.Fatal("overwrote nonempty API URL", err)
	}
	corrupt := bytes.Clone(raw)
	corrupt[11] ^= 1
	if _, err := socialLoginResponse(keys, corrupt); err == nil {
		t.Fatal("corrupt login frame accepted")
	}
	short, _ := wire.EncryptPayload(keys, 1, []byte{1, 0, 0})
	shortFrame, _ := wire.ServerFrame(1, 1, short)
	if _, err := socialLoginResponse(keys, shortFrame); err == nil {
		t.Fatal("truncated login body accepted")
	}
}

func TestSocialHTTPUnsupportedRequestsCannotFakeMetadata(t *testing.T) {
	called := false
	h := social.Handler(func(context.Context, []string) ([]social.CharacterInfo, error) { called = true; return nil, nil }, nil)
	for _, body := range []string{`{"req_type":7}`, `{"req_type":6} {}`, `{"req_type":6,"member_name":null} junk`} {
		r := httptest.NewRecorder()
		h.ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/community", strings.NewReader(body)))
		if r.Code == 200 || called {
			t.Fatal("invalid request reached profile lookup", body)
		}
	}
}
