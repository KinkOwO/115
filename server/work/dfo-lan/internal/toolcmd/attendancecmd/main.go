// Package attendancecmd 调整某个角色的每日签到（活动 331）进度。
//
// 用途：实机验证「第二天可领」与「角色隔离」——把一个角色的**上次领取日**往前推，
// 让它的下一天立刻可领，而其它角色的进度一个字节都不动。这正是角色级存档要证明的事：
// 两个角色的签到天数可以不一致。
//
// 默认只预览；写库要显式 `-apply`，并给一个 `-grant-id` 作幂等键（与 setlevel 同一条
// 审计路径：重复执行同一个 grant-id 不会二次生效）。
//
// 它**不读 PVF**：签到的运营窗口与换日口径都在 `internal/attendance` 里（与网关同一个
// 来源），工具只改角色 state JSON 里的那一段。
package attendancecmd

import (
	"context"
	"dfolan/internal/attendance"
	"dfolan/internal/database"
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
	shift := flag.Int("shift-days", 0, "pretend the last claim happened N days earlier (makes the next day claimable)")
	reset := flag.Bool("reset", false, "clear this character's attendance progress")
	apply := flag.Bool("apply", false, "write the change; without this the command only previews")
	grantID := flag.String("grant-id", "", "idempotency key (required with -apply)")
	reason := flag.String("reason", "gm attendance adjust", "audit reason")
	operator := flag.String("operator", "local-operator", "audit operator")
	flag.Parse()

	if *name == "" && *id == 0 {
		log.Fatal("attendance: give -name or -id")
	}
	if !*reset && *shift == 0 {
		log.Fatal("attendance: give -shift-days N (or -reset) so the command has something to do")
	}
	if *reset && *shift != 0 {
		log.Fatal("attendance: -reset and -shift-days are mutually exclusive")
	}
	if *apply && *grantID == "" {
		log.Fatal("attendance: -apply needs -grant-id (the idempotency key recorded in the audit row)")
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
		fmt.Fprintf(os.Stderr, "attendance: account %s has no such character (available:", *accountName)
		for _, r := range roles {
			fmt.Fprintf(os.Stderr, " %d:%s", r.ID, r.Name)
		}
		fmt.Fprintln(os.Stderr, ")")
		os.Exit(2)
	}

	// 期号必须与网关用的是同一个值（attendance.EventStart）：写错会让运行期的
	// Normalize 认为"换期了"，把进度整个重置 —— 那正好是这个工具要避免的事。
	cycleStart := int64(attendance.EventStart)
	before, err := attendance.ReadState(role.State)
	if err != nil {
		log.Fatal(err)
	}
	before = before.Normalize(cycleStart)
	after := before
	if *reset {
		after = attendance.State{Version: 1, EventID: attendance.EventID,
			CycleStart: cycleStart, ClaimedDays: 0, LastClaimDay: -1}
	} else {
		after.LastClaimDay -= int64(*shift)
		if after.LastClaimDay < -1 {
			after.LastClaimDay = -1
		}
	}
	today := attendance.DayNumber(time.Now().Unix())

	fmt.Printf("角色 %d:%s  存储=%s  本期起点=%d\n", role.ID, role.Name, cfg.Driver, cycleStart)
	fmt.Printf("  今天的签到日号 = %d（换日时刻 06:00 UTC）\n", today)
	printState("改前", before, today)
	printState("改后", after, today)
	fmt.Printf("  ⇒ 客户端窗口应显示：attended=%d，day_state[0..%d]=2、第 %d 格=%d\n",
		attendedOf(after, today), after.ClaimedDays-1, after.ClaimedDays, availableMark(after, today))

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
		// 按 JSON 对象逐键改写（与 setlevel 同一条纪律）：不把 state 整体往返成
		// 结构体，否则会丢掉背包等由别的读者写的键。
		state, err := attendance.WriteState(current.State, after)
		if err != nil {
			return nil, nil, err
		}
		receipt, err := json.Marshal(after)
		if err != nil {
			return nil, nil, err
		}
		return state, receipt, nil
	})
	if err != nil {
		log.Fatal(err)
	}
	written, err := attendance.ReadState(result.Character.State)
	if err != nil {
		log.Fatal(err)
	}
	written = written.Normalize(cycleStart)
	fmt.Printf("\n已写入（applied=%v）：claimed %d → %d，last_claim_day %d → %d\n",
		result.Applied, before.ClaimedDays, written.ClaimedDays, before.LastClaimDay, written.LastClaimDay)
	fmt.Println("注：角色在线时客户端仍显示旧值，重选角色（或重登）即可看到新的签到状态。")
}

func printState(label string, st attendance.State, today int64) {
	fmt.Printf("  %s: claimed=%d last_claim_day=%d（%s）\n", label, st.ClaimedDays, st.LastClaimDay,
		describeLastClaim(st, today))
}

func describeLastClaim(st attendance.State, today int64) string {
	if st.ClaimedDays == 0 || st.LastClaimDay < 0 {
		return "本期还没领过"
	}
	if st.LastClaimDay >= today {
		return "今天已领"
	}
	return fmt.Sprintf("上次领取在 %d 天前", today-st.LastClaimDay)
}

func availableMark(st attendance.State, today int64) int {
	if !st.AvailableToday(today) {
		return 0
	}
	return 1
}

func attendedOf(st attendance.State, today int64) uint32 {
	if !st.AvailableToday(today) {
		return uint32(st.ClaimedDays)
	}
	return uint32(st.ClaimedDays) + 1
}
