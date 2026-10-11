package main

import (
	"bytes"
	"context"
	"dfolan/internal/admin"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestGMExpandedOperationsAtomicOwnedAudited(t *testing.T) {
	ctx := context.Background()
	f, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = f.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := f.DevelopmentAccount(ctx, "gm-expanded")
	if err != nil {
		t.Fatal(err)
	}
	other, err := f.DevelopmentAccount(ctx, "gm-foreign")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("a", 64)
	raw := json.RawMessage(`{"level":5,"unknown":9007199254740993,"inventory":{"version":"ordinary-bag-v1","gold":12,"items":[{"slot":121,"Template":3037,"Amount":4,"future":9007199254740995}]}}`)
	role, err := f.CreateCharacter(ctx, database.Character{AccountID: account, Name: "GM临时存档", Request: []byte{0}, ConfigVersion: (pvf.ArchiveSnapshot{}).SaveIdentity(), State: raw}, 2)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := pvf.ArchiveSnapshot{Checksum: version}
	loot := catalog.LootCatalog{Source: snapshot, Items: map[uint32]catalog.LootItem{3037: {ID: 3037, Kind: "stackable", StackableType: "[material]", StackLimit: 1000}}}
	rules := inventory.BagRules{Model: "reference90-bag-v1", Source: version, Slots: map[string][2]uint16{"[material]": {121, 176}}, MissingStackLimit: 999, EquipmentSlots: [2]uint16{9, 64}}
	ec, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{Source: snapshot, Rows: []inventory.EquipmentDefinition{{ID: 100, Path: "equipment/test.equ", SHA256: strings.Repeat("b", 64), Fields: map[string][]pvf.Token{"[equipment type]": {{Type: 6, Text: "[weapon]"}}, "[rarity]": {{Type: 0, Value: 4}}, "[durability]": {{Type: 0, Value: 50}}}}}}, version)
	if err != nil {
		t.Fatal(err)
	}
	ix := &ItemIndex{byID: map[uint32]int{}, namesClient: NameTable{}, namesZH: NameTable{}, namesEN: NameTable{}}
	ix.push(IndexItem{ID: 3037, Kind: "stackable", Name: "无色小晶块", NativeName: true, StackType: "[material]", StackLimit: 1000})
	ix.push(IndexItem{ID: 100, Kind: "equipment", Name: "测试武器", NativeName: true})
	ix.finish()
	e := &editionManager{kind: "local", config: database.Config{Driver: "sqlite", SQLitePath: f.Path()}, backups: filepath.Join(t.TempDir(), "backups"), quests: catalog.QuestCatalog{Source: snapshot, Quests: map[uint32]catalog.QuestDefinition{50: {ID: 50}, 51: {ID: 51}}}}
	s := &server{store: f.Storage(), token: "test", index: ix, admin: &admin.Service{Store: f.Storage(), Awarder: &inventory.Awarder{Catalog: loot, Rules: rules, Equipment: ec}}, edition: e}
	mux := http.NewServeMux()
	s.registerEditionRoutes(mux)
	n := 0
	call := func(request gmCommand) *httptest.ResponseRecorder {
		n++
		if request.GrantID == "" {
			request.GrantID = fmt.Sprintf("expanded-%04d", n)
		}
		if request.Account == 0 {
			request.Account = account
		}
		request.Character = role.ID
		request.Reason = "fixture validation"
		b, _ := json.Marshal(request)
		r := httptest.NewRequest("POST", "/api/gm/command", bytes.NewReader(b))
		r.Header.Set("X-GM-Token", s.token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		return w
	}
	good := func(request gmCommand) {
		t.Helper()
		if w := call(request); w.Code != 200 {
			t.Fatalf("%s: %d %s", request.Action, w.Code, w.Body)
		}
	}
	bad := func(request gmCommand) {
		t.Helper()
		if w := call(request); w.Code == 200 {
			t.Fatalf("accepted invalid %s: %s", request.Action, w.Body)
		}
	}
	bad(gmCommand{Action: "gold_set", Account: other, Value: 999})
	bad(gmCommand{Action: "gold_set", Revision: "stale", Value: 999})
	good(gmCommand{Action: "gold_set", Value: 777})
	good(gmCommand{Action: "cera_set", Value: 1234})
	good(gmCommand{Action: "cera_set", Value: 12})
	balance, err := f.AccountCera(ctx, account)
	if err != nil || balance != 12 {
		t.Fatalf("set cera %d %v", balance, err)
	}
	good(gmCommand{Action: "sp_set", Value: 12000, Secondary: 9000})
	bad(gmCommand{Action: "tp_set", Value: 65536})
	bad(gmCommand{Action: "tp_set", Value: 5}) // Legacy TP editor is not a VP editor.
	good(gmCommand{Action: "item_edit", Container: "items", Slot: 121, Template: 3037, Amount: 8})
	bad(gmCommand{Action: "item_edit", Container: "items", Slot: 121, Template: 3037, Amount: 1001})
	after, err := f.CharacterState(ctx, role.ID)
	if err != nil || !bytes.Contains(after, []byte("9007199254740995")) || !bytes.Contains(after, []byte("9007199254740993")) {
		t.Fatalf("unknown fields lost: %s %v", after, err)
	}
	bad(gmCommand{Action: "item_delete", Container: "items", Slot: 121, Template: 3036})
	good(gmCommand{Action: "item_grant", Template: 3037, Amount: 2})
	after, _ = f.CharacterState(ctx, role.ID)
	if !bytes.Contains(after, []byte("9007199254740995")) {
		t.Fatal("direct grant discarded future stack metadata")
	}
	key := "expanded-mail-idempotency"
	good(gmCommand{Action: "item_grant", GrantID: key, Template: 3037, Amount: 10, Mail: true, Message: "native mailbox"})
	good(gmCommand{Action: "item_grant", GrantID: key, Template: 3037, Amount: 10, Mail: true, Message: "native mailbox"})
	mail, err := f.Mailbox(ctx, account, role.ID)
	if err != nil || len(mail) != 1 || len(mail[0].Assets) != 1 {
		t.Fatalf("native mail: %+v %v", mail, err)
	}
	good(gmCommand{Action: "item_grant", Template: 100, Amount: 2, Mail: true, Equipment: &inventory.GMEquipment{Upgrade: 16, Amplify: 3, AmplifyValue: 10}})
	mail, err = f.Mailbox(ctx, account, role.ID)
	if err != nil || len(mail) != 2 || len(mail[1].Assets) != 2 {
		t.Fatalf("crafted mail: %+v %v", mail, err)
	}
	var item inventory.MailItem
	if err = json.Unmarshal(mail[1].Assets[0].Item, &item); err != nil {
		t.Fatal(err)
	}
	v := inventory.GMEquipmentValues(*item.Equipment)
	if v.Upgrade != 16 || v.Amplify != 3 || v.AmplifyValue != 10 {
		t.Fatalf("crafted fields %+v", v)
	}
	// A failed attachment request must not leave a header, currency, or audit row.
	bad(gmCommand{Action: "mail_send", Template: 3037, Amount: 1001, Value: 999, Message: "rollback"})
	mail, _ = f.Mailbox(ctx, account, role.ID)
	if len(mail) != 2 {
		t.Fatal("partial mail committed")
	}
	bad(gmCommand{Action: "mail_delete", MailIDs: []int64{999999}})
	good(gmCommand{Action: "mail_delete", MailIDs: []int64{mail[0].ID}})
	good(gmCommand{Action: "material_set", Template: 3037, Value: 500})
	counts, err := f.AccountMaterials(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	m, err := inventory.ReadAccountMaterials(counts)
	if err != nil || m.Count(3037) != 500 {
		t.Fatalf("materials %s %v", counts, err)
	}
	bad(gmCommand{Action: "material_set", Template: 999, Value: 500})
	if _, err = f.AcceptQuest(ctx, account, role.ID, 50, role.ConfigVersion, 1, 115, nil, 3, "fixture-progress"); err != nil {
		t.Fatal(err)
	}
	good(gmCommand{Action: "quest_complete", IDs: []uint16{50, 51}})
	states, err := f.Quests(ctx, account, role.ID)
	if err != nil || len(states) != 2 || states[0].Status != "completed" || states[1].Status != "completed" {
		t.Fatalf("quests %+v %v", states, err)
	}
	bad(gmCommand{Action: "quest_abandon", IDs: []uint16{50}})
	history, err := f.GrantHistory(ctx, account, 100)
	if err != nil || len(history) < 10 {
		t.Fatalf("audit missing: %d %v", len(history), err)
	}
	for _, h := range history {
		if !bytes.Contains(h.Request, []byte(`"details"`)) {
			t.Fatalf("command missing audit details: %s", h.Request)
		}
	}
	for _, path := range []string{"snapshot", "quests", "mailbox", "materials"} {
		r := httptest.NewRequest("GET", fmt.Sprintf("/api/gm/%s?account=%d&character=%d", path, account, role.ID), nil)
		r.Header.Set("X-GM-Token", s.token)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("read %s: %s", path, w.Body)
		}
	}
}

func TestGMItemSearchPagesAndExactTypes(t *testing.T) {
	ix := &ItemIndex{items: []ItemEntry{{ID: 1, Kind: "equipment", Type: "[talisman]", Name: "a"}, {ID: 2, Kind: "equipment", Type: "[talisman]", Name: "b"}, {ID: 3, Kind: "stackable", Type: "[rune]", Name: "c"}}}
	rows := ix.Search(ItemFilter{Kind: "equipment", RawType: "[talisman]", Limit: 1, Offset: 1})
	if len(rows) != 1 || rows[0].ID != 2 {
		t.Fatalf("page %+v", rows)
	}
	rows = ix.Search(ItemFilter{RawType: "[rune]"})
	if len(rows) != 1 || rows[0].ID != 3 {
		t.Fatalf("exact type %+v", rows)
	}
}
