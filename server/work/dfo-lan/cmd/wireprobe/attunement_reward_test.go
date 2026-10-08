package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
)

// noti 2859 的三个字段在源里**还没有语义结论**（见 attunement_reward.go 文件头），
// 所以这一组测试钉的不是「值该是多少」，而是**注入通道本身的形状**：
// 长度必须正好 12 字节、三种写法等价、默认一个字节都不发、开口后严格贴在 2838 后面。

func TestParseAttunementRewardAcceptsHexAndDecimals(t *testing.T) {
	raw := make([]byte, 12)
	binary.LittleEndian.PutUint32(raw[0:4], 44)
	binary.LittleEndian.PutUint32(raw[4:8], 43)
	binary.LittleEndian.PutUint32(raw[8:12], 0)

	for name, spec := range map[string]string{
		"紧凑十六进制":  hex.EncodeToString(raw),
		"带空格十六进制": "2c000000 2b000000 00000000",
		"带逗号十六进制": "2c000000,2b000000,00000000",
		"带 0x 前缀": "0x2c0000002b00000000000000",
		"三个十进制":   "44,43,0",
		"十进制带空格":  "44, 43, 0",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := parseAttunementReward(spec)
			if err != nil {
				t.Fatalf("parse(%q): %v", spec, err)
			}
			if !bytes.Equal(got, raw) {
				t.Fatalf("parse(%q) = %x, want %x", spec, got, raw)
			}
		})
	}
}

// 空值 = 不发。默认一个字节都不发是有意的：三个字段的语义没定，
// 写错档位会让客户端的珠子演出与掉落实质性不符，比不发更糟。
// 模块 init 里 `+88` 的初值是 -1（全 F）—— 试值时要能照着写。
func TestParseAttunementRewardAcceptsTheNegativeSentinel(t *testing.T) {
	got, err := parseAttunementReward("72,72,-1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []byte{72, 0, 0, 0, 72, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF}
	if !bytes.Equal(got, want) {
		t.Fatalf("= %x, want %x（-1 按二补数写成全 F）", got, want)
	}
	// 超出 u32 的值必须拒掉，而不是静默截断。
	for _, spec := range []string{"4294967296,0,0", "-2147483649,0,0"} {
		if _, err := parseAttunementReward(spec); err == nil {
			t.Fatalf("parse(%q) 应当报错", spec)
		}
	}
}

func TestParseAttunementRewardEmptyMeansNoInjection(t *testing.T) {
	for _, spec := range []string{"", "   ", "\t"} {
		got, err := parseAttunementReward(spec)
		if err != nil {
			t.Fatalf("parse(%q) 不该报错：%v", spec, err)
		}
		if got != nil {
			t.Fatalf("parse(%q) = %x, want nil（留空 = 不发）", spec, got)
		}
	}
}

// 长度必须是 12：解析器是 `sub_146EA0BE0(&v1, 12)` 一次定长读满
// （2838 是同一个形状的 8 字节，那条长度护栏 2026-09-27 用 2839 撞过）。
// 写成 8 / 16 / 只有两个字段一律拒绝，不给客户端留错位的半包。
func TestParseAttunementRewardRejectsWrongLength(t *testing.T) {
	for name, spec := range map[string]string{
		"两个字段":      "44,43",
		"四个字段":      "44,43,0,1",
		"8 字节十六进制":  "2c0000002b000000",
		"16 字节十六进制": "2c0000002b0000000000000000000000",
		"非数字":       "a,b,c",
		"溢出 u32":    "4294967296,0,0",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseAttunementReward(spec); err == nil {
				t.Fatalf("parse(%q) 应当报错", spec)
			}
		})
	}
}

// 日志里要把三个数按位移念出来，并标注它是不是客户端认识的八档之一 ——
// 实机试值时这张「值 → 名字」的对照就是唯一的现场线索。
func TestDescribeAttunementRewardNamesTheTiers(t *testing.T) {
	raw := make([]byte, 12)
	binary.LittleEndian.PutUint32(raw[0:4], 44)
	binary.LittleEndian.PutUint32(raw[4:8], 43)
	binary.LittleEndian.PutUint32(raw[8:12], 0)
	text := describeAttunementReward(raw)
	for _, want := range []string{"[0]=44(epic)", "[1]=43(legendary)", "[2]=0(not-a-client-tier)", hex.EncodeToString(raw)} {
		if !bytes.Contains([]byte(text), []byte(want)) {
			t.Fatalf("日志 %q 缺少 %q", text, want)
		}
	}
}

// 注入必须严格贴在 2838 后面，而且**只在显式给值时**出现。
// 2838 本身是常驻状态（非调律副本也要下发 normal/normal），不能被这条诊断顶掉。
func TestOathInfoPacketsAppendsTheAttunementRewardOnlyWhenInjected(t *testing.T) {
	w := &worldSession{}
	plan, err := w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].ID != 2838 {
		t.Fatalf("默认计划 = %+v，应当只有 2838", plan)
	}

	want := make([]byte, 12)
	binary.LittleEndian.PutUint32(want[0:4], 45)
	binary.LittleEndian.PutUint32(want[4:8], 45)
	binary.LittleEndian.PutUint32(want[8:12], 45)
	w.attunementReward = "45,45,45"
	plan, err = w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 {
		t.Fatalf("注入后计划 = %d 项，want 2（2838 + 2859）", len(plan))
	}
	if plan[0].ID != 2838 || plan[1].ID != attunementRewardPacketID {
		t.Fatalf("顺序 = %d,%d，want 2838 在前、2859 在后", plan[0].ID, plan[1].ID)
	}
	if !bytes.Equal(plan[1].Payload, want) {
		t.Fatalf("2859 载荷 = %x, want %x（必须逐字节原样，不做任何加工）", plan[1].Payload, want)
	}
	if plan[1].Name != "attunement_reward" {
		t.Fatalf("name = %q，日志里要能一眼认出这是注入的那一帧", plan[1].Name)
	}
}

// 试值要靠 `@文件` 才能不重启换值：**每次进本重新解析**，改完存盘下一次进本就生效。
// 这条测试直接钉住「两次解析之间文件被改了，第二次必须拿到新值」。
func TestAttunementRewardSpecReadsTheFileOnEveryEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.txt")
	if err := os.WriteFile(path, []byte("44,43,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	spec := "@" + path

	w := &worldSession{attunementReward: spec}
	plan, err := w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || !bytes.Equal(plan[1].Payload, []byte{44, 0, 0, 0, 43, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatalf("第一次进本载荷 = %+v", plan)
	}

	// 存盘换值 —— 不重启服务端，下一次进本必须拿到新值。
	if err := os.WriteFile(path, []byte("# 试第二个值\n45,45,45\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err = w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 2 || !bytes.Equal(plan[1].Payload, []byte{45, 0, 0, 0, 45, 0, 0, 0, 45, 0, 0, 0}) {
		t.Fatalf("改文件后载荷 = %+v（没有重读？）", plan)
	}

	// 空文件 = 不发：这样「关掉注入」也不用重启。
	if err := os.WriteFile(path, []byte("\n# 只留注释\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err = w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].ID != 2838 {
		t.Fatalf("空文件时应当只剩 2838：%+v", plan)
	}
}

// 试值期间把文件改坏，代价不该是「这局进不去」：进本时解析失败只记一笔并跳过。
func TestAttunementRewardSkipsABrokenTrialFileInsteadOfFailingTheEntry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.txt")
	if err := os.WriteFile(path, []byte("这不是载荷\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := &worldSession{attunementReward: "@" + path}
	plan, err := w.oathInfoPackets()
	if err != nil {
		t.Fatalf("进本不该因为试值文件写坏而失败：%v", err)
	}
	if len(plan) != 1 || plan[0].ID != 2838 {
		t.Fatalf("坏文件时应当只发 2838：%+v", plan)
	}
}

// 但**启动期**必须挡住写错的路径/内容（1 秒内失败，而不是跑完 48 秒 PVF 准备）——
// 这条同时钉住「缺文件也会早期失败」，否则试值时会以为注入生效了。
func TestAttunementRewardSpecRejectsAMissingTrialFile(t *testing.T) {
	cfg := Config{AttunementReward: "@" + filepath.Join(t.TempDir(), "nope.txt")}
	err := cfg.validate()
	if err == nil {
		t.Fatal("指向不存在的文件必须在 validate 就被拒")
	}
	if !strings.Contains(err.Error(), "attunement-reward") {
		t.Fatalf("错误要写明是哪个开关：%v", err)
	}
}

// 正常接线（有活动副本、档位已预掷、**没有**任何诊断开关）时，一个字节都不发。
//
// 2026-10-08 起这条自动发送**已移交 noti 2756**（见 border_reward_flow.go 的
// borderRewardPackets）：2859 与 2756 注册的是同一个 handler、写同一组三个位移，
// 两个发送者并存 = 谁后到谁生效，而两者的取档口径还不相同 —— 那正是「动画和掉落
// 不匹配」会从缝里漏回来的地方。所以这里只保留诊断通道。
func TestAttunementRewardIsDiagnosticOnly(t *testing.T) {
	// 大深渊：即便档位已经预掷好，也不自动发。
	big := &worldSession{activeDungeon: boundaryDungeon(), attunementRunTiers: loot.RunTiers{Primer: 43, Oath: 43}}
	if plan := big.attunementRewardPackets(); len(plan) != 0 {
		t.Fatalf("自动发送已移交 2756，这里不该再发：%+v", plan)
	}
	// 小深渊（另一套玩法）同样不发 —— 现在它的保障来自 2756 自己的副本门禁
	// （internal/loot/border_plan.go 的 IsBorderDungeon）。
	small := &worldSession{activeDungeon: smallAbyssDungeon(), attunementRunTiers: loot.RunTiers{Primer: 43, Oath: 43}}
	if plan := small.attunementRewardPackets(); len(plan) != 0 {
		t.Fatalf("小深渊不该收到 2859：%+v", plan)
	}
	// 不在副本上下文里更不该发。
	none := &worldSession{attunementRunTiers: loot.RunTiers{Primer: 43, Oath: 43}}
	if plan := none.attunementRewardPackets(); len(plan) != 0 {
		t.Fatalf("没有活动副本时不该发：%+v", plan)
	}
	// 诊断覆盖**不受门禁限制**（试值要在任何副本里都能做），且载荷逐字节原样。
	probe := &worldSession{activeDungeon: smallAbyssDungeon(), attunementReward: "42,42,42"}
	plan := probe.attunementRewardPackets()
	if len(plan) != 1 || plan[0].ID != attunementRewardPacketID {
		t.Fatalf("诊断覆盖应当照发：%+v", plan)
	}
	if got := binary.LittleEndian.Uint32(plan[0].Payload[0:4]); got != 42 {
		t.Fatalf("诊断载荷被改动了：%d", got)
	}
}

// 大深渊那一帧的换算链：本场档位 → 客户端四格演出表 → 2756 的 12 字节载荷。
//
// 这一条钉住「动画与掉落同档」（业主 2026-10-08 实机验收：传说砝码 → LegendaryDrop(41)
// → 誓约物品 [rarity] 3），以及「低于独有就不发」（四格演出表从独有起）。
func TestBorderRewardFrameFollowsTheFrozenGrade(t *testing.T) {
	for _, c := range []struct {
		name  string
		grade uint32
		slot  uint32
		none  bool
	}{
		{"传说", 43, 41, false},        // LegendaryDrop
		{"史诗", 44, 42, false},        // EpicDrop
		{"太初", 45, 43, false},        // PrimevalDrop
		{"独有（四格的最低那格）", 42, 40, false}, // UniqueDrop
		{"普通（演出表没有这一格）", 40, 0, true},
		{"稀有（演出表没有这一格）", 41, 0, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			slot, ok := loot.AnimationGrade(c.grade)
			if c.none {
				if ok {
					t.Fatalf("档位 %d 不该有演出格，得到 %d", c.grade, slot)
				}
				return
			}
			if !ok || slot != c.slot {
				t.Fatalf("AnimationGrade(%d) = (%d,%v)，want %d", c.grade, slot, ok, c.slot)
			}
			p, err := protocol.BorderRewardInfo(slot)
			if err != nil {
				t.Fatal(err)
			}
			// 三格同值：实测只确认了驱动格是第 1 个（`40,43,43` 播的是最低那格），
			// 而任何一格越界都会让整帧退化成最后一格 —— 旧版把第 1 格留在模块初值
			// -1，于是每次都被兜底成太初。
			for i := 0; i < 3; i++ {
				if got := binary.LittleEndian.Uint32(p[i*4:]); got != slot {
					t.Fatalf("位移 %d = %d(%s)，want 与第 1 格同值 %d",
						i, got, loot.AnimationSlot(got), slot)
				}
			}
		})
	}
}

// boundaryDungeon 造一个「调律之边界」的假副本（只需要 [dungeon type]）。
// 门禁按源声明分流，所以测试也必须给出源声明，而不是省掉它。
func boundaryDungeon() *dungeon.Session {
	return &dungeon.Session{Definition: catalog.DungeonDefinition{
		Script: catalog.ScriptRecord{Cells: []pvf.Token{
			{Type: 3, Text: "[dungeon type]"},
			{Type: 8, Reference: "boundary of attunement"},
		}},
	}}
}

// smallAbyssDungeon 是**另一套玩法**（终末之边界）。
func smallAbyssDungeon() *dungeon.Session {
	return &dungeon.Session{Definition: catalog.DungeonDefinition{
		Script: catalog.ScriptRecord{Cells: []pvf.Token{
			{Type: 3, Text: "[dungeon type]"},
			{Type: 8, Reference: "endkeeper of order"},
		}},
	}}
}
