// Package setlevel 按**原生 PVF 的累计经验阈值**把某个角色设到指定等级。
//
// 为什么不能只改 state.level：character/growth_rules.go 用累计经验校验升级，
// 孤立的等级会在下一次结算时报 `level exceeds cumulative experience`。
// 本工具写 level = N 且 experience = Thresholds[N-2]（恰好是该等级的起点，
// 既不会立刻再升一级，也不会触发上面那条校验）。既有范式见
// character/odyssey.go 的 ApplyOdysseyTarget（它同样用 Thresholds[target-2]）。
//
// 默认只预览；写库要显式 -apply，并需要一个 -grant-id 作幂等键——重复执行同一个
// grant-id 不会二次生效（与 GM 发放点券/金币同一条审计路径）。
//
// 有意不动的字段：skill_points / technique_points。自然升级会一并发放技能点，
// 这里没有做，避免写出「等级很高但技能点是半吊子」的存档；需要技能点请用游戏内
// 正常升级或后续单独的工具。
package setlevel

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/managementdata"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"
)

func Run() {
	config := flag.String("config", "runtime/storage/local.json", "local storage configuration")
	accountName := flag.String("account", "probe", "development account name")
	name := flag.String("name", "", "character name (or use -id)")
	id := flag.Int64("id", 0, "character id (or use -name)")
	level := flag.Int("level", 0, "target level")
	apply := flag.Bool("apply", false, "write the change; without this the command only previews")
	grantID := flag.String("grant-id", "", "idempotency key (required with -apply)")
	reason := flag.String("reason", "gm set level", "audit reason")
	operator := flag.String("operator", "local-operator", "audit operator")
	sourceFlags := managementdata.Register(flag.CommandLine)
	flag.Parse()

	if *level < 2 {
		log.Fatal("setlevel: -level must be 2 or higher")
	}
	if *name == "" && *id == 0 {
		log.Fatal("setlevel: give -name or -id")
	}
	if *apply && *grantID == "" {
		log.Fatal("setlevel: -apply needs -grant-id (the idempotency key recorded in the audit row)")
	}

	// 只准备 progression 一个域：只读 PVF，且比整档准备快得多。
	native, err := sourceFlags.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer native.Close()
	progression, err := native.Progression("")
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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

	account, err := store.DevelopmentAccount(ctx, *accountName)
	if err != nil {
		log.Fatal(err)
	}
	roles, err := store.Characters(ctx, account)
	if err != nil {
		log.Fatal(err)
	}
	var role database.Character
	found := false
	for _, r := range roles {
		if (*id != 0 && r.ID == *id) || (*id == 0 && r.Name == *name) {
			role, found = r, true
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "setlevel: account %s has no such character (available:", *accountName)
		for _, r := range roles {
			fmt.Fprintf(os.Stderr, " %d:%s", r.ID, r.Name)
		}
		fmt.Fprintln(os.Stderr, ")")
		os.Exit(2)
	}

	before, err := readLevel(role.State)
	if err != nil {
		log.Fatal(err)
	}
	target := byte(*level)
	threshold, err := levelThreshold(progression, target)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("角色 %d:%s  存储=%s\n", role.ID, role.Name, cfg.Driver)
	fmt.Printf("  改前: 等级 %d  经验 %d\n", before.Level, before.Experience)
	fmt.Printf("  改后: 等级 %d  经验 %d   (Thresholds[%d]，即该等级起点)\n", target, threshold, int(target)-2)
	fmt.Printf("  PVF 累计经验阈值共 %d 条 ⇒ 可设等级 2..%d\n", len(progression.Thresholds), len(progression.Thresholds)+1)

	if !*apply {
		fmt.Println("\n预览模式：未写库。确认无误后加 -apply -grant-id <唯一键> 再执行。")
		return
	}

	result, err := store.ApplyGrant(ctx, database.Grant{
		ID:        *grantID,
		AccountID: account,
		Character: role.ID,
		Reason:    *reason,
		Operator:  *operator,
	}, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		return applyLevel(current, target, threshold)
	})
	if err != nil {
		log.Fatal(err)
	}
	after, err := readLevel(result.Character.State)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\n已写入（applied=%v）：等级 %d → %d，经验 %d → %d\n", result.Applied, before.Level, after.Level, before.Experience, after.Experience)
	fmt.Println("注：角色在线时客户端仍显示旧值，重选角色即可看到新等级。")
}

// applyLevel 只补丁 state 里的 level 与 experience 两个键。
//
// 关键：**不能**把 state 整体往返成 character.State —— 那份结构体里没有 inventory
// （背包/金币由 inventory.ReadBag/SaveBag 各自读改），整份往返会把背包清空。
// 副本实测抓到过这一点（改等级后金币 1000 → 0），所以这里按 JSON 对象逐键改写，
// 其余键（inventory、creation_options 等）原样保留。
func applyLevel(current database.Character, target byte, threshold uint64) (json.RawMessage, json.RawMessage, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(current.State, &doc); err != nil {
		return nil, nil, err
	}
	before, err := readLevel(current.State)
	if err != nil {
		return nil, nil, err
	}
	level, err := json.Marshal(target)
	if err != nil {
		return nil, nil, err
	}
	experience, err := json.Marshal(threshold)
	if err != nil {
		return nil, nil, err
	}
	doc["level"] = level
	doc["experience"] = experience
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, err
	}
	receipt, err := json.Marshal(map[string]any{
		"level":             target,
		"experience":        threshold,
		"level_before":      before.Level,
		"experience_before": before.Experience,
	})
	return raw, receipt, err
}

type levelView struct {
	Level      byte
	Experience uint64
}

func readLevel(raw json.RawMessage) (levelView, error) {
	var state character.State
	if err := json.Unmarshal(raw, &state); err != nil {
		return levelView{}, err
	}
	return levelView{Level: state.Level, Experience: state.Experience}, nil
}

// levelThreshold 返回「恰好处于 target 级」所需的累计经验：Thresholds[target-2]。
func levelThreshold(progression catalog.Progression, target byte) (uint64, error) {
	if int(target)-2 < 0 || int(target)-2 >= len(progression.Thresholds) {
		return 0, fmt.Errorf("setlevel: level %d is outside the native progression table (%d thresholds)", target, len(progression.Thresholds))
	}
	return progression.Thresholds[int(target)-2], nil
}
