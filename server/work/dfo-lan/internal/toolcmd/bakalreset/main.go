// Package bakalreset is an operator-requested weekly quota reset. It changes
// only two counters through the existing audited, transactional grant path.
package bakalreset

import (
	"context"
	"crypto/rand"
	"dfolan/internal/database"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"
)

func Run() {
	config := flag.String("config", "runtime/storage/local.json", "active storage configuration")
	account := flag.String("account", "probe", "existing account name (wireprobe uses probe)")
	apply := flag.Bool("apply", false, "restore this account's Bakal entry and reward counters")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cfg, err := database.LoadConfig(*config)
	if err != nil {
		log.Fatal(err)
	}
	store, err := database.Open(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()
	if err := ResetAccount(ctx, store, *account, *apply); err != nil {
		log.Fatal(err)
	}
}

// RunFromConfig opens the named storage config and applies the Bakal weekly quota
// reset. It is the reusable entry point the standalone Run (above) and the
// dfolauncher bakal-reset subcommand both go through, so the CLI and the launcher
// share exactly one implementation.
func RunFromConfig(ctx context.Context, configPath, username string, apply bool) error {
	cfg, err := database.LoadConfig(configPath)
	if err != nil {
		return err
	}
	store, err := database.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	return ResetAccount(ctx, store, username, apply)
}

// ResetAccount restores the Bakal entry/reward counters for every character of the
// named account through the audited grant path. With apply=false it only previews
// which characters would change.
func ResetAccount(ctx context.Context, store *database.Store, username string, apply bool) error {
	accounts, err := store.Accounts(ctx)
	if err != nil {
		return err
	}
	var account int64
	for _, row := range accounts {
		if row.Username == username {
			account = row.ID
			break
		}
	}
	if account == 0 {
		return fmt.Errorf("account %q does not exist; no account was created", username)
	}
	roles, err := store.Characters(ctx, account)
	if err != nil {
		return err
	}
	// Validate every target before the first mutation. Never replace malformed
	// ledgers with an empty state or silently drop unknown player data.
	var targets []database.Character
	for _, role := range roles {
		_, _, changed, err := resetState(role)
		if err != nil {
			return fmt.Errorf("character %d:%s: %w", role.ID, role.Name, err)
		}
		if changed {
			targets = append(targets, role)
		}
	}
	fmt.Printf("Account: %s (id=%d), characters needing reset: %d\n", username, account, len(targets))
	if !apply {
		fmt.Println("Preview only. Use -apply to restore the counters.")
		return nil
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	batch := "bakal-reset-" + hex.EncodeToString(token[:])
	for _, role := range targets {
		grant := database.Grant{ID: fmt.Sprintf("%s:%d", batch, role.ID), AccountID: account, Character: role.ID,
			Reason: "operator requested Bakal weekly quota recovery", Operator: "local-bakal-reset"}
		result, err := store.ApplyGrant(ctx, grant, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			next, receipt, _, err := resetState(current)
			return next, receipt, err
		})
		if err != nil {
			return fmt.Errorf("reset character %d:%s failed: %w", role.ID, role.Name, err)
		}
		fmt.Printf("Restored %d:%s (applied=%v, audit=%s)\n", role.ID, role.Name, result.Applied, grant.ID)
	}
	fmt.Println("Finished. Start the game again to reload the restored weekly quota.")
	return nil
}

func resetState(role database.Character) (json.RawMessage, json.RawMessage, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(role.State, &top); err != nil || top == nil {
		return nil, nil, false, fmt.Errorf("invalid character JSON object")
	}
	raw, exists := top["bakal_raid_rewards"]
	if !exists {
		return role.State, json.RawMessage(`{"changed":false}`), false, nil
	}
	var ledger map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ledger); err != nil || ledger == nil {
		return nil, nil, false, fmt.Errorf("invalid Bakal weekly ledger")
	}
	var clears, rewards uint32
	for name, counter := range map[string]*uint32{"clears": &clears, "rewards": &rewards} {
		if value, ok := ledger[name]; ok {
			if err := json.Unmarshal(value, counter); err != nil || string(value) == "null" {
				return nil, nil, false, fmt.Errorf("invalid Bakal counter %s", name)
			}
		}
	}
	if clears == 0 && rewards == 0 {
		return role.State, json.RawMessage(`{"changed":false}`), false, nil
	}
	ledger["clears"], ledger["rewards"] = json.RawMessage("0"), json.RawMessage("0")
	after, err := json.Marshal(ledger)
	if err != nil {
		return nil, nil, false, err
	}
	top["bakal_raid_rewards"] = after
	next, err := json.Marshal(top)
	if err != nil {
		return nil, nil, false, err
	}
	// Preserve the complete previous ledger in the existing admin audit row;
	// old run receipts, pending products and unknown fields are not erased.
	receipt, err := json.Marshal(struct {
		Before json.RawMessage `json:"before"`
		After  json.RawMessage `json:"after"`
	}{raw, after})
	return next, receipt, true, err
}
