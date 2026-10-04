package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/hex"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testFatigueItems(t *testing.T, fixture *database.TestFixture, role database.Character) {
	t.Helper()
	ctx := context.Background()
	s := fixture.Storage()
	shop := nativeShopPilot(t, false)
	c := catalog.LootCatalog{Source: shop.Config.Source, Items: map[uint32]catalog.LootItem{}}
	use, e := shop.StorageCatalog(c)
	if e != nil {
		t.Fatal(e)
	}
	// Bind the native shop and material overlay to the same archive, as
	// runtime preparation does. Clear-cube binding stays on this native source;
	// it has no getter, so do not guess its previous value.
	previousSource := catalog.OdysseySource
	catalog.SetOdysseySource(c.Source.Checksum)
	inventory.SetClearCubeSource(c.Source.Checksum)
	t.Cleanup(func() {
		catalog.SetOdysseySource(previousSource)
	})
	archive := catalog.OpenNativeArchive(t)
	index, e := catalog.ImportItemIndex(archive)
	if e != nil {
		t.Fatal(e)
	}
	cube, e := catalog.ImportClearCube(archive, index)
	if e != nil {
		t.Fatal(e)
	}
	use, e = inventory.WithClearCubeItem(use, cube)
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := c.Items[3037]; ok {
		t.Fatal("material polluted drop pool")
	}
	role.ConfigVersion = c.Source.SaveIdentity()
	role.State = []byte(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":66,"Template":10000541,"Amount":3},{"slot":121,"Template":3037,"Amount":3},{"slot":122,"Template":3037,"Amount":1000}]}}`)
	if e = fixture.SeedCharacterSnapshot(ctx, role.ID, role.State, role.ConfigVersion); e != nil {
		t.Fatal(e)
	}
	if e = s.MigrateCharacterEvents(ctx); e != nil {
		t.Fatal(e)
	}
	f := &character.FatigueService{Store: s, Rules: character.FatigueRules{DailyLimit: 1056, ResetHour: 9}, Location: time.UTC}
	now := time.Date(2030, 1, 1, 10, 0, 0, 0, time.UTC)
	if _, e = f.State(ctx, role.AccountID, role.ID, now); e != nil {
		t.Fatal(e)
	}
	if e = fixture.SeedFatigueUsage(ctx, role.ID, 40, 40); e != nil {
		t.Fatal(e)
	}
	var success atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, e := f.RecoverPotion(ctx, role, use, 66, now); e == nil {
				success.Add(1)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 {
		t.Fatal("concurrent recovery", success.Load())
	}
	fp, e := f.State(ctx, role.AccountID, role.ID, now)
	if e != nil || fp.Used != 10 || fp.UsedMax != 40 || fp.Limit != 1056 {
		t.Fatal(fp, e)
	}
	var state []byte
	if state, e = fixture.CharacterState(ctx, role.ID); e != nil {
		t.Fatal(e)
	}
	b, e := inventory.ReadBag(state)
	if e != nil || b.Items[0].Amount != 2 {
		t.Fatal(b, e)
	}
	if _, _, e = f.RecoverPotion(ctx, role, use, 66, now.Add(time.Hour)); e == nil {
		t.Fatal("daily replay admitted")
	}
	next := now.Add(24 * time.Hour)
	if _, e = f.State(ctx, role.AccountID, role.ID, next); e != nil {
		t.Fatal(e)
	}
	if _, _, e = f.RecoverPotion(ctx, role, use, 66, next); e == nil {
		t.Fatal("full fatigue consumed potion")
	}
	if e = fixture.SeedFatigueUsage(ctx, role.ID, 5, 5); e != nil {
		t.Fatal(e)
	}
	if _, _, e = f.RecoverPotion(ctx, role, use, 67, next); e == nil {
		t.Fatal("empty slot restored fatigue")
	}
	saved, fp, e := f.RecoverPotion(ctx, role, use, 66, next)
	if e != nil || fp.Used != 0 || fp.UsedMax != 5 {
		t.Fatal(fp, e)
	}
	if _, _, e = f.RecoverPotion(ctx, role, use, 66, now); e == nil {
		t.Fatal("clock rollback admitted")
	}
	w := &worldSession{role: saved, store: s, loot: &loot.Service{}, vault: &workflow.VaultService{VaultService: inventory.VaultService{Catalog: use}}, activeDungeon: &dungeon.Session{RunID: "01234567890123456789012345678901", Loaded: true}}
	p, _ := hex.DecodeString("0f00000010001a090802107918dd17200120000000000000000000000000000000")
	for i := 0; i < 3; i++ {
		raw := []byte(fmt.Sprintf("wire-unique-%d", i))
		plan, e := w.deleteSkillMaterial(p, raw)
		if e != nil || len(plan) != 2 || plan[0].ID != 18 {
			t.Fatal(plan, e)
		}
		if _, e = w.deleteSkillMaterial(p, raw); e != nil {
			t.Fatal("replay", e)
		}
	}
	b, e = inventory.ReadBag(w.role.State)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range b.Items {
		if v.Slot == 121 {
			t.Fatal("depleted stack retained")
		}
		if v.Slot == 122 && v.Amount != 1000 {
			t.Fatal("wrong stack decremented")
		}
	}
	if _, e = w.deleteSkillMaterial(p, []byte("fresh-empty")); e == nil {
		t.Fatal("empty material accepted")
	}
	p[11] = 122
	if _, e = w.deleteSkillMaterial(p, []byte("next-stack")); e != nil {
		t.Fatal("next material stack", e)
	}
	w.activeDungeon = nil
	if _, e = w.deleteSkillMaterial(p, []byte("town")); e == nil {
		t.Fatal("town skill material accepted")
	}
	t.Log("ITEMS PASS: potion concurrency/daily quota/full/empty/cap/rollback; CMD18 durable decrement/replay/stack exhaustion/next stack/town rejection; original drop pool unchanged")
}
