package main

import (
	"bytes"
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"strings"
	"testing"
)

func TestBoostRosterUsesListPositionsNotIdentifiers(t *testing.T) {
	makeRole := func(id int64, wire uint16, finished bool) database.Character {
		raw, e := boostup.WriteState(json.RawMessage(`{}`), boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 2, Finished: finished}})
		must115(t, e)
		return database.Character{ID: id, AccountID: 7, WireID: wire, State: raw}
	}
	a, b := makeRole(90, 41, false), makeRole(800, 702, true)
	got, e := boostRosterForRoles([]database.Character{{ID: 9, AccountID: 7, State: json.RawMessage(`{}`)}, a, b}, database.Character{})
	must115(t, e)
	want, e := protocol.BoostRoster115([]protocol.BoostRosterRow115{{Slot: 1, Mode: 0}, {Slot: 2, Mode: 2}})
	must115(t, e)
	if !bytes.Equal(got, want) {
		t.Fatal("stored ID/wire ID substituted for list slot")
	}
	foreign := a
	foreign.AccountID = 8
	if _, e = boostRosterForRoles([]database.Character{a, b}, foreign); e == nil {
		t.Fatal("foreign override accepted")
	}
	if _, e = boostRosterForRoles([]database.Character{a}, b); e == nil {
		t.Fatal("missing override quietly omitted")
	}
}

func assertBoostRosterSQL(t *testing.T, ctx context.Context, store *database.Store) {
	t.Helper()
	account, e := store.DevelopmentAccount(ctx, "boost-roster-fixture")
	must115(t, e)
	create := func(name string, state boostup.State) database.Character {
		raw := json.RawMessage(`{"level":115,"advancement":1,"awakening":3}`)
		if state.Activated {
			raw, e = boostup.WriteState(raw, state)
			must115(t, e)
		}
		r, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: name, Profession: 0, Request: []byte{0}, ConfigVersion: strings.Repeat("a", 64), State: raw}, 100)
		must115(t, err)
		return r
	}
	ordinary := create("rosterplain", boostup.State{})
	create("rostertraining", boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 6, Phase: 2}})
	create("rostergraduate", boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 12, Finished: true}})
	// donor 基线的 character.Service.Updated115 开关不在本树（本树 roster 变体由
	// selectionRosterPackets 自己判断），机械去掉不存在字段。
	s := &character.Service{Store: store, Rules: character.Rules{MaxCharacters: 100}}
	c := &boostup.Catalog{}
	plan, e := selectionRosterPackets(ctx, s, account, nil, c)
	must115(t, e)
	want, e := protocol.BoostRoster115([]protocol.BoostRosterRow115{{Slot: 1, Mode: 0}, {Slot: 2, Mode: 2}})
	must115(t, e)
	if len(plan) != 2 || plan[0].ID != 2639 || plan[1].ID != 2 || !bytes.Equal(plan[0].Payload, want) {
		t.Fatal("markers must precede same-snapshot roster")
	}
	legacy, e := s.List(ctx, account)
	must115(t, e)
	if !bytes.Equal(plan[1].Payload, legacy) {
		t.Fatal("ordinary character list shape changed")
	}
	// donor 基线的 protocol.Login115Packet{Kind,ID,Body} 在本树不存在：登录期
	// 计划一律用 cmd 本地 outboundPacket{Kind,ID,Payload}，字节断言不变。
	login, e := boostLoginRoster(ctx, store, account, c)
	must115(t, e)
	if len(login) != 1 || login[0].ID != 2639 || !bytes.Equal(login[0].Payload, want) {
		t.Fatal("login uses another account/slot view")
	}
	_, e = store.DeleteCharacter(ctx, account, 0, ordinary.Name)
	must115(t, e)
	plan, e = selectionRosterPackets(ctx, s, account, nil, c)
	must115(t, e)
	want, e = protocol.BoostRoster115([]protocol.BoostRosterRow115{{Slot: 0, Mode: 0}, {Slot: 1, Mode: 2}})
	must115(t, e)
	if !bytes.Equal(plan[0].Payload, want) {
		t.Fatal("deleted prefix left old boost indices")
	}
	off, e := selectionRosterPackets(ctx, s, account, nil, nil)
	must115(t, e)
	if len(off) != 1 || off[0].ID != 2 || !bytes.Equal(off[0].Payload, plan[1].Payload) {
		t.Fatal("disabled activity changed normal roster")
	}
	emptyAccount, e := store.DevelopmentAccount(ctx, "boost-roster-empty")
	must115(t, e)
	empty, e := boostLoginRoster(ctx, store, emptyAccount, c)
	must115(t, e)
	if len(empty) != 1 || !bytes.Equal(empty[0].Payload, []byte{0, 0, 0, 0}) {
		t.Fatal("account inherited another account's markers")
	}
}
