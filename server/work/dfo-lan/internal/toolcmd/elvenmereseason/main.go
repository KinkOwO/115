package elvenmereseason

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/loot"
	"dfolan/internal/managementdata"
	"dfolan/internal/workflow"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

func Run() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("elvenmereseason", flag.ContinueOnError)
	fs.SetOutput(out)
	config := fs.String("config", "runtime/storage/local.json", "storage configuration")
	id := fs.Int64("id", 0, "owned character used to identify the account")
	operation := fs.String("operation", "", "32 hex digits; stable account-wide reset identity")
	reason := fs.String("reason", "", "verified season-transition audit reason")
	operator := fs.String("operator", "local-operator", "audit operator")
	apply := fs.Bool("apply", false, "apply an announced season reset; default is preview")
	sourceFlags := managementdata.Register(fs)
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if !sourceFlags.CheckOnly && (*id <= 0 || *operator == "") {
		return fmt.Errorf("elvenmereseason needs -id and an audit operator")
	}
	if *apply {
		decoded, err := hex.DecodeString(*operation)
		if err != nil || len(decoded) != 16 || *reason == "" {
			return fmt.Errorf("apply needs a stable 32-hex -operation and -reason")
		}
	}
	native, err := sourceFlags.Open()
	if err != nil {
		return err
	}
	defer native.Close()
	if sourceFlags.CheckOnly {
		return json.NewEncoder(out).Encode(map[string]any{"source": native.Snapshot().Checksum, "storage_accessed": false})
	}
	// No parallel reward content is loaded. The verified archive snapshot is
	// only the storage-source identity; this command never grants any item.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cfg, err := database.LoadConfig(*config)
	if err != nil {
		return err
	}
	store, err := database.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer store.Close()
	role, err := store.AdminCharacter(ctx, *id)
	if err != nil {
		return err
	}
	svc := &workflow.LootService{Store: store, Loot: &loot.Service{Catalog: catalog.LootCatalog{Source: native.Snapshot()}}}
	before, err := svc.RestoreElvenmereProgress(ctx, role)
	if err != nil {
		return err
	}
	p, _, err := dungeon.ReadElvenmereProgress(before.State)
	if err != nil {
		return err
	}
	result := map[string]any{"account_id": role.AccountID, "character_id": role.ID, "applied": false, "current_season": p.SeasonID, "week_start": p.WeekStart, "weekly_progress": p.MaxCleared, "operator": *operator}
	if *apply {
		saved, err := resetGuarded(ctx, svc, role, *operation, *reason+" [operator="+*operator+"]", serverRunning)
		if err != nil {
			return err
		}
		p, _, err = dungeon.ReadElvenmereProgress(saved.State)
		if err != nil {
			return err
		}
		result["applied"] = true
		result["season"] = p.SeasonID
	}
	return json.NewEncoder(out).Encode(result)
}

func serverRunning() bool {
	c, err := net.DialTimeout("tcp", "127.0.0.1:7001", 250*time.Millisecond)
	if err == nil {
		c.Close()
		return true
	}
	return false
}

func resetGuarded(ctx context.Context, svc *workflow.LootService, role database.Character, operation, reason string, running func() bool) (database.Character, error) {
	if running() {
		return role, fmt.Errorf("game server is running on 7001; season reset refused")
	}
	release, err := svc.Store.HoldAdminGuard(ctx)
	if err != nil {
		return role, err
	}
	defer release()
	if running() {
		return role, fmt.Errorf("game server started during season reset preparation")
	}
	return svc.ResetElvenmereSeason(ctx, role, operation, reason)
}
