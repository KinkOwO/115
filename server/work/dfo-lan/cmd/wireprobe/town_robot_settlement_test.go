package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func robotLiveResults(t *testing.T) [][]byte {
	t.Helper()
	data, err := os.ReadFile("../../internal/game/protocol/testdata/live_robot_result115_r16.json")
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Captures []struct {
			Payload string `json:"payload_hex"`
		}
	}
	if err = json.Unmarshal(data, &capture); err != nil {
		t.Fatal(err)
	}
	var bodies [][]byte
	for _, c := range capture.Captures {
		b, e := hex.DecodeString(c.Payload)
		if e != nil {
			t.Fatal(e)
		}
		bodies = append(bodies, b)
	}
	return bodies
}

// Uses a temporary SQLite save. The actual r16 CMD46 body drives the ordinary
// reward/card services; no receipt or completion-state bypass is added to runtime.
func robotSettlementFixture(t *testing.T) (*gameConnection, *townRobotFleet) {
	t.Helper()
	c, f, _, _ := townPartyFixture(t)
	c.dispatchTownRobotParty(townInviteRequest())
	w := c.worldState
	ctx := context.Background()
	store, e := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "settlement.sqlite3"), MaxConnections: 2})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(store.Close)
	account, e := store.DevelopmentAccount(ctx, "robot-settlement")
	if e != nil {
		t.Fatal(e)
	}
	role, e := w.characters.TownDisplayCharacter(2, "ResultOwner", 0, 1, nil)
	if e != nil {
		t.Fatal(e)
	}
	role.AccountID, role.ID = account, 0
	role.Request = []byte{0}
	role, e = store.CreateCharacter(ctx, role, 10)
	if e != nil {
		t.Fatal(e)
	}
	role.WireID = 2
	w.role, w.account, w.store, w.level = role, account, store, 1
	w.peer.actorID, w.peer.roleID = 2, role.ID
	c.selectedCharacterID = role.ID
	f.parties[w.peer].transport.ownerActor = 2
	w.channelType = 22
	source := w.characters.Catalog.Source
	w.progression = &character.ProgressionService{Store: store, Professions: w.characters.Catalog,
		Rules: character.GrowthRules{Model: "robot-settlement-test", LevelCap: 2},
		Catalog: catalog.Progression{Source: source, Thresholds: []uint64{1000000}, DifficultyRates: []float32{1, 1, 1, 1, 1}, Scripts: map[string]catalog.ScriptRecord{
			"n_quest/questparameter.etc": {Cells: []pvf.Token{{Type: 3, Text: "[exp reward table]"}, {Type: 0, Value: 100}, {Type: 0, Value: -1}}},
		}},
	}
	lc := catalog.LootCatalog{Source: source}
	w.loot = &loot.Service{Catalog: lc, Tables: loot.Tables{Gold: []float64{1, 100, 0}}, CardPolicy: &loot.CardRules{Model: "reference90-free-gold-v1", GoldNumerator: 1, GoldDenominator: 1, Difficulty: []float64{1, 1, 1, 1, 1}}, BagRules: f.manager.award.Rules}
	w.items = &inventory.ItemService{Catalog: lc}
	// Packet restoration uses separate character helpers, irrelevant to the
	// settlement contract and not needed for this minimal fixture.
	w.characters = nil
	room := catalog.DungeonRoom{Map: 991, Boss: true}
	d := catalog.DungeonDefinition{ID: 3, BasisLevel: 1, Mazes: []catalog.DungeonMaze{{Start: [2]byte{}, Boss: [2]byte{}, Rooms: []catalog.DungeonRoom{room}}}}
	w.dungeons = &catalog.DungeonCatalog{Source: source, Dungeons: map[uint32]catalog.DungeonDefinition{3: d}, Maps: map[uint32]catalog.ScriptRecord{991: {}}}
	w.activeDungeon = &dungeon.Session{RunID: strings.Repeat("a", 32), StartedAt: time.Now().Add(-time.Minute), Definition: d, Maze: d.Mazes[0], Room: room, Loaded: true, Difficulty: 1}
	w.activeDungeon.MarkCompleted()
	w.completionSent = true
	w.state.Position = database.WorldPosition{Town: 38, Area: 0, X: 500, Y: 200}
	f.parties[w.peer].entryPrepared = true
	w.leaveScene()
	return c, f
}

func TestTownRobotLiveSettlementRewardsAndExit(t *testing.T) {
	for _, option := range []byte{0, 2, 3} {
		t.Run(string(rune('0'+option)), func(t *testing.T) {
			c, f := robotSettlementFixture(t)
			w := c.worldState
			p := f.parties[w.peer]
			botBefore := bytes.Clone(f.manager.records[1].Role.State)
			live := robotLiveResults(t)[0]
			plan, e := w.dungeonResult(live)
			if e != nil {
				t.Fatal(e)
			}
			if len(plan) < 4 || plan[0].ID != 34 || plan[1].ID != 37 || plan[2].ID != 35 || plan[3].ID != 261 || !bytes.Equal(plan[3].Payload, protocol.EplpRechallenge(protocol.EplpRechallengeReady)) {
				t.Fatal("settlement/rechallenge not ready", plan)
			}
			if w.cardPlan == nil || w.cardPlan.Run != w.activeDungeon.RunID {
				t.Fatal("no owned card plan")
			}
			receipt, e := w.store.CharacterEventReceipt(context.Background(), w.account, w.role.ID, "clear:"+w.activeDungeon.RunID)
			if e != nil {
				t.Fatal(e)
			}
			var cleared character.ClearReceipt
			if e = json.Unmarshal(receipt, &cleared); e != nil || cleared.Rank != 99 || cleared.Base != 100 {
				t.Fatal("reward did not use the human participant's row", cleared, e)
			}
			afterClear := bytes.Clone(w.role.State)
			// Retry before the network post-send flag still uses the durable receipt.
			if _, e = w.dungeonResult(live); e != nil || !bytes.Equal(afterClear, w.role.State) {
				t.Fatal("duplicate clear changed save", e)
			}
			w.resultSent = true
			if plan, e = w.dungeonResult(live); e != nil || len(plan) != 0 {
				t.Fatal("duplicate result sent again", e)
			}
			if e = w.cardsReady(); e != nil {
				t.Fatal(e)
			}
			old := w.activeDungeon
			pending, route, e := w.settlementExit([]byte{1, option, 1})
			if e != nil {
				t.Fatal(e)
			}
			if w.cardReceipt == nil {
				t.Fatal("unclaimed free card was lost")
			}
			bag, e := inventory.ReadBag(w.role.State)
			if e != nil || bag.Gold != 100 {
				t.Fatal("card reward lost or duplicated", bag.Gold, e)
			}
			if !bytes.Equal(botBefore, f.manager.records[1].Role.State) {
				t.Fatal("player reward modified fake player's save")
			}
			if option != 0 {
				if pending != nil || f.parties[w.peer] != p || p.dungeon || p.transport.closed || countPacket(route, 24) != 1 || countPacket(route, 9) != 0 || countPacket(route, 10) != 0 {
					t.Fatal("settlement town return disbanded the party", route)
				}
				f.mu.Lock()
				f.prune()
				f.mu.Unlock()
				if f.parties[w.peer] != p || !visible(w.peer, p.bots[0].peer) || p.transport.room != nil || p.transport.roomLoading {
					t.Fatal("town party pruned or still simulating dungeon")
				}
				// Same party can load again without another invite/handshake.
				w.activeDungeon = nil
				w.resetCards()
				entry, err := w.dungeonEntryPlan(context.Background(), "dungeon_select_ack", 16, protocol.DungeonSelection{ID: old.Definition.ID, Party: 65535}, old)
				if err != nil || !p.entryPrepared || countPacket(entry, 9) != 0 {
					t.Fatal("return lost next entry", err)
				}
				w.activeDungeon = old
				w.leaveScene()
				loaded, err := w.finishDungeonLoading(make([]byte, 16))
				if err != nil || p.transport.closed || !p.dungeon || countPacket(loaded, 29) != 1 {
					t.Fatal("kept party could not reload", err)
				}
			} else {
				if pending == nil || pending.RunID == old.RunID || pending.Completed() || f.parties[w.peer] != p || !p.entryPrepared {
					t.Fatal("retry lost party/new run")
				}
				if countPacket(route, 15) != 1 || countPacket(route, 27) != 1 || countPacket(route, 28) != 1 || countPacket(route, 24) != 0 || countPacket(route, 9) != 2 || countPacket(route, 11) != 1 {
					t.Fatal("retry route destroys roster", route)
				}
				w.activeDungeon = pending
				w.leaveScene()
				if p.transport.closed || !p.dungeon || p.entryPrepared {
					t.Fatal("retry released endpoint")
				}
				loaded, e := w.finishDungeonLoading(make([]byte, 16))
				if e != nil || countPacket(loaded, 29) != 1 {
					t.Fatal("retry omitted fake actor", e, loaded)
				}
			}
		})
	}
}

func TestTownRobotReturnRetainsIdentityButExplicitLeaveStillDisbands(t *testing.T) {
	for _, action := range []string{"kick", "leave", "dungeon_leave"} {
		t.Run(action, func(t *testing.T) {
			c, f := robotSettlementFixture(t)
			w := c.worldState
			p := f.parties[w.peer]
			tr := p.transport
			tr.mu.Lock()
			tr.completed = true
			tr.authorityPath = filepath.Join(t.TempDir(), "authority.bin")
			tr.mu.Unlock()
			tr.setRoom(&[2]uint16{1, 2})
			readLease := func(active byte) {
				t.Helper()
				body, err := os.ReadFile(tr.authorityPath)
				if err != nil || len(body) != 64 || body[28] != active {
					t.Fatal("invalid authority state", active, err)
				}
			}
			readLease(1)
			plan, err := w.leaveDungeonToTown(true)
			if err != nil || countPacket(plan, 9) != 0 || countPacket(plan, 10) != 0 || f.parties[w.peer] != p || tr.closed || !tr.completed {
				t.Fatal("return reset native identity/handshake", err)
			}
			readLease(0)
			w.activeDungeon = nil
			w.characters = f.manager.characters
			if c.dispatchTownRobotParty(townInviteRequest()) != dispatchHandled || f.parties[w.peer] != p || p.transport != tr {
				t.Fatal("duplicate invite replaced kept party")
			}
			switch action {
			case "kick":
				body := make([]byte, 16)
				body[0] = 1
				c.dispatchTownRobotParty(&clientRequest{frame: wire.Frame{Type: 1, ID: 14}, plaintext: body, verified: true})
			case "leave":
				c.dispatchTownRobotParty(&clientRequest{frame: wire.Frame{Type: 1, ID: 13}, plaintext: make([]byte, 8), verified: true})
			case "dungeon_leave":
				p.entryPrepared = true
				w.leaveScene()
				plan, err = w.leaveDungeon()
				if err != nil || countPacket(plan, 9) != 1 || countPacket(plan, 10) != 1 {
					t.Fatal("explicit dungeon leave lost native cleanup", err)
				}
			}
			if f.parties[w.peer] != nil || !tr.closed {
				t.Fatal("explicit departure retained party")
			}
			readLease(0)
			if c.dispatchTownRobotParty(townInviteRequest()) != dispatchHandled || f.parties[w.peer] == nil || f.parties[w.peer] == p {
				t.Fatal("cannot reinvite after explicit departure")
			}
		})
	}
}

func TestTownRobotSettlementRejectsUnownedRoster(t *testing.T) {
	for _, mode := range []string{"town", "foreign", "solo", "incomplete", "corrupt"} {
		t.Run(mode, func(t *testing.T) {
			c, f := robotSettlementFixture(t)
			w := c.worldState
			p := f.parties[w.peer]
			body := robotLiveResults(t)[1]
			before := bytes.Clone(w.role.State)
			switch mode {
			case "town":
				p.dungeon = false
			case "foreign":
				p.bots[0].peer.actorID = 65533
			case "solo":
				delete(f.parties, w.peer)
				t.Cleanup(p.transport.close)
			case "incomplete":
				w.completionSent = false
			case "corrupt":
				body[120] ^= 1
			}
			if _, e := w.dungeonResult(body); e == nil || w.cardPlan != nil || !bytes.Equal(before, w.role.State) {
				t.Fatal("unowned/corrupt result authorized a reward", e)
			}
		})
	}
}
