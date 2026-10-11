// Package honeytimecmd 调整某个角色在活动 313（HoneyTime 每日在线奖励）上的进度。
//
// 用途有两类：
//
//	1) **验证面板**：`-set-seconds K` 直接把"今天已累计在线秒数"设成 K —— 不用真挂机等
//	   10/30/60/120 分钟，就能看到面板的档位、可领按钮、以及"已领"状态怎么随 1294 变。
//	2) **复现领取**：`-clear-claims` 把"已领档位"清掉，配合 `-set-seconds` 可以反复验证
//	   领取链路（正常玩要等到第二天 06:00 UTC 才会重置）。
//	3) **迁移期号/整份还原**：`-set-claimed` / `-set-period-claimed` 把已领位图**设回去**。
//	   用途是`CycleStart` 换值时（`Normalize` 会把整份判成"换期"⇒ 清档）把进度一次性搬过去，
//	   免得裸改数据库绕过审计。
//
// 默认只预览；写库要显式 `-apply`，并给一个 `-grant-id` 作幂等键（与 setlevel / attendance
// 同一条审计路径：重复执行同一个 grant-id 不会二次生效）。
//
// 它**不读 PVF**：期号与换日口径都取自 `internal/honeytime` / `internal/attendance`
// （与网关同一个来源），工具只改角色 state JSON 里的那一段。
package honeytimecmd

import (
	"context"
	"dfolan/internal/attendance"
	"dfolan/internal/database"
	"dfolan/internal/honeytime"
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
	setSeconds := flag.Int("set-seconds", -1, "set today's accumulated online seconds (daily track)")
	setPeriodSeconds := flag.Int("set-period-seconds", -1, "set the accumulated online seconds of the PERIOD track (the \"cumulative hours\" row)")
	clearClaims := flag.Bool("clear-claims", false, "clear today's claimed tiers (so the daily GET buttons can be tested again)")
	clearPeriodClaims := flag.Bool("clear-period-claims", false, "clear the period track's claimed tiers")
	setClaimed := flag.Int("set-claimed", -1, "set today's claimed-tier mask (bit i = the script row whose first value is i); 0x7 works")
	setPeriodClaimed := flag.Int("set-period-claimed", -1, "set the period track's claimed-tier mask")
	dayShift := flag.Int("shift-days", 0, "pretend the stored day is N days older (makes the state look stale ⇒ it resets on next login)")
	show := flag.Bool("show", false, "only print the current state (no change)")
	apply := flag.Bool("apply", false, "write the change; without this the command only previews")
	grantID := flag.String("grant-id", "", "idempotency key (required with -apply)")
	reason := flag.String("reason", "gm honeytime adjust", "audit reason")
	operator := flag.String("operator", "local-operator", "audit operator")
	flag.Parse()

	if *name == "" && *id == 0 {
		log.Fatal("honeytime: give -name or -id")
	}
	if !*show && *setSeconds < 0 && *setPeriodSeconds < 0 && !*clearClaims && !*clearPeriodClaims &&
		*setClaimed < 0 && *setPeriodClaimed < 0 && *dayShift == 0 {
		log.Fatal("honeytime: give -show, -set-seconds K, -set-period-seconds K, -clear-claims, -clear-period-claims, -set-claimed MASK, -set-period-claimed MASK or -shift-days N so the command has something to do")
	}
	if *setClaimed > 0xFFFFFFFF || *setPeriodClaimed > 0xFFFFFFFF {
		log.Fatal("honeytime: claimed masks are u32")
	}
	if *setSeconds > honeytime.MaxDailySeconds {
		log.Fatalf("honeytime: -set-seconds exceeds the cap %d", honeytime.MaxDailySeconds)
	}
	if *setPeriodSeconds > honeytime.MaxPeriodSeconds {
		log.Fatalf("honeytime: -set-period-seconds exceeds the cap %d", honeytime.MaxPeriodSeconds)
	}
	if *apply && *grantID == "" {
		log.Fatal("honeytime: -apply needs -grant-id (the idempotency key recorded in the audit row)")
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
		fmt.Fprintf(os.Stderr, "honeytime: account %s has no such character (available:", *accountName)
		for _, r := range roles {
			fmt.Fprintf(os.Stderr, " %d:%s", r.ID, r.Name)
		}
		fmt.Fprintln(os.Stderr, ")")
		os.Exit(2)
	}

	// 期号必须与网关用的是同一个值：写错会让运行期的 Normalize 认为"换期了"、把进度整个重置
	// —— 那正好是这个工具要避免的事。（这里没有脚本可读，所以直接用网关那个常量。）
	cycleStart := honeytime.CycleStart
	today := honeytime.Today(time.Now().Unix())

	before, err := honeytime.ReadState(role.State)
	if err != nil {
		log.Fatal(err)
	}
	before = before.Normalize(cycleStart, today)
	after := before
	if *setSeconds >= 0 {
		after.Seconds = uint32(*setSeconds)
	}
	if *setPeriodSeconds >= 0 {
		after.PeriodSeconds = uint32(*setPeriodSeconds)
	}
	if *clearClaims {
		after.Claimed = 0
	}
	if *clearPeriodClaims {
		after.PeriodClaimed = 0
	}
	if *setClaimed >= 0 {
		after.Claimed = uint32(*setClaimed)
	}
	if *setPeriodClaimed >= 0 {
		after.PeriodClaimed = uint32(*setPeriodClaimed)
	}
	if *dayShift != 0 {
		after.Day -= int64(*dayShift)
	}

	fmt.Printf("角色 %d:%s  存储=%s  本期起点=%d\n", role.ID, role.Name, cfg.Driver, cycleStart)
	fmt.Printf("  今天的签到日号 = %d（换日时刻 06:00 UTC，与 331 同口径）\n", today)
	printState("改前", before)
	printState("改后", after)
	fmt.Printf("  日档：已累计 %d 秒；已领 %d 档（累计门槛 10/30/60/120 分钟）\n",
		after.Seconds, after.ClaimedCount())
	fmt.Printf("  期间轨：已累计 %d 秒（%.1f 小时）；已领 %d 档（累计门槛 5/20/50/80 小时，索引版脚本）\n",
		after.PeriodSeconds, float64(after.PeriodSeconds)/3600, after.PeriodClaimedCount())

	if *show || !*apply {
		fmt.Println("\n预览模式：未写库（-show 也不会写）。确认无误后加 -apply -grant-id <唯一键> 再执行。")
		return
	}
	result, err := store.ApplyGrant(ctx, database.Grant{
		ID:        *grantID,
		AccountID: account,
		Character: role.ID,
		Reason:    *reason,
		Operator:  *operator,
	}, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		// 按 JSON 对象逐键改写（与 setlevel 同一条纪律）：不把 state 整体往返成结构体，
		// 否则会丢掉背包等由别的读者写的键。
		state, err := honeytime.WriteState(current.State, after)
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
	written, err := honeytime.ReadState(result.Character.State)
	if err != nil {
		log.Fatal(err)
	}
	written = written.Normalize(cycleStart, today)
	fmt.Printf("\n已写入（applied=%v）：日档 seconds %d → %d、claimed 0x%x → 0x%x；"+
		"期间轨 seconds %d → %d、claimed 0x%x → 0x%x；day %d → %d\n",
		result.Applied, before.Seconds, written.Seconds, before.Claimed, written.Claimed,
		before.PeriodSeconds, written.PeriodSeconds, before.PeriodClaimed, written.PeriodClaimed,
		before.Day, written.Day)
	fmt.Println("注：角色在线时客户端仍显示旧值，重选角色（或重登）即可看到新的状态。")
}

func printState(label string, st honeytime.State) {
	fmt.Printf("  %s: day=%d 日档(seconds=%d claimed=0x%x/%d 档) 期间轨(seconds=%d claimed=0x%x/%d 档)\n",
		label, st.Day, st.Seconds, st.Claimed, st.ClaimedCount(),
		st.PeriodSeconds, st.PeriodClaimed, st.PeriodClaimedCount())
}

// 让 `internal/attendance` 也被引用：换日口径两条活动共用同一个函数，这里显式说明，
// 免得将来有人以为 honeytime 的 Today 是另一套实现。
var _ = attendance.DayNumber
