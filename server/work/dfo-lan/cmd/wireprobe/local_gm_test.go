package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/localgm"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

type gmTestConn struct {
	net.Conn
	bytes.Buffer
}

func (c *gmTestConn) Read(b []byte) (int, error) { return 0, io.EOF }

func (c *gmTestConn) Write(b []byte) (int, error)      { return c.Buffer.Write(b) }
func (c *gmTestConn) SetWriteDeadline(time.Time) error { return nil }
func onlineGMFixture(t *testing.T) (*gameConnection, uint32) {
	t.Helper()
	svc, store, role := awakeningSQLiteFixture(t, 1)
	ctx := context.Background()
	if e := store.MigrateGrants(ctx); e != nil {
		t.Fatal(e)
	}
	db, e := store.DatabaseName(ctx)
	if e != nil {
		t.Fatal(e)
	}
	release, e := store.HoldAdminGuard(ctx)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(release)
	cat, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	rules, e := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if e != nil {
		t.Fatal(e)
	}
	var item uint32
	for id, it := range cat.Items {
		if it.StackableType == "[throw]" {
			item = id
			break
		}
	}
	if item == 0 {
		t.Fatal("no item")
	}
	role.WireID = 1
	runtime := &gatewayRuntime{gameStore: store, lootService: &loot.Service{Catalog: cat, BagRules: rules}, onlineGM: &onlineGMRegistry{database: db}, progressionService: &character.ProgressionService{Catalog: catalog.Progression{Thresholds: []uint64{100, 300, 600}}}}
	client := &gameConnection{gatewayRuntime: runtime, characters: svc, worldState: &worldSession{role: role}, selectedCharacterID: role.ID, bootstrapped: true, keys: make([]byte, wire.SessionKeyBytes), event: func(map[string]any) {}}
	client.output = newConnectionOutput(&gmTestConn{}, client.keys, "fixture", client.event)
	return client, item
}
func gmRequest(c *gameConnection, path, id string, values map[string]any) (localgm.Request, onlineGMRequest) {
	values["account"] = c.worldState.role.AccountID
	values["character"] = c.selectedCharacterID
	values["grant_id"] = id
	values["reason"] = "online test"
	b, _ := json.Marshal(values)
	var r onlineGMRequest
	json.Unmarshal(b, &r)
	return localgm.Request{Path: path, Body: b}, r
}
func TestOnlineGMGrantWalletLevelAndRelog(t *testing.T) {
	c, item := onlineGMFixture(t)
	ctx := context.Background()
	call := func(path, id string, values map[string]any) localgm.Result {
		q, r := gmRequest(c, path, id, values)
		return c.applyOnlineGM(ctx, q, r)
	}
	values := map[string]any{"gold": 100, "cera": 50, "items": []database.GrantItem{{Template: item, Amount: 3}}}
	for i := 0; i < 2; i++ {
		r := call("/api/grant", "live-grant-0001", values)
		if r.Status != 200 {
			t.Fatal(string(r.Body))
		}
		var m map[string]any
		json.Unmarshal(r.Body, &m)
		if m["applied"] != (i == 0) {
			t.Fatal("replay", m)
		}
	}
	bag, e := inventory.ReadBag(c.worldState.role.State)
	if e != nil || bag.Gold != 12445 || len(bag.Items) != 1 || bag.Items[0].Amount != 3 {
		t.Fatal(bag, e)
	}
	before := append([]byte(nil), c.worldState.role.State...)
	values["gold"] = 999
	r := call("/api/grant", "live-grant-0001", values)
	if r.Status == 200 {
		t.Fatal("changed retry accepted")
	}
	r = call("/api/grant", "live-invalid-0001", map[string]any{"gold": 100, "cera": 50, "items": []database.GrantItem{{Template: 4294967294, Amount: 3}}})
	if r.Status == 200 {
		t.Fatal("invalid item accepted")
	}
	saved, _ := c.gameStore.AdminCharacter(ctx, c.selectedCharacterID)
	if !bytes.Equal(saved.State, before) {
		t.Fatal("failed grant changed save")
	}
	cera, _ := c.gameStore.AccountCera(ctx, saved.AccountID)
	if cera != 50 {
		t.Fatal("failed grant charged", cera)
	}
	for _, v := range []struct {
		action string
		value  uint32
	}{{"gold_set", 22}, {"cera_set", 33}} {
		r = call("/api/gm/command", "live-"+v.action, map[string]any{"action": v.action, "value": v.value})
		if r.Status != 200 {
			t.Fatal(string(r.Body))
		}
	}
	start := time.Now()
	r = call("/api/gm/level", "live-level-0001", map[string]any{"level": 4})
	if r.Status != 200 {
		t.Fatal(string(r.Body))
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("nested SQLite lookup")
	}
	saved, e = c.gameStore.AdminCharacter(ctx, c.selectedCharacterID)
	if e != nil {
		t.Fatal(e)
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(saved.State, &fields)
	if string(fields["level"]) != "4" || string(fields["experience"]) != "600" || string(fields["future_field"]) != `{"keep":true}` {
		t.Fatal(string(saved.State))
	}
	bag, _ = inventory.ReadBag(saved.State)
	cera, _ = c.gameStore.AccountCera(ctx, saved.AccountID)
	if bag.Gold != 22 || cera != 33 || bag.Items[0].Amount != 3 {
		t.Fatal("relog lost values", bag, cera)
	}
	if len(c.selectedBasic) == 0 || len(c.selectedAddition) == 0 {
		t.Fatal("actor caches stale")
	}
	backups, e := database.ListBackups(strings.TrimSuffix(c.onlineGM.database, "awakening.db") + "gm-backups/local")
	if e != nil || len(backups) == 0 {
		t.Fatal("no backups", e)
	}
	for _, b := range backups {
		if e = database.VerifyBackup(b); e != nil {
			t.Fatal(e)
		}
	}
	// Inspect every emitted transport frame and require all three refresh types.
	raw := c.output.conn.(*gmTestConn).Bytes()
	ids := map[uint16]bool{}
	for len(raw) > 0 {
		if len(raw) < wire.ServerHeaderSize {
			t.Fatal("short packet")
		}
		n := int(binary.LittleEndian.Uint32(raw[3:7]))
		if n < wire.ServerHeaderSize || n > len(raw) {
			t.Fatal("invalid length", n)
		}
		if e = wire.ValidateServer(raw[:n]); e != nil {
			t.Fatal(e)
		}
		if raw[11] != wire.Checksum(raw[16:n]) {
			t.Fatal("packet checksum")
		}
		ids[binary.LittleEndian.Uint16(raw[1:3])] = true
		raw = raw[n:]
	}
	for _, id := range []uint16{13, 2, 37, 53} {
		if !ids[id] {
			t.Fatal("missing refresh", id)
		}
	}
}
func TestOnlineGMRejectsOverflowAndEncodingFailureAtomically(t *testing.T) {
	c, _ := onlineGMFixture(t)
	ctx := context.Background()
	before, _ := c.gameStore.AdminCharacter(ctx, c.selectedCharacterID)
	for i, v := range []map[string]any{{"cera": int64(2147483648)}, {"gold": uint32(4294967295)}} {
		q, r := gmRequest(c, "/api/grant", []string{"overflow-cera", "overflow-gold"}[i], v)
		result := c.applyOnlineGM(ctx, q, r)
		if result.Status == 200 {
			t.Fatal("overflow accepted")
		}
	}
	c.keys = nil
	q, r := gmRequest(c, "/api/grant", "encoding-failure", map[string]any{"gold": 100, "cera": 100})
	result := c.applyOnlineGM(ctx, q, r)
	if result.Status == 200 {
		t.Fatal("unencodable refresh committed")
	}
	after, _ := c.gameStore.AdminCharacter(ctx, c.selectedCharacterID)
	balance, _ := c.gameStore.AccountCera(ctx, after.AccountID)
	if !bytes.Equal(before.State, after.State) || balance != 0 {
		t.Fatal("failure changed save")
	}
}
func TestOnlineGMQueueOwnershipAndDisconnect(t *testing.T) {
	c, _ := onlineGMFixture(t)
	c.connection = newConnectionSession(false)
	defer c.connection.close()
	c.gmCalls = make(chan onlineGMCall)
	remove := c.onlineGM.add(c)
	defer remove()
	frames := make(chan clientRead)
	c.frames = frames
	done := make(chan struct{})
	go func() { c.serve(); close(done) }()
	q, _ := gmRequest(c, "/api/grant", "queue-grant-0001", map[string]any{"gold": 5})
	r := c.onlineGM.execute(context.Background(), q)
	if r.Status != 200 {
		t.Fatal(string(r.Body))
	}
	var wrong map[string]any
	json.Unmarshal(q.Body, &wrong)
	wrong["account"] = 99999
	q.Body, _ = json.Marshal(wrong)
	r = c.onlineGM.execute(context.Background(), q)
	if r.Status != 409 {
		t.Fatal("foreign role accepted", r)
	}
	frames <- clientRead{err: io.EOF}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("loop did not stop")
	}
	c.connection.close()
	r = c.onlineGM.execute(context.Background(), q)
	if r.Status != 409 {
		t.Fatal(r)
	}
}
