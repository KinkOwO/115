// admin hands out cera, gold and source items to a local account/character.
//
// Every hand-out needs an explicit --grant-id: it is the idempotency key, so
// re-running the same command cannot pay out twice. Every hand-out also needs
// a --reason and is written to an audit table. Nothing here invents an item:
// a template must exist in the imported source and be acceptable to the same
// bag code the game uses.
//
//	admin -storage runtime/storage/local.json -account probe \
//	      -grant-id local-2026-09-11-a -reason "local test" \
//	      -character 3 -cera 5000 -gold 100000 -item 3037x10
//
//	admin -storage ... -account probe -history
package main

import (
	"context"
	"dfolan/internal/admin"
	"dfolan/internal/inventory"
	"dfolan/internal/managementdata"
	"dfolan/internal/storage"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
)

type itemList []storage.GrantItem

func (l *itemList) String() string { return fmt.Sprint(*l) }

func (l *itemList) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		template, amount := part, "1"
		if i := strings.IndexAny(part, "x:*"); i > 0 {
			template, amount = part[:i], part[i+1:]
		}
		t, e := strconv.ParseUint(strings.TrimSpace(template), 10, 32)
		if e != nil {
			return fmt.Errorf("item template %q: %w", template, e)
		}
		n, e := strconv.ParseUint(strings.TrimSpace(amount), 10, 32)
		if e != nil || n == 0 {
			return fmt.Errorf("item amount %q must be a positive number", amount)
		}
		*l = append(*l, storage.GrantItem{Template: uint32(t), Amount: uint32(n)})
	}
	return nil
}

func main() {
	sourceFlags := managementdata.Register(flag.CommandLine)
	storageConfig := flag.String("storage", "runtime/storage/local.json", "local storage configuration")
	accountName := flag.String("account", "probe", "development account name")
	grantID := flag.String("grant-id", "", "idempotency key; re-running the same id pays out once")
	reason := flag.String("reason", "", "why this hand-out happened (recorded for audit)")
	operator := flag.String("operator", "local-operator", "who performed it (recorded for audit)")
	character := flag.Int64("character", 0, "character id for gold/items; omit for cera only")
	cera := flag.Int64("cera", 0, "cera adjustment; negative deducts and refuses to go below zero")
	gold := flag.Uint64("gold", 0, "gold to add")
	flag.String("loot-catalog", "", "deprecated; native PVF content is always used")
	flag.String("item-index", "", "deprecated; native PVF content is always used")
	bagRules := flag.String("bag-rules", "configs/inventory.next29.json", "bag slot policy")
	flag.String("equipment-catalog", "", "deprecated; native PVF content is always used")
	flag.String("equipment-full-catalog", "", "deprecated; native PVF equipment is always used")
	history := flag.Bool("history", false, "print this account's recorded grants and exit")
	balance := flag.Bool("balance", false, "print this account's cera balance and exit")
	var items itemList
	flag.Var(&items, "item", "items as template[x amount], comma separated (e.g. 3037x10,20002)")
	flag.Parse()
	if sourceFlags.ArchivePath == "" {
		sourceFlags.ArchivePath = "../client-build/Script.inner.pvf"
	}

	if *gold > 0xffffffff {
		log.Fatal("gold exceeds the wire field")
	}
	// Catalog checks and native preparation finish before any storage access.
	var prepared *inventory.Awarder
	if sourceFlags.CheckOnly || (!*balance && !*history && (*gold != 0 || len(items) > 0)) {
		native, err := sourceFlags.Open()
		if err != nil {
			log.Fatal(err)
		}
		defer native.Close()
		prepared, err = managementdata.Awarder(native, sourceFlags.DropPolicy, *bagRules)
		if err != nil {
			log.Fatal(err)
		}
		if prepared.Equipment.Full != nil {
			defer prepared.Equipment.Full.Close()
		}
		if sourceFlags.CheckOnly {
			if err := managementdata.Report(map[string]any{"source": prepared.Catalog.Source.Checksum, "items": len(prepared.Catalog.Items), "equipment_rows": len(prepared.Equipment.Rows), "storage_accessed": false}); err != nil {
				log.Fatal(err)
			}
			return
		}
	} else if sourceFlags.Mode != "pvf" {
		log.Fatal("management catalogs require catalog-source=pvf")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, e := storage.LoadConfig(*storageConfig)
	if e != nil {
		log.Fatal(e)
	}
	s, e := storage.Open(ctx, cfg)
	if e != nil {
		log.Fatal(e)
	}
	defer s.Close()
	if e = s.MigrateGrants(ctx); e != nil {
		log.Fatal(e)
	}
	account, e := s.DevelopmentAccount(ctx, *accountName)
	if e != nil {
		log.Fatal(e)
	}

	if *balance {
		v, e := s.AccountCera(ctx, account)
		if e != nil {
			log.Fatal(e)
		}
		fmt.Printf("account %s cera=%d\n", *accountName, v)
		return
	}
	if *history {
		rows, e := s.GrantHistory(ctx, account, 50)
		if e != nil {
			log.Fatal(e)
		}
		if len(rows) == 0 {
			fmt.Println("no recorded grants for this account")
		}
		for _, r := range rows {
			fmt.Println(r)
		}
		return
	}
	if *grantID == "" || *reason == "" {
		log.Fatal("a hand-out requires -grant-id (idempotency key) and -reason (audit)")
	}
	if *cera == 0 && *gold == 0 && len(items) == 0 {
		log.Fatal("nothing to hand out")
	}

	service := &admin.Service{Store: s, Operator: *operator}
	if *gold != 0 || len(items) > 0 {
		if *character == 0 {
			log.Fatal("gold and items need -character")
		}
		service.Awarder = prepared
	}

	receipt, applied, e := service.Apply(ctx, storage.Grant{
		ID: *grantID, AccountID: account, Character: *character,
		Cera: *cera, Gold: uint32(*gold), Items: items,
		Reason: *reason, Operator: *operator,
	})
	if e != nil {
		log.Fatal(e)
	}
	out, _ := json.MarshalIndent(receipt, "", "  ")
	if !applied {
		fmt.Println("already applied earlier; this is the original receipt (nothing paid out again):")
	}
	fmt.Println(string(out))
	if *character != 0 {
		fmt.Println("\nA character that is logged in right now keeps its in-memory bag and")
		fmt.Println("balance until it re-enters, so re-select the character to see this.")
	}
}
