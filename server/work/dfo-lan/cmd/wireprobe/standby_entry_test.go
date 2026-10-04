package main

import (
	"bytes"
	"dfolan/internal/database"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

// 2026-10-03 实测教训（回归护栏）：伊斯频道（Type 81）待机区入场序中
// 不得插入官服会话特定的 848/637/16B-op2 头帧重放。官服抓包里这三帧是
// 对客户端四探测（c2s 2/848/433/637）的应答，字节内嵌官服会话的角色
// ID（0x598dceb2）与时间戳；跨会话重放引入幽灵迁移/RAID 状态，且与
// 场景装载期推送叠加导致客户端硬崩（client_trace 止于 ImageScheduler
// LV Changed，USERDMP 为空）。待机分支的行为差异只允许体现在落点
// （world_flow.go enter()）与推送门控（main.go N2254/N733）上。
func TestEntrySendsNoUnsolicitedMigrationOrRaidFrames(t *testing.T) {
	for _, p := range []entryPayloads{
		{Basic: []byte{1}},
	} {
		for _, f := range p.packets() {
			if f.ID == 848 || f.ID == 637 {
				t.Fatalf("entry sequence must not carry unsolicited op %d frame %s", f.ID, f.Name)
			}
			for _, name := range []string{"ispins_standby_userinfo_sent", "ispins_standby_charac_migration_sent", "ispins_standby_raid_inout_sent"} {
				if f.Name == name {
					t.Fatalf("entry sequence must not carry %s", name)
				}
			}
		}
	}
}

// weeklyDungeonInfoTable 的 0x2de 是「本周完成 ≥2 场」标记（next79 §18 二轮，
// 2026-10-03）：s1 表（10-02 战前，已完成 1 场仍能创建队伍）与 switch 表
//（10-03，已完成 2 场）逐字节 diff 仅此一处——s1=0x00、switch=0x01。早前
// 一轮曾误当「奖励可领槽位」改成 0x01，把「完成 2 场」状态回放给从未作战
// 的私服角色，实测待机区创建队伍仍弹「本周奖励已全部领取」。必须保持
// s1 战前原值 0x00；0x29f..0x2e2 其余 67 字节保持 01。
func TestWeeklyDungeonInfoRewardSlotsStayClaimable(t *testing.T) {
	table, err := hex.DecodeString(weeklyDungeonInfoTableHex)
	if err != nil {
		t.Fatal(err)
	}
	if len(table) != 1464 {
		t.Fatalf("weekly dungeon table len=%d, want 1464", len(table))
	}
	if table[0x2de] != 0 {
		t.Fatalf("completion marker at 02de = %02x, want 00 (s1 pre-battle value, NOT the switch 01)", table[0x2de])
	}
	for i := 0x29f; i <= 0x2e2; i++ {
		if i == 0x2de {
			continue
		}
		if table[i] != 1 {
			t.Fatalf("reward slot at %04x = %02x, want 01", i, table[i])
		}
	}
}

// raidInOutSystemReply：对 c2s 637 探测的应答必须与官服抓包帧 86/792
// 逐字节一致（8B：flag 01 + u32 0x5e2cd238 + u16 0x0040）。2026-10-03
// 实测：应答缺失或计数全零时，客户端在伊斯待机区点发起作战会本地拦截
// （连 CMD2043 都不发），机制同黑鸦计数缺省 0 拦截建队请求。
func TestRaidInOutSystemReplyMatchesOfficial(t *testing.T) {
	want := []byte{0x01, 0x00, 0x38, 0xd2, 0x2c, 0x5e, 0x40, 0x00}
	if len(raidInOutSystemReply) != 8 {
		t.Fatalf("reply len=%d, want 8", len(raidInOutSystemReply))
	}
	if !bytes.Equal(raidInOutSystemReply, want) {
		t.Fatalf("reply = %x, want %x", raidInOutSystemReply, want)
	}
	// 选角前 145250040 读的 pending deletion count 在 offset 1，必须保持 0。
	if raidInOutSystemReply[1] != 0 {
		t.Fatalf("pending deletion count byte = %02x, want 0", raidInOutSystemReply[1])
	}
}

// 尾部契约（worn_display_handoff_test 钉死）不受待机分支影响：
// actor_appearance_ready → 342 → 21 → 2310 → 2827。
func TestEntryTailOrderHolds(t *testing.T) {
	frames := (entryPayloads{Basic: []byte{1}}).packets()
	var tail []string
	for _, f := range frames {
		switch f.Name {
		case "actor_appearance_ready", "completed_quests_restored", "available_quests_restored", "synopsis_read_restored", "skill_locks_restored":
			tail = append(tail, f.Name)
		}
	}
	want := []string{"actor_appearance_ready", "completed_quests_restored", "available_quests_restored", "synopsis_read_restored", "skill_locks_restored"}
	for i := range want {
		if tail[i] != want[i] {
			t.Fatalf("entry tail order %v, want %v", tail, want)
		}
	}
}

// 待机区落点（world_flow.go enter() 的 Type 81 分支写入
// w.state.Position）必须经 NOTI23/NOTI24 下发给客户端：
// UserArea 布局 u16 actor + u32 town + u32 area + u16 x + u16 y + flags，
// AreaUsers 布局 u32 town + u32 area + u16 count + …。
func TestStandbyPlacementCarriesStandbyArea(t *testing.T) {
	w := &worldSession{
		role:  database.Character{WireID: 7},
		state: database.WorldState{Position: database.WorldPosition{Town: 146, Area: 0, X: 562, Y: 234}},
	}
	userArea, e := w.userAreaPayload()
	if e != nil {
		t.Fatal(e)
	}
	if len(userArea) < 14 {
		t.Fatalf("user area body len=%d, want >= 14", len(userArea))
	}
	if binary.LittleEndian.Uint32(userArea[2:6]) != 146 {
		t.Fatalf("user area town=%d, want 146", binary.LittleEndian.Uint32(userArea[2:6]))
	}
	if binary.LittleEndian.Uint32(userArea[6:10]) != 0 {
		t.Fatalf("user area area=%d, want 0", binary.LittleEndian.Uint32(userArea[6:10]))
	}
	if binary.LittleEndian.Uint16(userArea[10:12]) != 562 || binary.LittleEndian.Uint16(userArea[12:14]) != 234 {
		t.Fatalf("user area at (%d,%d), want (562,234)", binary.LittleEndian.Uint16(userArea[10:12]), binary.LittleEndian.Uint16(userArea[12:14]))
	}
	area, e := w.areaPayload()
	if e != nil {
		t.Fatal(e)
	}
	if binary.LittleEndian.Uint32(area[0:4]) != 146 || binary.LittleEndian.Uint32(area[4:8]) != 0 {
		t.Fatalf("area list town/area=%d/%d, want 146/0", binary.LittleEndian.Uint32(area[0:4]), binary.LittleEndian.Uint32(area[4:8]))
	}
}

// 周本「无限难度」链路（next79 作战次数调查，2026-10-03）：官服在登录连接
// 应答 c2s 782（帧 115，16B 会话头，与 op2 16B 头帧帧 97 逐字节相同），并在
// 城镇入场（帧 367/368）与伊斯待机区入场（帧 1032/1033）场景就绪后各推一次
// 781 USER(272B) + 782 CHARAC(400B)。私服此前整条链路缺失，客户端周计数
// 缺省 0，伊斯待机区点「创建队伍」弹「无法继续，因为本周奖励已全部领取」
// 并本地拦截（连 CMD2043 都不发）。
func TestWeeklyDifficultyInfoBodiesMatchOfficial(t *testing.T) {
	// 16B 登录应答：01 + LE u32 官服会话角色 ID 0x598dceb2 + 40 + 零
	//（私服各 NOTI 同类会话头全为零客户端也正常；verbatim 回放同 637 应答
	// raidInOutSystemReply 先例）。
	reply, err := hex.DecodeString(weeklyDungeonInfoReplyHex)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reply, weeklyDungeonInfoReply) || len(reply) != 16 {
		t.Fatalf("reply decoded mismatch: len=%d", len(weeklyDungeonInfoReply))
	}
	if reply[0] != 0x01 || binary.LittleEndian.Uint32(reply[1:5]) != 0x598dceb2 || reply[5] != 0x40 {
		t.Fatalf("reply header = %x", reply[:6])
	}
	for i, b := range reply[6:] {
		if b != 0 {
			t.Fatalf("reply tail byte %d = %02x, want 0", 6+i, b)
		}
	}

	cases := []struct {
		name    string
		bodyHex string
		body    []byte
		prefix  []byte // 头部常量（781: 04010000 / 782: 84010000）
		state   uint32 // 官服帧内 LE u32 状态字段
	}{
		{"town_user", weeklyDifficultyInfoUserTownHex, weeklyDifficultyInfoUserTown, []byte{0x04, 0x01, 0x00, 0x00}, 0x391e},
		{"town_charac", weeklyDifficultyInfoCharacTownHex, weeklyDifficultyInfoCharacTown, []byte{0x84, 0x01, 0x00, 0x00}, 0x391e},
		{"standby_user", weeklyDifficultyInfoUserStandbyHex, weeklyDifficultyInfoUserStandby, []byte{0x04, 0x01, 0x00, 0x00}, 0xf4},
		{"standby_charac", weeklyDifficultyInfoCharacStandbyHex, weeklyDifficultyInfoCharacStandby, []byte{0x84, 0x01, 0x00, 0x00}, 0xf4},
	}
	for _, c := range cases {
		want, err := hex.DecodeString(c.bodyHex)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(want, c.body) {
			t.Fatalf("%s decoded mismatch", c.name)
		}
		if len(c.body) < 8 || !bytes.Equal(c.body[:4], c.prefix) {
			t.Fatalf("%s prefix = %x, want %x", c.name, c.body[:4], c.prefix)
		}
		if got := binary.LittleEndian.Uint32(c.body[4:8]); got != c.state {
			t.Fatalf("%s state = %04x, want %04x", c.name, got, c.state)
		}
	}
	// 城镇版 272B / 400B，待机版同尺寸；两版状态字段不同（官服抓包 367/368
	// vs 1032/1033 逐字节差异），不得互相混用。
	if len(weeklyDifficultyInfoUserTown) != 272 || len(weeklyDifficultyInfoUserStandby) != 272 {
		t.Fatalf("781 body lens town=%d standby=%d, want 272/272", len(weeklyDifficultyInfoUserTown), len(weeklyDifficultyInfoUserStandby))
	}
	if len(weeklyDifficultyInfoCharacTown) != 400 || len(weeklyDifficultyInfoCharacStandby) != 400 {
		t.Fatalf("782 body lens town=%d standby=%d, want 400/400", len(weeklyDifficultyInfoCharacTown), len(weeklyDifficultyInfoCharacStandby))
	}
	if bytes.Equal(weeklyDifficultyInfoUserTown, weeklyDifficultyInfoUserStandby) {
		t.Fatal("town and standby 781 bodies must stay distinct (official frames 367 vs 1032)")
	}
	// 2026-10-03 定向修正（next79 §18）：待机版 781 offset 0xff 是「本周伊斯
	// 完成次数」进度字节（基线 0x68=0 次，+1/场）。官服 switch 抓包值为
	// 0x6a（被抓角色当周已完成 2 场并领奖，面板 0/1 被客户端本地拦截建队）；
	// s4 抓包战前待机为 0x69（已完成 1 场）——官服实证该状态能创建队伍进
	// 作战。取 0x69 与 706 表（s1 战前 verbatim）组成 yan55 10-02 战前的
	// 完整实证状态；0x6a 会让客户端拦截建队。
	if got := weeklyDifficultyInfoUserStandby[0xff]; got != 0x69 {
		t.Fatalf("standby 781 completion byte @0xff = %02x, want 69 (s4 pre-battle proven value)", got)
	}
}

// 781/782 推送必须留在场景就绪后的触发窗（城镇 1.1s goroutine / 待机首帧
// c2s），不得搬进 packets() 入场序：官服两处抓包中它们均在 c2s 35 之后
// 送达，装载期内推送会硬崩客户端（同 N2254 的 next79 §17 教训）。
func TestWeeklyDifficultyPushesStayOutOfEntrySequence(t *testing.T) {
	for _, f := range (entryPayloads{Basic: []byte{1}}).packets() {
		if f.ID == 781 || f.ID == 782 {
			t.Fatalf("entry sequence must not carry weekly difficulty op %d frame %s (post-scene-ready only)", f.ID, f.Name)
		}
	}
}
