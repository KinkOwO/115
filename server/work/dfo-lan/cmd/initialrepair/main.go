// initialrepair 给显式选定的开发角色补上两件 2026-09-20 之前缺的东西：
// 建号请求 option[8] 的转职落账，以及源 [create equipment list] 的初始穿戴。
//
// 默认只预览；-apply 才落盘。只补缺：已有转职、已有同槽穿戴一律保留，其它 state 字段不动。
// 与创建路径共用同一处判定与投影函数（character.applyCreationAdvancement / creationWorn），
// 所以修复后的状态与"今天新建一个同名分支角色"一致。
//
//	initialrepair -character 6                       # 预览
//	initialrepair -character 6 -apply                # 落盘
package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

// repairEventKey 是幂等键：同一个角色重复执行不会补第二次（character_events 主键）。
const repairEventKey = "creation-equipment-repair-v1"

func main() {
	storageConfig := flag.String("storage", "runtime/storage/local.json", "storage configuration")
	characterCatalog := flag.String("character-catalog", "configs/characters.skycastle-release.json", "profession catalog（必须带 growtype 分段数据）")
	equipmentCatalog := flag.String("quest-equipment-catalog", "configs/equipment.current37.json", "source equipment metadata")
	wearRules := flag.String("equipment-wear-rules", "configs/equipment-wear.current35.json", "part to worn-slot rules")
	characterRules := flag.String("character-rules", "configs/character-rules.odyssey-release.json", "creation rules（all_jobs_pilot / swordmaster_pilot 决定是否补转职落账）")
	id := flag.Int64("character", 0, "exact development character ID")
	apply := flag.Bool("apply", false, "apply the previewed repair; default only previews")
	flag.Parse()
	if *id <= 0 {
		log.Fatal("an exact character ID is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conf, e := storage.LoadConfig(*storageConfig)
	if e != nil {
		log.Fatal(e)
	}
	s, e := storage.Open(ctx, conf)
	if e != nil {
		log.Fatal(e)
	}
	defer s.Close()
	if e = s.MigrateCharacterEvents(ctx); e != nil {
		log.Fatal(e)
	}
	if e = s.MigrateCharacterNotices(ctx); e != nil {
		log.Fatal(e)
	}
	chars, e := catalog.LoadCharacters(*characterCatalog)
	if e != nil {
		log.Fatal(e)
	}
	equipment, e := inventory.LoadEquipmentCatalog(*equipmentCatalog, chars.Source.Checksum)
	if e != nil {
		log.Fatal(e)
	}
	rules, e := inventory.LoadWearRules(*wearRules, chars.Source.Checksum)
	if e != nil {
		log.Fatal(e)
	}
	rawRules, e := os.ReadFile(*characterRules)
	if e != nil {
		log.Fatal(e)
	}
	var creationRules character.Rules
	if e = json.Unmarshal(rawRules, &creationRules); e != nil {
		log.Fatal(e)
	}
	service := &character.Service{Catalog: chars, Rules: creationRules, Equipment: equipment, WearRules: rules}

	// 只允许显式选定的开发账号角色：拒绝在别的账号或正式存档上误用。
	var account int64
	if e = s.DB.QueryRow(ctx, `SELECT c.account_id FROM characters c JOIN accounts a ON a.id=c.account_id WHERE c.id=$1 AND a.development_only`, *id).Scan(&account); e != nil {
		log.Fatal("development character not found")
	}
	roles, e := s.Characters(ctx, account)
	if e != nil {
		log.Fatal(e)
	}
	var role storage.Character
	found := false
	for _, r := range roles {
		if r.ID == *id {
			role, found = r, true
			break
		}
	}
	if !found {
		log.Fatal("character is not in the account roster")
	}
	var before character.State
	if e = json.Unmarshal(role.State, &before); e != nil {
		log.Fatal(e)
	}

	// 预览只需变更说明；落盘时在事务里按"当前角色"重新计算（见下面的 apply 回调）。
	_, changes, e := service.CreationPreview(role)
	if e != nil {
		log.Fatal(e)
	}
	report := map[string]any{
		"applied":            false,
		"character":          *id,
		"name":               role.Name,
		"profession":         role.Profession,
		"advancement_before": before.Advancement,
		"all_jobs_pilot":     creationRules.AllJobsPilot,
		"swordmaster_pilot":  creationRules.SwordmasterPilot,
		"changes":            changes,
	}
	switch {
	case len(changes) == 0:
		if before.Advancement == 0 {
			report["note"] = fmt.Sprintf("没有需要补的内容：advancement 仍为 0，但当前规则 all_jobs_pilot=%v swordmaster_pilot=%v 未让建号请求落账",
				creationRules.AllJobsPilot, creationRules.SwordmasterPilot)
		} else {
			report["note"] = "没有需要补的内容"
		}
	case !*apply:
		report["note"] = "预览；确认后加 -apply 落盘"
	default:
		updated, applied, e := s.CommitCharacterEvent(ctx, account, *id, role.ConfigVersion, repairEventKey, repairEventKey, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			fresh, changes, e := service.CreationPreview(current)
			if e != nil {
				return nil, nil, e
			}
			if fresh == nil {
				return nil, nil, fmt.Errorf("预览之后状态已变化：没有需要补的内容")
			}
			receipt, e := json.Marshal(map[string]any{"changes": changes})
			if e != nil {
				return nil, nil, e
			}
			return fresh, receipt, nil
		})
		if e != nil {
			log.Fatal(e)
		}
		report["applied"] = applied
		report["state_bytes"] = len(updated.State)
		if !applied {
			report["note"] = "该修复事件已存在，未重复应用"
		}
	}
	b, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(b))
}
