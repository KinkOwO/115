// Read-only pilot snapshot and wire fixture for avatar initialization testing.
package avatarrestorecheck

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

func Run() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, e := storage.LoadConfig("runtime/swordmaster-pilot-20260916/storage.json")
	must(e)
	u, e := url.Parse(cfg.PostgresDSN)
	must(e)
	if u.Hostname() != "127.0.0.1" || u.Path != "/dfo_swordmaster_pilot_20260916" {
		log.Fatal("pilot database required")
	}
	s, e := storage.Open(ctx, cfg)
	must(e)
	defer s.Close()
	var role storage.Character
	e = s.DB.QueryRow(ctx, `SELECT c.id,c.state FROM characters c JOIN accounts a ON a.id=c.account_id WHERE a.username='probe' AND c.name='normal_test' AND c.deleted_at IS NULL`).Scan(&role.ID, &role.State)
	must(e)
	role.WireID = 503
	var projection struct {
		Inventory struct {
			Worn []protocol.DetailedWorn `json:"worn"`
		} `json:"inventory"`
	}
	must(json.Unmarshal(role.State, &projection))
	rows := []protocol.DetailedWorn{}
	for _, v := range projection.Inventory.Worn {
		if v.Slot <= 11 {
			rows = append(rows, v)
		}
	}
	if len(rows) == 0 {
		log.Fatal("pilot has no persisted worn avatars")
	}
	const source = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	native, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: "../client-build/Script.inner.pvf"})
	must(e)
	defer native.Close()
	index, e := native.ItemIndex("")
	must(e)
	catalog, e := native.Equipment(index)
	must(e)
	defer catalog.Close()
	rules, e := inventory.LoadWearRules("configs/equipment-wear.full-candidate.json", source)
	must(e)
	for _, row := range rows {
		definition, e := catalog.Definition(row.Template)
		must(e)
		kind := definition.Fields["[equipment type]"]
		if len(kind) == 0 {
			log.Fatal("missing source equipment type")
		}
		slot, ok := rules.Slots[kind[0].Text]
		if !ok || slot != row.Slot || slot > 11 {
			log.Fatal("source avatar type/slot mismatch")
		}
	}
	service := character.Service{}
	baseline, e := service.EntryAddition(role)
	must(e)
	service.DetailedWornCandidate = true
	modified, e := service.EntryAddition(role)
	must(e)
	block, e := protocol.DetailedEquipment(rows)
	must(e)
	dir := "docs/evidence/avatar-restore-20260917"
	must(os.MkdirAll(dir, 0755))
	for name, data := range map[string][]byte{"baseline-addition.bin": baseline, "modified-addition.bin": modified, "avatar-block.bin": block} {
		must(os.WriteFile(filepath.Join(dir, name), data, 0644))
	}
	hash := sha256.Sum256(role.State)
	out, e := json.MarshalIndent(map[string]any{"character_id": role.ID, "state_sha256": hex.EncodeToString(hash[:]), "avatars": rows, "baseline_bytes": len(baseline), "modified_bytes": len(modified), "database_changed": false}, "", "  ")
	must(e)
	must(os.WriteFile(filepath.Join(dir, "pilot-wire-fixture.json"), out, 0644))
	fmt.Printf("FIXTURE PASS character=%d avatars=%d baseline_bytes=%d modified_bytes=%d database_changed=false\n", role.ID, len(rows), len(baseline), len(modified))
}
func must(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
