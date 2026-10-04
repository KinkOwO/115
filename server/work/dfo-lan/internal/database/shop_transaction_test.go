package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestShopCallbacksSerializeAccountLimitAndRollback(t *testing.T) {
	for _, material := range []bool{false, true} {
		t.Run(fmt.Sprintf("material=%v", material), func(t *testing.T) {
			s, ctx := sqlcTestStore(t)
			for _, migrate := range []func(context.Context) error{s.Migrate, s.MigrateCharacterEvents, s.MigrateShopPurchases, s.MigrateAccountMaterials} {
				if err := migrate(ctx); err != nil {
					t.Fatal(err)
				}
			}
			account, err := s.DevelopmentAccount(ctx, "shared-limit")
			if err != nil {
				t.Fatal(err)
			}
			version := strings.Repeat("a", 64)
			var roles []Character
			for _, name := range []string{"ShopA", "ShopB"} {
				role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: name, ConfigVersion: version,
					Request: []byte{0}, State: json.RawMessage(`{"unknown":"keep"}`)}, 24)
				if err != nil {
					t.Fatal(err)
				}
				roles = append(roles, role)
			}
			limitError, failError := errors.New("fixture limit reached"), errors.New("fixture apply failed")
			purchase := func(role Character, key string, template uint32, fail, replay bool) (bool, error) {
				apply := func(tx *Tx, current Character) (json.RawMessage, json.RawMessage, error) {
					if replay {
						t.Error("replay executed the callback")
					}
					used, err := tx.CountShopPurchases(ctx, ShopScopeAccount, 123, template, time.Time{})
					if err != nil {
						return nil, nil, err
					}
					if used >= 1 {
						return nil, nil, limitError
					}
					// Without the owning account lock, both character callbacks
					// can observe zero and exceed the shared limit.
					time.Sleep(30 * time.Millisecond)
					if err := tx.RecordShopPurchase(ctx, 123, template); err != nil {
						return nil, nil, err
					}
					// Reads must see the write on the transaction connection.
					if used, err := tx.CountShopPurchases(ctx, ShopScopeCharacter, 123, template, time.Time{}); err != nil || used != 1 {
						return nil, nil, fmt.Errorf("transaction cannot see its own purchase: %d %v", used, err)
					}
					if fail {
						return nil, nil, failError
					}
					return current.State, json.RawMessage(`{"bought":true}`), nil
				}
				if material {
					_, _, applied, err := s.CommitAccountMaterialEventTx(ctx, account, role.ID, version, key, "fixture-shop-v1",
						func(tx *Tx, current Character, counts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
							state, _, err := apply(tx, current)
							return state, counts, err
						})
					return applied, err
				}
				_, applied, err := s.CommitCharacterEventTx(ctx, account, role.ID, version, key, "fixture-shop-v1", apply)
				return applied, err
			}
			var wg sync.WaitGroup
			type result struct {
				role    Character
				key     string
				applied bool
				err     error
			}
			results := make(chan result, 2)
			for i, role := range roles {
				wg.Add(1)
				go func(role Character, key string) {
					defer wg.Done()
					applied, err := purchase(role, key, 456, false, false)
					results <- result{role, key, applied, err}
				}(role, fmt.Sprintf("purchase:%d", i))
			}
			wg.Wait()
			close(results)
			var winner result
			committed := 0
			for r := range results {
				if r.applied && r.err == nil {
					committed++
					winner = r
				} else if !errors.Is(r.err, limitError) {
					t.Errorf("unexpected concurrent result: %+v", r)
				}
			}
			if committed != 1 {
				t.Fatalf("shared account limit committed %d purchases", committed)
			}
			if applied, err := purchase(winner.role, winner.key, 456, false, true); err != nil || applied {
				t.Fatalf("replay: applied=%v err=%v", applied, err)
			}
			if used, err := s.CountShopPurchases(ctx, ShopScopeAccount, account, winner.role.ID, 123, 456, time.Time{}); err != nil || used != 1 {
				t.Fatalf("replay duplicated count: %d %v", used, err)
			}
			if applied, err := purchase(roles[0], "purchase:rollback", 789, true, false); !errors.Is(err, failError) || applied {
				t.Fatalf("failure: applied=%v err=%v", applied, err)
			}
			if used, err := s.CountShopPurchases(ctx, ShopScopeAccount, account, roles[0].ID, 123, 789, time.Time{}); err != nil || used != 0 {
				t.Fatalf("failed save left purchase ledger: %d %v", used, err)
			}
			if _, err := s.CharacterEventReceipt(ctx, account, roles[0].ID, "purchase:rollback"); !errors.Is(err, ErrNotFound) {
				t.Fatalf("failed save left receipt: %v", err)
			}
			stored, err := s.Characters(ctx, account)
			if err != nil || len(stored) != 2 {
				t.Fatalf("stored characters: %d %v", len(stored), err)
			}
			for _, role := range stored {
				if !sameJSON(t, role.State, json.RawMessage(`{"unknown":"keep"}`)) {
					t.Fatalf("transaction changed existing state: %s", role.State)
				}
			}
		})
	}
}
