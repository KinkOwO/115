package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func robotManagementFixture(t *testing.T) *townRobotFleet {
	t.Helper()
	c, err := catalog.LoadCharacters("../../configs/characters.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	loot, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	gear, err := inventory.LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := inventory.LoadWearRules("../../configs/equipment-wear.current35.json", c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	bagRules, err := inventory.LoadBagRules("../../configs/inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}
	s := &character.Service{Catalog: c, Equipment: gear, WearRules: rules}
	role, err := s.TownDisplayCharacter(60001, "旅人0001", 0, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	json.Unmarshal(role.State, &state)
	state["level"] = 6
	role.State, _ = json.Marshal(state)
	role.ID = -1
	role.WireID = 0
	m := &townRobotManager{path: filepath.Join(t.TempDir(), "robots.json"), records: map[int]townRobotRecord{}, characters: s,
		award: &inventory.Awarder{Catalog: loot, Rules: bagRules, Equipment: gear}, wear: &inventory.WearService{Catalog: gear, Professions: c, BagRules: bagRules, Rules: rules}, host: "127.0.0.1:32100", token: "unit-key"}
	if err = m.store(townRobotRecord{ID: 1, Revision: 1, Role: role}); err != nil {
		t.Fatal(err)
	}
	return &townRobotFleet{manager: m, hub: newLanHub(), stop: make(chan struct{})}
}

func TestTownRobotManagedGrantWearReloadAndRevision(t *testing.T) {
	f := robotManagementFixture(t)
	m := f.manager
	if err := f.changeRobot(townRobotChange{ID: 1, Revision: 1, Action: "grant", Template: 20002, Amount: 1}); err != nil {
		t.Fatal(err)
	}
	before := append([]byte(nil), m.records[1].Role.State...)
	if err := f.changeRobot(townRobotChange{ID: 1, Revision: 1, Action: "grant", Template: 20002, Amount: 1}); err == nil {
		t.Fatal("duplicate request minted again")
	}
	if !bytes.Equal(before, m.records[1].Role.State) {
		t.Fatal("stale request changed state")
	}
	bag, err := inventory.ReadBag(m.records[1].Role.State)
	if err != nil {
		t.Fatal(err)
	}
	var item inventory.BagEquipment
	for _, e := range bag.Equipment {
		if e.Template == 20002 {
			item = e
		}
	}
	if item.Template == 0 {
		t.Fatal("grant missing")
	}
	if err = f.changeRobot(townRobotChange{ID: 1, Revision: 2, Action: "move", Move: protocol.ItemMoveRequest{SourceSlot: item.Slot, SourceItem: item.Template, DestinationList: 3, DestinationSlot: 19, Count: 1, Selection: 0xffffffff}}); err != nil {
		t.Fatal(err)
	}
	bag, _ = inventory.ReadBag(m.records[1].Role.State)
	found := false
	for _, e := range bag.Worn {
		if e.Slot == 19 && e.Template == 20002 {
			found = true
		}
	}
	if !found {
		t.Fatal("not worn")
	}
	saved, err := readTownRobotArchive(m.path)
	if err != nil || saved[1].Revision != m.records[1].Revision {
		t.Fatal("reload mismatch", err)
	}
	var storedState, currentState any
	json.Unmarshal(saved[1].Role.State, &storedState)
	json.Unmarshal(m.records[1].Role.State, &currentState)
	storedJSON, _ := json.Marshal(storedState)
	currentJSON, _ := json.Marshal(currentState)
	if !bytes.Equal(storedJSON, currentJSON) {
		t.Fatal("reload inventory mismatch")
	}
	if saved[1].Role.AccountID != 0 || saved[1].Role.WireID != 0 || saved[1].Role.ID != -1 {
		t.Fatal("persisted player or scene identity")
	}
}

func TestTownRobotManagedMutationFailureKeepsDiskAndMemory(t *testing.T) {
	f := robotManagementFixture(t)
	m := f.manager
	before, _ := os.ReadFile(m.path)
	for _, q := range []townRobotChange{{ID: 1, Revision: 1, Action: "grant", Template: 0, Amount: 1}, {ID: 1, Revision: 1, Action: "grant", Template: 4294967295, Amount: 1}, {ID: 1, Revision: 1, Action: "grant", Template: 20002, Amount: 51}, {ID: 1, Revision: 1, Action: "move", Move: protocol.ItemMoveRequest{SourceItem: 20002, Selection: 0xffffffff}}, {ID: 2, Revision: 1, Action: "grant", Template: 20002, Amount: 1}} {
		if err := f.changeRobot(q); err == nil {
			t.Fatal("accepted invalid mutation", q)
		}
	}
	after, _ := os.ReadFile(m.path)
	if !bytes.Equal(before, after) || m.records[1].Revision != 1 {
		t.Fatal("failure committed")
	}
	original := m.path
	m.path = filepath.Join(original, "impossible.json")
	if err := f.changeRobot(townRobotChange{ID: 1, Revision: 1, Action: "grant", Template: 20002, Amount: 1}); err == nil {
		t.Fatal("write failure ignored")
	}
	if m.records[1].Revision != 1 {
		t.Fatal("memory advanced after write failure")
	}
}

func TestTownRobotManagementHTTPGatesAndNoCrossSiteWrites(t *testing.T) {
	f := robotManagementFixture(t)
	for _, tc := range []struct {
		host, key, origin, method, path, body string
		want                                  int
	}{
		{"127.0.0.1:32100", "", "", "GET", "/api/robots", "", 403},
		{"attacker.invalid", "unit-key", "", "GET", "/api/robots", "", 403},
		{"127.0.0.1:32100", "unit-key", "https://attacker.invalid", "POST", "/api/change", "{}", 403},
		{"127.0.0.1:32100", "unit-key", "", "POST", "/api/change", "{} {}", 400},
		{"127.0.0.1:32100", "unit-key", "", "GET", "/api/robots", "", 200},
		{"127.0.0.1:32100", "unit-key", "", "POST", "/api/change", `{"id":1,"revision":1,"action":"grant","template":20002,"amount":1}`, 200},
		{"127.0.0.1:32100", "unit-key", "", "POST", "/api/change", `{"id":1,"revision":1,"action":"grant","template":20002,"amount":1}`, 409},
	} {
		r := httptest.NewRequest(tc.method, "http://"+tc.host+tc.path, strings.NewReader(tc.body))
		r.Host = tc.host
		r.Header.Set("X-Robot-Key", tc.key)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		f.managementHandler(w, r)
		if w.Code != tc.want {
			t.Fatalf("%s %s got%d want%d: %s", tc.method, tc.path, w.Code, tc.want, w.Body.String())
		}
	}
}

func TestTownRobotArchiveRejectsPlayerIdentityAndCorruption(t *testing.T) {
	f := robotManagementFixture(t)
	m := f.manager
	r := m.records[1]
	r.Role.AccountID = 42
	m.records[1] = r
	raw, _ := json.Marshal(townRobotArchive{Version: 1, Records: m.records})
	os.WriteFile(m.path, raw, 0600)
	if _, err := readTownRobotArchive(m.path); err == nil {
		t.Fatal("player identity allowed")
	}
	os.WriteFile(m.path, []byte("broken"), 0600)
	if _, err := readTownRobotArchive(m.path); err == nil {
		t.Fatal("corrupt archive silently reset")
	}
}

func TestTownRobotManagedEquipmentSurvivesSceneIdentityConflict(t *testing.T) {
	f := robotManagementFixture(t)
	if err := f.changeRobot(townRobotChange{ID: 1, Revision: 1, Action: "grant", Template: 20002, Amount: 1}); err != nil {
		t.Fatal(err)
	}
	w, _ := townRobotWorld(t, 1)
	w.characters = f.manager.characters
	f.hub = w.hub
	f.target = 1
	w.hub.townRobots = f
	w.ensureTownRobots()
	if len(f.bots) != 1 {
		t.Fatal("no managed actor")
	}
	actor := f.bots[0].peer.actorID
	stored := append([]byte(nil), f.manager.records[1].Role.State...)
	real := newTestPeer(99, actor)
	w.hub.add(real)
	w.ensureTownRobots()
	if len(f.bots) != 1 || f.bots[0].peer.actorID == actor || f.bots[0].recordID != 1 {
		t.Fatal("scene reassignment lost stable identity")
	}
	if !bytes.Equal(stored, f.manager.records[1].Role.State) {
		t.Fatal("scene reassignment mutated inventory")
	}
	w.hub.retire(w.peer)
	f.tick()
	if len(f.bots) != 0 {
		t.Fatal("scene not reclaimed")
	}
	w.hub.publish(w.peer, 1, 38, 0, 500, 250, false)
	w.ensureTownRobots()
	if len(f.bots) != 1 || f.manager.records[1].Revision != 2 || !bytes.Equal(stored, f.manager.records[1].Role.State) {
		t.Fatal("return reset equipment")
	}
}

func TestTownRobotManagedPetDeliveryUsesCreatureInventory(t *testing.T) {
	f := robotManagementFixture(t)
	m := f.manager
	// This template is taken from the existing historical fixture's creature
	// source; current-PVF coverage separately discovers its IDs from the source.
	for _, d := range m.award.Equipment.Rows {
		kind, err := m.award.Equipment.EquipmentKind(d.ID)
		if err != nil || kind != "[creature]" {
			continue
		}
		raw, err := m.grant(m.records[1].Role.State, d.ID, 1)
		if err != nil {
			t.Fatal(err)
		}
		bag, err := inventory.ReadBag(raw)
		if err != nil || len(bag.Special[7]) != 1 {
			t.Fatal("pet not in creature inventory", err)
		}
		if len(bag.Special[7][0].AvatarOptions) != 0 {
			t.Fatal("avatar extension attached to creature")
		}
		return
	}
	t.Fatal("fixture has no creature")
}

func TestTownRobotContextUsesVisibleSyntheticActor(t *testing.T) {
	for _, isolation := range []string{"none", "channel", "private", "departed", "unverified", "foreign"} {
		t.Run(isolation, func(t *testing.T) {
			c, out, _ := newDispatchTestClient()
			h := newLanHub()
			h.townRobots = &townRobotFleet{}
			c.hub = h
			c.selectedCharacterID = 2
			p := &lanPeer{roleID: 2, actorID: 3, info: []byte{1}}
			b := &lanPeer{robot: true, roleID: -1, actorID: 65534, info: []byte{1}}
			h.add(p)
			h.registerTownRobot(b)
			h.publish(p, 1, 38, 0, 500, 200, false)
			h.publish(b, 1, 38, 0, 600, 200, false)
			c.worldState = &worldSession{hub: h, peer: p, account: 12, role: database.Character{ID: 2, AccountID: 12, WireID: 2}}
			body := make([]byte, 16)
			binary.LittleEndian.PutUint16(body, 65534)
			body[2] = 15
			q := &clientRequest{frame: wire.Frame{Type: 1, ID: 10}, plaintext: body, verified: true}
			switch isolation {
			case "channel":
				b.channel = 2
			case "private":
				b.private = true
			case "departed":
				delete(h.peers, b)
			case "unverified":
				q.verified = false
			case "foreign":
				c.worldState.role.AccountID = 99
			}
			c.dispatchTownRobotContext(q)
			if isolation != "none" {
				if out.Len() != 0 {
					t.Fatal("out-of-scene menu response")
				}
				return
			}
			if out.Len() == 0 {
				t.Fatal("no context response")
			}
			payload, err := wire.DecryptPayload(c.keys, 7, out.Bytes()[wire.ServerHeaderSize:])
			if err != nil {
				t.Fatal(err)
			}
			want, _ := protocol.PlayerContextReply115(65534, 3, 65535)
			if !bytes.HasPrefix(payload, want) {
				t.Fatalf("wrong scene identity %x", payload)
			}
		})
	}
}
